package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/dgraph-io/ristretto"
	"github.com/google/uuid"
	sqlite "github.com/mattn/go-sqlite3"
	"github.com/schollz/sqlite3dump"
	"golang.org/x/sync/singleflight"
)

type database struct {
	// Basic things
	db  *sql.DB            // database
	em  sync.Mutex         // command execution (insert, update, delete ...)
	sg  singleflight.Group // singleflight group for prepared statements
	psc *ristretto.Cache   // prepared statement cache
	// Other things
	pc    singleflight.Group // persistant cache
	pcm   sync.Mutex         // post creation
	sp    singleflight.Group // singleflight group for short path requests
	spc   *ristretto.Cache   // shortpath cache
	debug bool
}

func (a *goBlog) initDatabase(logging bool) (err error) {
	if a.db != nil && a.db.db != nil {
		return
	}
	if logging {
		log.Println("Initialize database...")
	}
	// Setup db
	db, err := a.openDatabase(a.cfg.Db.File, logging)
	if err != nil {
		return err
	}
	// Create appDB
	a.db = db
	a.shutdown.Add(func() {
		if err := db.close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		} else {
			log.Println("Closed database")
		}
	})
	if a.cfg.Db.DumpFile != "" {
		a.hourlyHooks = append(a.hourlyHooks, func() {
			db.dump(a.cfg.Db.DumpFile)
		})
		db.dump(a.cfg.Db.DumpFile)
	}
	if logging {
		log.Println("Initialized database")
	}
	return nil
}

func (a *goBlog) openDatabase(file string, logging bool) (*database, error) {
	// Register driver
	dbDriverName := "goblog_db_" + uuid.NewString()
	sql.Register(dbDriverName, &sqlite.SQLiteDriver{
		ConnectHook: func(c *sqlite.SQLiteConn) error {
			// Register functions
			for n, f := range map[string]any{
				"mdtext":         a.renderTextSafe,
				"tolocal":        toLocalSafe,
				"toutc":          toUTCSafe,
				"wordcount":      wordCount,
				"charcount":      charCount,
				"urlize":         urlize,
				"lowerx":         strings.ToLower,
				"lowerunescaped": lowerUnescapedPath,
			} {
				if err := c.RegisterFunc(n, f, true); err != nil {
					return err
				}
			}
			return nil
		},
	})
	// Open db
	db, err := sql.Open(dbDriverName, file+"?mode=rwc&_journal=WAL&_timeout=100&cache=shared&_fk=1")
	if err != nil {
		return nil, err
	}
	numConns := 5
	db.SetMaxOpenConns(numConns)
	db.SetMaxIdleConns(numConns)
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	// Check available SQLite features
	rows, err := db.Query("pragma compile_options")
	if err != nil {
		return nil, err
	}
	cos := map[string]struct{}{}
	var co string
	for rows.Next() {
		err = rows.Scan(&co)
		if err != nil {
			return nil, err
		}
		cos[co] = struct{}{}
	}
	if _, ok := cos["ENABLE_FTS5"]; !ok {
		return nil, errors.New("sqlite not compiled with FTS5")
	}
	// Migrate DB
	err = migrateDb(db, logging)
	if err != nil {
		return nil, err
	}
	// Debug
	debug := false
	if c := a.cfg.Db; c != nil && c.Debug {
		debug = true
	}
	psc, err := ristretto.NewCache(&ristretto.Config{
		NumCounters:        1000,
		MaxCost:            100,
		BufferItems:        64,
		IgnoreInternalCost: true,
	})
	if err != nil {
		return nil, err
	}
	spc, err := ristretto.NewCache(&ristretto.Config{
		NumCounters:        5000,
		MaxCost:            500,
		BufferItems:        64,
		IgnoreInternalCost: true,
	})
	if err != nil {
		return nil, err
	}
	return &database{
		db:    db,
		debug: debug,
		psc:   psc,
		spc:   spc,
	}, nil
}

// Main features

func (db *database) dump(file string) {
	if db == nil || db.db == nil {
		return
	}
	// Lock execution
	db.em.Lock()
	defer db.em.Unlock()
	// Dump database
	f, err := os.Create(file)
	if err != nil {
		log.Println("Error while dump db:", err.Error())
		return
	}
	if err = sqlite3dump.DumpDB(db.db, f, sqlite3dump.WithTransaction(true)); err != nil {
		log.Println("Error while dump db:", err.Error())
	}
}

func (db *database) close() error {
	if db == nil || db.db == nil {
		return nil
	}
	return db.db.Close()
}

func (db *database) prepare(query string, args ...any) (*sql.Stmt, []any, error) {
	if db == nil || db.db == nil {
		return nil, nil, errors.New("database not initialized")
	}
	if len(args) > 0 && args[0] == dbNoCache {
		return nil, args[1:], nil
	}
	stmt, err, _ := db.sg.Do(query, func() (any, error) {
		// Look if statement already exists
		st, ok := db.psc.Get(query)
		if ok {
			return st, nil
		}
		// ... otherwise prepare ...
		st, err := db.db.Prepare(query)
		if err != nil {
			return nil, err
		}
		// ... and store it
		db.psc.Set(query, st, 1)
		return st, nil
	})
	if err != nil {
		if db.debug {
			log.Printf(`Failed to prepare query "%s": %s`, query, err.Error())
		}
		return nil, args, err
	}
	return stmt.(*sql.Stmt), args, nil
}

const dbNoCache = "nocache"

func (db *database) Exec(query string, args ...any) (sql.Result, error) {
	return db.ExecContext(context.Background(), query, args...)
}

func (db *database) ExecContext(c context.Context, query string, args ...any) (sql.Result, error) {
	if db == nil || db.db == nil {
		return nil, errors.New("database not initialized")
	}
	// Maybe prepare
	st, args, _ := db.prepare(query, args...)
	// Lock execution
	db.em.Lock()
	defer db.em.Unlock()
	// Prepare context, call hook
	ctx := db.dbBefore(c, query, args...)
	defer db.dbAfter(ctx, query, args...)
	// Execute
	if st != nil {
		return st.ExecContext(ctx, args...)
	}
	return db.db.ExecContext(ctx, query, args...)
}

func (db *database) Query(query string, args ...any) (*sql.Rows, error) {
	return db.QueryContext(context.Background(), query, args...)
}

func (db *database) QueryContext(c context.Context, query string, args ...any) (rows *sql.Rows, err error) {
	if db == nil || db.db == nil {
		return nil, errors.New("database not initialized")
	}
	// Maybe prepare
	st, args, _ := db.prepare(query, args...)
	// Prepare context, call hook
	ctx := db.dbBefore(c, query, args...)
	// Query
	if st != nil {
		rows, err = st.QueryContext(ctx, args...)
	} else {
		rows, err = db.db.QueryContext(ctx, query, args...)
	}
	// Call hook
	db.dbAfter(ctx, query, args...)
	return
}

func (db *database) QueryRow(query string, args ...any) (*sql.Row, error) {
	return db.QueryRowContext(context.Background(), query, args...)
}

func (db *database) QueryRowContext(c context.Context, query string, args ...any) (row *sql.Row, err error) {
	if db == nil || db.db == nil {
		return nil, errors.New("database not initialized")
	}
	// Maybe prepare
	st, args, _ := db.prepare(query, args...)
	// Prepare context, call hook
	ctx := db.dbBefore(c, query, args...)
	// Query
	if st != nil {
		row = st.QueryRowContext(ctx, args...)
	} else {
		row = db.db.QueryRowContext(ctx, query, args...)
	}
	// Call hook
	db.dbAfter(ctx, query, args...)
	return
}

// Other things

func (d *database) rebuildFTSIndex() {
	_, _ = d.Exec("insert into posts_fts(posts_fts) values ('rebuild')")
}

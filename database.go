package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	sqlite "github.com/mattn/go-sqlite3"
	"github.com/schollz/sqlite3dump"
	"golang.org/x/sync/singleflight"
)

type database struct {
	// Basic things
	db *sql.DB            // database
	em sync.Mutex         // command execution (insert, update, delete ...)
	sg singleflight.Group // singleflight group for prepared statements
	ps sync.Map           // map with prepared statements
	// Other things
	pc    singleflight.Group // persistant cache
	pcm   sync.Mutex         // post creation
	sp    singleflight.Group // singleflight group for short path requests
	spc   sync.Map           // shortpath cache
	debug bool
}

func (a *goBlog) initDatabase(logging bool) (err error) {
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
			for n, f := range map[string]interface{}{
				"mdtext":    a.renderText,
				"tolocal":   toLocalSafe,
				"toutc":     toUTCSafe,
				"wordcount": wordCount,
				"charcount": charCount,
				"urlize":    urlize,
				"lowerx":    strings.ToLower,
			} {
				if err := c.RegisterFunc(n, f, true); err != nil {
					return err
				}
			}
			return nil
		},
	})
	// Open db
	db, err := sql.Open(dbDriverName, file+"?mode=rwc&_journal_mode=WAL&_busy_timeout=100&cache=shared")
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
	cos := map[string]bool{}
	var co string
	for rows.Next() {
		err = rows.Scan(&co)
		if err != nil {
			return nil, err
		}
		cos[co] = true
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
	return &database{
		db:    db,
		debug: debug,
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

func (db *database) prepare(query string, args ...interface{}) (*sql.Stmt, []interface{}, error) {
	if db == nil || db.db == nil {
		return nil, nil, errors.New("database not initialized")
	}
	if len(args) > 0 && args[0] == dbNoCache {
		return nil, args[1:], nil
	}
	stmt, err, _ := db.sg.Do(query, func() (interface{}, error) {
		// Look if statement already exists
		st, ok := db.ps.Load(query)
		if ok {
			return st, nil
		}
		// ... otherwise prepare ...
		st, err := db.db.Prepare(query)
		if err != nil {
			return nil, err
		}
		// ... and store it
		db.ps.Store(query, st)
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

func (db *database) exec(query string, args ...interface{}) (sql.Result, error) {
	if db == nil || db.db == nil {
		return nil, errors.New("database not initialized")
	}
	// Maybe prepare
	st, args, _ := db.prepare(query, args...)
	// Lock execution
	db.em.Lock()
	defer db.em.Unlock()
	// Prepare context, call hook
	ctx := db.dbBefore(context.Background(), query, args...)
	defer db.dbAfter(ctx, query, args...)
	// Execute
	if st != nil {
		return st.ExecContext(ctx, args...)
	}
	return db.db.ExecContext(ctx, query, args...)
}

func (db *database) query(query string, args ...interface{}) (*sql.Rows, error) {
	if db == nil || db.db == nil {
		return nil, errors.New("database not initialized")
	}
	// Maybe prepare
	st, args, _ := db.prepare(query, args...)
	// Prepare context, call hook
	ctx := db.dbBefore(context.Background(), query, args...)
	defer db.dbAfter(ctx, query, args...)
	// Query
	if st != nil {
		return st.QueryContext(ctx, args...)
	}
	return db.db.QueryContext(ctx, query, args...)
}

func (db *database) queryRow(query string, args ...interface{}) (*sql.Row, error) {
	if db == nil || db.db == nil {
		return nil, errors.New("database not initialized")
	}
	// Maybe prepare
	st, args, _ := db.prepare(query, args...)
	// Prepare context, call hook
	ctx := db.dbBefore(context.Background(), query, args...)
	defer db.dbAfter(ctx, query, args...)
	// Query
	if st != nil {
		return st.QueryRowContext(ctx, args...), nil
	}
	return db.db.QueryRowContext(ctx, query, args...), nil
}

// Other things

func (d *database) rebuildFTSIndex() {
	_, _ = d.exec("insert into posts_fts(posts_fts) values ('rebuild')")
}

package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/google/uuid"
	sqlite "github.com/mattn/go-sqlite3"
	"github.com/schollz/sqlite3dump"
	"golang.org/x/sync/singleflight"
)

type database struct {
	a *goBlog
	// Basic things
	db  *sql.DB            // database
	psc *ristretto.Cache   // prepared statement cache
	sg  singleflight.Group // singleflight group for prepared statements
	// Other things
	pc    singleflight.Group // persistant cache
	pcm   sync.Mutex         // post creation
	sp    singleflight.Group // singleflight group for short path requests
	spc   *ristretto.Cache   // shortpath cache
	debug bool
}

func (a *goBlog) initDatabase(logging bool) error {
	if a.db != nil && a.db.db != nil {
		return nil
	}
	if logging {
		a.info("Initialize database")
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
			a.error("Failed to close database", "err", err)
		} else {
			a.info("Closed database")
		}
	})

	if a.cfg.Db.DumpFile != "" {
		a.hourlyHooks = append(a.hourlyHooks, func() {
			db.dump(a.cfg.Db.DumpFile)
		})
		db.dump(a.cfg.Db.DumpFile)
	}

	if logging {
		a.info("Initialized database")
	}
	return nil
}

func (a *goBlog) openDatabase(file string, logging bool) (*database, error) {
	// Register driver
	dbDriverName := "goblog_db_" + uuid.NewString()
	sql.Register(dbDriverName, &sqlite.SQLiteDriver{
		ConnectHook: func(c *sqlite.SQLiteConn) error {
			funcs := map[string]any{
				"mdtext":         a.renderTextSafe,
				"tolocal":        toLocalSafe,
				"toutc":          toUTCSafe,
				"wordcount":      wordCount,
				"charcount":      charCount,
				"urlize":         urlize,
				"lowerx":         strings.ToLower,
				"lowerunescaped": lowerUnescapedPath,
			}
			for n, f := range funcs {
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

	const numConns = 10
	db.SetMaxOpenConns(numConns)
	db.SetMaxIdleConns(numConns)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Check available SQLite features
	if err := checkSQLiteFeatures(db); err != nil {
		return nil, err
	}

	// Migrate DB
	if err := a.migrateDb(db, logging); err != nil {
		return nil, err
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
		a:     a,
		db:    db,
		debug: a.cfg.Db != nil && a.cfg.Db.Debug,
		psc:   psc,
		spc:   spc,
	}, nil
}

func checkSQLiteFeatures(db *sql.DB) error {
	rows, err := db.Query("pragma compile_options")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var option string
		if err := rows.Scan(&option); err != nil {
			return err
		}
		if option == "ENABLE_FTS5" {
			return nil
		}
	}
	return errors.New("sqlite not compiled with FTS5")
}

func (db *database) prepare(query string, args ...any) (*sql.Stmt, []any, error) {
	if db == nil || db.db == nil {
		return nil, nil, errors.New("database not initialized")
	}
	if len(args) > 0 && args[0] == dbNoCache {
		return nil, args[1:], nil
	}
	stmt, err, _ := db.sg.Do(query, func() (any, error) {
		// Check cache
		if st, ok := db.psc.Get(query); ok {
			return st, nil
		}
		// ... otherwise prepare ...
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		st, err := db.db.PrepareContext(ctx, query)
		if err != nil {
			return nil, err
		}
		db.psc.SetWithTTL(query, st, 1, 1*time.Minute)
		db.psc.Wait()
		return st, nil
	})
	if err != nil {
		if db.debug {
			db.a.error("Failed to prepare query", "query", query, "err", err)
		}
		return nil, args, err
	}
	return stmt.(*sql.Stmt), args, nil
}

const dbNoCache = "nocache"

func (db *database) Exec(query string, args ...any) (sql.Result, error) {
	return db.ExecContext(context.Background(), query, args...)
}

func (db *database) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if db == nil || db.db == nil {
		return nil, errors.New("database not initialized")
	}
	st, args, _ := db.prepare(query, args...)
	ctx = db.dbBefore(ctx, query, args...)
	defer db.dbAfter(ctx, query, args...)

	if st != nil {
		return st.ExecContext(ctx, args...)
	}
	return db.db.ExecContext(ctx, query, args...)
}

func (db *database) Query(query string, args ...any) (*sql.Rows, error) {
	return db.QueryContext(context.Background(), query, args...)
}

func (db *database) QueryContext(ctx context.Context, query string, args ...any) (rows *sql.Rows, err error) {
	if db == nil || db.db == nil {
		return nil, errors.New("database not initialized")
	}
	st, args, _ := db.prepare(query, args...)
	ctx = db.dbBefore(ctx, query, args...)
	defer db.dbAfter(ctx, query, args...)

	if st != nil {
		rows, err = st.QueryContext(ctx, args...)
	} else {
		rows, err = db.db.QueryContext(ctx, query, args...)
	}
	return
}

func (db *database) QueryRow(query string, args ...any) (*sql.Row, error) {
	return db.QueryRowContext(context.Background(), query, args...)
}

func (db *database) QueryRowContext(ctx context.Context, query string, args ...any) (row *sql.Row, err error) {
	if db == nil || db.db == nil {
		return nil, errors.New("database not initialized")
	}
	st, args, _ := db.prepare(query, args...)
	ctx = db.dbBefore(ctx, query, args...)
	defer db.dbAfter(ctx, query, args...)

	if st != nil {
		row = st.QueryRowContext(ctx, args...)
	} else {
		row = db.db.QueryRowContext(ctx, query, args...)
	}
	return
}

func (db *database) dump(file string) {
	if db == nil || db.db == nil {
		return
	}
	f, err := os.Create(file)
	if err != nil {
		db.a.error("Error while dump db", "err", err)
		return
	}
	defer f.Close()
	if err = sqlite3dump.DumpDB(db.db, f, sqlite3dump.WithTransaction(true)); err != nil {
		db.a.error("Error while dump db", "err", err)
	}
}

func (db *database) close() error {
	if db == nil || db.db == nil {
		return nil
	}
	return db.db.Close()
}

func (db *database) rebuildFTSIndex() {
	_, _ = db.Exec("insert into posts_fts(posts_fts) values ('rebuild')")
}

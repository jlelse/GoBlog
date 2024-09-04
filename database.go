package main

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	sqlite "github.com/mattn/go-sqlite3"
	"github.com/samber/go-singleflightx"
	"github.com/samber/lo"
	"github.com/schollz/sqlite3dump"
)

type database struct {
	a                       *goBlog
	writeDb, readDb, dumpDb *sql.DB
	// Other things
	pc    singleflightx.Group[string, []byte] // persistant cache
	pcm   sync.RWMutex                        // post creation
	sp    singleflightx.Group[string, string] // short path creation
	debug bool
}

func (a *goBlog) initDatabase(logging bool) error {
	if a.db != nil && a.db.writeDb != nil && a.db.readDb != nil {
		return nil
	}
	if logging {
		a.info("Initialize database")
	}
	// Setup db
	dumpEnabled := a.cfg.Db.DumpFile != ""
	db, err := a.openDatabase(a.cfg.Db.File, logging, dumpEnabled)
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

	if dumpEnabled {
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

func (a *goBlog) openDatabase(file string, logging, dump bool) (*database, error) {
	file = lo.If(strings.Contains(file, "?"), file+"&").Else(file + "?")
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
	// Open write connection
	writeParams := make(url.Values)
	writeParams.Add("mode", "rwc")
	writeParams.Add("_txlock", "immediate")
	writeParams.Add("_journal_mode", "WAL")
	writeParams.Add("_busy_timeout", "1000")
	writeParams.Add("_synchronous", "NORMAL")
	writeParams.Add("_foreign_keys", "true")
	writeDb, err := sql.Open(dbDriverName, file+writeParams.Encode())
	if err != nil {
		return nil, err
	}
	writeDb.SetMaxOpenConns(1)
	writeDb.SetMaxIdleConns(1)
	if err := writeDb.Ping(); err != nil {
		return nil, err
	}
	// Check available SQLite features
	if err := checkSQLiteFeatures(writeDb); err != nil {
		return nil, err
	}
	// Migrate DB
	if err := a.migrateDb(writeDb, logging); err != nil {
		return nil, err
	}
	// Open read connections
	readParams := make(url.Values)
	readParams.Add("mode", "ro") // read-only
	readParams.Add("_txlock", "deferred")
	readParams.Add("_journal_mode", "WAL")
	readParams.Add("_busy_timeout", "1000")
	readParams.Add("_synchronous", "NORMAL")
	readParams.Add("_foreign_keys", "true")
	readDb, err := sql.Open(dbDriverName, file+readParams.Encode())
	if err != nil {
		return nil, err
	}
	if err := readDb.Ping(); err != nil {
		return nil, err
	}
	// Dump db
	var dumpDb *sql.DB
	if dump {
		dumpDb, err = sql.Open("sqlite3", file+readParams.Encode())
		if err != nil {
			return nil, err
		}
		dumpDb.SetMaxIdleConns(0)
		if err := dumpDb.Ping(); err != nil {
			return nil, err
		}
	}
	// Create custom database struct
	return &database{
		a:       a,
		writeDb: writeDb,
		readDb:  readDb,
		dumpDb:  dumpDb,
		debug:   a.cfg.Db != nil && a.cfg.Db.Debug,
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

func (db *database) Exec(query string, args ...any) (sql.Result, error) {
	return db.ExecContext(context.Background(), query, args...)
}

func (db *database) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if db == nil || db.writeDb == nil {
		return nil, errors.New("database not initialized")
	}
	ctx = db.dbBefore(ctx, query, args...)
	defer db.dbAfter(ctx, query, args...)
	return db.writeDb.ExecContext(ctx, query, args...)
}

func (db *database) Query(query string, args ...any) (*sql.Rows, error) {
	return db.QueryContext(context.Background(), query, args...)
}

func (db *database) QueryContext(ctx context.Context, query string, args ...any) (rows *sql.Rows, err error) {
	if db == nil || db.readDb == nil {
		return nil, errors.New("database not initialized")
	}
	ctx = db.dbBefore(ctx, query, args...)
	defer db.dbAfter(ctx, query, args...)
	return db.readDb.QueryContext(ctx, query, args...)
}

func (db *database) QueryRow(query string, args ...any) (*sql.Row, error) {
	return db.QueryRowContext(context.Background(), query, args...)
}

func (db *database) QueryRowContext(ctx context.Context, query string, args ...any) (row *sql.Row, err error) {
	if db == nil || db.readDb == nil {
		return nil, errors.New("database not initialized")
	}
	ctx = db.dbBefore(ctx, query, args...)
	defer db.dbAfter(ctx, query, args...)
	return db.readDb.QueryRowContext(ctx, query, args...), nil
}

func (db *database) dump(file string) {
	if db == nil || db.readDb == nil {
		return
	}
	f, err := os.Create(file)
	if err != nil {
		db.a.error("Error while dump db", "err", err)
		return
	}
	defer f.Close()
	if err = sqlite3dump.DumpDB(db.dumpDb, f, sqlite3dump.WithTransaction(false)); err != nil {
		db.a.error("Error while dump db", "err", err)
	}
}

func (db *database) close() error {
	if db == nil {
		return nil
	}
	var errs []error
	for _, db := range []*sql.DB{db.writeDb, db.readDb, db.dumpDb} {
		if db != nil {
			errs = append(errs, db.Close())
		}
	}
	return errors.Join(errs...)
}

func (db *database) rebuildFTSIndex() {
	_, _ = db.Exec("insert into posts_fts(posts_fts) values ('rebuild')")
}

package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync"

	"github.com/google/uuid"
	sqlite "github.com/mattn/go-sqlite3"
	"github.com/samber/go-singleflightx"
	"github.com/schollz/sqlite3dump"
)

type database struct {
	a  *goBlog
	db *sql.DB
	// Other things
	pc    singleflightx.Group[string, []byte] // persistant cache
	pcm   sync.RWMutex                        // post creation
	sp    sync.Mutex                          // short path creation
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

	return &database{
		a:     a,
		db:    db,
		debug: a.cfg.Db != nil && a.cfg.Db.Debug,
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
	if db == nil || db.db == nil {
		return nil, errors.New("database not initialized")
	}
	ctx = db.dbBefore(ctx, query, args...)
	defer db.dbAfter(ctx, query, args...)

	return db.db.ExecContext(ctx, query, args...)
}

func (db *database) Query(query string, args ...any) (*sql.Rows, error) {
	return db.QueryContext(context.Background(), query, args...)
}

func (db *database) QueryContext(ctx context.Context, query string, args ...any) (rows *sql.Rows, err error) {
	if db == nil || db.db == nil {
		return nil, errors.New("database not initialized")
	}
	ctx = db.dbBefore(ctx, query, args...)
	defer db.dbAfter(ctx, query, args...)

	return db.db.QueryContext(ctx, query, args...)
}

func (db *database) QueryRow(query string, args ...any) (*sql.Row, error) {
	return db.QueryRowContext(context.Background(), query, args...)
}

func (db *database) QueryRowContext(ctx context.Context, query string, args ...any) (row *sql.Row, err error) {
	if db == nil || db.db == nil {
		return nil, errors.New("database not initialized")
	}
	ctx = db.dbBefore(ctx, query, args...)
	defer db.dbAfter(ctx, query, args...)

	return db.db.QueryRowContext(ctx, query, args...), nil
}

type transaction struct {
	tx *sql.Tx
	db *database
}

func (db *database) Begin() (*transaction, error) {
	return db.BeginTx(context.Background(), nil)
}

func (db *database) BeginTx(ctx context.Context, opts *sql.TxOptions) (*transaction, error) {
	if db == nil || db.db == nil {
		return nil, errors.New("database not initialized")
	}
	tx, err := db.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &transaction{tx: tx, db: db}, nil
}

func (tx *transaction) Exec(query string, args ...any) (sql.Result, error) {
	return tx.ExecContext(context.Background(), query, args...)
}

func (tx *transaction) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	ctx = tx.db.dbBefore(ctx, query, args...)
	defer tx.db.dbAfter(ctx, query, args...)
	return tx.tx.ExecContext(ctx, query, args...)
}

func (tx *transaction) Query(query string, args ...any) (*sql.Rows, error) {
	return tx.QueryContext(context.Background(), query, args...)
}

func (tx *transaction) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	ctx = tx.db.dbBefore(ctx, query, args...)
	defer tx.db.dbAfter(ctx, query, args...)
	return tx.tx.QueryContext(ctx, query, args...)
}

func (tx *transaction) QueryRow(query string, args ...any) *sql.Row {
	return tx.QueryRowContext(context.Background(), query, args...)
}

func (tx *transaction) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	ctx = tx.db.dbBefore(ctx, query, args...)
	defer tx.db.dbAfter(ctx, query, args...)
	return tx.tx.QueryRowContext(ctx, query, args...)
}

func (tx *transaction) Commit() error {
	return tx.tx.Commit()
}

func (tx *transaction) Rollback() error {
	return tx.tx.Rollback()
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

package main

import (
	"database/sql"
	"errors"
	"log"
	"os"

	sqlite "github.com/mattn/go-sqlite3"
	"github.com/schollz/sqlite3dump"
	"golang.org/x/sync/singleflight"
)

type database struct {
	db                   *sql.DB
	stmts                map[string]*sql.Stmt
	g                    singleflight.Group
	persistentCacheGroup singleflight.Group
}

func (a *goBlog) initDatabase() (err error) {
	// Setup db
	db, err := a.openDatabase(a.cfg.Db.File)
	if err != nil {
		return err
	}
	// Create appDB
	a.db = db
	db.vacuum()
	addShutdownFunc(func() {
		_ = db.close()
		log.Println("Closed database")
	})
	if a.cfg.Db.DumpFile != "" {
		hourlyHooks = append(hourlyHooks, func() {
			db.dump(a.cfg.Db.DumpFile)
		})
		db.dump(a.cfg.Db.DumpFile)
	}
	return nil
}

func (a *goBlog) openDatabase(file string) (*database, error) {
	// Register driver
	dbDriverName := generateRandomString(15)
	sql.Register("goblog_db_"+dbDriverName, &sqlite.SQLiteDriver{
		ConnectHook: func(c *sqlite.SQLiteConn) error {
			if err := c.RegisterFunc("tolocal", toLocalSafe, true); err != nil {
				return err
			}
			if err := c.RegisterFunc("wordcount", wordCount, true); err != nil {
				return err
			}
			if err := c.RegisterFunc("mdtext", a.renderText, true); err != nil {
				return err
			}
			return nil
		},
	})
	// Open db
	db, err := sql.Open("goblog_db_"+dbDriverName, file+"?cache=shared&mode=rwc&_journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
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
	err = migrateDb(db)
	if err != nil {
		return nil, err
	}
	return &database{
		db:    db,
		stmts: map[string]*sql.Stmt{},
	}, nil
}

// Main features

func (db *database) dump(file string) {
	f, err := os.Create(file)
	if err != nil {
		log.Println("Error while dump db:", err.Error())
		return
	}
	if err = sqlite3dump.DumpDB(db.db, f); err != nil {
		log.Println("Error while dump db:", err.Error())
	}
}

func (db *database) close() error {
	db.vacuum()
	return db.db.Close()
}

func (db *database) vacuum() {
	_, _ = db.exec("VACUUM")
}

func (db *database) prepare(query string) (*sql.Stmt, error) {
	stmt, err, _ := db.g.Do(query, func() (interface{}, error) {
		stmt, ok := db.stmts[query]
		if ok && stmt != nil {
			return stmt, nil
		}
		stmt, err := db.db.Prepare(query)
		if err != nil {
			return nil, err
		}
		db.stmts[query] = stmt
		return stmt, nil
	})
	if err != nil {
		return nil, err
	}
	return stmt.(*sql.Stmt), nil
}

func (db *database) exec(query string, args ...interface{}) (sql.Result, error) {
	stmt, err := db.prepare(query)
	if err != nil {
		return nil, err
	}
	return stmt.Exec(args...)
}

func (db *database) execMulti(query string, args ...interface{}) (sql.Result, error) {
	// Can't prepare the statement
	return db.db.Exec(query, args...)
}

func (db *database) query(query string, args ...interface{}) (*sql.Rows, error) {
	stmt, err := db.prepare(query)
	if err != nil {
		return nil, err
	}
	return stmt.Query(args...)
}

func (db *database) queryRow(query string, args ...interface{}) (*sql.Row, error) {
	stmt, err := db.prepare(query)
	if err != nil {
		return nil, err
	}
	return stmt.QueryRow(args...), nil
}

// Other things

func (d *database) rebuildFTSIndex() {
	_, _ = d.exec("insert into posts_fts(posts_fts) values ('rebuild')")
}

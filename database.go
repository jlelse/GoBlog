package main

import (
	"database/sql"
	"errors"
	"log"
	"os"

	sqlite "github.com/mattn/go-sqlite3"
	"github.com/schollz/sqlite3dump"
)

var appDb *goblogDb

type goblogDb struct {
	db             *sql.DB
	statementCache map[string]*sql.Stmt
}

func initDatabase() (err error) {
	// Setup db
	sql.Register("goblog_db", &sqlite.SQLiteDriver{
		ConnectHook: func(c *sqlite.SQLiteConn) error {
			if err := c.RegisterFunc("tolocal", toLocalSafe, true); err != nil {
				return err
			}
			if err := c.RegisterFunc("wordcount", wordCount, true); err != nil {
				return err
			}
			if err := c.RegisterFunc("mdtext", renderText, true); err != nil {
				return err
			}
			return nil
		},
	})
	db, err := sql.Open("goblog_db", appConfig.Db.File+"?cache=shared&mode=rwc&_journal_mode=WAL")
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	err = db.Ping()
	if err != nil {
		return err
	}
	// Check available SQLite features
	rows, err := db.Query("pragma compile_options")
	if err != nil {
		return err
	}
	cos := map[string]bool{}
	var co string
	for rows.Next() {
		err = rows.Scan(&co)
		if err != nil {
			return err
		}
		cos[co] = true
	}
	if _, ok := cos["ENABLE_FTS5"]; !ok {
		return errors.New("sqlite not compiled with FTS5")
	}
	// Migrate DB
	err = migrateDb(db)
	if err != nil {
		return err
	}
	// Create appDB
	appDb = &goblogDb{
		db:             db,
		statementCache: map[string]*sql.Stmt{},
	}
	appDb.vacuum()
	addShutdownFunc(func() {
		_ = appDb.close()
		log.Println("Closed database")
	})
	if appConfig.Db.DumpFile != "" {
		hourlyHooks = append(hourlyHooks, func() {
			appDb.dump()
		})
		appDb.dump()
	}
	return nil
}

func (db *goblogDb) dump() {
	f, err := os.Create(appConfig.Db.DumpFile)
	if err != nil {
		log.Println("Error while dump db:", err.Error())
		return
	}
	if err = sqlite3dump.DumpDB(db.db, f); err != nil {
		log.Println("Error while dump db:", err.Error())
	}
}

func (db *goblogDb) close() error {
	db.vacuum()
	return db.db.Close()
}

func (db *goblogDb) vacuum() {
	_, _ = db.exec("VACUUM")
}

func (db *goblogDb) prepare(query string) (*sql.Stmt, error) {
	stmt, err, _ := cacheGroup.Do(query, func() (interface{}, error) {
		stmt, ok := db.statementCache[query]
		if ok && stmt != nil {
			return stmt, nil
		}
		stmt, err := db.db.Prepare(query)
		if err != nil {
			return nil, err
		}
		db.statementCache[query] = stmt
		return stmt, nil
	})
	if err != nil {
		return nil, err
	}
	return stmt.(*sql.Stmt), nil
}

func (db *goblogDb) exec(query string, args ...interface{}) (sql.Result, error) {
	stmt, err := db.prepare(query)
	if err != nil {
		return nil, err
	}
	return stmt.Exec(args...)
}

func (db *goblogDb) execMulti(query string, args ...interface{}) (sql.Result, error) {
	// Can't prepare the statement
	return db.db.Exec(query, args...)
}

func (db *goblogDb) query(query string, args ...interface{}) (*sql.Rows, error) {
	stmt, err := db.prepare(query)
	if err != nil {
		return nil, err
	}
	return stmt.Query(args...)
}

func (db *goblogDb) queryRow(query string, args ...interface{}) (*sql.Row, error) {
	stmt, err := db.prepare(query)
	if err != nil {
		return nil, err
	}
	return stmt.QueryRow(args...), nil
}

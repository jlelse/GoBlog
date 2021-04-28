package main

import (
	"database/sql"
	"log"
	"os"
	"sync"

	sqlite "github.com/mattn/go-sqlite3"
	"github.com/schollz/sqlite3dump"
)

var (
	appDb            *sql.DB
	appDbWriteMutex  = &sync.Mutex{}
	dbStatementCache = map[string]*sql.Stmt{}
)

func initDatabase() (err error) {
	sql.Register("goblog_db", &sqlite.SQLiteDriver{
		ConnectHook: func(c *sqlite.SQLiteConn) error {
			if err := c.RegisterFunc("tolocal", toLocalSafe, true); err != nil {
				return err
			}
			if err := c.RegisterFunc("wordcount", wordCount, true); err != nil {
				return err
			}
			return nil
		},
	})
	appDb, err = sql.Open("goblog_db", appConfig.Db.File+"?cache=shared&mode=rwc&_journal_mode=WAL")
	if err != nil {
		return err
	}
	err = appDb.Ping()
	if err != nil {
		return err
	}
	vacuumDb()
	err = migrateDb()
	if err != nil {
		return err
	}
	if appConfig.Db.DumpFile != "" {
		hourlyHooks = append(hourlyHooks, dumpDb)
		dumpDb()
	}
	return nil
}

func dumpDb() {
	f, err := os.Create(appConfig.Db.DumpFile)
	if err != nil {
		log.Println("Error while dump db:", err.Error())
		return
	}
	startWritingToDb()
	defer finishWritingToDb()
	if err = sqlite3dump.DumpDB(appDb, f); err != nil {
		log.Println("Error while dump db:", err.Error())
	}
}

func startWritingToDb() {
	appDbWriteMutex.Lock()
}

func finishWritingToDb() {
	appDbWriteMutex.Unlock()
}

func closeDb() error {
	vacuumDb()
	return appDb.Close()
}

func vacuumDb() {
	_, _ = appDbExec("VACUUM")
}

func prepareAppDbStatement(query string) (*sql.Stmt, error) {
	stmt, err, _ := cacheGroup.Do(query, func() (interface{}, error) {
		stmt, ok := dbStatementCache[query]
		if ok && stmt != nil {
			return stmt, nil
		}
		stmt, err := appDb.Prepare(query)
		if err != nil {
			return nil, err
		}
		dbStatementCache[query] = stmt
		return stmt, nil
	})
	if err != nil {
		return nil, err
	}
	return stmt.(*sql.Stmt), nil
}

func appDbExec(query string, args ...interface{}) (sql.Result, error) {
	stmt, err := prepareAppDbStatement(query)
	if err != nil {
		return nil, err
	}
	startWritingToDb()
	defer finishWritingToDb()
	return stmt.Exec(args...)
}

func appDbQuery(query string, args ...interface{}) (*sql.Rows, error) {
	stmt, err := prepareAppDbStatement(query)
	if err != nil {
		return nil, err
	}
	return stmt.Query(args...)
}

func appDbQueryRow(query string, args ...interface{}) (*sql.Row, error) {
	stmt, err := prepareAppDbStatement(query)
	if err != nil {
		return nil, err
	}
	return stmt.QueryRow(args...), nil
}

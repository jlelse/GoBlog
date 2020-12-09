package main

import (
	"database/sql"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

var (
	appDb                 *sql.DB
	appDbWriteMutex       = &sync.Mutex{}
	dbStatementCache      = map[string]*sql.Stmt{}
	dbStatementCacheMutex = &sync.RWMutex{}
)

func initDatabase() (err error) {
	appDb, err = sql.Open("sqlite3", appConfig.Db.File+"?cache=shared&mode=rwc&_journal_mode=WAL")
	if err != nil {
		return err
	}
	return migrateDb()
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
	_, _ = appDbExec("VACUUM;")
}

func prepareAppDbStatement(query string) (*sql.Stmt, error) {
	stmt, err, _ := cacheGroup.Do(query, func() (interface{}, error) {
		dbStatementCacheMutex.RLock()
		stmt, ok := dbStatementCache[query]
		dbStatementCacheMutex.RUnlock()
		if ok && stmt != nil {
			return stmt, nil
		}
		stmt, err := appDb.Prepare(query)
		if err != nil {
			return nil, err
		}
		dbStatementCacheMutex.Lock()
		dbStatementCache[query] = stmt
		dbStatementCacheMutex.Unlock()
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

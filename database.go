package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"sync"
)

var appDb *sql.DB
var appDbWriteMutex = &sync.Mutex{}

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
	startWritingToDb()
	_, _ = appDb.Exec("VACUUM;")
	finishWritingToDb()
}

package main

import (
	"bufio"
	"database/sql"
	"log"
	"os"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	"github.com/schollz/sqlite3dump"
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
	dumpDb()
}

func closeDb() error {
	vacuumDb()
	return appDb.Close()
}

func vacuumDb() {
	startWritingToDb()
	defer finishWritingToDb()
	_, _ = appDb.Exec("VACUUM;")
}

func dumpDb() {
	appDbWriteMutex.Lock()
	defer appDbWriteMutex.Unlock()
	f, err := os.OpenFile(appConfig.Db.File+".dump", os.O_RDWR|os.O_CREATE, 0644)
	defer f.Close()
	if err != nil {
		log.Println("Failed to open dump file:", err.Error())
		return
	}
	w := bufio.NewWriter(f)
	err = sqlite3dump.DumpDB(appDb, w)
	if err != nil {
		log.Println("Failed to dump database:", err.Error())
		return
	}
	err = w.Flush()
	if err != nil {
		log.Println("Failed to write dump:", err.Error())
		return
	}
}

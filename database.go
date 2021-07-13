package main

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"log"
	"os"
	"sync"

	"github.com/gchaincl/sqlhooks/v2"
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
	pc  singleflight.Group // persistant cache
	pcm sync.Mutex         // post creation
	sp  singleflight.Group // singleflight group for short path requests
	spc sync.Map           // shortpath cache
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
	dbDriverName := generateRandomString(15)
	var dr driver.Driver = &sqlite.SQLiteDriver{
		ConnectHook: func(c *sqlite.SQLiteConn) error {
			// Depends on app
			if err := c.RegisterFunc("mdtext", a.renderText, true); err != nil {
				return err
			}
			// Independent
			if err := c.RegisterFunc("tolocal", toLocalSafe, true); err != nil {
				return err
			}
			if err := c.RegisterFunc("toutc", toUTCSafe, true); err != nil {
				return err
			}
			if err := c.RegisterFunc("wordcount", wordCount, true); err != nil {
				return err
			}
			if err := c.RegisterFunc("charcount", charCount, true); err != nil {
				return err
			}
			return nil
		},
	}
	if c := a.cfg.Db; c != nil && c.Debug {
		dr = sqlhooks.Wrap(dr, &dbHooks{})
	}
	sql.Register("goblog_db_"+dbDriverName, dr)
	// Open db
	db, err := sql.Open("goblog_db_"+dbDriverName, file+"?mode=rwc&_journal_mode=WAL&_busy_timeout=100&cache=shared")
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
	return &database{
		db: db,
	}, nil
}

// Main features

func (db *database) dump(file string) {
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
	return db.db.Close()
}

func (db *database) prepare(query string) (*sql.Stmt, error) {
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
		return nil, err
	}
	return stmt.(*sql.Stmt), nil
}

const dbNoCache = "nocache"

func (db *database) exec(query string, args ...interface{}) (sql.Result, error) {
	// Lock execution
	db.em.Lock()
	defer db.em.Unlock()
	// Check if prepared cache should be skipped
	if len(args) > 0 && args[0] == dbNoCache {
		return db.db.Exec(query, args[1:]...)
	}
	// Use prepared statement
	st, _ := db.prepare(query)
	if st != nil {
		return st.Exec(args...)
	}
	// Or execute directly
	return db.db.Exec(query, args...)
}

func (db *database) query(query string, args ...interface{}) (*sql.Rows, error) {
	// Check if prepared cache should be skipped
	if len(args) > 0 && args[0] == dbNoCache {
		return db.db.Query(query, args[1:]...)
	}
	// Use prepared statement
	st, _ := db.prepare(query)
	if st != nil {
		return st.Query(args...)
	}
	// Or query directly
	return db.db.Query(query, args...)
}

func (db *database) queryRow(query string, args ...interface{}) (*sql.Row, error) {
	// Check if prepared cache should be skipped
	if len(args) > 0 && args[0] == dbNoCache {
		return db.db.QueryRow(query, args[1:]...), nil
	}
	// Use prepared statement
	st, _ := db.prepare(query)
	if st != nil {
		return st.QueryRow(args...), nil
	}
	// Or query directly
	return db.db.QueryRow(query, args...), nil
}

// Other things

func (d *database) rebuildFTSIndex() {
	_, _ = d.exec("insert into posts_fts(posts_fts) values ('rebuild')")
}

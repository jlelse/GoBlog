package main

import (
	"database/sql"
	"embed"
	"io/fs"
	"log"
	"strings"

	"github.com/lopezator/migrator"
)

//go:embed dbmigrations/*
var dbMigrations embed.FS

func migrateDb(db *sql.DB, logging bool) error {
	var sqlMigrations []any
	err := fs.WalkDir(dbMigrations, "dbmigrations", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.Type().IsDir() {
			return err
		}
		mig := &migrator.Migration{}
		mig.Name = strings.TrimSuffix(d.Name(), ".sql")
		fd, fe := dbMigrations.ReadFile(path)
		if fe != nil {
			return fe
		}
		if len(fd) == 0 {
			return nil
		}
		mig.Func = func(t *sql.Tx) error {
			_, txe := t.Exec(string(fd))
			return txe
		}
		sqlMigrations = append(sqlMigrations, mig)
		return nil
	})
	if err != nil {
		return err
	}
	m, err := migrator.New(
		migrator.WithLogger(migrator.LoggerFunc(func(s string, i ...any) {
			if logging {
				log.Printf(s, i)
			}
		})),
		migrator.Migrations(sqlMigrations...),
	)
	if err != nil {
		return err
	}
	return m.Migrate(db)
}

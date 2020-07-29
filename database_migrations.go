package main

import (
	"database/sql"
	"github.com/lopezator/migrator"
)

func migrateDb() error {
	startWritingToDb()
	defer finishWritingToDb()
	m, err := migrator.New(
		migrator.Migrations(
			&migrator.Migration{
				Name: "00001",
				Func: func(tx *sql.Tx) error {
					_, err := tx.Exec("create table posts (path text not null primary key, content text, published text, updated text);")
					return err
				},
			},
			&migrator.Migration{
				Name: "00002",
				Func: func(tx *sql.Tx) error {
					_, err := tx.Exec("create table redirects (fromPath text not null, toPath text not null, primary key (fromPath, toPath));")
					return err
				},
			},
		),
	)
	if err != nil {
		return err
	}
	if err := m.Migrate(appDb); err != nil {
		return err
	}
	return nil
}

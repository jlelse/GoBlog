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
					_, err := tx.Exec(`
					CREATE TABLE posts (path text not null primary key, content text, published text, updated text, blog text not null, section text);
					CREATE TABLE post_parameters (id integer primary key autoincrement, path text not null, parameter text not null, value text);
					CREATE INDEX index_pp_path on post_parameters (path);
					CREATE TABLE redirects (fromPath text not null, toPath text not null, primary key (fromPath, toPath));
					`)
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

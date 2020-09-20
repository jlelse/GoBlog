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
			&migrator.Migration{
				Name: "00003",
				Func: func(tx *sql.Tx) error {
					_, err := tx.Exec("create table post_parameters (path text not null, parameter text not null, value text, primary key (path, parameter));")
					return err
				},
			},
			&migrator.Migration{
				Name: "00004",
				Func: func(tx *sql.Tx) error {
					_, err := tx.Exec("create table cache (path text not null primary key, time integer, header blob, body blob);")
					return err
				},
			},
			&migrator.Migration{
				Name: "00005",
				Func: func(tx *sql.Tx) error {
					_, err := tx.Exec("create table pp_tmp(id integer primary key autoincrement, path text not null, parameter text not null, value text); insert into pp_tmp(path, parameter, value) select path, parameter, value from post_parameters; drop table post_parameters; alter table pp_tmp rename to post_parameters;")
					return err
				},
			},
			&migrator.Migration{
				Name: "00006",
				Func: func(tx *sql.Tx) error {
					_, err := tx.Exec("create index pp_path_index on post_parameters (path);")
					return err
				},
			},
			&migrator.Migration{
				Name: "00007",
				Func: func(tx *sql.Tx) error {
					_, err := tx.Exec("alter table posts add column section text; create trigger add_section after update on posts begin update posts set section = (select substr(path, 2, len) from (select path, instr(substr(path, 2),'/')-1 as len from (select new.path as path))) where path = new.path; end;")
					return err
				},
			},
			&migrator.Migration{
				Name: "00008",
				Func: func(tx *sql.Tx) error {
					_, err := tx.Exec("create index p_section_index on posts (section);")
					return err
				},
			},
			&migrator.Migration{
				Name: "00009",
				Func: func(tx *sql.Tx) error {
					_, err := tx.Exec("create trigger add_section_insert after insert on posts begin update posts set section = (select substr(path, 2, len) from (select path, instr(substr(path, 2),'/')-1 as len from (select new.path as path))) where path = new.path; end;")
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

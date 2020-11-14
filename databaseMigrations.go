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
					CREATE TRIGGER AFTER DELETE on posts BEGIN delete from post_parameters where path = old.path; END;
					CREATE TABLE indieauthauth (time text not null, code text not null, me text not null, client text not null, redirect text not null, scope text not null);
					CREATE TABLE indieauthtoken (time text not null, token text not null, me text not null, client text not null, scope text not null);
					CREATE INDEX index_iat_token on indieauthtoken (token);
					CREATE TABLE autocert (key text not null primary key, data blob not null, created text not null);
					CREATE TABLE activitypub_followers (blog text not null, follower text not null, inbox text not null, primary key (blog, follower));
					CREATE TABLE webmentions (id integer primary key autoincrement, source text not null, target text not null, created integer not null, status text not null default "new", title text, content text, author text, type text, UNIQUE(source, target));
					CREATE INDEX index_wm_target on webmentions (target);
					`)
					return err
				},
			},
			&migrator.Migration{
				Name: "00002",
				Func: func(tx *sql.Tx) error {
					_, err := tx.Exec(`
					DROP TABLE autocert;
					`)
					return err
				},
			},
			&migrator.Migration{
				Name: "00003",
				Func: func(tx *sql.Tx) error {
					_, err := tx.Exec(`
					DROP TRIGGER AFTER;
					CREATE TRIGGER trigger_posts_delete_pp AFTER DELETE on posts BEGIN delete from post_parameters where path = old.path; END;
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

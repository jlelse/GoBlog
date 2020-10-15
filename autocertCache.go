package main

import (
	"context"
	"database/sql"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

type autocertCache struct {
	db          *sql.DB
	getQuery    string
	putQuery    string
	deleteQuery string
}

func newAutocertCache() (*autocertCache, error) {
	return &autocertCache{
		db:          appDb,
		getQuery:    "select data from autocert where key = ?",
		putQuery:    "insert or replace into autocert (key, data, created) values (?, ?, ?)",
		deleteQuery: "delete from autocert where key = ?",
	}, nil
}

func (c *autocertCache) Get(ctx context.Context, key string) ([]byte, error) {
	var data []byte
	err := c.db.QueryRowContext(ctx, c.getQuery, key).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, autocert.ErrCacheMiss
	}
	return data, err
}

func (c *autocertCache) Put(ctx context.Context, key string, data []byte) error {
	startWritingToDb()
	defer finishWritingToDb()
	_, err := c.db.ExecContext(ctx, c.putQuery, key, data, time.Now().String())
	return err
}

func (c *autocertCache) Delete(ctx context.Context, key string) error {
	startWritingToDb()
	defer finishWritingToDb()
	_, err := c.db.ExecContext(ctx, c.deleteQuery, key)
	return err
}

package main

import (
	"context"
	"database/sql"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

type autocertCache struct{}

func (c *autocertCache) Get(ctx context.Context, key string) ([]byte, error) {
	var data []byte
	row, err := appDbQueryRow("select data from autocert where key = @key", sql.Named("key", key))
	if err != nil {
		return nil, err
	}
	err = row.Scan(&data)
	if err == sql.ErrNoRows {
		return nil, autocert.ErrCacheMiss
	}
	return data, err
}

func (c *autocertCache) Put(ctx context.Context, key string, data []byte) error {
	startWritingToDb()
	defer finishWritingToDb()
	_, err := appDbExec("insert or replace into autocert (key, data, created) values (@key, @data, @created)", sql.Named("key", key), sql.Named("data", data), sql.Named("created", time.Now().String()))
	return err
}

func (c *autocertCache) Delete(ctx context.Context, key string) error {
	startWritingToDb()
	defer finishWritingToDb()
	_, err := appDbExec("delete from autocert where key = @key", sql.Named("key", key))
	return err
}

package main

import (
	"context"
	"database/sql"
	"errors"
)

func (db *database) cachePersistently(key string, data []byte) error {
	return db.cachePersistentlyContext(context.Background(), key, data)
}

func (db *database) cachePersistentlyContext(ctx context.Context, key string, data []byte) error {
	if db == nil {
		return errors.New("database is nil")
	}
	_, err := db.ExecContext(ctx, "insert or replace into persistent_cache(key, data, date) values(@key, @data, @date)", sql.Named("key", key), sql.Named("data", data), sql.Named("date", utcNowString()))
	return err
}

func (db *database) retrievePersistentCache(key string) (data []byte, err error) {
	return db.retrievePersistentCacheContext(context.Background(), key)
}

func (db *database) retrievePersistentCacheContext(c context.Context, key string) (data []byte, err error) {
	if db == nil {
		return nil, errors.New("database is nil")
	}
	d, err, _ := db.pc.Do(key, func() (any, error) {
		if row, err := db.QueryRowContext(c, "select data from persistent_cache where key = @key", sql.Named("key", key)); err != nil {
			return nil, err
		} else {
			err = row.Scan(&data)
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return data, err
		}
	})
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, nil
	}
	return d.([]byte), nil
}

func (db *database) clearPersistentCache(pattern string) error {
	return db.clearPersistentCacheContext(context.Background(), pattern)
}

func (db *database) clearPersistentCacheContext(c context.Context, pattern string) error {
	_, err := db.ExecContext(c, "delete from persistent_cache where key like @pattern", sql.Named("pattern", pattern))
	return err
}

package main

import (
	"database/sql"
	"errors"
)

func (db *database) cachePersistently(key string, data []byte) error {
	if db == nil {
		return errors.New("database is nil")
	}
	_, err := db.exec("insert or replace into persistent_cache(key, data, date) values(@key, @data, @date)", sql.Named("key", key), sql.Named("data", data), sql.Named("date", utcNowString()))
	return err
}

func (db *database) retrievePersistentCache(key string) (data []byte, err error) {
	if db == nil {
		return nil, errors.New("database is nil")
	}
	d, err, _ := db.pc.Do(key, func() (interface{}, error) {
		if row, err := db.queryRow("select data from persistent_cache where key = @key", sql.Named("key", key)); err != nil {
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
	_, err := db.exec("delete from persistent_cache where key like @pattern", sql.Named("pattern", pattern))
	return err
}

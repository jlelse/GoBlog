package main

import (
	"database/sql"
	"time"
)

func (db *database) cachePersistently(key string, data []byte) error {
	date, _ := toLocal(time.Now().String())
	_, err := db.exec("insert or replace into persistent_cache(key, data, date) values(@key, @data, @date)", sql.Named("key", key), sql.Named("data", data), sql.Named("date", date))
	return err
}

func (db *database) retrievePersistentCache(key string) (data []byte, err error) {
	d, err, _ := db.persistentCacheGroup.Do(key, func() (interface{}, error) {
		if row, err := db.queryRow("select data from persistent_cache where key = @key", sql.Named("key", key)); err == sql.ErrNoRows {
			return nil, nil
		} else if err != nil {
			return nil, err
		} else {
			err = row.Scan(&data)
			return data, err
		}
	})
	if err != nil {
		return nil, err
	}
	return d.([]byte), nil
}

func (db *database) clearPersistentCache(pattern string) error {
	_, err := db.exec("delete from persistent_cache where key like @pattern", sql.Named("pattern", pattern))
	return err
}

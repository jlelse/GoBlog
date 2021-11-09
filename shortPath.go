package main

import (
	"database/sql"
	"errors"
)

func (db *database) shortenPath(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	spi, err, _ := db.sp.Do(p, func() (interface{}, error) {
		// Check if already cached
		if spi, ok := db.spc.Load(p); ok {
			return spi.(string), nil
		}
		// Insert in case it isn't shortened yet
		_, err := db.exec("insert or ignore into shortpath (path) values (@path)", sql.Named("path", p))
		if err != nil {
			return nil, err
		}
		// Query short path
		row, err := db.queryRow("select printf('/s/%x', id) from shortpath where path = @path", sql.Named("path", p))
		if err != nil {
			return nil, err
		}
		var sp string
		err = row.Scan(&sp)
		if err != nil {
			return nil, err
		}
		// Cache result
		db.spc.Store(p, sp)
		return sp, nil
	})
	if err != nil {
		return "", err
	}
	return spi.(string), nil
}

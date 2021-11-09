package main

import (
	"database/sql"
	"errors"
	"fmt"
)

func (db *database) shortenPath(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	idi, err, _ := db.sp.Do(p, func() (interface{}, error) {
		id := db.getShortPathID(p)
		if id == -1 {
			_, err := db.exec("insert or ignore into shortpath (path) values (@path)", sql.Named("path", p))
			if err != nil {
				return nil, err
			}
			id = db.getShortPathID(p)
		}
		return id, nil
	})
	if err != nil {
		return "", err
	}
	id := idi.(int)
	if id == -1 {
		return "", errors.New("failed to retrieve short path for " + p)
	}
	return fmt.Sprintf("/s/%x", id), nil
}

func (db *database) getShortPathID(p string) (id int) {
	if p == "" {
		return -1
	}
	if idi, ok := db.spc.Load(p); ok {
		return idi.(int)
	}
	row, err := db.queryRow("select id from shortpath where path = @path", sql.Named("path", p))
	if err != nil {
		return -1
	}
	err = row.Scan(&id)
	if err != nil {
		return -1
	}
	db.spc.Store(p, id)
	return id
}

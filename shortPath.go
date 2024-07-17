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

	// Use singleflight to deduplicate concurrent requests and handle caching
	v, err, _ := db.sp.Do(p, func() (interface{}, error) {
		return db.shortenPathTransaction(p)
	})

	if err != nil {
		return "", err
	}

	return v.(string), nil
}

func (db *database) shortenPathTransaction(p string) (string, error) {
	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	var id int64
	err = tx.QueryRow("select id from shortpath where path = @path", sql.Named("path", p)).Scan(&id)
	if err == sql.ErrNoRows {
		// Path doesn't exist, insert new entry
		result, err := tx.Exec(`
            insert into shortpath (id, path)
			values (
				(select min(id) + 1 from (select id from shortpath union all select 0) where id + 1 not in (select id from shortpath)),
				@path
			)
        `, sql.Named("path", p))
		if err != nil {
			return "", err
		}
		id, err = result.LastInsertId()
		if err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}

	if err = tx.Commit(); err != nil {
		return "", err
	}

	return fmt.Sprintf("/s/%x", id), nil
}

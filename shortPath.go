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

	db.sp.Lock()
	defer db.sp.Unlock()

	result, err := db.shortenPathTransaction(p)
	if err != nil {
		return "", err
	}

	return result, nil
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
		// Path doesn't exist, insert new entry with the lowest available id
		err = tx.QueryRow(`
            WITH RECURSIVE ids(n) AS (
                SELECT 1
                UNION ALL
                SELECT n + 1 FROM ids
                WHERE n < (SELECT COALESCE(MAX(id), 0) + 1 FROM shortpath)
            )
            INSERT INTO shortpath (id, path)
            VALUES (
                (SELECT n FROM ids
                 LEFT JOIN shortpath s ON ids.n = s.id
                 WHERE s.id IS NULL
                 ORDER BY n
                 LIMIT 1),
                ?
            )
            RETURNING id
        `, p).Scan(&id)
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

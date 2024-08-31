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
	result, err, _ := db.sp.Do(p, func() (string, error) {
		sp, err := db.queryShortPath(p)
		if err == nil && sp != "" {
			return sp, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		// In case it wasn't shortened yet ...
		err = db.createShortPath(p)
		if err != nil {
			return "", err
		}
		// Query again
		sp, err = db.queryShortPath(p)
		if err == nil && sp != "" {
			return sp, nil
		}
		return "", err
	})
	return result, err
}

func (db *database) queryShortPath(p string) (string, error) {
	row, err := db.QueryRow("select id from shortpath where path = @path", sql.Named("path", p))
	if err != nil {
		return "", err
	}
	var id int64
	err = row.Scan(&id)
	return fmt.Sprintf("/s/%x", id), err
}

func (db *database) createShortPath(p string) error {
	_, err := db.Exec(`
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
            )`, p)
	return err
}

package main

import (
	"database/sql"
	"errors"

	"github.com/mattn/go-sqlite3"
)

func (db *database) shortenPath(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	spi, err, _ := db.sp.Do(p, func() (any, error) {
		// Check if already cached
		if spi, ok := db.spc.Get(p); ok {
			return spi.(string), nil
		}
		// Insert in case it isn't shortened yet
		_, err := db.Exec(`
		insert or rollback into shortpath (id, path)
		values (
			-- next available id (reuse skipped ids due to bug)
			(select min(id) + 1 from (select id from shortpath union all select 0) where id + 1 not in (select id from shortpath)),
			@path
		)`, sql.Named("path", p))
		if err != nil {
			if no, ok := err.(sqlite3.Error); !ok || sqlite3.ErrNo(no.ExtendedCode) != sqlite3.ErrNo(sqlite3.ErrConstraintUnique) {
				// Some other error than unique constraint violation because path is already shortened
				return nil, err
			}
		}
		// Query short path
		row, err := db.QueryRow("select printf('/s/%x', id) from shortpath where path = @path", sql.Named("path", p))
		if err != nil {
			return nil, err
		}
		var sp string
		err = row.Scan(&sp)
		if err != nil {
			return nil, err
		}
		// Cache result
		db.spc.Set(p, sp, 1)
		return sp, nil
	})
	if err != nil {
		return "", err
	}
	return spi.(string), nil
}

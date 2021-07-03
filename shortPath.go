package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
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

func (a *goBlog) redirectToLongPath(rw http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 16, 64)
	if err != nil {
		a.serve404(rw, r)
		return
	}
	row, err := a.db.queryRow("select path from shortpath where id = @id", sql.Named("id", id))
	if err != nil {
		a.serve404(rw, r)
		return
	}
	var path string
	err = row.Scan(&path)
	if err != nil {
		a.serve404(rw, r)
		return
	}
	http.Redirect(rw, r, path, http.StatusMovedPermanently)
}

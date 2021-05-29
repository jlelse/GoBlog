package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func shortenPath(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	id := getShortPathID(p)
	if id == -1 {
		_, err := appDb.exec("insert or ignore into shortpath (path) values (@path)", sql.Named("path", p))
		if err != nil {
			return "", err
		}
		id = getShortPathID(p)
	}
	if id == -1 {
		return "", errors.New("failed to retrieve short path for " + p)
	}
	return fmt.Sprintf("/s/%x", id), nil
}

func getShortPathID(p string) (id int) {
	if p == "" {
		return -1
	}
	row, err := appDb.queryRow("select id from shortpath where path = @path", sql.Named("path", p))
	if err != nil {
		return -1
	}
	err = row.Scan(&id)
	if err != nil {
		return -1
	}
	return id
}

func redirectToLongPath(rw http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 16, 64)
	if err != nil {
		serve404(rw, r)
		return
	}
	row, err := appDb.queryRow("select path from shortpath where id = @id", sql.Named("id", id))
	if err != nil {
		serve404(rw, r)
		return
	}
	var path string
	err = row.Scan(&path)
	if err != nil {
		serve404(rw, r)
		return
	}
	http.Redirect(rw, r, path, http.StatusMovedPermanently)
}

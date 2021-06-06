package main

import (
	"database/sql"
	"net/http"
)

func (db *database) allPostAliases() ([]string, error) {
	var aliases []string
	rows, err := db.query("select distinct value from post_parameters where parameter = 'aliases' and value != path")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var path string
		_ = rows.Scan(&path)
		if path != "" {
			aliases = append(aliases, path)
		}
	}
	return aliases, nil
}

func (a *goBlog) servePostAlias(w http.ResponseWriter, r *http.Request) {
	row, err := a.db.queryRow("select path from post_parameters where parameter = 'aliases' and value = @alias", sql.Named("alias", r.URL.Path))
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	var path string
	if err := row.Scan(&path); err == sql.ErrNoRows {
		a.serve404(w, r)
		return
	} else if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, path, http.StatusFound)
}

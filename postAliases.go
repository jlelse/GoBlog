package main

import (
	"database/sql"
	"net/http"
)

func allPostAliases() ([]string, error) {
	var aliases []string
	rows, err := appDbQuery("select distinct value from post_parameters where parameter = 'aliases' and value != path")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var path string
		_ = rows.Scan(&path)
		aliases = append(aliases, path)
	}
	return aliases, nil
}

func servePostAlias(w http.ResponseWriter, r *http.Request) {
	row, err := appDbQueryRow("select path from post_parameters where parameter = 'aliases' and value = @alias", sql.Named("alias", r.URL.Path))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var path string
	err = row.Scan(&path)
	if err == sql.ErrNoRows {
		serve404(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, path, http.StatusFound)
	return
}

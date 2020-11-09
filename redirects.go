package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

var errRedirectNotFound = errors.New("redirect not found")

func serveRedirect(w http.ResponseWriter, r *http.Request) {
	redirect, err := getRedirect(r.URL.Path)
	if err == errRedirectNotFound {
		serve404(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Send redirect
	w.Header().Set("Location", redirect)
	render(w, templateRedirect, &renderData{
		Data: redirect,
	})
	w.WriteHeader(http.StatusFound)
}

func getRedirect(fromPath string) (string, error) {
	var toPath string
	row, err := appDbQueryRow("with recursive f (i, fp, tp) as (select 1, fromPath, toPath from redirects where fromPath = ? union all select f.i + 1, r.fromPath, r.toPath from redirects as r join f on f.tp = r.fromPath) select tp from f order by i desc limit 1", fromPath)
	if err != nil {
		return "", err
	}
	err = row.Scan(&toPath)
	if err == sql.ErrNoRows {
		return "", errRedirectNotFound
	} else if err != nil {
		return "", err
	}
	return toPath, nil
}

func allRedirectPaths() ([]string, error) {
	var redirectPaths []string
	rows, err := appDbQuery("select fromPath from redirects")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var path string
		_ = rows.Scan(&path)
		redirectPaths = append(redirectPaths, path)
	}
	return redirectPaths, nil
}

func createOrReplaceRedirect(from, to string) error {
	if from == "" || to == "" {
		return errors.New("empty path")
	}
	if from == to {
		// Don't need a redirect
		return nil
	}
	from = strings.TrimSuffix(from, "/")
	_, err := appDbExec("insert or replace into redirects (fromPath, toPath) values (?, ?)", from, to)
	if err != nil {
		return err
	}
	return reloadRouter()
}

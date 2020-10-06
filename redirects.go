package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

var errRedirectNotFound = errors.New("redirect not found")

func serveRedirect(w http.ResponseWriter, r *http.Request) {
	redirect, err := getRedirect(r.Context(), slashTrimmedPath(r))
	if err == errRedirectNotFound {
		serve404(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Send redirect
	w.Header().Set("Location", redirect)
	render(w, templateRedirect, struct {
		Blog      string
		Permalink string
	}{
		Blog:      appConfig.DefaultBlog,
		Permalink: redirect,
	})
	w.WriteHeader(http.StatusFound)
}

func getRedirect(context context.Context, fromPath string) (string, error) {
	var toPath string
	row := appDb.QueryRowContext(context, "with recursive f (i, fp, tp) as (select 1, fromPath, toPath from redirects where fromPath = ? union all select f.i + 1, r.fromPath, r.toPath from redirects as r join f on f.tp = r.fromPath) select tp from f order by i desc limit 1", fromPath)
	err := row.Scan(&toPath)
	if err == sql.ErrNoRows {
		return "", errRedirectNotFound
	} else if err != nil {
		return "", err
	}
	return toPath, nil
}

func allRedirectPaths() ([]string, error) {
	var redirectPaths []string
	rows, err := appDb.Query("select fromPath from redirects")
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
	from = strings.TrimSuffix(from, "/")
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		finishWritingToDb()
		return err
	}
	_, err = tx.Exec("insert or replace into redirects (fromPath, toPath) values (?, ?)", from, to)
	if err != nil {
		_ = tx.Rollback()
		finishWritingToDb()
		return err
	}
	err = tx.Commit()
	if err != nil {
		finishWritingToDb()
		return err
	}
	finishWritingToDb()
	return reloadRouter()
}

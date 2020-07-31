package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
)

var errRedirectNotFound = errors.New("redirect not found")

func serveRedirect(w http.ResponseWriter, r *http.Request) {
	redirect, err := getRedirect(r.Context(), slashTrimmedPath(r))
	if err == errRedirectNotFound {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusFound)
	_ = templates.ExecuteTemplate(w, templateRedirectName, struct {
		Permalink string
	}{
		Permalink: redirect,
	})
}

func getRedirect(context context.Context, fromPath string) (string, error) {
	var toPath string
	row := appDb.QueryRowContext(context, "select toPath from redirects where fromPath=?", fromPath)
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

package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
)

var redirectNotFound = errors.New("redirect not found")

type redirect struct {
	fromPath string
	toPath   string
}

func serveRedirect(w http.ResponseWriter, r *http.Request) {
	redirect, err := getRedirect(r.RequestURI, r.Context())
	if err == redirectNotFound {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// TODO: Change status code
	http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
}

func getRedirect(fromPath string, context context.Context) (string, error) {
	var toPath string
	row := appDb.QueryRowContext(context, "select toPath from redirects where fromPath=?", fromPath)
	err := row.Scan(&toPath)
	if err == sql.ErrNoRows {
		return "", redirectNotFound
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

package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
)

var errRedirectNotFound = errors.New("redirect not found")

func serveRedirect(w http.ResponseWriter, r *http.Request) {
	redirect, more, err := getRedirect(r.Context(), slashTrimmedPath(r))
	if err == errRedirectNotFound {
		serve404(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Flatten redirects
	if more {
		for more == true {
			redirect, more, _ = getRedirect(r.Context(), trimSlash(redirect))
		}
	}
	// Send redirect
	w.Header().Set("Location", redirect)
	render(w, templateRedirect, struct {
		Permalink string
	}{
		Permalink: redirect,
	})
	w.WriteHeader(http.StatusFound)
}

func getRedirect(context context.Context, fromPath string) (string, bool, error) {
	var toPath string
	var moreRedirects int
	row := appDb.QueryRowContext(context, "select toPath, (select count(*) from redirects where fromPath=(select toPath from redirects where fromPath=?)) as more from redirects where fromPath=?", fromPath, fromPath)
	err := row.Scan(&toPath, &moreRedirects)
	if err == sql.ErrNoRows {
		return "", false, errRedirectNotFound
	} else if err != nil {
		return "", false, err
	}
	return toPath, moreRedirects > 0, nil
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

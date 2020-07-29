package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

var postNotFound = errors.New("post not found")

type post struct {
	path      string
	content   string
	published string
	updated   string
}

func servePost(w http.ResponseWriter, r *http.Request) {
	post, err := getPost(strings.TrimSuffix(strings.TrimPrefix(r.RequestURI, "/"), "/"))
	if err == postNotFound {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	htmlContent, err := renderMarkdown(post.content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write(htmlContent)
}

func getPost(path string) (*post, error) {
	queriedPost := &post{}
	row := appDb.QueryRow("select path, COALESCE(content, ''), COALESCE(published, ''), COALESCE(updated, '') from posts where path=?", path)
	err := row.Scan(&queriedPost.path, &queriedPost.content, &queriedPost.published, &queriedPost.updated)
	if err == sql.ErrNoRows {
		return nil, postNotFound
	} else if err != nil {
		return nil, err
	}
	return queriedPost, nil
}

package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
)

var errPostNotFound = errors.New("post not found")

type post struct {
	path       string
	content    string
	published  string
	updated    string
	parameters map[string]string
}

func servePost(w http.ResponseWriter, r *http.Request) {
	path := slashTrimmedPath(r)
	post, err := getPost(r.Context(), path)
	if err == errPostNotFound {
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

func getPost(context context.Context, path string) (*post, error) {
	queriedPost := &post{}
	row := appDb.QueryRowContext(context, "select path, COALESCE(content, ''), COALESCE(published, ''), COALESCE(updated, '') from posts where path=?", path)
	err := row.Scan(&queriedPost.path, &queriedPost.content, &queriedPost.published, &queriedPost.updated)
	if err == sql.ErrNoRows {
		return nil, errPostNotFound
	} else if err != nil {
		return nil, err
	}
	err = queriedPost.fetchParameters(context)
	if err != nil {
		return nil, err
	}
	return queriedPost, nil
}

func (p *post) fetchParameters(context context.Context) error {
	rows, err := appDb.QueryContext(context, "select parameter, COALESCE(value, '') from post_parameters where path=?", p.path)
	if err != nil {
		return err
	}
	p.parameters = make(map[string]string)
	for rows.Next() {
		var parameter string
		var value string
		_ = rows.Scan(&parameter, &value)
		p.parameters[parameter] = value
	}
	return nil
}

func allPostPaths() ([]string, error) {
	var postPaths []string
	rows, err := appDb.Query("select path from posts")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var path string
		_ = rows.Scan(&path)
		postPaths = append(postPaths, path)
	}
	return postPaths, nil
}

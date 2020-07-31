package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

var errPostNotFound = errors.New("post not found")

type Post struct {
	Path       string            `json:"path"`
	Content    string            `json:"content"`
	Published  string            `json:"published"`
	Updated    string            `json:"updated"`
	Parameters map[string]string `json:"parameters"`
}

func servePost(w http.ResponseWriter, r *http.Request) {
	path := slashTrimmedPath(r)
	post, err := getPost(r.Context(), path)
	if err == errPostNotFound {
		serve404(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, templatePost, post)
}

func getPost(context context.Context, path string) (*Post, error) {
	queriedPost := &Post{}
	row := appDb.QueryRowContext(context, "select path, COALESCE(content, ''), COALESCE(published, ''), COALESCE(updated, '') from posts where path=?", path)
	err := row.Scan(&queriedPost.Path, &queriedPost.Content, &queriedPost.Published, &queriedPost.Updated)
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

func (p *Post) fetchParameters(context context.Context) error {
	rows, err := appDb.QueryContext(context, "select parameter, COALESCE(value, '') from post_parameters where path=?", p.Path)
	if err != nil {
		return err
	}
	p.Parameters = make(map[string]string)
	for rows.Next() {
		var parameter string
		var value string
		_ = rows.Scan(&parameter, &value)
		p.Parameters[parameter] = value
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

func createPost(post *Post) error {
	if post == nil {
		return nil
	}
	if post.Path == "" || !strings.HasPrefix(post.Path, "/") {
		return errors.New("wrong path")
	}
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec("insert into posts (path, content, published, updated) values (?, ?, ?, ?)", post.Path, post.Content, post.Published, post.Updated)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	for param, value := range post.Parameters {
		_, err = tx.Exec("insert into post_parameters (path, parameter, value) values (?, ?, ?)", post.Path, param, value)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	finishWritingToDb()
	return reloadRouter()
}

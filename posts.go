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

func serveIndex(w http.ResponseWriter, r *http.Request) {
	posts, err := getAllPosts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, templateIndex, posts)
}

func getPost(context context.Context, path string) (*Post, error) {
	posts, err := getPosts(context, path)
	if err != nil {
		return nil, err
	} else if len(posts) == 0 {
		return nil, errPostNotFound
	}
	return posts[0], nil
}

func getAllPosts(context context.Context) (posts []*Post, err error) {
	return getPosts(context, "")
}

func getPosts(context context.Context, path string) (posts []*Post, err error) {
	paths := make(map[string]int)
	var rows *sql.Rows
	if path != "" {
		rows, err = appDb.QueryContext(context, "select p.path, COALESCE(content, ''), COALESCE(published, ''), COALESCE(updated, ''), COALESCE(parameter, ''), COALESCE(value, '') from posts p left outer join post_parameters pp on p.path = pp.path where p.path=?", path)
	} else {
		rows, err = appDb.QueryContext(context, "select p.path, COALESCE(content, ''), COALESCE(published, ''), COALESCE(updated, ''), COALESCE(parameter, ''), COALESCE(value, '') from posts p left outer join post_parameters pp on p.path = pp.path")
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	for rows.Next() {
		post := &Post{}
		var parameterName, parameterValue string
		err = rows.Scan(&post.Path, &post.Content, &post.Published, &post.Updated, &parameterName, &parameterValue)
		if err != nil {
			return nil, err
		}
		if paths[post.Path] == 0 {
			index := len(posts)
			paths[post.Path] = index + 1
			post.Parameters = make(map[string]string)
			posts = append(posts, post)
		}
		if parameterName != "" && posts != nil {
			posts[paths[post.Path]-1].Parameters[parameterName] = parameterValue
		}
	}
	return posts, nil
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

func checkPost(post *Post) error {
	if post == nil {
		return errors.New("no post")
	}
	if post.Path == "" || !strings.HasPrefix(post.Path, "/") {
		return errors.New("wrong path")
	}
	return nil
}

func createPost(post *Post) error {
	err := checkPost(post)
	if err != nil {
		return err
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
	go purgeCache(post.Path)
	return reloadRouter()
}

func deletePost(post *Post) error {
	err := checkPost(post)
	if err != nil {
		return err
	}
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec("delete from posts where path=?", post.Path)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	_, err = tx.Exec("delete from post_parameters where path=?", post.Path)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	finishWritingToDb()
	go purgeCache(post.Path)
	return reloadRouter()
}

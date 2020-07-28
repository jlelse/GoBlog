package main

import (
	"database/sql"
	"errors"
	"github.com/labstack/echo/v4"
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

func servePost(c echo.Context) error {
	post, err := getPost(strings.TrimSuffix(strings.TrimPrefix(c.Request().RequestURI, "/"), "/"))
	if err == postNotFound {
		return echo.ErrNotFound
	} else if err != nil {
		return err
	}
	htmlContent, err := renderMarkdown(post.content)
	if err != nil {
		return err
	}
	return c.HTML(http.StatusOK, htmlContent)
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

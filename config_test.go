package main

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
)

func createDefaultTestConfig(t *testing.T) *config {
	c := createDefaultConfig()
	c.Db.File = filepath.Join(t.TempDir(), "blog.db")
	return c
}

func reqWithDefaultBlog(req *http.Request) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), blogKey, "default"))
}

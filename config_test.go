package main

import (
	"path/filepath"
	"testing"
)

func createDefaultTestConfig(t *testing.T) *config {
	c := createDefaultConfig()
	c.Db.File = filepath.Join(t.TempDir(), "blog.db")
	return c
}

package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const staticFolder = "static"

func allStaticPaths() (paths []string) {
	paths = []string{}
	err := filepath.Walk(staticFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			paths = append(paths, strings.TrimPrefix(path, staticFolder))
		}
		return nil
	})
	if err != nil {
		return
	}
	return
}

func hasStaticPath(path string) bool {
	// Check if file exists
	_, err := os.Stat(filepath.Join(staticFolder, path))
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

// Gets only called by registered paths
func (a *goBlog) serveStaticFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(cacheControl, fmt.Sprintf("public,max-age=%d,s-max-age=%d,stale-while-revalidate=%d", a.cfg.Cache.Expiration, a.cfg.Cache.Expiration/3, a.cfg.Cache.Expiration))
	http.ServeFile(w, r, filepath.Join(staticFolder, r.URL.Path))
}

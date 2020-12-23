package main

import (
	"net/http"
	"os"
	"path"
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

// Gets only called by registered paths
func serveStaticFile(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, path.Join(staticFolder, r.URL.Path))
}

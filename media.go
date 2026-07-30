package main

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
)

var safeImageExtensions = []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif"}

const (
	mediaFilePath  = "data/media"
	mediaFileRoute = `/{file:[0-9a-fA-F]+(\.[0-9a-zA-Z]+)?}`
)

func (*goBlog) serveMediaFile(w http.ResponseWriter, r *http.Request) {
	f := filepath.Join(mediaFilePath, chi.URLParam(r, "file"))
	_, err := os.Stat(f) //nolint:gosec
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set(cacheControl, "public,max-age=31536000,immutable")
	if !slices.Contains(safeImageExtensions, strings.ToLower(filepath.Ext(f))) {
		w.Header().Set("Content-Disposition", "attachment")
	}
	http.ServeFile(w, r, f) //nolint:gosec
}

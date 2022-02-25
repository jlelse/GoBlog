package main

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

const (
	mediaFilePath  = "data/media"
	mediaFileRoute = `/{file:[0-9a-fA-F]+(\.[0-9a-zA-Z]+)?}`
)

func (*goBlog) serveMediaFile(w http.ResponseWriter, r *http.Request) {
	f := filepath.Join(mediaFilePath, chi.URLParam(r, "file"))
	_, err := os.Stat(f)
	if err != nil {
		// Serve 404, but don't use normal serve404 method because of media domain
		http.NotFound(w, r)
		return
	}
	w.Header().Add(cacheControl, "public,max-age=31536000,immutable")
	http.ServeFile(w, r, f)
}

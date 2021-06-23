package main

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

const mediaFilePath = "data/media"

func (a *goBlog) serveMediaFile(w http.ResponseWriter, r *http.Request) {
	f := filepath.Join(mediaFilePath, chi.URLParam(r, "file"))
	_, err := os.Stat(f)
	if err != nil {
		a.serve404(w, r)
		return
	}
	w.Header().Add("Cache-Control", "public,max-age=31536000,immutable")
	http.ServeFile(w, r, f)
}

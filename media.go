package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

const mediaFilePath = "data/media"

func saveMediaFile(filename string, mediaFile io.Reader) (string, error) {
	err := os.MkdirAll(mediaFilePath, 0644)
	if err != nil {
		return "", err
	}
	newFile, err := os.Create(filepath.Join(mediaFilePath, filename))
	if err != nil {
		return "", err
	}
	_, err = io.Copy(newFile, mediaFile)
	if err != nil {
		return "", err
	}
	return "/m/" + filename, nil
}

func serveMediaFile(w http.ResponseWriter, r *http.Request) {
	f := filepath.Join(mediaFilePath, chi.URLParam(r, "file"))
	_, err := os.Stat(f)
	if err != nil {
		serve404(w, r)
		return
	}
	w.Header().Add("Cache-Control", "public,max-age=31536000,immutable")
	http.ServeFile(w, r, f)
}

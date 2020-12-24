package main

import (
	"fmt"
	"net/http"
	"strings"
)

type errorData struct {
	Title   string
	Message string
}

func serve404(w http.ResponseWriter, r *http.Request) {
	serveError(w, r, fmt.Sprintf("%s was not found", r.RequestURI), http.StatusNotFound)
}

func serveError(w http.ResponseWriter, r *http.Request, message string, status int) {
	if !strings.Contains(strings.ToLower(r.Header.Get("Accept")), contentTypeHTML) {
		http.Error(w, message, status)
		return
	}
	title := fmt.Sprintf("%d %s", status, http.StatusText(status))
	if message == "" {
		message = http.StatusText(status)
	}
	render(w, templateError, &renderData{
		Data: &errorData{
			Title:   title,
			Message: message,
		},
	})
	w.WriteHeader(status)
}

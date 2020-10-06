package main

import (
	"fmt"
	"net/http"
)

type errorData struct {
	Blog    string
	Title   string
	Message string
}

func serve404(w http.ResponseWriter, r *http.Request) {
	render(w, templateError, &errorData{
		Blog:    appConfig.DefaultBlog,
		Title:   "404 Not Found",
		Message: fmt.Sprintf("`%s` was not found", r.RequestURI),
	})
	w.WriteHeader(http.StatusNotFound)
}

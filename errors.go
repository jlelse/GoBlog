package main

import (
	"fmt"
	"net/http"

	"github.com/elnormous/contenttype"
)

type errorData struct {
	Title   string
	Message string
}

func serve404(w http.ResponseWriter, r *http.Request) {
	serveError(w, r, fmt.Sprintf("%s was not found", r.RequestURI), http.StatusNotFound)
}

func serveNotAllowed(w http.ResponseWriter, r *http.Request) {
	serveError(w, r, "", http.StatusMethodNotAllowed)
}

var errorCheckMediaTypes = []contenttype.MediaType{
	contenttype.NewMediaType(contentTypeHTML),
}

func serveError(w http.ResponseWriter, r *http.Request, message string, status int) {
	if mt, _, err := contenttype.GetAcceptableMediaType(r, errorCheckMediaTypes); err != nil || mt.String() != errorCheckMediaTypes[0].String() {
		// Request doesn't accept HTML
		http.Error(w, message, status)
		return
	}
	title := fmt.Sprintf("%d %s", status, http.StatusText(status))
	if message == "" {
		message = http.StatusText(status)
	}
	w.WriteHeader(status)
	render(w, r, templateError, &renderData{
		Data: &errorData{
			Title:   title,
			Message: message,
		},
	})
}

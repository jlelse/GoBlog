package main

import (
	"fmt"
	"net/http"

	"git.jlel.se/jlelse/GoBlog/pkgs/contenttype"
	ct "github.com/elnormous/contenttype"
)

type errorData struct {
	Title   string
	Message string
}

func (a *goBlog) serve404(w http.ResponseWriter, r *http.Request) {
	a.serveError(w, r, fmt.Sprintf("%s was not found", r.RequestURI), http.StatusNotFound)
}

func (a *goBlog) serveNotAllowed(w http.ResponseWriter, r *http.Request) {
	a.serveError(w, r, "", http.StatusMethodNotAllowed)
}

var errorCheckMediaTypes = []ct.MediaType{
	ct.NewMediaType(contenttype.HTML),
}

func (a *goBlog) serveError(w http.ResponseWriter, r *http.Request, message string, status int) {
	if mt, _, err := ct.GetAcceptableMediaType(r, errorCheckMediaTypes); err != nil || mt.String() != errorCheckMediaTypes[0].String() {
		// Request doesn't accept HTML
		http.Error(w, message, status)
		return
	}
	title := fmt.Sprintf("%d %s", status, http.StatusText(status))
	if message == "" {
		message = http.StatusText(status)
	}
	w.WriteHeader(status)
	a.render(w, r, templateError, &renderData{
		Data: &errorData{
			Title:   title,
			Message: message,
		},
	})
}

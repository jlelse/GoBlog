package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
)

const editorPath = "/editor"

func (a *goBlog) serveEditor(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	a.render(w, r, templateEditor, &renderData{
		BlogString: blog,
		Data: map[string]interface{}{
			"Drafts": a.db.getDrafts(blog),
		},
	})
}

func (a *goBlog) serveEditorPost(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	if action := r.FormValue("editoraction"); action != "" {
		switch action {
		case "loaddelete":
			a.render(w, r, templateEditor, &renderData{
				BlogString: blog,
				Data: map[string]interface{}{
					"DeleteURL": r.FormValue("url"),
					"Drafts":    a.db.getDrafts(blog),
				},
			})
		case "loadupdate":
			parsedURL, err := url.Parse(r.FormValue("url"))
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			post, err := a.db.getPost(parsedURL.Path)
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			a.render(w, r, templateEditor, &renderData{
				BlogString: blog,
				Data: map[string]interface{}{
					"UpdatePostURL":     parsedURL.String(),
					"UpdatePostContent": a.toMfItem(post).Properties.Content[0],
					"Drafts":            a.db.getDrafts(blog),
				},
			})
		case "updatepost":
			jsonBytes, err := json.Marshal(map[string]interface{}{
				"action": actionUpdate,
				"url":    r.FormValue("url"),
				"replace": map[string][]string{
					"content": {
						r.FormValue("content"),
					},
				},
			})
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			req, err := http.NewRequest(http.MethodPost, "", bytes.NewReader(jsonBytes))
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			req.Header.Set(contentType, contentTypeJSON)
			a.editorMicropubPost(w, req, false)
		case "upload":
			a.editorMicropubPost(w, r, true)
		default:
			a.serveError(w, r, "Unknown editoraction", http.StatusBadRequest)
		}
		return
	}
	a.editorMicropubPost(w, r, false)
}

func (a *goBlog) editorMicropubPost(w http.ResponseWriter, r *http.Request, media bool) {
	recorder := httptest.NewRecorder()
	if media {
		addAllScopes(http.HandlerFunc(a.serveMicropubMedia)).ServeHTTP(recorder, r)
	} else {
		addAllScopes(http.HandlerFunc(a.serveMicropubPost)).ServeHTTP(recorder, r)
	}
	result := recorder.Result()
	if location := result.Header.Get("Location"); location != "" {
		http.Redirect(w, r, location, http.StatusFound)
		return
	}
	if result.StatusCode >= 200 && result.StatusCode <= 400 {
		http.Redirect(w, r, editorPath, http.StatusFound)
		return
	}
	w.WriteHeader(result.StatusCode)
	_, _ = io.Copy(w, result.Body)
	_ = result.Body.Close()
}

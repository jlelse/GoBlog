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

func serveEditor(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	render(w, r, templateEditor, &renderData{
		BlogString: blog,
		Data: map[string]interface{}{
			"Drafts": loadDrafts(blog),
		},
	})
}

func serveEditorPost(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	if action := r.FormValue("editoraction"); action != "" {
		switch action {
		case "loaddelete":
			render(w, r, templateEditor, &renderData{
				BlogString: blog,
				Data: map[string]interface{}{
					"DeleteURL": r.FormValue("url"),
					"Drafts":    loadDrafts(blog),
				},
			})
		case "loadupdate":
			parsedURL, err := url.Parse(r.FormValue("url"))
			if err != nil {
				serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			post, err := getPost(parsedURL.Path)
			if err != nil {
				serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			render(w, r, templateEditor, &renderData{
				BlogString: blog,
				Data: map[string]interface{}{
					"UpdatePostURL":     parsedURL.String(),
					"UpdatePostContent": post.toMfItem().Properties.Content[0],
					"Drafts":            loadDrafts(blog),
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
				serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			req, err := http.NewRequest(http.MethodPost, "", bytes.NewReader(jsonBytes))
			if err != nil {
				serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			req.Header.Set(contentType, contentTypeJSON)
			editorMicropubPost(w, req, false)
		case "upload":
			editorMicropubPost(w, r, true)
		default:
			serveError(w, r, "Unknown editoraction", http.StatusBadRequest)
		}
		return
	}
	editorMicropubPost(w, r, false)
}

func loadDrafts(blog string) []*post {
	ps, _ := getPosts(&postsRequestConfig{status: statusDraft, blog: blog})
	return ps
}

func editorMicropubPost(w http.ResponseWriter, r *http.Request, media bool) {
	recorder := httptest.NewRecorder()
	if media {
		addAllScopes(http.HandlerFunc(serveMicropubMedia)).ServeHTTP(recorder, r)
	} else {
		addAllScopes(http.HandlerFunc(serveMicropubPost)).ServeHTTP(recorder, r)
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

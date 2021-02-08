package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
)

const editorPath = "/editor"

func serveEditor(blog string) func(w http.ResponseWriter, _ *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		render(w, templateEditor, &renderData{
			BlogString: blog,
			Data: map[string]interface{}{
				"Drafts": loadDrafts(blog),
			},
		})
	}
}

func serveEditorPost(blog string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if action := r.FormValue("editoraction"); action != "" {
			switch action {
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
				mf := post.toMfItem()
				render(w, templateEditor, &renderData{
					BlogString: blog,
					Data: map[string]interface{}{
						"UpdatePostURL":     parsedURL.String(),
						"UpdatePostContent": mf.Properties.Content[0],
						"Drafts":            loadDrafts(blog),
					},
				})
			case "updatepost":
				urlValue := r.FormValue("url")
				content := r.FormValue("content")
				mf := map[string]interface{}{
					"action": actionUpdate,
					"url":    urlValue,
					"replace": map[string][]string{
						"content": {
							content,
						},
					},
				}
				jsonBytes, err := json.Marshal(mf)
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
	body, _ := ioutil.ReadAll(result.Body)
	_, _ = w.Write(body)
}

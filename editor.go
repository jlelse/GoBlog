package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
)

const editorPath = "/editor"

func serveEditor(w http.ResponseWriter, _ *http.Request) {
	render(w, templateEditor, &renderData{
		Data: map[string]interface{}{
			"Drafts": loadDrafts(),
		},
	})
}

func serveEditorPost(w http.ResponseWriter, r *http.Request) {
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
				Data: map[string]interface{}{
					"UpdatePostURL":     parsedURL.String(),
					"UpdatePostContent": mf.Properties.Content[0],
					"Drafts":            loadDrafts(),
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

func loadDrafts() []*post {
	ps, _ := getPosts(&postsRequestConfig{status: statusDraft})
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
	w.Write(body)
}

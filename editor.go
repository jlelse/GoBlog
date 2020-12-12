package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
)

const editorPath = "/editor"

func serveEditor(w http.ResponseWriter, r *http.Request) {
	render(w, templateEditor, &renderData{})
}

func serveEditorPost(w http.ResponseWriter, r *http.Request) {
	if action := r.FormValue("editoraction"); action != "" {
		if action == "loadupdate" {
			parsedURL, err := url.Parse(r.FormValue("url"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			post, err := getPost(parsedURL.Path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mf := post.toMfItem()
			render(w, templateEditor, &renderData{
				Data: map[string]interface{}{
					"UpdatePostURL":     parsedURL.String(),
					"UpdatePostContent": mf.Properties.Content[0],
				},
			})
			return
		} else if action == "updatepost" {
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
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			req, err := http.NewRequest(http.MethodPost, "", bytes.NewReader(jsonBytes))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			req.Header.Set(contentType, contentTypeJSON)
			editorMicropubPost(w, req)
			return
		}
		http.Error(w, "unknown editoraction", http.StatusBadRequest)
		return
	}
	editorMicropubPost(w, r)
}

func editorMicropubPost(w http.ResponseWriter, r *http.Request) {
	recorder := httptest.NewRecorder()
	addAllScopes(http.HandlerFunc(serveMicropubPost)).ServeHTTP(recorder, r)
	result := recorder.Result()
	if location := result.Header.Get("Location"); location != "" {
		http.Redirect(w, r, result.Header.Get("Location"), http.StatusFound)
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

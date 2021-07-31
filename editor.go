package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"go.goblog.app/app/pkgs/contenttype"
	"gopkg.in/yaml.v3"
)

const editorPath = "/editor"

func (a *goBlog) serveEditor(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogKey).(string)
	a.render(w, r, templateEditor, &renderData{
		BlogString: blog,
		Data:       map[string]interface{}{},
	})
}

func (a *goBlog) serveEditorPost(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogKey).(string)
	if action := r.FormValue("editoraction"); action != "" {
		switch action {
		case "loaddelete":
			a.render(w, r, templateEditor, &renderData{
				BlogString: blog,
				Data: map[string]interface{}{
					"DeleteURL": r.FormValue("url"),
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
					"UpdatePostContent": a.postToMfItem(post).Properties.Content[0],
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
			req.Header.Set(contentType, contenttype.JSON)
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
	if result.StatusCode >= 200 && result.StatusCode < 400 {
		http.Redirect(w, r, editorPath, http.StatusFound)
		return
	}
	w.WriteHeader(result.StatusCode)
	_, _ = io.Copy(w, result.Body)
	_ = result.Body.Close()
}

func (a *goBlog) editorPostTemplate(blog string) string {
	var builder strings.Builder
	marsh := func(param string, i interface{}) {
		_ = yaml.NewEncoder(&builder).Encode(map[string]interface{}{
			param: i,
		})
	}
	bc := a.cfg.Blogs[blog]
	builder.WriteString("---\n")
	marsh("blog", blog)
	marsh("section", bc.DefaultSection)
	marsh("status", statusDraft)
	marsh("priority", 0)
	marsh("slug", "")
	marsh("title", "")
	for _, t := range bc.Taxonomies {
		marsh(t.Name, []string{""})
	}
	builder.WriteString("---\n")
	return builder.String()
}

func (a *goBlog) editorMoreParams(blog string) string {
	var builder strings.Builder
	bc := a.cfg.Blogs[blog]
	builder.WriteString(a.ts.GetTemplateStringVariant(bc.Lang, "emptyparams"))
	builder.WriteByte(' ')
	builder.WriteString(a.ts.GetTemplateStringVariant(bc.Lang, "moreparams"))
	for i, param := range []string{
		"summary",
		"translationkey",
		"original",
		a.cfg.Micropub.AudioParam,
		a.cfg.Micropub.BookmarkParam,
		a.cfg.Micropub.LikeParam,
		a.cfg.Micropub.LikeTitleParam,
		a.cfg.Micropub.LocationParam,
		a.cfg.Micropub.PhotoParam,
		a.cfg.Micropub.PhotoDescriptionParam,
		a.cfg.Micropub.ReplyParam,
		a.cfg.Micropub.ReplyTitleParam,
	} {
		if param == "" {
			continue
		}
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteByte('`')
		builder.WriteString(param)
		builder.WriteByte('`')
	}
	builder.WriteByte('.')
	return builder.String()
}

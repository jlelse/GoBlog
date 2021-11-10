package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/microcosm-cc/bluemonday"
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

func (a *goBlog) serveEditorPreview(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogKey).(string)
	c, err := a.webSocketUpgrader().Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()
	for {
		// Retrieve content
		mt, message, err := c.ReadMessage()
		if err != nil {
			break
		}
		// Create preview
		preview, err := a.createMarkdownPreview(blog, message)
		if err != nil {
			preview = []byte(err.Error())
		}
		// Write preview to socket
		err = c.WriteMessage(mt, preview)
		if err != nil {
			break
		}
	}
}

func (a *goBlog) createMarkdownPreview(blog string, markdown []byte) (rendered []byte, err error) {
	p := post{
		Content:   string(markdown),
		Blog:      blog,
		Path:      "/editor/preview",
		Published: localNowString(),
	}
	err = a.computeExtraPostParameters(&p)
	if err != nil {
		return nil, err
	}
	if t := p.Title(); t != "" {
		p.RenderedTitle = a.renderMdTitle(t)
	}
	// Render post
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/editor/preview", nil)
	a.render(rec, req, templateEditorPreview, &renderData{
		BlogString: p.Blog,
		Data:       &p,
	})
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("failed to render preview")
	}
	defer res.Body.Close()
	rendered, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	// Sanitize HTML
	rendered = bluemonday.UGCPolicy().SanitizeBytes(rendered)
	return
}

func (a *goBlog) serveEditorPost(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogKey).(string)
	if action := r.FormValue("editoraction"); action != "" {
		switch action {
		case "loadupdate":
			post, err := a.getPost(r.FormValue("path"))
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			a.render(w, r, templateEditor, &renderData{
				BlogString: blog,
				Data: map[string]interface{}{
					"UpdatePostURL":     a.fullPostURL(post),
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
		case "tts":
			parsedURL, err := url.Parse(r.FormValue("url"))
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			post, err := a.getPost(parsedURL.Path)
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			if err = a.createPostTTSAudio(post); err != nil {
				a.serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, post.Path, http.StatusFound)
		case "helpgpx":
			file, _, err := r.FormFile("file")
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			gpx, err := io.ReadAll(file)
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			var gpxBuffer bytes.Buffer
			_, _ = a.min.Write(&gpxBuffer, contenttype.XML, gpx)
			resultMap := map[string]string{
				"gpx": gpxBuffer.String(),
			}
			resultBytes, err := yaml.Marshal(resultMap)
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set(contentType, contenttype.TextUTF8)
			_, _ = w.Write(resultBytes)
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

func (a *goBlog) editorPostDesc(blog string) string {
	bc := a.cfg.Blogs[blog]
	t := a.ts.GetTemplateStringVariant(bc.Lang, "editorpostdesc")
	var paramBuilder, statusBuilder strings.Builder
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
			paramBuilder.WriteString(", ")
		}
		paramBuilder.WriteByte('`')
		paramBuilder.WriteString(param)
		paramBuilder.WriteByte('`')
	}
	for i, status := range []postStatus{
		statusDraft, statusPublished, statusUnlisted, statusPrivate,
	} {
		if i > 0 {
			statusBuilder.WriteString(", ")
		}
		statusBuilder.WriteByte('`')
		statusBuilder.WriteString(string(status))
		statusBuilder.WriteByte('`')
	}
	return fmt.Sprintf(t, paramBuilder.String(), "status", statusBuilder.String())
}

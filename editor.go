package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/carlmjohnson/requests"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"gopkg.in/yaml.v3"
	ws "nhooyr.io/websocket"
)

const editorPath = "/editor"

func (a *goBlog) serveEditor(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, a.renderEditor, &renderData{
		Data: &editorRenderData{
			presetParams: parsePresetPostParamsFromQuery(r),
		},
	})
}

func (a *goBlog) serveEditorPreview(w http.ResponseWriter, r *http.Request) {
	blog, _ := a.getBlog(r)
	c, err := ws.Accept(w, r, &ws.AcceptOptions{CompressionMode: ws.CompressionContextTakeover})
	if err != nil {
		return
	}
	c.SetReadLimit(1 << 20) // 1MB
	defer c.Close(ws.StatusNormalClosure, "")
	ctx, cancel := context.WithTimeout(r.Context(), time.Hour*6)
	defer cancel()
	for {
		// Retrieve content
		mt, message, err := c.Reader(ctx)
		if err != nil {
			break
		}
		if mt != ws.MessageText {
			continue
		}
		// Create preview
		w, err := c.Writer(ctx, ws.MessageText)
		if err != nil {
			break
		}
		a.createMarkdownPreview(w, blog, message)
		if err = w.Close(); err != nil {
			break
		}
	}
}

func (a *goBlog) createMarkdownPreview(w io.Writer, blog string, markdown io.Reader) {
	md, err := io.ReadAll(markdown)
	if err != nil {
		_, _ = io.WriteString(w, err.Error())
		return
	}
	p := &post{
		Blog:    blog,
		Content: string(md),
	}
	if err = a.extractParamsFromContent(p); err != nil {
		_, _ = io.WriteString(w, err.Error())
		return
	}
	if err := a.checkPost(p, true); err != nil {
		_, _ = io.WriteString(w, err.Error())
		return
	}
	if t := p.Title(); t != "" {
		p.RenderedTitle = a.renderMdTitle(t)
	}
	// Render post (using post's blog config)
	hb := htmlbuilder.NewHtmlBuilder(w)
	a.renderEditorPreview(hb, a.getBlogFromPost(p), p)
}

func (a *goBlog) serveEditorPost(w http.ResponseWriter, r *http.Request) {
	switch action := r.FormValue("editoraction"); action {
	case "loadupdate":
		post, err := a.getPost(r.FormValue("path"))
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		a.render(w, r, a.renderEditor, &renderData{
			Data: &editorRenderData{
				presetParams:      parsePresetPostParamsFromQuery(r),
				updatePostUrl:     a.fullPostURL(post),
				updatePostContent: a.postToMfItem(post).Properties.Content[0],
			},
		})
	case "createpost", "updatepost":
		reqBody := map[string]any{}
		if action == "updatepost" {
			reqBody["action"] = actionUpdate
			reqBody["url"] = r.FormValue("url")
			reqBody["replace"] = map[string][]string{"content": {r.FormValue("content")}}
		} else {
			reqBody["type"] = []string{"h-entry"}
			reqBody["properties"] = map[string][]string{"content": {r.FormValue("content")}}
		}
		req, _ := requests.URL("").BodyJSON(reqBody).Request(r.Context())
		a.editorMicropubPost(w, req, false)
	case "upload":
		a.editorMicropubPost(w, r, true)
	case "delete", "undelete":
		req, _ := requests.URL("").Method(http.MethodPost).ContentType(contenttype.WWWForm).Param("action", action).Param("url", r.FormValue("url")).Request(r.Context())
		a.editorMicropubPost(w, req, false)
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
		gpx, err := io.ReadAll(a.min.Get().Reader(contenttype.XML, file))
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set(contentType, contenttype.TextUTF8)
		_ = yaml.NewEncoder(w).Encode(map[string]string{
			"gpx": string(gpx),
		})
	default:
		a.serveError(w, r, "Unknown or missing editoraction", http.StatusBadRequest)
	}
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

func (*goBlog) editorPostTemplate(blog string, bc *configBlog, presetParams map[string][]string) string {
	builder := bufferpool.Get()
	defer bufferpool.Put(builder)
	marsh := func(param string, preset bool, i any) {
		if _, presetPresent := presetParams[param]; !preset && presetPresent {
			return
		}
		_ = yaml.NewEncoder(builder).Encode(map[string]any{
			param: i,
		})
	}
	builder.WriteString("---\n")
	marsh("blog", false, blog)
	marsh("section", false, bc.DefaultSection)
	marsh("status", false, statusDraft)
	marsh("visibility", false, visibilityPublic)
	marsh("priority", false, 0)
	marsh("slug", false, "")
	marsh("title", false, "")
	for _, t := range bc.Taxonomies {
		marsh(t.Name, false, []string{""})
	}
	for key, param := range presetParams {
		marsh(key, true, param)
	}
	builder.WriteString("---\n")
	return builder.String()
}

func (a *goBlog) editorPostDesc(bc *configBlog) string {
	t := a.ts.GetTemplateStringVariant(bc.Lang, "editorpostdesc")
	paramBuilder, statusBuilder, visibilityBuilder := bufferpool.Get(), bufferpool.Get(), bufferpool.Get()
	defer bufferpool.Put(paramBuilder, statusBuilder, visibilityBuilder)
	for i, param := range []string{
		"published",
		"updated",
		"summary",
		"translationkey",
		"original",
		a.cfg.Micropub.AudioParam,
		a.cfg.Micropub.BookmarkParam,
		a.cfg.Micropub.LikeParam,
		a.cfg.Micropub.LikeTitleParam,
		a.cfg.Micropub.LikeContextParam,
		a.cfg.Micropub.LocationParam,
		a.cfg.Micropub.PhotoParam,
		a.cfg.Micropub.PhotoDescriptionParam,
		a.cfg.Micropub.ReplyParam,
		a.cfg.Micropub.ReplyTitleParam,
		a.cfg.Micropub.ReplyContextParam,
		gpxParameter,
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
		statusPublished, statusDraft, statusScheduled,
	} {
		if i > 0 {
			statusBuilder.WriteString(", ")
		}
		statusBuilder.WriteByte('`')
		statusBuilder.WriteString(string(status))
		statusBuilder.WriteByte('`')
	}
	for i, visibility := range []postVisibility{
		visibilityPublic, visibilityUnlisted, visibilityPrivate,
	} {
		if i > 0 {
			visibilityBuilder.WriteString(", ")
		}
		visibilityBuilder.WriteByte('`')
		visibilityBuilder.WriteString(string(visibility))
		visibilityBuilder.WriteByte('`')
	}
	return fmt.Sprintf(t, paramBuilder.String(), "status", "visibility", statusBuilder.String(), visibilityBuilder.String())
}

func parsePresetPostParamsFromQuery(r *http.Request) map[string][]string {
	m := map[string][]string{}
	for key, param := range r.URL.Query() {
		if strings.HasPrefix(key, "p:") {
			paramKey := strings.TrimPrefix(key, "p:")
			m[paramKey] = append(m[paramKey], param...)
		}
	}
	return m
}

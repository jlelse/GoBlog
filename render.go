package main

import (
	"io"
	"net/http"

	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

type renderData struct {
	BlogString                 string
	Canonical                  string
	TorAddress                 string
	Blog                       *configBlog
	User                       *configUser
	Data                       any
	WebmentionReceivingEnabled bool
	TorUsed                    bool
	EasterEgg                  bool
	// Not directly accessible
	app *goBlog
	req *http.Request
}

func (d *renderData) LoggedIn() bool {
	return d.app.isLoggedIn(d.req)
}

func (a *goBlog) render(w http.ResponseWriter, r *http.Request, f func(*htmlbuilder.HtmlBuilder, *renderData), data *renderData) {
	a.renderWithStatusCode(w, r, http.StatusOK, f, data)
}

func (a *goBlog) renderWithStatusCode(w http.ResponseWriter, r *http.Request, statusCode int, f func(*htmlbuilder.HtmlBuilder, *renderData), data *renderData) {
	// Check render data
	a.checkRenderData(r, data)
	// Set content type
	w.Header().Set(contentType, contenttype.HTMLUTF8)
	// Write status code
	w.WriteHeader(statusCode)
	// Render
	renderPipeReader, renderPipeWriter := io.Pipe()
	go func() {
		f(htmlbuilder.NewHtmlBuilder(renderPipeWriter), data)
		renderPipeWriter.Close()
	}()
	// Run UI plugins
	pluginPipeReader, pluginPipeWriter := io.Pipe()
	go func() {
		a.chainUiPlugins(a.getPlugins(pluginUiType), &pluginRenderContext{
			blog: data.BlogString,
			path: r.URL.Path,
		}, renderPipeReader, pluginPipeWriter)
		pluginPipeWriter.Close()
	}()
	// Return minified HTML
	_ = pluginPipeReader.CloseWithError(a.min.Get().Minify(contenttype.HTML, w, pluginPipeReader))
}

func (a *goBlog) chainUiPlugins(plugins []any, rc *pluginRenderContext, rendered io.Reader, modified io.Writer) {
	if len(plugins) == 0 {
		_, _ = io.Copy(modified, rendered)
		return
	}
	reader, writer := io.Pipe()
	go func() {
		plugins[0].(plugintypes.UI).Render(rc, rendered, writer)
		_ = writer.Close()
	}()
	a.chainUiPlugins(plugins[1:], rc, reader, modified)
}

func (a *goBlog) checkRenderData(r *http.Request, data *renderData) {
	if data.app == nil {
		data.app = a
	}
	if data.req == nil {
		data.req = r
	}
	// User
	if data.User == nil {
		data.User = a.cfg.User
	}
	// Blog
	if data.Blog == nil && data.BlogString == "" {
		data.BlogString, data.Blog = a.getBlog(r)
	} else if data.Blog == nil {
		data.Blog = a.cfg.Blogs[data.BlogString]
	} else if data.BlogString == "" {
		for name, blog := range a.cfg.Blogs {
			if blog == data.Blog {
				data.BlogString = name
				break
			}
		}
	}
	// Tor
	if a.cfg.Server.Tor && a.torAddress != "" {
		data.TorAddress = a.torAddress + r.RequestURI
	}
	if torUsed, ok := r.Context().Value(torUsedKey).(bool); ok && torUsed {
		data.TorUsed = true
	}
	// Check if able to receive webmentions
	data.WebmentionReceivingEnabled = a.cfg.Webmention == nil || !a.cfg.Webmention.DisableReceiving
	// Easter egg
	if ee := a.cfg.EasterEgg; ee != nil && ee.Enabled {
		data.EasterEgg = true
	}
	// Data
	if data.Data == nil {
		data.Data = map[string]any{}
	}
}

// Plugins

type pluginRenderContext struct {
	blog string
	path string
}

func (d *pluginRenderContext) GetBlog() string {
	return d.blog
}

func (d *pluginRenderContext) GetPath() string {
	return d.path
}

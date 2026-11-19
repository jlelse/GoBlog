package main

import (
	"bytes"
	"io"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/minify"
	"go.goblog.app/app/pkgs/plugintypes"
)

type renderData struct {
	BlogString                 string
	Canonical                  string
	Description                string
	TorAddress                 string
	Blog                       *configBlog
	User                       *configUser
	Data                       any
	WebmentionReceivingEnabled bool
	TorUsed                    bool
	EasterEgg                  bool
	IsHome                     bool
	// For plugins
	prc *pluginRenderContext
	// Not directly accessible
	app *goBlog
	req *http.Request
}

func (d *renderData) LoggedIn() bool {
	return d.app != nil && d.app.isLoggedIn(d.req)
}

func (a *goBlog) render(w http.ResponseWriter, r *http.Request, f func(*htmlbuilder.HTMLBuilder, *renderData), data *renderData) {
	a.renderWithStatusCode(w, r, http.StatusOK, f, data)
}

func (a *goBlog) renderWithStatusCode(w http.ResponseWriter, r *http.Request, statusCode int, f func(*htmlbuilder.HTMLBuilder, *renderData), data *renderData) {
	// Check render data
	a.checkRenderData(r, data)
	// Set content type
	w.Header().Set(contentType, contenttype.HTMLUTF8)
	// Write status code
	w.WriteHeader(statusCode)
	// Render into a buffer so UI2 plugins and minification run synchronously
	// without pipes or goroutines.
	rendered := bufferpool.Get()
	defer func() {
		bufferpool.Put(rendered)
	}()
	func() {
		defer func() {
			if r := recover(); r != nil {
				a.error("HTML rendering panicked", "err", r)
			}
		}()
		hb, finish := a.wrapForPlugins(
			rendered,
			pluginUI2Type,
			a.getPlugins(pluginUI2Type),
			func(plugin any, doc *goquery.Document) {
				plugin.(plugintypes.UI2).RenderWithDocument(data.prc, doc)
			},
			"",
		)
		defer finish()
		f(hb, data)
	}()
	// Run io based UI plugins
	if uiPlugins := a.getPlugins(pluginUIType); len(uiPlugins) > 0 {
		pluginRendered := bufferpool.Get()
		a.chainUIPlugins(uiPlugins, data.prc, bytes.NewReader(rendered.Bytes()), pluginRendered)
		bufferpool.Put(rendered)
		rendered = pluginRendered
	}
	// Return minified HTML
	_ = minify.Get().Minify(contenttype.HTML, w, rendered)
}

// chainUIPlugins runs the UI plugins sequentially over buffers. Each plugin
// consumes the previous plugin's output, so no pipe or goroutine is needed per
// plugin.
func (a *goBlog) chainUIPlugins(plugins []any, rc *pluginRenderContext, rendered io.Reader, modified io.Writer) {
	current := rendered
	buffers := make([]*bytes.Buffer, 0, len(plugins))
	defer func() {
		bufferpool.Put(buffers...)
	}()
	for i, plugin := range plugins {
		buf := bufferpool.Get()
		buffers = append(buffers, buf)
		func() {
			defer func() {
				if r := recover(); r != nil {
					a.logPluginPanic(pluginUIType, i, r)
					// Pass the remaining input through so the rest of the page is not lost.
					_, _ = io.Copy(buf, current)
				}
			}()
			plugin.(plugintypes.UI).Render(rc, current, buf)
		}()
		current = bytes.NewReader(buf.Bytes())
	}
	_, _ = io.Copy(modified, current)
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
	// Plugins
	if data.prc == nil {
		data.prc = &pluginRenderContext{
			blog:     data.BlogString,
			path:     r.URL.Path,
			url:      a.getFullAddress(r.URL.Path),
			loggedIn: a.isLoggedIn(r),
		}
	}
	// Data
	if data.Data == nil {
		data.Data = map[string]any{}
	}
}

// Plugins

type pluginRenderContext struct {
	blog     string
	path     string
	url      string
	loggedIn bool
}

func (d *pluginRenderContext) GetBlog() string {
	return d.blog
}

func (d *pluginRenderContext) GetPath() string {
	return d.path
}

func (d *pluginRenderContext) GetURL() string {
	return d.url
}

func (d *pluginRenderContext) IsLoggedIn() bool {
	return d.loggedIn
}

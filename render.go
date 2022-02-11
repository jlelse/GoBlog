package main

import (
	"io"
	"net/http"

	"go.goblog.app/app/pkgs/contenttype"
)

type renderData struct {
	BlogString                 string
	Canonical                  string
	TorAddress                 string
	Blog                       *configBlog
	User                       *configUser
	Data                       interface{}
	CommentsEnabled            bool
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

func (a *goBlog) render(w http.ResponseWriter, r *http.Request, f func(*htmlBuilder, *renderData), data *renderData) {
	a.renderWithStatusCode(w, r, http.StatusOK, f, data)
}

func (a *goBlog) renderWithStatusCode(w http.ResponseWriter, r *http.Request, statusCode int, f func(*htmlBuilder, *renderData), data *renderData) {
	// Check render data
	a.checkRenderData(r, data)
	// Set content type
	w.Header().Set(contentType, contenttype.HTMLUTF8)
	// Write status code
	w.WriteHeader(statusCode)
	// Render
	pipeReader, pipeWriter := io.Pipe()
	go func() {
		mw := a.min.Writer(contenttype.HTML, pipeWriter)
		f(newHtmlBuilder(mw), data)
		_ = mw.Close()
		_ = pipeWriter.Close()
	}()
	_, _ = io.Copy(w, pipeReader)
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
	// Check if comments enabled
	data.CommentsEnabled = data.Blog.Comments != nil && data.Blog.Comments.Enabled
	// Check if able to receive webmentions
	data.WebmentionReceivingEnabled = a.cfg.Webmention == nil || !a.cfg.Webmention.DisableReceiving
	// Easter egg
	if ee := a.cfg.EasterEgg; ee != nil && ee.Enabled {
		data.EasterEgg = true
	}
	// Data
	if data.Data == nil {
		data.Data = map[string]interface{}{}
	}
}

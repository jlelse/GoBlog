package main

import (
	"html/template"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

const (
	templatesDir = "templates"
	templatesExt = ".gohtml"

	templateBase               = "base"
	templateEditor             = "editor"
	templateEditorFiles        = "editorfiles"
	templateCommentsAdmin      = "commentsadmin"
	templateNotificationsAdmin = "notificationsadmin"
	templateWebmentionAdmin    = "webmentionadmin"
	templateIndieAuth          = "indieauth"
)

func (a *goBlog) initRendering() error {
	a.templates = map[string]*template.Template{}
	templateFunctions := template.FuncMap{
		"md":      a.safeRenderMarkdownAsHTML,
		"mdtitle": a.renderMdTitle,
		"html":    wrapStringAsHTML,
		// Code based rendering
		"tor": func(rd *renderData) template.HTML {
			buf := bufferpool.Get()
			hb := newHtmlBuilder(buf)
			a.renderTorNotice(hb, rd)
			res := template.HTML(buf.String())
			bufferpool.Put(buf)
			return res
		},
		// Others
		"dateformat":     dateFormat,
		"isodate":        isoDateFormat,
		"unixtodate":     unixToLocalDateString,
		"now":            localNowString,
		"asset":          a.assetFileName,
		"string":         a.ts.GetTemplateStringVariantFunc(),
		"absolute":       a.getFullAddress,
		"opensearch":     openSearchUrl,
		"mbytes":         mBytesString,
		"editortemplate": a.editorPostTemplate,
		"editorpostdesc": a.editorPostDesc,
	}
	baseTemplate, err := template.New("base").Funcs(templateFunctions).ParseFiles(path.Join(templatesDir, templateBase+templatesExt))
	if err != nil {
		return err
	}
	err = filepath.Walk(templatesDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() && path.Ext(p) == templatesExt {
			if name := strings.TrimSuffix(path.Base(p), templatesExt); name != templateBase {
				if a.templates[name], err = template.Must(baseTemplate.Clone()).New(name).ParseFiles(p); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

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

func (a *goBlog) render(w http.ResponseWriter, r *http.Request, template string, data *renderData) {
	a.renderWithStatusCode(w, r, http.StatusOK, template, data)
}

func (a *goBlog) renderWithStatusCode(w http.ResponseWriter, r *http.Request, statusCode int, template string, data *renderData) {
	// Check render data
	a.checkRenderData(r, data)
	// Set content type
	w.Header().Set(contentType, contenttype.HTMLUTF8)
	// Render template and write minified HTML
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	if err := a.templates[template].ExecuteTemplate(buf, template, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(statusCode)
	_ = a.min.Minify(contenttype.HTML, w, buf)
}

func (a *goBlog) renderNew(w http.ResponseWriter, r *http.Request, f func(*htmlBuilder, *renderData), data *renderData) {
	a.renderNewWithStatusCode(w, r, http.StatusOK, f, data)
}

func (a *goBlog) renderNewWithStatusCode(w http.ResponseWriter, r *http.Request, statusCode int, f func(*htmlBuilder, *renderData), data *renderData) {
	// Check render data
	a.checkRenderData(r, data)
	// Set content type
	w.Header().Set(contentType, contenttype.HTMLUTF8)
	// Write status code
	w.WriteHeader(statusCode)
	// Render
	buf := bufferpool.Get()
	minWriter := a.min.Get().Writer(contenttype.HTML, buf)
	hb := newHtmlBuilder(minWriter)
	f(hb, data)
	_ = minWriter.Close()
	_, _ = buf.WriteTo(w)
	bufferpool.Put(buf)
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

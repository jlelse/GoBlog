package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	servertiming "github.com/mitchellh/go-server-timing"
)

const (
	templatesDir = "templates"
	templatesExt = ".gohtml"

	templateBase               = "base"
	templatePost               = "post"
	templateError              = "error"
	templateIndex              = "index"
	templateTaxonomy           = "taxonomy"
	templateSearch             = "search"
	templateSummary            = "summary"
	templatePhotosSummary      = "photosummary"
	templateEditor             = "editor"
	templateLogin              = "login"
	templateStaticHome         = "statichome"
	templateBlogStats          = "blogstats"
	templateBlogStatsTable     = "blogstatstable"
	templateComment            = "comment"
	templateCaptcha            = "captcha"
	templateCommentsAdmin      = "commentsadmin"
	templateNotificationsAdmin = "notificationsadmin"
	templateWebmentionAdmin    = "webmentionadmin"
	templateBlogroll           = "blogroll"
)

var templates map[string]*template.Template = map[string]*template.Template{}

func initRendering() error {
	templateFunctions := template.FuncMap{
		"menu": func(blog *configBlog, id string) *menu {
			return blog.Menus[id]
		},
		"user": func() *configUser {
			return appConfig.User
		},
		"md": func(content string) template.HTML {
			htmlContent, err := renderMarkdown(content, false)
			if err != nil {
				log.Fatal(err)
				return ""
			}
			return template.HTML(htmlContent)
		},
		"html": func(s string) template.HTML {
			return template.HTML(s)
		},
		// Post specific
		"p": func(p *post, parameter string) string {
			return p.firstParameter(parameter)
		},
		"ps": func(p *post, parameter string) []string {
			return p.Parameters[parameter]
		},
		"hasp": func(p *post, parameter string) bool {
			return len(p.Parameters[parameter]) > 0
		},
		"title": func(p *post) string {
			return p.title()
		},
		"content": func(p *post) template.HTML {
			return p.html()
		},
		"summary": func(p *post) string {
			return p.summary()
		},
		"translations": func(p *post) []*post {
			return p.translations()
		},
		"shorturl": func(p *post) string {
			return p.shortURL()
		},
		// Others
		"dateformat": dateFormat,
		"isodate": func(date string) string {
			return dateFormat(date, "2006-01-02")
		},
		"unixtodate": func(unix int64) string {
			return time.Unix(unix, 0).Local().String()
		},
		"now": func() string {
			return time.Now().Local().String()
		},
		"dateadd": func(date string, years, months, days int) string {
			d, err := dateparse.ParseLocal(date)
			if err != nil {
				return ""
			}
			return d.AddDate(years, months, days).Local().String()
		},
		"datebefore": func(date string, before string) bool {
			d, err := dateparse.ParseLocal(date)
			if err != nil {
				return false
			}
			b, err := dateparse.ParseLocal(before)
			if err != nil {
				return false
			}
			return d.Before(b)
		},
		"asset":    assetFileName,
		"assetsri": assetSRI,
		"string":   appTs.GetTemplateStringVariantFunc(),
		"include": func(templateName string, data ...interface{}) (template.HTML, error) {
			if len(data) == 0 || len(data) > 2 {
				return "", errors.New("wrong argument count")
			}
			if rd, ok := data[0].(*renderData); ok {
				if len(data) == 2 {
					nrd := *rd
					nrd.Data = data[1]
					rd = &nrd
				}
				var buf bytes.Buffer
				err := templates[templateName].ExecuteTemplate(&buf, templateName, rd)
				return template.HTML(buf.String()), err
			}
			return "", errors.New("wrong arguments")
		},
		"urlize": urlize,
		"sort":   sortedStrings,
		"absolute": func(path string) string {
			return appConfig.Server.PublicAddress + path
		},
		"blogrelative": func(blog *configBlog, path string) string {
			return blog.getRelativePath(path)
		},
		"jsonFile": func(filename string) *map[string]interface{} {
			parsed := &map[string]interface{}{}
			content, err := os.ReadFile(filename)
			if err != nil {
				return nil
			}
			err = json.Unmarshal(content, parsed)
			if err != nil {
				fmt.Println(err.Error())
				return nil
			}
			return parsed
		},
		"mentions": func(absolute string) []*mention {
			mentions, _ := getWebmentions(&webmentionsRequestConfig{
				target: absolute,
				status: webmentionStatusApproved,
				asc:    true,
			})
			return mentions
		},
		"urlToString": func(u url.URL) string {
			return u.String()
		},
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
				if templates[name], err = template.Must(baseTemplate.Clone()).New(name).ParseFiles(p); err != nil {
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
	Data                       interface{}
	LoggedIn                   bool
	CommentsEnabled            bool
	WebmentionReceivingEnabled bool
	TorUsed                    bool
}

func render(w http.ResponseWriter, r *http.Request, template string, data *renderData) {
	// Server timing
	t := servertiming.FromContext(r.Context()).NewMetric("r").Start()
	// Check render data
	if data.Blog == nil {
		if len(data.BlogString) == 0 {
			data.BlogString = appConfig.DefaultBlog
		}
		data.Blog = appConfig.Blogs[data.BlogString]
	}
	if data.BlogString == "" {
		for s, b := range appConfig.Blogs {
			if b == data.Blog {
				data.BlogString = s
				break
			}
		}
	}
	if appConfig.Server.Tor && torAddress != "" {
		data.TorAddress = fmt.Sprintf("http://%v%v", torAddress, r.RequestURI)
	}
	if data.Data == nil {
		data.Data = map[string]interface{}{}
	}
	// Check login
	if loggedIn, ok := r.Context().Value(loggedInKey).(bool); ok && loggedIn {
		data.LoggedIn = true
	}
	// Check if comments enabled
	data.CommentsEnabled = data.Blog.Comments != nil && data.Blog.Comments.Enabled
	// Check if able to receive webmentions
	data.WebmentionReceivingEnabled = appConfig.Webmention == nil || !appConfig.Webmention.DisableReceiving
	// Check if Tor request
	if torUsed, ok := r.Context().Value(torUsedKey).(bool); ok && torUsed {
		data.TorUsed = true
	}
	// Minify and write response
	mw := minifier.Writer(contentTypeHTML, w)
	defer func() {
		_ = mw.Close()
	}()
	err := templates[template].ExecuteTemplate(mw, template, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Set content type
	w.Header().Set(contentType, contentTypeHTMLUTF8)
	// Server timing
	t.Stop()
}

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
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
	templateComment            = "comment"
	templateCaptcha            = "captcha"
	templateCommentsAdmin      = "commentsadmin"
	templateNotificationsAdmin = "notificationsadmin"
	templateWebmentionAdmin    = "webmentionadmin"
)

var templates map[string]*template.Template
var templateFunctions template.FuncMap

func initRendering() error {
	templateFunctions = template.FuncMap{
		"blog": func(blog string) *configBlog {
			return appConfig.Blogs[blog]
		},
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
		"asset":  assetFileName,
		"string": getTemplateStringVariant,
		"include": func(templateName string, data ...interface{}) (template.HTML, error) {
			if len(data) == 1 {
				if rd, ok := data[0].(*renderData); ok {
					buf := new(bytes.Buffer)
					err := templates[templateName].ExecuteTemplate(buf, templateName, rd)
					return template.HTML(buf.String()), err
				}
				return "", errors.New("wrong argument")
			} else if len(data) == 2 {
				if blog, ok := data[0].(*configBlog); ok {
					buf := new(bytes.Buffer)
					err := templates[templateName].ExecuteTemplate(buf, templateName, &renderData{
						Blog: blog,
						Data: data[1],
					})
					return template.HTML(buf.String()), err
				}
				return "", errors.New("wrong arguments")
			}
			return "", errors.New("wrong argument count")
		},
		"default": func(dflt interface{}, given ...interface{}) interface{} {
			if len(given) == 0 {
				return dflt
			}
			g := reflect.ValueOf(given[0])
			if !g.IsValid() {
				return dflt
			}
			set := false
			switch g.Kind() {
			case reflect.Bool:
				set = true
			case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
				set = g.Len() != 0
			case reflect.Int:
				set = g.Int() != 0
			default:
				set = !g.IsNil()
			}
			if set {
				return given[0]
			}
			return dflt
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
		"commentsenabled": func(blog *configBlog) bool {
			return blog.Comments != nil && blog.Comments.Enabled
		},
		"mentions": func(absolute string) []*mention {
			mentions, _ := getWebmentions(&webmentionsRequestConfig{
				target: absolute,
				status: webmentionStatusApproved,
				asc:    true,
			})
			return mentions
		},
	}

	templates = map[string]*template.Template{}

	baseTemplatePath := path.Join(templatesDir, templateBase+templatesExt)
	if err := filepath.Walk(templatesDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() && path.Ext(p) == templatesExt {
			if name := strings.TrimSuffix(path.Base(p), templatesExt); name != templateBase {
				if templates[name], err = template.New(name).Funcs(templateFunctions).ParseFiles(baseTemplatePath, p); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

type renderData struct {
	BlogString string
	Canonical  string
	Blog       *configBlog
	Data       interface{}
	LoggedIn   bool
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
	if data.Data == nil {
		data.Data = map[string]interface{}{}
	}
	// Check login
	if loggedIn, ok := r.Context().Value(loggedInKey).(bool); ok && loggedIn {
		data.LoggedIn = true
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

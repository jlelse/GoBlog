package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/goodsign/monday"
)

const templatesDir = "templates"
const templatesExt = ".gohtml"

const templateBase = "base"
const templatePost = "post"
const templateError = "error"
const templateIndex = "index"
const templateTaxonomy = "taxonomy"
const templatePhotos = "photos"

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
			htmlContent, err := renderMarkdown(content)
			if err != nil {
				log.Fatal(err)
				return ""
			}
			return template.HTML(htmlContent)
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
		"postmentions": func(p *post) []*mention {
			mentions, _ := getWebmentions(&webmentionsRequestConfig{
				target: appConfig.Server.PublicAddress + p.Path,
				status: webmentionStatusApproved,
				asc:    true,
			})
			return mentions
		},
		// Others
		"dateformat": func(date string, format string) string {
			d, err := dateparse.ParseIn(date, time.Local)
			if err != nil {
				return ""
			}
			return d.Format(format)
		},
		"longdate": func(date string, localeString string) string {
			d, err := dateparse.ParseIn(date, time.Local)
			if err != nil {
				return ""
			}
			ml := monday.Locale(localeString)
			return monday.Format(d, monday.LongFormatsByLocale[ml], ml)
		},
		"unixtodate": func(unix int64) string {
			return time.Unix(unix, 0).String()
		},
		"now": func() string {
			return time.Now().String()
		},
		"dateadd": func(date string, years, months, days int) string {
			d, err := dateparse.ParseIn(date, time.Local)
			if err != nil {
				return ""
			}
			return d.AddDate(years, months, days).String()
		},
		"datebefore": func(date string, before string) bool {
			d, err := dateparse.ParseIn(date, time.Local)
			if err != nil {
				return false
			}
			b, err := dateparse.ParseIn(before, time.Local)
			if err != nil {
				return false
			}
			return d.Before(b)
		},
		"asset":  assetFile,
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
			content, err := ioutil.ReadFile(filename)
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
	}

	templates = make(map[string]*template.Template)

	baseTemplatePath := path.Join(templatesDir, templateBase+templatesExt)
	err := filepath.Walk(templatesDir, func(p string, info os.FileInfo, err error) error {
		if info.Mode().IsRegular() && path.Ext(p) == templatesExt {
			name := strings.TrimSuffix(path.Base(p), templatesExt)
			if name != templateBase {
				templates[name], err = template.New(name).Funcs(templateFunctions).ParseFiles(baseTemplatePath, p)
				if err != nil {
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
	blogString string
	Canonical  string
	Blog       *configBlog
	Data       interface{}
}

func render(w http.ResponseWriter, template string, data *renderData) {
	// Check render data
	if data.Blog == nil {
		if len(data.blogString) == 0 {
			data.blogString = appConfig.DefaultBlog
		}
		data.Blog = appConfig.Blogs[data.blogString]
	}
	// We need to use a buffer here to enable minification
	var buffer bytes.Buffer
	err := templates[template].ExecuteTemplate(&buffer, template, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	// Set content type (needed for minification middleware
	w.Header().Set(contentType, contentTypeHTMLUTF8)
	// Write buffered response
	_, _ = w.Write(buffer.Bytes())
}

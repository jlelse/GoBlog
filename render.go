package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
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
const templateRedirect = "redirect"
const templateIndex = "index"
const templateTaxonomy = "taxonomy"
const templatePhotos = "photos"

var templates map[string]*template.Template
var templateFunctions template.FuncMap

func initRendering() error {
	templateFunctions = template.FuncMap{
		"menu": func(blog *configBlog, id string) *menu {
			return blog.Menus[id]
		},
		"md": func(content string) template.HTML {
			htmlContent, err := renderMarkdown(content)
			if err != nil {
				log.Fatal(err)
				return ""
			}
			return template.HTML(htmlContent)
		},
		// First parameter value
		"p": func(post *Post, parameter string) string {
			return post.firstParameter(parameter)
		},
		// All parameter values
		"ps": func(post *Post, parameter string) []string {
			return post.Parameters[parameter]
		},
		"title": func(post *Post) string {
			return post.title()
		},
		"summary": func(post *Post) string {
			return post.summary()
		},
		"dateformat": func(date string, format string) string {
			d, err := dateparse.ParseIn(date, time.Local)
			if err != nil {
				return ""
			}
			return d.Format(format)
		},
		"longDate": func(date string, localeString string) string {
			d, err := dateparse.ParseIn(date, time.Local)
			if err != nil {
				return ""
			}
			ml := monday.Locale(localeString)
			return monday.Format(d, monday.LongFormatsByLocale[ml], ml)
		},
		"asset":  assetFile,
		"string": getTemplateStringVariant,
		"include": func(templateName string, blog *configBlog, data interface{}) (template.HTML, error) {
			buf := new(bytes.Buffer)
			err := templates[templateName].ExecuteTemplate(buf, templateName, &renderData{
				Blog: blog,
				Data: data,
			})
			return template.HTML(buf.String()), err
		},
		"urlize": urlize,
		"sort":   sortedStrings,
		"blogRelative": func(blog *configBlog, path string) string {
			if blog.Path != "/" {
				return blog.Path + path
			}
			return path
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

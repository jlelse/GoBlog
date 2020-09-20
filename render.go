package main

import (
	"bytes"
	"github.com/araddon/dateparse"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const templatesDir = "templates"
const templatesExt = ".gohtml"

const templateBase = "base"
const templatePost = "post"
const templateError = "error"
const templateRedirect = "redirect"
const templateIndex = "index"
const templateTaxonomy = "taxonomy"

var templates map[string]*template.Template
var templateFunctions template.FuncMap

func initRendering() error {
	templateFunctions = template.FuncMap{
		"blog": func() *configBlog {
			return appConfig.Blog
		},
		"menu": func(id string) *menu {
			return appConfig.Blog.Menus[id]
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
		"asset":  assetFile,
		"string": getDefaultTemplateString,
		"include": func(templateName string, data interface{}) (template.HTML, error) {
			buf := new(bytes.Buffer)
			err := templates[templateName].ExecuteTemplate(buf, templateName, data)
			return template.HTML(buf.String()), err
		},
		"urlize": urlize,
		"sort":   sortedStrings,
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

func render(w http.ResponseWriter, template string, data interface{}) {
	// We need to use a buffer here to enable minification
	var buffer bytes.Buffer
	err := templates[template].ExecuteTemplate(&buffer, template, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	// Set content type (needed for minification middleware
	w.Header().Set("Content-Type", contentTypeHTML)
	// Write buffered response
	_, _ = w.Write(buffer.Bytes())
}

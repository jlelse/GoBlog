package main

import (
	"bytes"
	"fmt"
	"github.com/araddon/dateparse"
	"html/template"
	"log"
	"net/http"
	"time"
)

const templatePost = "post"
const templateError = "error"
const templateRedirect = "redirect"
const templateIndex = "index"
const templateSummary = "summary"
const templateTaxonomy = "taxonomy"

var templates map[string]*template.Template
var templateFunctions template.FuncMap

func initRendering() {
	templateFunctions = template.FuncMap{
		"blog": func() *configBlog {
			return appConfig.Blog
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
		"include": func(templateName string, data interface{}) (template.HTML, error) {
			buf := new(bytes.Buffer)
			err := templates[templateName].ExecuteTemplate(buf, templateName, data)
			return template.HTML(buf.String()), err
		},
		"urlize": urlize,
	}

	templates = make(map[string]*template.Template)
	for _, name := range []string{templatePost, templateError, templateRedirect, templateIndex, templateSummary, templateTaxonomy} {
		templates[name] = loadTemplate(name)
	}
}

func loadTemplate(name string) *template.Template {
	return template.Must(template.New(name).Funcs(templateFunctions).ParseFiles("templates/base.gohtml", fmt.Sprintf("templates/%s.gohtml", name)))
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

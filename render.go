package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
)

const templatePost = "post"
const templateError = "error"
const templateRedirect = "redirect"

var templates map[string]*template.Template
var templateFunctions template.FuncMap

func initRendering() {
	templateFunctions = template.FuncMap{
		"blog": func() *configBlog {
			return appConfig.blog
		},
		"md": func(content string) template.HTML {
			htmlContent, err := renderMarkdown(content)
			if err != nil {
				log.Fatal(err)
				return ""
			}
			return template.HTML(htmlContent)
		},
		"p": func(post Post, parameter string) string {
			return post.Parameters[parameter]
		},
	}

	templates = make(map[string]*template.Template)
	for _, name := range []string{templatePost, templateError, templateRedirect} {
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

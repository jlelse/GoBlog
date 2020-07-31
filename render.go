package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
)

const templatePostName = "post.gohtml"
const templateRedirectName = "redirect.gohtml"

var templates *template.Template

func initRendering() {
	templateFunctions := template.FuncMap{
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
		"title": func(post Post) string {
			return post.Parameters["title"]
		},
	}

	var err error

	templates, err = template.New("templates").Funcs(templateFunctions).ParseGlob("templates/*.gohtml")
	if err != nil {
		log.Fatal(err)
	}
}

func render(w http.ResponseWriter, template string, data interface{}) {
	// We need to use a buffer here to enable minification
	var buffer bytes.Buffer
	err := templates.ExecuteTemplate(&buffer, template, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	// Set content type (needed for minification middleware
	w.Header().Set("Content-Type", contentTypeHTML)
	// Write buffered response
	_, _ = w.Write(buffer.Bytes())
}

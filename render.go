package main

import (
	"html/template"
	"log"
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

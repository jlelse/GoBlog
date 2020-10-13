package main

import "net/http"

func serveCustomPage(blog *configBlog, page *customPage) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		render(w, page.Template, &renderData{
			Blog: blog,
			Data: page.Data,
		})
	}
}

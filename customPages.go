package main

import "net/http"

const customPageContextKey = "custompage"

func (a *goBlog) serveCustomPage(w http.ResponseWriter, r *http.Request) {
	page := r.Context().Value(customPageContextKey).(*configCustomPage)
	a.render(w, r, page.Template, &renderData{
		BlogString: r.Context().Value(blogContextKey).(string),
		Canonical:  a.getFullAddress(page.Path),
		Data:       page.Data,
	})
}

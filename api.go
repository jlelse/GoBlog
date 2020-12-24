package main

import (
	"io/ioutil"
	"net/http"
	"strings"
)

func apiPostCreateHugo(w http.ResponseWriter, r *http.Request) {
	blog := r.URL.Query().Get("blog")
	path := r.URL.Query().Get("path")
	section := r.URL.Query().Get("section")
	slug := r.URL.Query().Get("slug")
	alias := r.URL.Query().Get("alias")
	defer func() {
		_ = r.Body.Close()
	}()
	bodyContent, err := ioutil.ReadAll(r.Body)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	p, aliases, err := parseHugoFile(string(bodyContent))
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	p.Blog = blog
	p.Path = path
	p.Section = section
	p.Slug = slug
	err = p.replace()
	if err != nil {
		serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	aliases = append(aliases, alias)
	for i, alias := range aliases {
		// Fix relativ paths
		if !strings.HasPrefix(alias, "/") {
			splittedPostPath := strings.Split(p.Path, "/")
			alias = strings.TrimSuffix(p.Path, splittedPostPath[len(splittedPostPath)-1]) + alias
		}
		alias = strings.TrimSuffix(alias, "/")
		if alias == p.Path {
			alias = ""
		}
		aliases[i] = alias
	}
	if len(aliases) > 0 {
		p.Parameters["aliases"] = aliases
		err = p.replace()
		if err != nil {
			serveError(w, r, err.Error(), http.StatusBadRequest)
			return
		}
	}
	w.Header().Set("Location", p.fullURL())
	w.WriteHeader(http.StatusCreated)
}

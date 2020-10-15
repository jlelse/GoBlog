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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p, aliases, err := parseHugoFile(string(bodyContent))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p.Blog = blog
	p.Path = path
	p.Section = section
	p.Slug = slug
	aliases = append(aliases, alias)
	err = p.replace()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, alias := range aliases {
		// Fix relativ paths
		if !strings.HasPrefix(alias, "/") {
			splittedPostPath := strings.Split(p.Path, "/")
			alias = strings.TrimSuffix(p.Path, splittedPostPath[len(splittedPostPath)-1]) + alias
		}
		if alias != "" {
			_ = createOrReplaceRedirect(alias, p.Path)
		}
	}
	w.Header().Set("Location", appConfig.Server.PublicAddress+p.Path)
	w.WriteHeader(http.StatusCreated)
}

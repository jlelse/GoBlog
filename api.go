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
	post, aliases, err := parseHugoFile(string(bodyContent))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	post.Blog = blog
	post.Path = path
	post.Section = section
	post.Slug = slug
	aliases = append(aliases, alias)
	err = post.replace()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, alias := range aliases {
		// Fix relativ paths
		if !strings.HasPrefix(alias, "/") {
			splittedPostPath := strings.Split(post.Path, "/")
			alias = strings.TrimSuffix(post.Path, splittedPostPath[len(splittedPostPath)-1]) + alias
		}
		if alias != "" {
			_ = createOrReplaceRedirect(alias, post.Path)
		}
	}
	w.Header().Set("Location", appConfig.Server.PublicAddress+post.Path)
	w.WriteHeader(http.StatusCreated)
}

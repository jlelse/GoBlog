package main

import (
	"io/ioutil"
	"net/http"
	"strings"
)

func apiPostCreate(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()
	post := &Post{}
	err := json.NewDecoder(r.Body).Decode(post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = post.createOrReplace(false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Location", appConfig.Server.PublicAddress+post.Path)
	w.WriteHeader(http.StatusCreated)
}

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
	err = post.createOrReplace(false)
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

func apiPostRead(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "No path defined", http.StatusBadRequest)
		return
	}
	post, err := getPost(r.Context(), path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set(contentType, contentTypeJSONUTF8)
	err = json.NewEncoder(w).Encode(post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func apiPostDelete(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()
	post := &Post{}
	err := json.NewDecoder(r.Body).Decode(post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = post.delete()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

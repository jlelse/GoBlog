package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
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
	err = post.createOrReplace()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Location", appConfig.Server.PublicAddress+post.Path)
	w.WriteHeader(http.StatusCreated)
}

func apiPostCreateHugo(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "No path defined", http.StatusBadRequest)
		return
	}
	defer func() {
		_ = r.Body.Close()
	}()
	bodyContent, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	post, err := parseHugoFile(string(bodyContent), path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = post.createOrReplace()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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
	w.Header().Set("Content-Type", contentTypeJSON)
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

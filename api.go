package main

import (
	"encoding/json"
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
	err = createPost(post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Location", post.Path)
	w.WriteHeader(http.StatusCreated)
}

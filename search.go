package main

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const searchPlaceholder = "{search}"

func serveSearch(blog string, servePath string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			serveError(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		if q := r.Form.Get("q"); q != "" {
			http.Redirect(w, r, path.Join(servePath, searchEncode(q)), http.StatusFound)
			return
		}
		render(w, r, templateSearch, &renderData{
			BlogString: blog,
			Canonical:  appConfig.Server.PublicAddress + servePath,
		})
	}
}

func searchEncode(search string) string {
	return url.PathEscape(strings.ReplaceAll(base64.StdEncoding.EncodeToString([]byte(search)), "/", "_"))
}

func searchDecode(encoded string) string {
	encoded, err := url.PathUnescape(encoded)
	if err != nil {
		return ""
	}
	encoded = strings.ReplaceAll(encoded, "_", "/")
	db, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return string(db)
}

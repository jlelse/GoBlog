package main

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
)

const searchPlaceholder = "{search}"

func serveSearch(blog string, path string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			serveError(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		if q := r.Form.Get("q"); q != "" {
			http.Redirect(w, r, path+"/"+searchEncode(q), http.StatusFound)
			return
		}
		render(w, templateSearch, &renderData{
			BlogString: blog,
			Canonical:  appConfig.Server.PublicAddress + path,
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

package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

const defaultSearchPath = "/search"
const searchPlaceholder = "{search}"

func (a *goBlog) serveSearch(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogKey).(string)
	servePath := r.Context().Value(pathKey).(string)
	err := r.ParseForm()
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	if q := r.Form.Get("q"); q != "" {
		// Clean query
		q = htmlText([]byte(bluemonday.StrictPolicy().Sanitize(q)))
		// Redirect to results
		http.Redirect(w, r, path.Join(servePath, searchEncode(q)), http.StatusFound)
		return
	}
	a.render(w, r, templateSearch, &renderData{
		BlogString: blog,
		Canonical:  a.getFullAddress(servePath),
	})
}

func (a *goBlog) serveSearchResult(w http.ResponseWriter, r *http.Request) {
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path: r.Context().Value(pathKey).(string) + "/" + searchPlaceholder,
	})))
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

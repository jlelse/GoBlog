package main

import (
	"encoding/json"
	"io"
	"net/http"

	"go.goblog.app/app/pkgs/contenttype"
)

func (a *goBlog) serveNodeInfoDiscover(w http.ResponseWriter, r *http.Request) {
	result := map[string]any{
		"links": []map[string]any{
			{
				"href": a.getFullAddress("/nodeinfo"),
				"rel":  "http://nodeinfo.diaspora.software/ns/schema/2.1",
			},
		},
	}
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(json.NewEncoder(pw).Encode(result))
	}()
	w.Header().Set(contentType, contenttype.JSONUTF8)
	_ = pr.CloseWithError(a.min.Get().Minify(contenttype.JSON, w, pr))
}

func (a *goBlog) serveNodeInfo(w http.ResponseWriter, r *http.Request) {
	localPosts, _ := a.db.countPosts(&postsRequestConfig{
		status:     []postStatus{statusPublished},
		visibility: []postVisibility{visibilityPublic},
	})
	result := map[string]any{
		"version": "2.1",
		"software": map[string]any{
			"name":       "goblog",
			"repository": "https://go.goblog.app/app",
		},
		"usage": map[string]any{
			"users": map[string]any{
				"total": len(a.cfg.Blogs),
			},
			"localPosts": localPosts,
		},
		"protocols": []string{
			"activitypub",
			"micropub",
			"webmention",
		},
		"metadata": map[string]any{},
	}
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(json.NewEncoder(pw).Encode(result))
	}()
	w.Header().Set(contentType, contenttype.JSONUTF8)
	_ = pr.CloseWithError(a.min.Get().Minify(contenttype.JSON, w, pr))
}

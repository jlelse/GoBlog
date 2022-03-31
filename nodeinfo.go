package main

import (
	"encoding/json"
	"io"
	"net/http"

	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

func (a *goBlog) serveNodeInfoDiscover(w http.ResponseWriter, r *http.Request) {
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	err := json.NewEncoder(buf).Encode(map[string]any{
		"links": []map[string]any{
			{
				"href": a.getFullAddress("/nodeinfo"),
				"rel":  "http://nodeinfo.diaspora.software/ns/schema/2.1",
			},
		},
	})
	if err != nil {
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, contenttype.JSONUTF8)
	mw := a.min.Writer(contenttype.JSON, w)
	_, _ = io.Copy(mw, buf)
	_ = mw.Close()
}

func (a *goBlog) serveNodeInfo(w http.ResponseWriter, r *http.Request) {
	localPosts, _ := a.db.countPosts(&postsRequestConfig{
		status: statusPublished,
	})
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	err := json.NewEncoder(buf).Encode(map[string]any{
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
	})
	if err != nil {
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, contenttype.JSONUTF8)
	mw := a.min.Writer(contenttype.JSON, w)
	_, _ = io.Copy(mw, buf)
	_ = mw.Close()
}

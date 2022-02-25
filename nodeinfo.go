package main

import (
	"encoding/json"
	"net/http"

	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

func (a *goBlog) serveNodeInfoDiscover(w http.ResponseWriter, r *http.Request) {
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	err := json.NewEncoder(buf).Encode(map[string]interface{}{
		"links": []map[string]interface{}{
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
	_, _ = buf.WriteTo(mw)
	_ = mw.Close()
}

func (a *goBlog) serveNodeInfo(w http.ResponseWriter, r *http.Request) {
	localPosts, _ := a.db.countPosts(&postsRequestConfig{
		status: statusPublished,
	})
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	err := json.NewEncoder(buf).Encode(map[string]interface{}{
		"version": "2.1",
		"software": map[string]interface{}{
			"name":       "goblog",
			"repository": "https://go.goblog.app/app",
		},
		"usage": map[string]interface{}{
			"users": map[string]interface{}{
				"total": len(a.cfg.Blogs),
			},
			"localPosts": localPosts,
		},
		"protocols": []string{
			"activitypub",
			"micropub",
			"webmention",
		},
		"metadata": map[string]interface{}{},
	})
	if err != nil {
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, contenttype.JSONUTF8)
	mw := a.min.Writer(contenttype.JSON, w)
	_, _ = buf.WriteTo(mw)
	_ = mw.Close()
}

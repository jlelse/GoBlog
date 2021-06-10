package main

import (
	"encoding/json"
	"net/http"
)

func (a *goBlog) serveNodeInfoDiscover(w http.ResponseWriter, r *http.Request) {
	b, _ := json.Marshal(map[string]interface{}{
		"links": []map[string]interface{}{
			{
				"href": a.getFullAddress("/nodeinfo"),
				"rel":  "http://nodeinfo.diaspora.software/ns/schema/2.1",
			},
		},
	})
	w.Header().Set(contentType, contentTypeJSONUTF8)
	_, _ = writeMinified(w, contentTypeJSON, b)
}

func (a *goBlog) serveNodeInfo(w http.ResponseWriter, r *http.Request) {
	localPosts, _ := a.db.countPosts(&postsRequestConfig{
		status: statusPublished,
	})
	b, _ := json.Marshal(map[string]interface{}{
		"version": "2.1",
		"software": map[string]interface{}{
			"name":       "goblog",
			"repository": "https://git.jlel.se/jlelse/GoBlog",
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
	w.Header().Set(contentType, contentTypeJSONUTF8)
	_, _ = writeMinified(w, contentTypeJSON, b)
}

package main

import (
	"net/http"
)

func (a *goBlog) serveNodeInfoDiscover(w http.ResponseWriter, _ *http.Request) {
	result := map[string]any{
		"links": []map[string]any{
			{
				"href": a.getFullAddress("/nodeinfo"),
				"rel":  "http://nodeinfo.diaspora.software/ns/schema/2.1",
			},
		},
	}
	a.respondWithMinifiedJson(w, result)
}

func (a *goBlog) serveNodeInfo(w http.ResponseWriter, _ *http.Request) {
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
	a.respondWithMinifiedJson(w, result)
}

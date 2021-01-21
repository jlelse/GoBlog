package main

import (
	"encoding/json"
	"net/http"
)

func serveNodeInfoDiscover(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentType, contentTypeJSONUTF8)
	nid := map[string]interface{}{
		"links": []map[string]interface{}{
			{
				"href": appConfig.Server.PublicAddress + "/nodeinfo",
				"rel":  "http://nodeinfo.diaspora.software/ns/schema/2.1",
			},
		},
	}
	_ = json.NewEncoder(w).Encode(&nid)
}

func serveNodeInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentType, contentTypeJSONUTF8)
	localPosts, _ := countPosts(&postsRequestConfig{
		status: statusPublished,
	})
	nid := map[string]interface{}{
		"version": "2.1",
		"software": map[string]interface{}{
			"name":       "goblog",
			"repository": "https://git.jlel.se/jlelse/GoBlog",
		},
		"usage": map[string]interface{}{
			"users": map[string]interface{}{
				"total": len(appConfig.Blogs),
			},
			"localPosts": localPosts,
		},
		"protocols": []string{
			"activitypub",
			"micropub",
			"webmention",
		},
		"metadata": map[string]interface{}{},
	}
	_ = json.NewEncoder(w).Encode(&nid)
}

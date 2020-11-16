package main

import (
	"net/http"
	"time"

	"github.com/araddon/dateparse"
	"github.com/snabb/sitemap"
)

const sitemapPath = "/sitemap.xml"

func serveSitemap(w http.ResponseWriter, r *http.Request) {
	posts, err := getPosts(&postsRequestConfig{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	sm := sitemap.New()
	sm.Minify = true
	for _, p := range posts {
		item := &sitemap.URL{
			Loc: p.fullURL()}
		var lastMod time.Time
		if p.Updated != "" {
			lastMod, _ = dateparse.ParseIn(p.Updated, time.Local)
		}
		if p.Published != "" && lastMod.IsZero() {
			lastMod, _ = dateparse.ParseIn(p.Published, time.Local)
		}
		if !lastMod.IsZero() {
			item.LastMod = &lastMod
		}
		sm.Add(item)
	}
	_, _ = sm.WriteTo(w)
}

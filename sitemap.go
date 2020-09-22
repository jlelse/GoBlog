package main

import (
	"github.com/araddon/dateparse"
	"github.com/snabb/sitemap"
	"net/http"
	"time"
)

func serveSitemap() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		posts, err := getPosts(r.Context(), &postsRequestConfig{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		sm := sitemap.New()
		sm.Minify = true
		for _, p := range posts {
			var lastMod time.Time
			if p.Updated != "" {
				lastMod, _ = dateparse.ParseIn(p.Updated, time.Local)
			} else if p.Published != "" {
				lastMod, _ = dateparse.ParseIn(p.Published, time.Local)
			}
			if lastMod.IsZero() {
				lastMod = time.Now()
			}
			sm.Add(&sitemap.URL{
				Loc:     appConfig.Server.PublicAddress + p.Path,
				LastMod: &lastMod,
			})
		}
		_, _ = sm.WriteTo(w)
	}
}

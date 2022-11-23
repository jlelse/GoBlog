package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (a *goBlog) apShowFollowers(w http.ResponseWriter, r *http.Request) {
	blogName := chi.URLParam(r, "blog")
	blog, ok := a.cfg.Blogs[blogName]
	if !ok || blog == nil {
		a.serveError(w, r, "Blog not found", http.StatusNotFound)
		return
	}
	followers, err := a.db.apGetAllFollowers(blogName)
	if err != nil {
		a.serveError(w, r, "Failed to get followers", http.StatusInternalServerError)
		return
	}
	a.render(w, r, a.renderActivityPubFollowers, &renderData{
		BlogString: blogName,
		Data: &activityPubFollowersRenderData{
			followers: followers,
		},
	})
}

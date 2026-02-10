package main

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (a *goBlog) apRemoteFollow(w http.ResponseWriter, r *http.Request) {
	blogName := chi.URLParam(r, "blog")
	blog, ok := a.cfg.Blogs[blogName]
	if !ok || blog == nil {
		a.serveError(w, r, "Blog not found", http.StatusNotFound)
		return
	}
	if user := r.FormValue("user"); user != "" {
		// Parse instance
		userParts := strings.Split(user, "@")
		if len(userParts) < 2 {
			a.serveError(w, r, "User must be of the form user@example.org or @user@example.org", http.StatusBadRequest)
			return
		}
		user = userParts[len(userParts)-2]
		instance := userParts[len(userParts)-1]
		if user == "" || instance == "" {
			a.serveError(w, r, "User or instance are empty", http.StatusBadRequest)
			return
		}
		// Get webfinger
		wf, err := a.apFetchWebfinger(r.Context(), user, instance)
		if err != nil {
			a.serveError(w, r, "Failed to query webfinger", http.StatusInternalServerError)
			return
		}
		// Check webfinger and find template
		template := ""
		for _, link := range wf.Links {
			if link.Rel == "http://ostatus.org/schema/1.0/subscribe" {
				template = link.Template
				break
			}
		}
		if template == "" {
			a.serveError(w, r, "Instance does not support subscribe schema version 1.0", http.StatusInternalServerError)
			return
		}
		// Build redirect
		redirect := strings.ReplaceAll(template, "{uri}", url.PathEscape(a.apIri(blog)))
		http.Redirect(w, r, redirect, http.StatusFound)
		return
	}
	// Render remote follow form
	a.render(w, r, a.renderActivityPubRemoteFollow, &renderData{
		BlogString: blogName,
	})
}

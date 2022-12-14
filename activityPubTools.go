package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/carlmjohnson/requests"
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
		type webfingerLinkType struct {
			Rel      string `json:"rel"`
			Template string `json:"template"`
		}
		type webfingerType struct {
			Links []*webfingerLinkType `json:"links"`
		}
		webfinger := &webfingerType{}
		err := requests.URL(fmt.Sprintf("https://%s/.well-known/webfinger?resource=acct:%s@%s", instance, user, instance)).
			Client(a.httpClient).
			Handle(func(resp *http.Response) error {
				defer resp.Body.Close()
				return json.NewDecoder(io.LimitReader(resp.Body, 1000*1000)).Decode(webfinger)
			}).
			Fetch(r.Context())
		if err != nil {
			a.serveError(w, r, "Failed to query webfinger", http.StatusInternalServerError)
			return
		}
		// Check webfinger and find template
		template := ""
		for _, link := range webfinger.Links {
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

package main

import (
	"net/http"
	"strings"
)

// redirectAlternateDomain handles requests to alternate domains.
// ActivityPub and Webfinger requests are served normally,
// all other requests are redirected to the new domain.
func (a *goBlog) redirectAlternateDomain(rw http.ResponseWriter, r *http.Request) {
	// Check if this is an ActivityPub or Webfinger request
	// These should be served normally, not redirected
	if strings.HasPrefix(r.URL.Path, "/.well-known/webfinger") ||
		strings.HasPrefix(r.URL.Path, "/.well-known/host-meta") ||
		strings.HasPrefix(r.URL.Path, "/.well-known/nodeinfo") ||
		strings.HasPrefix(r.URL.Path, "/activitypub/") ||
		strings.HasPrefix(r.URL.Path, "/nodeinfo") {
		// Serve the request normally
		a.d.ServeHTTP(rw, r)
		return
	}

	// Check if Accept header indicates ActivityPub request
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/activity+json") ||
		strings.Contains(accept, "application/ld+json") {
		// Serve the request normally for ActivityPub clients
		a.d.ServeHTTP(rw, r)
		return
	}

	// Redirect all other requests to the new domain
	http.Redirect(rw, r, a.getFullAddress(r.URL.RequestURI()), http.StatusMovedPermanently)
}

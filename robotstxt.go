package main

import (
	"fmt"
	"net/http"
)

const robotsTXTPath = "/robots.txt"

func (a *goBlog) serveRobotsTXT(w http.ResponseWriter, r *http.Request) {
	if a.isPrivate() {
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /"))
		return
	}
	_, _ = w.Write([]byte(fmt.Sprintf("User-agent: *\nSitemap: %v", a.getFullAddress(sitemapPath))))
}

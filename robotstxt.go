package main

import (
	"fmt"
	"net/http"
)

func (a *goBlog) serveRobotsTXT(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(fmt.Sprintf("User-agent: *\nSitemap: %v", a.getFullAddress(sitemapPath))))
}

func servePrivateRobotsTXT(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("User-agent: *\nDisallow: /"))
}

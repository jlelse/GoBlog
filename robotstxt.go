package main

import (
	"fmt"
	"net/http"
)

func serveRobotsTXT(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(fmt.Sprintf("User-agent: *\nSitemap: %v", appConfig.Server.PublicAddress+sitemapPath)))
}

func servePrivateRobotsTXT(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("User-agent: *\nDisallow: /"))
}

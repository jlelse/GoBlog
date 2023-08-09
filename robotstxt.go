package main

import (
	"fmt"
	"net/http"
)

const robotsTXTPath = "/robots.txt"

func (a *goBlog) serveRobotsTXT(w http.ResponseWriter, _ *http.Request) {
	_, _ = fmt.Fprint(w, "User-agent: *\n")
	if a.isPrivate() {
		_, _ = fmt.Fprint(w, "Disallow: /\n")
		return
	}
	_, _ = fmt.Fprint(w, "Allow: /\n\n")
	if a.cfg.RobotsTxt != nil && len(a.cfg.RobotsTxt.BlockedBots) != 0 {
		for _, bot := range a.cfg.RobotsTxt.BlockedBots {
			_, _ = fmt.Fprint(w, "User-agent: ")
			_, _ = fmt.Fprint(w, bot)
			_, _ = fmt.Fprint(w, "\n")
			_, _ = fmt.Fprint(w, "Disallow: /\n\n")
		}
	}
	_, _ = fmt.Fprintf(w, "Sitemap: %s\n", a.getFullAddress(sitemapPath))
	for _, bc := range a.cfg.Blogs {
		_, _ = fmt.Fprintf(w, "Sitemap: %s\n", a.getFullAddress(bc.getRelativePath(sitemapBlogPath)))
	}
}

package main

import (
	"fmt"
	"net/http"
	"slices"

	"go.goblog.app/app/pkgs/plugintypes"
)

const robotsTXTPath = "/robots.txt"

func (a *goBlog) serveRobotsTXT(w http.ResponseWriter, _ *http.Request) {
	if a.isPrivate() {
		_, _ = fmt.Fprint(w, "User-agent: *\n")
		_, _ = fmt.Fprint(w, "Disallow: /\n")
		return
	}
	blockedBots := a.getBlockedBots()
	if len(blockedBots) > 0 {
		for _, bot := range blockedBots {
			_, _ = fmt.Fprint(w, "User-agent: ")
			_, _ = fmt.Fprint(w, bot)
			_, _ = fmt.Fprint(w, "\n")
		}
		_, _ = fmt.Fprint(w, "Disallow: /\n\n")
	}
	_, _ = fmt.Fprint(w, "User-agent: *\n")
	_, _ = fmt.Fprint(w, "Allow: /\n\n")
	_, _ = fmt.Fprintf(w, "Sitemap: %s\n", a.getFullAddress(sitemapPath))
	for _, bc := range a.cfg.Blogs {
		_, _ = fmt.Fprintf(w, "Sitemap: %s\n", a.getFullAddress(bc.getRelativePath(sitemapBlogPath)))
	}
}

func (a *goBlog) getBlockedBots() []string {
	bots := []string{}
	if a.cfg.RobotsTxt != nil {
		bots = append(bots, a.cfg.RobotsTxt.BlockedBots...)
	}
	for _, p := range a.getPlugins(pluginBlockedBotsType) {
		bots = append(bots, p.(plugintypes.BlockedBots).BlockedBots()...)
	}
	if len(bots) == 0 {
		return nil
	}
	slices.Sort(bots)
	return bots
}

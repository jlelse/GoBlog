package main

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const editorLinksPath = "/links"

type linkDomainStat struct {
	Domain    string
	Posts     []*post
	LinkCount int
}

func (a *goBlog) collectExternalDomains(blog string) ([]*linkDomainStat, error) {
	posts, err := a.getPosts(&postsRequestConfig{
		blogs: []string{blog},
	})
	if err != nil {
		return nil, err
	}
	pls, err := a.extractPostLinks(posts)
	if err != nil {
		return nil, err
	}
	domainPosts := map[string][]*post{}
	domainLinks := map[string]int{}
	for _, pl := range pls {
		seen := map[string]bool{}
		for _, link := range pl.links {
			u, err := url.Parse(link)
			if err != nil {
				continue
			}
			host := strings.ToLower(u.Hostname())
			if host == "" {
				continue
			}
			domainLinks[host]++
			if !seen[host] {
				seen[host] = true
				domainPosts[host] = append(domainPosts[host], pl.post)
			}
		}
	}
	stats := make([]*linkDomainStat, 0, len(domainPosts))
	for host, ps := range domainPosts {
		sort.Slice(ps, func(i, j int) bool { return ps[i].Path < ps[j].Path })
		stats = append(stats, &linkDomainStat{Domain: host, Posts: ps, LinkCount: domainLinks[host]})
	}
	sort.Slice(stats, func(i, j int) bool {
		if len(stats[i].Posts) != len(stats[j].Posts) {
			return len(stats[i].Posts) > len(stats[j].Posts)
		}
		if stats[i].LinkCount != stats[j].LinkCount {
			return stats[i].LinkCount > stats[j].LinkCount
		}
		return stats[i].Domain < stats[j].Domain
	})
	return stats, nil
}

func (a *goBlog) serveEditorLinks(w http.ResponseWriter, r *http.Request) {
	blog, _ := a.getBlog(r)
	stats, err := a.collectExternalDomains(blog)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	if domain := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("domain"))); domain != "" {
		var match *linkDomainStat
		for _, s := range stats {
			if s.Domain == domain {
				match = s
				break
			}
		}
		a.render(w, r, a.renderEditorLinkDomain, &renderData{
			Data: &editorLinkDomainRenderData{domain: domain, stat: match},
		})
		return
	}
	a.render(w, r, a.renderEditorLinks, &renderData{
		Data: &editorLinksRenderData{domains: stats},
	})
}

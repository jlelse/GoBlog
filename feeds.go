package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/feeds"
)

type feedType string

const (
	noFeed   feedType = ""
	rssFeed  feedType = "rss"
	atomFeed feedType = "atom"
	jsonFeed feedType = "json"
)

func generateFeed(blog string, f feedType, w http.ResponseWriter, r *http.Request, posts []*post, title string, description string) {
	now := time.Now()
	if title == "" {
		title = appConfig.Blogs[blog].Title
	}
	if description == "" {
		description = appConfig.Blogs[blog].Description
	}
	feed := &feeds.Feed{
		Title:       title,
		Description: description,
		Link:        &feeds.Link{Href: appConfig.Server.PublicAddress + strings.TrimSuffix(r.URL.Path, "."+string(f))},
		Created:     now,
	}
	for _, p := range posts {
		htmlContent, _ := renderMarkdown(p.Content)
		feed.Add(&feeds.Item{
			Title:       p.title(),
			Link:        &feeds.Link{Href: appConfig.Server.PublicAddress + p.Path},
			Description: p.summary(),
			Id:          p.Path,
			Content:     string(htmlContent),
		})
	}
	var feedStr string
	var err error
	switch f {
	case rssFeed:
		w.Header().Add(contentType, "application/rss+xml; charset=utf-8")
		feedStr, err = feed.ToRss()
	case atomFeed:
		w.Header().Add(contentType, "application/atom+xml; charset=utf-8")
		feedStr, err = feed.ToAtom()
	case jsonFeed:
		w.Header().Add(contentType, "application/feed+json; charset=utf-8")
		feedStr, err = feed.ToJSON()
	default:
		return
	}
	if err != nil {
		w.Header().Del(contentType)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(feedStr))
}

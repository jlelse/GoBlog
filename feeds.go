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

func generateFeed(blog string, f feedType, w http.ResponseWriter, r *http.Request, posts []*Post, title string, description string) {
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
	for _, postItem := range posts {
		htmlContent, _ := renderMarkdown(postItem.Content)
		feed.Add(&feeds.Item{
			Title:       postItem.title(),
			Link:        &feeds.Link{Href: appConfig.Server.PublicAddress + postItem.Path},
			Description: postItem.summary(),
			Id:          postItem.Path,
			Content:     string(htmlContent),
		})
	}
	var feedStr string
	var err error
	switch f {
	case rssFeed:
		feedStr, err = feed.ToRss()
	case atomFeed:
		feedStr, err = feed.ToAtom()
	case jsonFeed:
		feedStr, err = feed.ToJSON()
	default:
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch f {
	case rssFeed:
		w.Header().Add(contentType, "application/rss+xml; charset=utf-8")
	case atomFeed:
		w.Header().Add(contentType, "application/atom+xml; charset=utf-8")
	case jsonFeed:
		w.Header().Add(contentType, "application/feed+json; charset=utf-8")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(feedStr))
}

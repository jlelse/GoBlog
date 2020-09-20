package main

import (
	"github.com/gorilla/feeds"
	"net/http"
	"strings"
	"time"
)

type feedType string

const (
	NONE feedType = ""
	RSS  feedType = "rss"
	ATOM feedType = "atom"
	JSON feedType = "json"
)

func generateFeed(f feedType, w http.ResponseWriter, r *http.Request, posts []*Post, title string, description string) {
	now := time.Now()
	if title == "" {
		title = appConfig.Blog.Title
	}
	if description == "" {
		description = appConfig.Blog.Description
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
	case RSS:
		feedStr, err = feed.ToRss()
	case ATOM:
		feedStr, err = feed.ToAtom()
	case JSON:
		feedStr, err = feed.ToJSON()
	default:
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch f {
	case RSS:
		w.Header().Add("Content-Type", "application/rss+xml; charset=utf-8")
	case ATOM:
		w.Header().Add("Content-Type", "application/atom+xml; charset=utf-8")
	case JSON:
		w.Header().Add("Content-Type", "application/feed+json; charset=utf-8")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(feedStr))
}

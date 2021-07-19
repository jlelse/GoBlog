package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/jlelse/feeds"
	"go.goblog.app/app/pkgs/contenttype"
)

type feedType string

const (
	noFeed   feedType = ""
	rssFeed  feedType = "rss"
	atomFeed feedType = "atom"
	jsonFeed feedType = "json"
)

func (a *goBlog) generateFeed(blog string, f feedType, w http.ResponseWriter, r *http.Request, posts []*post, title string, description string) {
	now := time.Now()
	if title == "" {
		title = a.cfg.Blogs[blog].Title
	}
	if description == "" {
		description = a.cfg.Blogs[blog].Description
	}
	feed := &feeds.Feed{
		Title:       title,
		Description: description,
		Link:        &feeds.Link{Href: a.getFullAddress(strings.TrimSuffix(r.URL.Path, "."+string(f)))},
		Created:     now,
		Author: &feeds.Author{
			Name:  a.cfg.User.Name,
			Email: a.cfg.User.Email,
		},
		Image: &feeds.Image{
			Url: a.cfg.User.Picture,
		},
	}
	for _, p := range posts {
		created, _ := dateparse.ParseLocal(p.Published)
		updated, _ := dateparse.ParseLocal(p.Updated)
		feed.Add(&feeds.Item{
			Title:       p.Title(),
			Link:        &feeds.Link{Href: a.fullPostURL(p)},
			Description: a.postSummary(p),
			Id:          p.Path,
			Content:     string(a.postHtml(p, true)),
			Created:     created,
			Updated:     updated,
		})
	}
	var err error
	var feedString, feedMediaType string
	switch f {
	case rssFeed:
		feedMediaType = contenttype.RSS
		feedString, err = feed.ToRss()
	case atomFeed:
		feedMediaType = contenttype.ATOM
		feedString, err = feed.ToAtom()
	case jsonFeed:
		feedMediaType = contenttype.JSONFeed
		feedString, err = feed.ToJSON()
	default:
		return
	}
	if err != nil {
		w.Header().Del(contentType)
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, feedMediaType+contenttype.CharsetUtf8Suffix)
	_, _ = a.min.Write(w, feedMediaType, []byte(feedString))
}

package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/gorilla/feeds"
)

type feedType string

const (
	noFeed   feedType = ""
	rssFeed  feedType = "rss"
	atomFeed feedType = "atom"
	jsonFeed feedType = "json"

	feedAudioURL    = "audio"
	feedAudioType   = "audiomime"
	feedAudioLength = "audiolength"
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
		Author: &feeds.Author{
			Name:  appConfig.User.Name,
			Email: appConfig.User.Email,
		},
		Image: &feeds.Image{
			Url: appConfig.User.Picture,
		},
	}
	for _, p := range posts {
		created, _ := dateparse.ParseLocal(p.Published)
		updated, _ := dateparse.ParseLocal(p.Updated)
		var enc *feeds.Enclosure
		if p.firstParameter(feedAudioURL) != "" {
			enc = &feeds.Enclosure{
				Url:    p.firstParameter(feedAudioURL),
				Type:   p.firstParameter(feedAudioType),
				Length: p.firstParameter(feedAudioLength),
			}
		}
		feed.Add(&feeds.Item{
			Title:       p.title(),
			Link:        &feeds.Link{Href: p.fullURL()},
			Description: p.summary(),
			Id:          p.Path,
			Content:     string(p.absoluteHTML()),
			Created:     created,
			Updated:     updated,
			Enclosure:   enc,
		})
	}
	var err error
	switch f {
	case rssFeed:
		w.Header().Set(contentType, "application/rss+xml; charset=utf-8")
		err = feed.WriteRss(w)
	case atomFeed:
		w.Header().Set(contentType, "application/atom+xml; charset=utf-8")
		err = feed.WriteAtom(w)
	case jsonFeed:
		w.Header().Set(contentType, "application/feed+json; charset=utf-8")
		err = feed.WriteJSON(w)
	default:
		return
	}
	if err != nil {
		w.Header().Del(contentType)
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
}

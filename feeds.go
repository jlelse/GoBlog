package main

import (
	"bytes"
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
	title = a.renderMdTitle(defaultIfEmpty(title, a.cfg.Blogs[blog].Title))
	description = defaultIfEmpty(description, a.cfg.Blogs[blog].Description)
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
		content, _ := a.min.MinifyString(contenttype.HTML, a.feedHtml(p))
		feed.Add(&feeds.Item{
			Title:       p.RenderedTitle,
			Link:        &feeds.Link{Href: a.fullPostURL(p)},
			Description: a.postSummary(p),
			Id:          p.Path,
			Content:     content,
			Created:     timeNoErr(dateparse.ParseLocal(p.Published)),
			Updated:     timeNoErr(dateparse.ParseLocal(p.Updated)),
		})
	}
	var err error
	var feedBuffer bytes.Buffer
	var feedMediaType string
	switch f {
	case rssFeed:
		feedMediaType = contenttype.RSS
		err = feed.WriteRss(&feedBuffer)
	case atomFeed:
		feedMediaType = contenttype.ATOM
		err = feed.WriteAtom(&feedBuffer)
	case jsonFeed:
		feedMediaType = contenttype.JSONFeed
		err = feed.WriteJSON(&feedBuffer)
	default:
		a.serve404(w, r)
		return
	}
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, feedMediaType+contenttype.CharsetUtf8Suffix)
	_ = a.min.Minify(feedMediaType, w, &feedBuffer)
}

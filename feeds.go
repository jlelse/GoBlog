package main

import (
	"io"
	"net/http"
	"time"

	"github.com/araddon/dateparse"
	"github.com/jlelse/feeds"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

type feedType string

const (
	noFeed      feedType = ""
	rssFeed     feedType = "rss"
	atomFeed    feedType = "atom"
	jsonFeed    feedType = "json"
	minRssFeed  feedType = "min.rss"
	minAtomFeed feedType = "min.atom"
	minJsonFeed feedType = "min.json"
)

func (a *goBlog) generateFeed(blog string, f feedType, w http.ResponseWriter, r *http.Request, posts []*post, title, description, path, query string) {
	now := time.Now()
	title = a.renderMdTitle(defaultIfEmpty(title, a.cfg.Blogs[blog].Title))
	description = defaultIfEmpty(description, a.cfg.Blogs[blog].Description)
	feed := &feeds.Feed{
		Title:       title,
		Description: description,
		Link:        &feeds.Link{Href: a.getFullAddress(path) + query},
		Created:     now,
		Author: &feeds.Author{
			Name:  a.cfg.User.Name,
			Email: a.cfg.User.Email,
		},
		Image: &feeds.Image{
			Url: a.profileImagePath(profileImageFormatJPEG, 0, 0),
		},
	}
	for _, p := range posts {
		buf := bufferpool.Get()
		switch f {
		case minRssFeed, minAtomFeed, minJsonFeed:
			a.minFeedHtml(buf, p)
		default:
			a.feedHtml(buf, p)
		}
		feed.Add(&feeds.Item{
			Title:       p.RenderedTitle,
			Link:        &feeds.Link{Href: a.fullPostURL(p)},
			Description: a.postSummary(p),
			Id:          p.Path,
			Content:     buf.String(),
			Created:     noError(dateparse.ParseLocal(p.Published)),
			Updated:     noError(dateparse.ParseLocal(p.Updated)),
			Tags:        sortedStrings(p.Parameters[a.cfg.Micropub.CategoryParam]),
		})
		bufferpool.Put(buf)
	}
	var feedWriteFunc func(w io.Writer) error
	var feedMediaType string
	switch f {
	case rssFeed, minRssFeed:
		feedMediaType = contenttype.RSS
		feedWriteFunc = feed.WriteRss
	case atomFeed, minAtomFeed:
		feedMediaType = contenttype.ATOM
		feedWriteFunc = feed.WriteAtom
	case jsonFeed, minJsonFeed:
		feedMediaType = contenttype.JSONFeed
		feedWriteFunc = feed.WriteJSON
	default:
		a.serve404(w, r)
		return
	}
	pipeReader, pipeWriter := io.Pipe()
	go func() {
		_ = pipeWriter.CloseWithError(feedWriteFunc(pipeWriter))
	}()
	w.Header().Set(contentType, feedMediaType+contenttype.CharsetUtf8Suffix)
	_ = pipeReader.CloseWithError(a.min.Get().Minify(feedMediaType, w, pipeReader))
}

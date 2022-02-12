package main

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/jlelse/feeds"
	"go.goblog.app/app/pkgs/bufferpool"
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
		buf := bufferpool.Get()
		a.feedHtml(buf, p)
		feed.Add(&feeds.Item{
			Title:       p.RenderedTitle,
			Link:        &feeds.Link{Href: a.fullPostURL(p)},
			Description: a.postSummary(p),
			Id:          p.Path,
			Content:     buf.String(),
			Created:     timeNoErr(dateparse.ParseLocal(p.Published)),
			Updated:     timeNoErr(dateparse.ParseLocal(p.Updated)),
		})
		bufferpool.Put(buf)
	}
	var feedWriteFunc func(w io.Writer) error
	var feedMediaType string
	switch f {
	case rssFeed:
		feedMediaType = contenttype.RSS
		feedWriteFunc = feed.WriteRss
	case atomFeed:
		feedMediaType = contenttype.ATOM
		feedWriteFunc = feed.WriteAtom
	case jsonFeed:
		feedMediaType = contenttype.JSONFeed
		feedWriteFunc = feed.WriteJSON
	default:
		a.serve404(w, r)
		return
	}
	pipeReader, pipeWriter := io.Pipe()
	go func() {
		writeErr := feedWriteFunc(pipeWriter)
		_ = pipeWriter.CloseWithError(writeErr)
	}()
	w.Header().Set(contentType, feedMediaType+contenttype.CharsetUtf8Suffix)
	minifyErr := a.min.Minify(feedMediaType, w, pipeReader)
	_ = pipeReader.CloseWithError(minifyErr)
}

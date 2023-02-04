package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/carlmjohnson/requests"
	"github.com/kaorimatz/go-opml"
	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

const defaultBlogrollPath = "/blogroll"

func (a *goBlog) serveBlogroll(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)
	outlines, err, _ := a.blogrollCacheGroup.Do(blog, func() (any, error) {
		return a.getBlogrollOutlines(blog)
	})
	if err != nil {
		log.Printf("Failed to get outlines: %v", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	c := bc.Blogroll
	can := bc.getRelativePath(defaultIfEmpty(c.Path, defaultBlogrollPath))
	a.render(w, r, a.renderBlogroll, &renderData{
		Canonical: a.getFullAddress(can),
		Data: &blogrollRenderData{
			title:       c.Title,
			description: c.Description,
			outlines:    outlines.([]*opml.Outline),
			download:    can + ".opml",
		},
	})
}

func (a *goBlog) serveBlogrollExport(w http.ResponseWriter, r *http.Request) {
	blog, _ := a.getBlog(r)
	outlines, err, _ := a.blogrollCacheGroup.Do(blog, func() (any, error) {
		return a.getBlogrollOutlines(blog)
	})
	if err != nil {
		log.Printf("Failed to get outlines: %v", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(opml.Render(pw, &opml.OPML{
			Version:     "2.0",
			DateCreated: time.Now().UTC(),
			Outlines:    outlines.([]*opml.Outline),
		}))
	}()
	w.Header().Set(contentType, contenttype.XMLUTF8)
	_ = pr.CloseWithError(a.min.Get().Minify(contenttype.XML, w, pr))
}

func (a *goBlog) getBlogrollOutlines(blog string) ([]*opml.Outline, error) {
	// Get config
	config := a.cfg.Blogs[blog].Blogroll
	// Check cache
	if cache := a.db.loadOutlineCache(blog); cache != nil {
		return cache, nil
	}
	// Make request and parse OPML
	pr, pw := io.Pipe()
	rb := requests.URL(config.Opml).Client(a.httpClient).ToWriter(pw)
	if config.AuthHeader != "" && config.AuthValue != "" {
		rb.Header(config.AuthHeader, config.AuthValue)
	}
	go func() {
		_ = pw.CloseWithError(rb.Fetch(context.Background()))
	}()
	o, err := opml.Parse(pr)
	_ = pr.CloseWithError(err)
	if err != nil {
		return nil, err
	}
	// Filter and sort
	outlines := o.Outlines
	if len(config.Categories) > 0 {
		filtered := []*opml.Outline{}
		for _, category := range config.Categories {
			if outline, ok := lo.Find(outlines, func(outline *opml.Outline) bool {
				return outline.Title == category || outline.Text == category
			}); ok && outline != nil {
				outline.Outlines = sortOutlines(outline.Outlines)
				filtered = append(filtered, outline)
			}
		}
		outlines = filtered
	} else {
		outlines = sortOutlines(outlines)
	}
	// Cache
	a.db.cacheOutlines(blog, outlines)
	return outlines, nil
}

func (db *database) cacheOutlines(blog string, outlines []*opml.Outline) {
	opmlBuffer := bufferpool.Get()
	_ = opml.Render(opmlBuffer, &opml.OPML{
		Version:     "2.0",
		DateCreated: time.Now().UTC(),
		Outlines:    outlines,
	})
	_ = db.cachePersistently("blogroll_"+blog, opmlBuffer.Bytes())
	bufferpool.Put(opmlBuffer)
}

func (db *database) loadOutlineCache(blog string) []*opml.Outline {
	data, err := db.retrievePersistentCache("blogroll_" + blog)
	if err != nil || data == nil {
		return nil
	}
	o, err := opml.Parse(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	if time.Since(o.DateCreated).Minutes() > 60 {
		return nil
	}
	return o.Outlines
}

func sortOutlines(outlines []*opml.Outline) []*opml.Outline {
	sort.Slice(outlines, func(i, j int) bool {
		name1 := outlines[i].Title
		if name1 == "" {
			name1 = outlines[i].Text
		}
		name2 := outlines[j].Title
		if name2 == "" {
			name2 = outlines[j].Text
		}
		return strings.ToLower(name1) < strings.ToLower(name2)
	})
	for _, outline := range outlines {
		outline.Outlines = sortOutlines(outline.Outlines)
	}
	return outlines
}

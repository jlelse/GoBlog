package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/kaorimatz/go-opml"
	"github.com/thoas/go-funk"
	"go.goblog.app/app/pkgs/contenttype"
)

const defaultBlogrollPath = "/blogroll"

func (a *goBlog) serveBlogroll(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	outlines, err, _ := a.blogrollCacheGroup.Do(blog, func() (interface{}, error) {
		return a.getBlogrollOutlines(blog)
	})
	if err != nil {
		log.Printf("Failed to get outlines: %v", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	c := a.cfg.Blogs[blog].Blogroll
	can := a.getRelativePath(blog, defaultIfEmpty(c.Path, defaultBlogrollPath))
	a.render(w, r, templateBlogroll, &renderData{
		BlogString: blog,
		Canonical:  a.getFullAddress(can),
		Data: map[string]interface{}{
			"Title":       c.Title,
			"Description": c.Description,
			"Outlines":    outlines,
			"Download":    can + ".opml",
		},
	})
}

func (a *goBlog) serveBlogrollExport(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	outlines, err, _ := a.blogrollCacheGroup.Do(blog, func() (interface{}, error) {
		return a.getBlogrollOutlines(blog)
	})
	if err != nil {
		log.Printf("Failed to get outlines: %v", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, contenttype.XMLUTF8)
	var opmlBytes bytes.Buffer
	_ = opml.Render(&opmlBytes, &opml.OPML{
		Version:     "2.0",
		DateCreated: time.Now().UTC(),
		Outlines:    outlines.([]*opml.Outline),
	})
	_, _ = a.min.Write(w, contenttype.XML, opmlBytes.Bytes())
}

func (a *goBlog) getBlogrollOutlines(blog string) ([]*opml.Outline, error) {
	config := a.cfg.Blogs[blog].Blogroll
	if cache := a.db.loadOutlineCache(blog); cache != nil {
		return cache, nil
	}
	req, err := http.NewRequest(http.MethodGet, config.Opml, nil)
	if err != nil {
		return nil, err
	}
	if config.AuthHeader != "" && config.AuthValue != "" {
		req.Header.Set(config.AuthHeader, config.AuthValue)
	}
	res, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
		res.Body.Close()
	}()
	if code := res.StatusCode; code < 200 || 300 <= code {
		return nil, fmt.Errorf("opml request not successful, status code: %d", code)
	}
	o, err := opml.Parse(res.Body)
	if err != nil {
		return nil, err
	}
	outlines := o.Outlines
	if len(config.Categories) > 0 {
		filtered := []*opml.Outline{}
		for _, category := range config.Categories {
			if outline, ok := funk.Find(outlines, func(outline *opml.Outline) bool {
				return outline.Title == category || outline.Text == category
			}).(*opml.Outline); ok && outline != nil {
				outline.Outlines = sortOutlines(outline.Outlines)
				filtered = append(filtered, outline)
			}
		}
		outlines = filtered
	} else {
		outlines = sortOutlines(outlines)
	}
	a.db.cacheOutlines(blog, outlines)
	return outlines, nil
}

func (db *database) cacheOutlines(blog string, outlines []*opml.Outline) {
	var opmlBuffer bytes.Buffer
	_ = opml.Render(&opmlBuffer, &opml.OPML{
		Version:     "2.0",
		DateCreated: time.Now().UTC(),
		Outlines:    outlines,
	})
	_ = db.cachePersistently("blogroll_"+blog, opmlBuffer.Bytes())
}

func (db *database) loadOutlineCache(blog string) []*opml.Outline {
	data, err := db.retrievePersistentCache("blogroll_" + blog)
	if err != nil || data == nil {
		return nil
	}
	o, err := opml.NewParser(bytes.NewReader(data)).Parse()
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

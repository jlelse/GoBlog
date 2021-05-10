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
	servertiming "github.com/mitchellh/go-server-timing"
	"github.com/thoas/go-funk"
	"golang.org/x/sync/singleflight"
)

var blogrollCacheGroup singleflight.Group

func serveBlogroll(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	t := servertiming.FromContext(r.Context()).NewMetric("bg").Start()
	outlines, err, _ := blogrollCacheGroup.Do(blog, func() (interface{}, error) {
		return getBlogrollOutlines(blog)
	})
	t.Stop()
	if err != nil {
		log.Println("Failed to get outlines:", err.Error())
		serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	if appConfig.Cache != nil && appConfig.Cache.Enable {
		setInternalCacheExpirationHeader(w, r, int(appConfig.Cache.Expiration))
	}
	c := appConfig.Blogs[blog].Blogroll
	render(w, r, templateBlogroll, &renderData{
		BlogString: blog,
		Data: map[string]interface{}{
			"Title":       c.Title,
			"Description": c.Description,
			"Outlines":    outlines,
			"Download":    c.Path + ".opml",
		},
	})
}

func serveBlogrollExport(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	outlines, err, _ := blogrollCacheGroup.Do(blog, func() (interface{}, error) {
		return getBlogrollOutlines(blog)
	})
	if err != nil {
		log.Println("Failed to get outlines:", err.Error())
		serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	if appConfig.Cache != nil && appConfig.Cache.Enable {
		setInternalCacheExpirationHeader(w, r, int(appConfig.Cache.Expiration))
	}
	w.Header().Set(contentType, contentTypeXMLUTF8)
	mw := minifier.Writer(contentTypeXML, w)
	defer func() {
		_ = mw.Close()
	}()
	_ = opml.Render(mw, &opml.OPML{
		Version:     "2.0",
		DateCreated: time.Now().UTC(),
		Outlines:    outlines.([]*opml.Outline),
	})
}

func getBlogrollOutlines(blog string) ([]*opml.Outline, error) {
	config := appConfig.Blogs[blog].Blogroll
	if cache := loadOutlineCache(blog); cache != nil {
		return cache, nil
	}
	req, err := http.NewRequest(http.MethodGet, config.Opml, nil)
	if err != nil {
		return nil, err
	}
	if config.AuthHeader != "" && config.AuthValue != "" {
		req.Header.Set(config.AuthHeader, config.AuthValue)
	}
	res, err := appHttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
		res.Body.Close()
	}()
	if code := res.StatusCode; code < 200 || 300 <= code {
		return nil, fmt.Errorf("opml request not successfull, status code: %d", code)
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
	cacheOutlines(blog, outlines)
	return outlines, nil
}

func cacheOutlines(blog string, outlines []*opml.Outline) {
	var opmlBuffer bytes.Buffer
	_ = opml.Render(&opmlBuffer, &opml.OPML{
		Version:     "2.0",
		DateCreated: time.Now().UTC(),
		Outlines:    outlines,
	})
	cachePersistently("blogroll_"+blog, opmlBuffer.Bytes())
}

func loadOutlineCache(blog string) []*opml.Outline {
	data, err := retrievePersistentCache("blogroll_" + blog)
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

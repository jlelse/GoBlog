package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/kaorimatz/go-opml"
	"github.com/thoas/go-funk"
	"golang.org/x/sync/singleflight"
)

var blogrollCacheGroup singleflight.Group

func serveBlogroll(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	c := appConfig.Blogs[blog].Blogroll
	if !c.Enabled {
		serve404(w, r)
		return
	}
	outlines, err, _ := blogrollCacheGroup.Do(blog, func() (interface{}, error) {
		return getBlogrollOutlines(c)
	})
	if err != nil {
		log.Println("Failed to get outlines:", err.Error())
		serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	if appConfig.Cache != nil && appConfig.Cache.Enable {
		setInternalCacheExpirationHeader(w, r, int(appConfig.Cache.Expiration))
	}
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
	c := appConfig.Blogs[blog].Blogroll
	if !c.Enabled {
		serve404(w, r)
		return
	}
	outlines, err, _ := blogrollCacheGroup.Do(blog, func() (interface{}, error) {
		return getBlogrollOutlines(c)
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

func getBlogrollOutlines(config *configBlogroll) ([]*opml.Outline, error) {
	if config.cachedOutlines != nil && time.Since(config.lastCache).Minutes() < 60 {
		// return cache if younger than 60 min
		return config.cachedOutlines, nil
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
	config.cachedOutlines = outlines
	config.lastCache = time.Now()
	return outlines, nil
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

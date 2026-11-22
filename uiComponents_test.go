package main

import (
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/htmlbuilder"
)

func Test_renderMainMenu(t *testing.T) {
	cfg := createDefaultTestConfig(t)
	app := &goBlog{cfg: cfg}
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initTemplateStrings())
	blog := cfg.Blogs[cfg.DefaultBlog]
	blog.Menus = map[string]*configMenu{"main": {Items: []*configMenuItem{
		{Title: "Subscribe", Link: "/feeds"},
		{Title: "Mehr auf Deutsch 🇩🇪", Link: "/de"},
	}}}

	var html strings.Builder
	app.renderBase(htmlbuilder.NewHTMLBuilder(&html), &renderData{Blog: blog, IsHome: true}, nil, nil)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html.String()))
	require.NoError(t, err)
	menu := doc.Find("header nav")
	require.Equal(t, 2, menu.Children().Length())
	require.Equal(t, "Subscribe", menu.Find("a[href='/feeds']").Text())
	require.Equal(t, "Mehr auf Deutsch 🇩🇪", menu.Find("a[href='/de']").Text())
}

func Test_renderSummaryMeta(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	require.NoError(t, app.initTemplateStrings())
	blog := &configBlog{
		Path:       "/de",
		Lang:       "en",
		Sections:   map[string]*configSection{"posts": {Name: "posts", Title: "Posts"}},
		Taxonomies: []*configTaxonomy{{Name: "tags"}, {Name: "series"}},
	}
	p := &post{
		Section:    "posts",
		Published:  "2026-11-22T12:00:00Z",
		Updated:    "2026-11-22T12:30:00Z",
		Status:     statusPublished,
		Visibility: visibilityPublic,
		Parameters: map[string][]string{
			"tags":   {"Zebra", "Bike rides", "Autumn", "Fourth"},
			"series": {"Not a tag"},
		},
	}
	render := func(typ string) *goquery.Document {
		var html strings.Builder
		app.renderPostMeta(htmlbuilder.NewHTMLBuilder(&html), p, blog, typ)
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html.String()))
		require.NoError(t, err)
		return doc
	}

	doc := render("summary")
	meta := doc.Find(".summary-meta")
	published := meta.Find("time.dt-published")
	require.Equal(t, toLocalTime(p.Published).Format(isoDateFormat), published.Text())
	require.Equal(t, toLocalTime(p.Published).Format(time.RFC3339), published.AttrOr("datetime", ""))
	updated := meta.Find("time.dt-updated[hidden]")
	require.Equal(t, toLocalTime(p.Updated).Format(time.RFC3339), updated.AttrOr("datetime", ""))
	require.Equal(t, "Posts", meta.Find("a[href='/de/posts']").Text())
	tags := meta.Find("a.p-category[rel=tag]")
	require.Equal(t, []string{"Zebra", "Bike rides", "Autumn"}, goquery.Map(tags, func(i int, s *goquery.Selection) string {
		return s.Text()
	}))
	require.Equal(t, "/de/tags/"+urlize("Bike rides"), tags.Eq(1).AttrOr("href", ""))
	require.NotContains(t, meta.Text(), "Published")
	require.NotContains(t, meta.Text(), "Updated")
	require.NotContains(t, meta.Text(), "Not a tag")

	doc = render("post")
	require.Empty(t, doc.Find(".summary-meta").Nodes)
	require.Equal(t, toLocalTime(p.Updated).Format(isoDateFormat), doc.Find("time.dt-updated:not([hidden])").Text())

	blog.Taxonomies = nil
	require.Empty(t, render("summary").Find("a[rel=tag]").Nodes)
}

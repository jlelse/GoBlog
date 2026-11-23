package main

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/htmlbuilder"
)

func Test_renderShareButton(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	require.NoError(t, app.initTemplateStrings())
	app.cfg.Server.PublicAddress = "https://blog.example"
	app.cfg.Server.ShortPublicAddress = "https://short.example"
	p := &post{Path: "/posts/c++", RenderedTitle: "C++\n& Go: " + strings.Repeat("界", 81)}
	var html strings.Builder
	app.renderShareButton(htmlbuilder.NewHTMLBuilder(&html), p, &configBlog{Lang: "en"})
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html.String()))
	require.NoError(t, err)
	var data shareData
	require.NoError(t, json.Unmarshal([]byte(doc.Find("#shareData").Text()), &data))
	require.Equal(t, "https://blog.example/posts/c++", data.URL)
	for _, svc := range data.Services {
		if svc.ID == "hackernews" {
			link, err := url.Parse(svc.URL)
			require.NoError(t, err)
			require.Equal(t, data.URL, link.Query().Get("u"))
			title := link.Query().Get("t")
			require.LessOrEqual(t, len([]rune(title)), 80)
			require.True(t, strings.HasPrefix(title, "C++ & Go: 界"))
			require.True(t, strings.HasSuffix(title, "…"))
			return
		}
	}
	t.Fatal("missing Hacker News share link")
}

func Test_renderMenus(t *testing.T) {
	cfg := createDefaultTestConfig(t)
	app := &goBlog{cfg: cfg}
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initTemplateStrings())
	blog := cfg.Blogs[cfg.DefaultBlog]
	blog.Menus = map[string]*configMenu{"main": {Items: []*configMenuItem{
		{Title: "Subscribe", Link: "/feeds"},
		{Title: "Mehr auf Deutsch 🇩🇪", Link: "/de"},
	}}}
	blog.Menus["footer"] = &configMenu{Items: []*configMenuItem{{Title: "Privacy policy", Link: "/privacy"}}}

	render := func(loggedIn bool) *goquery.Selection {
		req := httptest.NewRequest("GET", "/", nil)
		setLoggedIn(req, loggedIn)
		var html strings.Builder
		app.renderBase(htmlbuilder.NewHTMLBuilder(&html), &renderData{Blog: blog, IsHome: true, app: app, req: req}, nil, nil)
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html.String()))
		require.NoError(t, err)
		return doc.Selection
	}

	page := render(false)
	footer := page.Find("footer nav.menu-bar")
	require.Equal(t, "Footer menu", footer.AttrOr("aria-label", ""))
	require.Equal(t, "Privacy policy", footer.Find("a[href='/privacy']").Text())
	bar := page.Find("header .menu-bar")
	require.Equal(t, 1, bar.Length())
	require.Equal(t, 2, bar.Find("a").Length())
	require.Equal(t, "Subscribe", bar.Find("a[href='/feeds']").Text())
	require.Equal(t, "Mehr auf Deutsch 🇩🇪", bar.Find("a[href='/de']").Text())

	bar = render(true).Find("header .menu-bar")
	require.Equal(t, 1, bar.Length())
	require.Equal(t, "Main menu", bar.Find("nav").First().AttrOr("aria-label", ""))
	require.Equal(t, "User menu", bar.Find("nav").Last().AttrOr("aria-label", ""))
	require.Equal(t, "/logout", bar.Find("a").Last().AttrOr("href", ""))

	blog.Menus = nil
	require.Empty(t, render(false).Find(".menu-bar").Nodes)
	require.Equal(t, "User menu", render(true).Find("header nav:only-child").AttrOr("aria-label", ""))
}

func Test_renderPostMeta(t *testing.T) {
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
			"series": {"cycling"},
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
	meta := doc.Find(".post-meta")
	published := meta.Find("time.dt-published")
	require.Equal(t, toLocalTime(p.Published).Format(isoDateFormat), published.Text())
	require.Equal(t, toLocalTime(p.Published).Format(time.RFC3339), published.AttrOr("datetime", ""))
	updated := meta.Find("time.dt-updated[hidden]")
	require.Equal(t, toLocalTime(p.Updated).Format(time.RFC3339), updated.AttrOr("datetime", ""))
	require.Equal(t, "Posts", meta.Find("a[href='/de/posts']").Text())
	tags := meta.Find("a.p-category[rel=tag]")
	require.Equal(t, []string{"Autumn", "Bike rides", "cycling", "Fourth", "Zebra"}, goquery.Map(tags, func(i int, s *goquery.Selection) string {
		return s.Text()
	}))
	require.Equal(t, "/de/tags/"+urlize("Bike rides"), tags.Eq(1).AttrOr("href", ""))
	require.Equal(t, "/de/series/cycling", tags.Eq(2).AttrOr("href", ""))
	require.NotContains(t, meta.Text(), "Published")
	require.NotContains(t, meta.Text(), "Updated")

	doc = render("post")
	require.Equal(t, meta.Find(".post-meta-row").Text(), doc.Find(".post-meta-row").Text())
	require.Equal(t, toLocalTime(p.Updated).Format(isoDateFormat), doc.Find("time.dt-updated:not([hidden])").Text())

	blog.Taxonomies = nil
	require.Empty(t, render("summary").Find("a[rel=tag]").Nodes)
}

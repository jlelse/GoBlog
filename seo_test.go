package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/carlmjohnson/requests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seoTestApp initializes a test app with an optional blogs override applied
// before initConfig, and returns it with a ready router.
func seoTestApp(t *testing.T, blogs map[string]*configBlog) *goBlog {
	t.Helper()
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	if blogs != nil {
		app.cfg.Blogs = blogs
	}
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initTemplateStrings())
	app.d = app.buildRouter()
	return app
}

// seoFetch fetches a rendered page and parses it with goquery.
func seoFetch(t *testing.T, app *goBlog, path string) (*goquery.Document, string) {
	t.Helper()
	client := newHandlerClient(app.d)
	var resString string
	err := requests.
		URL("http://localhost:8080" + path).
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(resString))
	require.NoError(t, err)
	return doc, resString
}

func Test_seoPostOpenGraph(t *testing.T) {
	app := seoTestApp(t, nil)
	app.cfg.User.Name = "Test User"

	require.NoError(t, app.createPost(&post{
		Path:       "/testpost",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post"}, "images": {"/m/abc123.jpg"}},
		Content:    "Test Content",
	}))

	doc, resString := seoFetch(t, app, "/testpost")

	// og:type article
	assert.Equal(t, "article", doc.Find(`meta[property="og:type"]`).AttrOr("content", ""), resString)
	// og:title identical to the <title> text before the blog title suffix
	ogTitle, ok := doc.Find(`meta[property="og:title"]`).Attr("content")
	require.True(t, ok, "og:title missing: "+resString)
	assert.Equal(t, "Test Post", ogTitle)
	titleText := strings.TrimSuffix(doc.Find("title").Text(), " - "+app.cfg.Blogs[app.cfg.DefaultBlog].Title)
	assert.Equal(t, "Test Post", titleText)
	// og:description from the post summary
	ogDesc, ok := doc.Find(`meta[property="og:description"]`).Attr("content")
	require.True(t, ok, resString)
	assert.Equal(t, "Test Content", ogDesc)
	// og:image absolute, exactly once
	ogImages := doc.Find(`meta[property="og:image"]`)
	require.Len(t, ogImages.Nodes, 1, resString)
	ogImage, _ := ogImages.Attr("content")
	assert.Equal(t, "http://localhost:8080/m/abc123.jpg", ogImage)
	// article:published_time, same RFC3339 local-time formatting as before
	pubTime, ok := doc.Find(`meta[property="article:published_time"]`).Attr("content")
	require.True(t, ok, resString)
	assert.Equal(t, toLocalTime("2020-10-15T10:00:00Z").Format(time.RFC3339), pubTime)
}

func Test_seoPostJSONLD(t *testing.T) {
	app := seoTestApp(t, nil)
	app.cfg.User.Name = "Test User"

	require.NoError(t, app.createPost(&post{
		Path:       "/testpost",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post"}},
		Content:    "Test Content",
	}))

	doc, resString := seoFetch(t, app, "/testpost")

	scripts := doc.Find(`script[type="application/ld+json"]`)
	require.Len(t, scripts.Nodes, 1, resString)
	var ld map[string]any
	require.NoError(t, json.Unmarshal([]byte(scripts.Text()), &ld), resString)

	assert.Equal(t, "https://schema.org", ld["@context"])
	assert.Equal(t, "BlogPosting", ld["@type"])
	assert.Equal(t, "Test Post", ld["headline"])
	assert.Equal(t, "Test Content", ld["description"])
	assert.Equal(t, toLocalTime("2020-10-15T10:00:00Z").Format(time.RFC3339), ld["datePublished"])
	assert.Equal(t, "http://localhost:8080/testpost", ld["mainEntityOfPage"])
	assert.Equal(t, "en", ld["inLanguage"])
	author, ok := ld["author"].(map[string]any)
	require.True(t, ok, "author missing: "+resString)
	assert.Equal(t, "Person", author["@type"])
	assert.Equal(t, "Test User", author["name"])
	assert.Equal(t, app.getFullAddress(app.cfg.Blogs[app.cfg.DefaultBlog].getRelativePath("")), author["url"])
}

func Test_seoStaticHome(t *testing.T) {
	blogs := map[string]*configBlog{
		"default": createDefaultBlog(),
	}
	blogs["default"].PostAsHome = true
	app := seoTestApp(t, blogs)
	app.cfg.User.Name = "Test User"

	require.NoError(t, app.createPost(&post{
		Path:       app.cfg.Blogs[app.cfg.DefaultBlog].getRelativePath(""),
		Section:    "posts",
		Status:     "published",
		Published:  "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{"title": {"Home Post"}, "images": {"/m/abc123.jpg"}},
		Content:    "Home Content",
	}))

	doc, resString := seoFetch(t, app, "/")

	// No article tags, no og:title on the static home
	assert.Equal(t, "website", doc.Find(`meta[property="og:type"]`).AttrOr("content", ""), resString)
	assert.Len(t, doc.Find(`meta[property="og:title"]`).Nodes, 0, resString)
	assert.Len(t, doc.Find(`meta[property="article:published_time"]`).Nodes, 0, resString)
	// JSON-LD describes the website, not a BlogPosting
	scripts := doc.Find(`script[type="application/ld+json"]`)
	require.Len(t, scripts.Nodes, 1, resString)
	var ld map[string]any
	require.NoError(t, json.Unmarshal([]byte(scripts.Text()), &ld), resString)
	assert.Equal(t, "WebSite", ld["@type"])
	assert.Equal(t, "My Blog", ld["name"])
	assert.Equal(t, "http://localhost:8080", ld["url"])
	assert.NotContains(t, ld, "headline", resString)
	assert.NotContains(t, ld, "datePublished", resString)
	// og:image is absolute
	ogImage, ok := doc.Find(`meta[property="og:image"]`).Attr("content")
	require.True(t, ok, resString)
	assert.Equal(t, "http://localhost:8080/m/abc123.jpg", ogImage)
}

func Test_seoHreflang(t *testing.T) {
	app := seoTestApp(t, map[string]*configBlog{
		"en": {Path: "/", Lang: "en"},
		"de": {Path: "/de", Lang: "de"},
	})
	app.cfg.DefaultBlog = "en"
	app.cfg.User.Name = "Test User"

	require.NoError(t, app.createPost(&post{
		Path:       "/testpost",
		Blog:       "en",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post"}, "translationkey": {"t1"}},
		Content:    "Test Content",
	}))
	require.NoError(t, app.createPost(&post{
		Path:       "/de/testpost2",
		Blog:       "de",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post 2"}, "translationkey": {"t1"}},
		Content:    "Test Content 2",
	}))
	require.NoError(t, app.createPost(&post{
		Path:       "/de/draft",
		Blog:       "de",
		Section:    "posts",
		Status:     "draft",
		Parameters: map[string][]string{"title": {"Draft Post"}, "translationkey": {"t1"}},
		Content:    "Draft Content",
	}))

	doc, resString := seoFetch(t, app, "/testpost")

	hreflang := func(lang string) (string, bool) {
		var href string
		var found bool
		doc.Find(`link[rel="alternate"]`).Each(func(_ int, s *goquery.Selection) {
			if s.AttrOr("hreflang", "") == lang {
				href, found = s.Attr("href")
			}
		})
		return href, found
	}

	href, ok := hreflang("en")
	require.True(t, ok, "hreflang=en missing: "+resString)
	assert.Equal(t, "http://localhost:8080/testpost", href)
	href, ok = hreflang("de")
	require.True(t, ok, "hreflang=de missing: "+resString)
	assert.Equal(t, "http://localhost:8080/de/testpost2", href)
	href, ok = hreflang("x-default")
	require.True(t, ok, "hreflang=x-default missing: "+resString)
	assert.Equal(t, "http://localhost:8080/testpost", href)
	// Draft translations must not be listed
	assert.NotContains(t, resString, "/de/draft", resString)

	doc, resString = seoFetch(t, app, "/de/testpost2")

	href, ok = hreflang("en")
	require.True(t, ok, "hreflang=en missing: "+resString)
	assert.Equal(t, "http://localhost:8080/testpost", href)
	href, ok = hreflang("de")
	require.True(t, ok, "hreflang=de missing: "+resString)
	assert.Equal(t, "http://localhost:8080/de/testpost2", href)
	href, ok = hreflang("x-default")
	require.True(t, ok, "hreflang=x-default missing: "+resString)
	assert.Equal(t, "http://localhost:8080/testpost", href)
	// Draft translations must not be listed
	assert.NotContains(t, resString, "/de/draft", resString)
}

func Test_seoPostNoHreflangWithoutTranslationKey(t *testing.T) {
	app := seoTestApp(t, nil)

	require.NoError(t, app.createPost(&post{
		Path:       "/testpost",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post"}},
		Content:    "Test Content",
	}))

	_, resString := seoFetch(t, app, "/testpost")
	assert.NotContains(t, resString, "hreflang", resString)
}

func Test_seoHomeFeedLinks(t *testing.T) {
	app := seoTestApp(t, nil)

	require.NoError(t, app.createPost(&post{
		Path:       "/testpost",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post"}},
		Content:    "Test Content",
	}))

	countFeedLinks := func(doc *goquery.Document, feedType string) int {
		return doc.Find(`head > link[rel="alternate"][type="` + feedType + `"]`).Length()
	}

	t.Run("Homepage dedups feed links", func(t *testing.T) {
		doc, resString := seoFetch(t, app, "/")
		assert.Equal(t, 1, countFeedLinks(doc, "application/rss+xml"), resString)
		assert.Equal(t, 1, countFeedLinks(doc, "application/atom+xml"), resString)
		assert.Equal(t, 1, countFeedLinks(doc, "application/feed+json"), resString)
		assert.Contains(t, resString, "http://localhost:8080/.rss")
		assert.NotContains(t, resString, "http://localhost:8080/.rss\" title=\"RSS\"")
	})

	t.Run("Homepage keeps parameterized feed links", func(t *testing.T) {
		doc, resString := seoFetch(t, app, "/?p:tags=x")
		assert.Equal(t, 2, countFeedLinks(doc, "application/rss+xml"), resString)
		assert.Equal(t, 2, countFeedLinks(doc, "application/atom+xml"), resString)
		assert.Equal(t, 2, countFeedLinks(doc, "application/feed+json"), resString)
		assert.Contains(t, resString, "http://localhost:8080/.rss?p%3Atags=x")
	})

	t.Run("Section index keeps both feed sets", func(t *testing.T) {
		doc, resString := seoFetch(t, app, "/posts")
		assert.Equal(t, 2, countFeedLinks(doc, "application/rss+xml"), resString)
		assert.Equal(t, 2, countFeedLinks(doc, "application/atom+xml"), resString)
		assert.Equal(t, 2, countFeedLinks(doc, "application/feed+json"), resString)
		assert.Contains(t, resString, "http://localhost:8080/posts.rss")
	})
}

func Test_seoDateArchiveNoindex(t *testing.T) {
	app := seoTestApp(t, nil)

	require.NoError(t, app.createPost(&post{
		Path:       "/testpost",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post"}},
		Content:    "Test Content",
	}))

	fetchHeader := func(u string) string {
		var h http.Header
		err := requests.
			URL(u).
			CheckStatus(http.StatusOK).
			Client(newHandlerClient(app.d)).
			Handle(func(resp *http.Response) error {
				defer resp.Body.Close()
				h = resp.Header
				return nil
			}).
			Fetch(t.Context())
		require.NoError(t, err, u)
		return h.Get("X-Robots-Tag")
	}

	assert.Equal(t, "noindex", fetchHeader("http://localhost:8080/2020/10/15"))
	assert.Equal(t, "noindex", fetchHeader("http://localhost:8080/x/10"))
	assert.Equal(t, "noindex", fetchHeader("http://localhost:8080/x/10/15"))
	assert.Equal(t, "noindex", fetchHeader("http://localhost:8080/x/x/15"))
	assert.Equal(t, "noindex", fetchHeader("http://localhost:8080/2020/10"))
	assert.Equal(t, "noindex", fetchHeader("http://localhost:8080/2020"))
	assert.Empty(t, fetchHeader("http://localhost:8080/posts"))
	assert.Empty(t, fetchHeader("http://localhost:8080/"))
}

func Test_seoSearchNoindex(t *testing.T) {
	blogs := map[string]*configBlog{
		"default": createDefaultBlog(),
	}
	blogs["default"].Search = &configSearch{Enabled: true}
	app := seoTestApp(t, blogs)

	require.NoError(t, app.createPost(&post{
		Path:       "/testpost",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post"}},
		Content:    "Test Content",
	}))

	fetchHeader := func(u string) string {
		var h http.Header
		err := requests.
			URL(u).
			CheckStatus(http.StatusOK).
			Client(newHandlerClient(app.d)).
			Handle(func(resp *http.Response) error {
				defer resp.Body.Close()
				h = resp.Header
				return nil
			}).
			Fetch(t.Context())
		require.NoError(t, err, u)
		return h.Get("X-Robots-Tag")
	}

	assert.Equal(t, "noindex", fetchHeader("http://localhost:8080/search"))
	assert.Equal(t, "noindex", fetchHeader("http://localhost:8080/search/"+searchEncode("test")))
}

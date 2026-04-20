package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/carlmjohnson/requests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_sitemap(t *testing.T) {
	var err error

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)

	app.d = app.buildRouter()

	err = app.createPost(&post{
		Path:      "/testpost",
		Section:   "posts",
		Status:    "published",
		Published: "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{
			"title": {"Test Post"},
			"tags":  {"Test"},
		},
		Content: "Test Content",
	})
	require.NoError(t, err)

	client := newHandlerClient(app.d)

	var resString string

	err = requests.
		URL("http://localhost:8080/sitemap.xml").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "http://localhost:8080/sitemap-blog.xml")

	err = requests.
		URL("http://localhost:8080/sitemap-blog.xml").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "http://localhost:8080/sitemap-blog-posts.xml")
	assert.Contains(t, resString, "http://localhost:8080/sitemap-blog-features.xml")
	assert.Contains(t, resString, "http://localhost:8080/sitemap-blog-archives.xml")

	err = requests.
		URL("http://localhost:8080/sitemap-blog-posts.xml").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "http://localhost:8080/testpost")

	err = requests.
		URL("http://localhost:8080/sitemap-blog-archives.xml").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "http://localhost:8080/2020/10/15</loc>")
	assert.Contains(t, resString, "http://localhost:8080/2020/10</loc>")
	assert.Contains(t, resString, "http://localhost:8080/2020</loc>")
	assert.Contains(t, resString, "http://localhost:8080/x/10/15</loc>")
	assert.Contains(t, resString, "http://localhost:8080/x/x/15</loc>")
	assert.Contains(t, resString, "http://localhost:8080/tags/test</loc>")

	err = requests.
		URL("http://localhost:8080/sitemap-blog-features.xml").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "http://localhost:8080</loc>")
}

func Test_sitemapXMLValidity(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)
	app.d = app.buildRouter()

	err := app.createPost(&post{
		Path:      "/testpost",
		Section:   "posts",
		Status:    "published",
		Published: "2020-10-15T10:00:00Z",
		Content:   "Test Content",
	})
	require.NoError(t, err)

	client := newHandlerClient(app.d)

	sitemapPaths := []string{
		"http://localhost:8080/sitemap.xml",
		"http://localhost:8080/sitemap-blog.xml",
		"http://localhost:8080/sitemap-blog-features.xml",
		"http://localhost:8080/sitemap-blog-archives.xml",
		"http://localhost:8080/sitemap-blog-posts.xml",
	}

	for _, u := range sitemapPaths {
		var resString string
		err := requests.
			URL(u).
			CheckStatus(http.StatusOK).
			ToString(&resString).
			Client(client).Fetch(context.Background())
		require.NoError(t, err, u)
		assert.Contains(t, resString, "http://www.sitemaps.org/schemas/sitemap/0.9", "missing XML namespace in %s", u)
		assert.Contains(t, resString, "xml-stylesheet", "missing XSL stylesheet reference in %s", u)
	}
}

func Test_sitemapFeatures(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	bc := app.cfg.Blogs[app.cfg.DefaultBlog]
	bc.Photos = &configPhotos{Enabled: true}
	bc.Search = &configSearch{Enabled: true}
	bc.BlogStats = &configBlogStats{Enabled: true}
	bc.Map = &configGeoMap{Enabled: true}
	bc.Contact = &configContact{Enabled: true}

	app.d = app.buildRouter()

	client := newHandlerClient(app.d)

	var resString string
	err := requests.
		URL("http://localhost:8080/sitemap-blog-features.xml").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "http://localhost:8080</loc>")
	assert.Contains(t, resString, "http://localhost:8080/photos</loc>")
	assert.Contains(t, resString, "http://localhost:8080/search</loc>")
	assert.Contains(t, resString, "http://localhost:8080/statistics</loc>")
	assert.Contains(t, resString, "http://localhost:8080/map</loc>")
	assert.Contains(t, resString, "http://localhost:8080/contact</loc>")
}

func Test_sitemapPostsLastMod(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)
	app.d = app.buildRouter()

	err := app.createPost(&post{
		Path:      "/testpost",
		Section:   "posts",
		Status:    "published",
		Published: "2020-10-15T10:00:00Z",
		Updated:   "2023-05-20T12:00:00Z",
		Content:   "Test Content",
	})
	require.NoError(t, err)

	client := newHandlerClient(app.d)

	var resString string
	err = requests.
		URL("http://localhost:8080/sitemap-blog-posts.xml").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "http://localhost:8080/testpost")
	assert.Contains(t, resString, "2023-05-20")
}

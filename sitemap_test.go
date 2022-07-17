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
	app.initMarkdown()
	_ = app.initCache()

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

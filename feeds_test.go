package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_generateFeeds(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig()
	_ = app.initDatabase(false)
	app.initComponents(false)

	app.createPost(&post{
		Path:       "/testpost",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-01-01T00:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post"}},
		Content:    "Test Content",
	})
	posts, err := app.getPosts(&postsRequestConfig{
		status: "published",
	})
	require.NoError(t, err)
	require.Len(t, posts, 1)

	for _, typ := range []feedType{rssFeed, atomFeed, jsonFeed} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)

		app.generateFeed("default", typ, rec, req, posts, "Test-Title", "Test-Description")

		res := rec.Result()

		require.Equal(t, http.StatusOK, res.StatusCode)

		fp := gofeed.NewParser()
		feed, err := fp.Parse(res.Body)
		_ = res.Body.Close()

		require.NoError(t, err)
		require.NotNil(t, feed)

		switch typ {
		case rssFeed:
			assert.Equal(t, "rss", feed.FeedType)
		case atomFeed:
			assert.Equal(t, "atom", feed.FeedType)
		case jsonFeed:
			assert.Equal(t, "json", feed.FeedType)
		}

		assert.Equal(t, "Test-Title", feed.Title)
		assert.Equal(t, "Test-Description", feed.Description)

		if assert.Len(t, feed.Items, 1) {
			assert.Equal(t, "Test Post", feed.Items[0].Title)
			assert.Equal(t, "Test Content", feed.Items[0].Description)
		}
	}
}

package main

import (
	"net/http"
	"testing"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_feeds(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig()
	_ = app.initDatabase(false)
	app.initComponents(false)
	app.d, _ = app.buildRouter()

	err := app.createPost(&post{
		Path:       "/testpost",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-01-01T00:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post"}},
		Content:    "Test Content",
	})

	require.NoError(t, err)

	for _, typ := range []feedType{rssFeed, atomFeed, jsonFeed} {
		req, _ := http.NewRequest(http.MethodGet, "http://localhost:8080/posts."+string(typ), nil)
		res, err := doHandlerRequest(req, app.d)

		require.NoError(t, err)

		require.Equal(t, http.StatusOK, res.StatusCode)

		fp := gofeed.NewParser()
		feed, err := fp.Parse(res.Body)
		_ = res.Body.Close()

		require.NoError(t, err)
		require.NotNil(t, feed)

		assert.Equal(t, string(typ), feed.FeedType)

		if assert.Len(t, feed.Items, 1) {
			assert.Equal(t, "Test Post", feed.Items[0].Title)
			assert.Equal(t, "Test Content", feed.Items[0].Description)
		}
	}
}

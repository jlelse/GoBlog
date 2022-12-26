package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/carlmjohnson/requests"
	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_feeds(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)
	app.initMarkdown()
	_ = app.initTemplateStrings()
	_ = app.initCache()
	app.initSessions()

	app.d = app.buildRouter()
	handlerClient := newHandlerClient(app.d)

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
		var feed *gofeed.Feed
		err := requests.URL("http://localhost:8080/posts." + string(typ)).Client(handlerClient).
			Handle(func(r *http.Response) (err error) {
				fp := gofeed.NewParser()
				defer r.Body.Close()
				feed, err = fp.Parse(r.Body)
				return
			}).
			Fetch(context.Background())
		require.NoError(t, err)
		require.NotNil(t, feed)

		assert.Equal(t, string(typ), feed.FeedType)

		if assert.Len(t, feed.Items, 1) {
			assert.Equal(t, "Test Post", feed.Items[0].Title)
			assert.Equal(t, "Test Content", feed.Items[0].Description)
		}
	}
}

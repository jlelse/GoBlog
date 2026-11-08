package main

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	_ = app.initTemplateStrings()

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

func Test_feedCORS(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initTemplateStrings())
	app.d = app.buildRouter()

	t.Run("Feed CORS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/posts.rss", nil)
		req.Header.Set("Origin", "https://example.com")
		rr := httptest.NewRecorder()
		app.d.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("Feed CORS cached without Origin", func(t *testing.T) {
		app.purgeCache()
		// A request without an Origin header must not poison the cache for cross-origin clients.
		req := httptest.NewRequest(http.MethodGet, "/posts.rss", nil)
		rr := httptest.NewRecorder()
		app.d.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)

		req = httptest.NewRequest(http.MethodGet, "/posts.rss", nil)
		req.Header.Set("Origin", "https://example.com")
		rr = httptest.NewRecorder()
		app.d.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("Feed preflight", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/posts.rss", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", http.MethodGet)
		rr := httptest.NewRecorder()
		app.d.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, http.MethodGet, rr.Header().Get("Access-Control-Allow-Methods"))
	})

	t.Run("Non-feed path has no CORS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/not-a-feed.json", nil)
		req.Header.Set("Origin", "https://example.com")
		rr := httptest.NewRecorder()
		app.d.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	})
}

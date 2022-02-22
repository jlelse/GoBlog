package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_regexRedirects(t *testing.T) {

	app := &goBlog{
		cfg: &config{
			PathRedirects: []*configRegexRedirect{
				{
					From: "\\/index\\.xml",
					To:   ".rss",
					Type: 301,
				},
				{
					From: "^\\/(abc|def)\\/posts(.*)$",
					To:   "/$1$2",
				},
			},
		},
	}

	err := app.initRegexRedirects()
	require.NoError(t, err)

	h := app.checkRegexRedirects(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		_, _ = rw.Write([]byte("OK"))
	}))

	t.Run("First redirect", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts/index.xml", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		res := rec.Result()
		_ = res.Body.Close()

		assert.Equal(t, 301, res.StatusCode)
		assert.Equal(t, "/posts.rss", res.Header.Get("Location"))
	})

	t.Run("Second redirect", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/def/posts/test", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		res := rec.Result()
		_ = res.Body.Close()

		assert.Equal(t, http.StatusFound, res.StatusCode)
		assert.Equal(t, "/def/test", res.Header.Get("Location"))
	})

	t.Run("No redirect", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/posts", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		res := rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Empty(t, res.Header.Get("Location"))
		assert.Equal(t, "OK", string(resBody))
	})

}

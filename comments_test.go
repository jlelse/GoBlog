package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/contenttype"
)

func Test_comments(t *testing.T) {
	app := &goBlog{
		cfg: &config{
			Db: &configDb{
				File: filepath.Join(t.TempDir(), "test.db"),
			},
			Server: &configServer{
				PublicAddress: "https://example.com",
			},
			Blogs: map[string]*configBlog{
				"en": {
					Lang: "en",
				},
			},
			DefaultBlog: "en",
			User:        &configUser{},
		},
	}

	_ = app.initDatabase(false)
	app.initComponents(false)

	t.Run("Successful comment", func(t *testing.T) {

		// Create comment

		data := url.Values{}
		data.Add("target", "https://example.com/test")
		data.Add("comment", "This is just a test")
		data.Add("name", "Test name")
		data.Add("website", "https://goblog.app")

		req := httptest.NewRequest(http.MethodPost, commentPath, strings.NewReader(data.Encode()))
		req.Header.Add(contentType, contenttype.WWWForm)
		rec := httptest.NewRecorder()

		app.createComment(rec, req.WithContext(context.WithValue(req.Context(), blogKey, "en")))

		res := rec.Result()

		assert.Equal(t, http.StatusFound, res.StatusCode)
		assert.Equal(t, "/comment/1", res.Header.Get("Location"))

		// View comment

		mux := chi.NewMux()
		mux.Use(middleware.WithValue(blogKey, "en"))
		mux.Get("/comment/{id}", app.serveComment)

		req = httptest.NewRequest(http.MethodGet, "/comment/1", nil)
		rec = httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		res = rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		resBodyStr := string(resBody)

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Contains(t, resBodyStr, "https://goblog.app")
		assert.Contains(t, resBodyStr, "Test name")
		assert.Contains(t, resBodyStr, "This is just a test")
		assert.Contains(t, resBodyStr, "/test")

		// Count comments

		cc, err := app.db.countComments(&commentsRequestConfig{
			limit:  100,
			offset: 0,
		})
		require.NoError(t, err)
		assert.Equal(t, 1, cc)

		// Get comment

		comments, err := app.db.getComments(&commentsRequestConfig{})
		require.NoError(t, err)
		if assert.Len(t, comments, 1) {
			comment := comments[0]
			assert.Equal(t, "https://goblog.app", comment.Website)
			assert.Equal(t, "Test name", comment.Name)
			assert.Equal(t, "This is just a test", comment.Comment)
			assert.Equal(t, "/test", comment.Target)
		}

		// Delete comment

		err = app.db.deleteComment(1)
		require.NoError(t, err)
		cc, err = app.db.countComments(&commentsRequestConfig{})
		require.NoError(t, err)
		assert.Equal(t, 0, cc)

	})

	t.Run("Anonymous comment", func(t *testing.T) {

		// Create comment

		data := url.Values{}
		data.Add("target", "https://example.com/test")
		data.Add("comment", "This is just a test")

		req := httptest.NewRequest(http.MethodPost, commentPath, strings.NewReader(data.Encode()))
		req.Header.Add(contentType, contenttype.WWWForm)
		rec := httptest.NewRecorder()

		app.createComment(rec, req.WithContext(context.WithValue(req.Context(), blogKey, "en")))

		res := rec.Result()

		assert.Equal(t, http.StatusFound, res.StatusCode)
		assert.Equal(t, "/comment/2", res.Header.Get("Location"))

		// Get comment

		comments, err := app.db.getComments(&commentsRequestConfig{})
		require.NoError(t, err)
		if assert.Len(t, comments, 1) {
			comment := comments[0]
			assert.Equal(t, "/test", comment.Target)
			assert.Equal(t, "This is just a test", comment.Comment)
			assert.Equal(t, "Anonymous", comment.Name)
			assert.Equal(t, "", comment.Website)
		}

		// Delete comment

		err = app.db.deleteComment(2)
		require.NoError(t, err)

	})

	t.Run("Empty comment", func(t *testing.T) {

		data := url.Values{}
		data.Add("target", "https://example.com/test")
		data.Add("comment", "")

		req := httptest.NewRequest(http.MethodPost, commentPath, strings.NewReader(data.Encode()))
		req.Header.Add(contentType, contenttype.WWWForm)
		rec := httptest.NewRecorder()

		app.createComment(rec, req.WithContext(context.WithValue(req.Context(), blogKey, "en")))

		res := rec.Result()

		assert.Equal(t, http.StatusBadRequest, res.StatusCode)

		cc, err := app.db.countComments(&commentsRequestConfig{})
		require.NoError(t, err)
		assert.Equal(t, 0, cc)

	})

}

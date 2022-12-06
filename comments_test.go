package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/contenttype"
)

func Test_comments(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
			Comments: &configComments{
				Enabled: true,
			},
		},
	}
	app.cfg.DefaultBlog = "en"

	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()
	app.initSessions()

	t.Run("Successful comment", func(t *testing.T) {

		// Create comment

		data := url.Values{}
		data.Add("target", "http://localhost:8080/test")
		data.Add("comment", "This is just a test")
		data.Add("name", "Test name")
		data.Add("website", "https://goblog.app")

		req := httptest.NewRequest(http.MethodPost, commentPath, strings.NewReader(data.Encode()))
		req.Header.Add(contentType, contenttype.WWWForm)
		rec := httptest.NewRecorder()

		app.createCommentFromRequest(rec, req.WithContext(context.WithValue(req.Context(), blogKey, "en")))

		res := rec.Result()
		assert.Equal(t, http.StatusFound, res.StatusCode)
		assert.Equal(t, "/comment/1", res.Header.Get("Location"))
		_ = res.Body.Close()

		// View comment

		mux := chi.NewMux()
		mux.Use(middleware.WithValue(blogKey, "en"))
		mux.Get("/comment/{id}", app.serveComment)

		req = httptest.NewRequest(http.MethodGet, "/comment/1", nil)
		rec = httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		resBodyStr := rec.Body.String()

		assert.Equal(t, http.StatusOK, rec.Code)
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
		data.Add("target", "http://localhost:8080/test")
		data.Add("comment", "This is just a test")

		req := httptest.NewRequest(http.MethodPost, commentPath, strings.NewReader(data.Encode()))
		req.Header.Add(contentType, contenttype.WWWForm)
		rec := httptest.NewRecorder()

		app.createCommentFromRequest(rec, req.WithContext(context.WithValue(req.Context(), blogKey, "en")))

		res := rec.Result()
		assert.Equal(t, http.StatusFound, res.StatusCode)
		assert.Equal(t, "/comment/2", res.Header.Get("Location"))
		_ = res.Body.Close()

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
		data.Add("target", "http://localhost:8080/test")
		data.Add("comment", "")

		req := httptest.NewRequest(http.MethodPost, commentPath, strings.NewReader(data.Encode()))
		req.Header.Add(contentType, contenttype.WWWForm)
		rec := httptest.NewRecorder()

		app.createCommentFromRequest(rec, req.WithContext(context.WithValue(req.Context(), blogKey, "en")))

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		cc, err := app.db.countComments(&commentsRequestConfig{})
		require.NoError(t, err)
		assert.Equal(t, 0, cc)

	})

}

func Test_commentsEnabled(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	err := app.initConfig(false)
	require.NoError(t, err)

	assert.False(t, app.commentsEnabledForPost(&post{
		Blog: app.cfg.DefaultBlog,
	}))

	app.cfg.Blogs[app.cfg.DefaultBlog].Comments = &configComments{
		Enabled: true,
	}

	assert.True(t, app.commentsEnabledForPost(&post{
		Blog: app.cfg.DefaultBlog,
	}))
	assert.True(t, app.commentsEnabledForPost(&post{
		Blog: app.cfg.DefaultBlog,
		Parameters: map[string][]string{
			"comments": {"true"},
		},
	}))
	assert.False(t, app.commentsEnabledForPost(&post{
		Blog: app.cfg.DefaultBlog,
		Parameters: map[string][]string{
			"comments": {"false"},
		},
	}))

}

func Test_commentsUpdateByOriginal(t *testing.T) {

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	err := app.initConfig(false)
	require.NoError(t, err)
	err = app.initCache()
	require.NoError(t, err)
	app.initMarkdown()
	app.initSessions()

	bc := app.cfg.Blogs[app.cfg.DefaultBlog]

	addr, _, err := app.createComment(bc, "https://example.com/abc", "Test", "Name", "https://example.org", "https://example.org/1")
	require.NoError(t, err)

	splittedAddr := strings.Split(addr, "/")
	id := cast.ToInt(splittedAddr[len(splittedAddr)-1])

	comments, err := app.db.getComments(&commentsRequestConfig{id: id})
	require.NoError(t, err)
	require.Len(t, comments, 1)

	comment := comments[0]

	assert.Equal(t, "/abc", comment.Target)
	assert.Equal(t, "Test", comment.Comment)
	assert.Equal(t, "Name", comment.Name)
	assert.Equal(t, "https://example.org", comment.Website)
	assert.Equal(t, "https://example.org/1", comment.Original)

	_, _, err = app.createComment(bc, "https://example.com/abc", "Edited comment", "Edited name", "", "https://example.org/1")
	require.NoError(t, err)

	comments, err = app.db.getComments(&commentsRequestConfig{id: id})
	require.NoError(t, err)
	require.Len(t, comments, 1)

	comment = comments[0]

	assert.Equal(t, "/abc", comment.Target)
	assert.Equal(t, "Edited comment", comment.Comment)
	assert.Equal(t, "Edited name", comment.Name)
	assert.Equal(t, "", comment.Website)

}

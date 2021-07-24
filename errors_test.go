package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.goblog.app/app/pkgs/contenttype"
)

func Test_errors(t *testing.T) {
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
	app.initMarkdown()
	app.initSessions()
	_ = app.initTemplateStrings()
	_ = app.initRendering()

	t.Run("Test 404, no HTML", func(t *testing.T) {
		h := http.HandlerFunc(app.serve404)

		req := httptest.NewRequest(http.MethodGet, "/abc", nil)
		req.Header.Set("Accept", contenttype.JSON)

		rec := httptest.NewRecorder()

		h(rec, req)

		res := rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		assert.Equal(t, http.StatusNotFound, res.StatusCode)
		assert.Contains(t, resString, "not found")
		assert.Contains(t, res.Header.Get("Content-Type"), "text/plain")
	})

	t.Run("Test 404, HTML", func(t *testing.T) {
		h := http.HandlerFunc(app.serve404)

		req := httptest.NewRequest(http.MethodGet, "/abc", nil)
		req.Header.Set("Accept", contenttype.HTML)

		rec := httptest.NewRecorder()

		h(rec, req)

		res := rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		assert.Equal(t, http.StatusNotFound, res.StatusCode)
		assert.Contains(t, resString, "not found")
		assert.Contains(t, res.Header.Get("Content-Type"), contenttype.HTML)
	})

	t.Run("Test Method Not Allowed, no HTML", func(t *testing.T) {
		h := http.HandlerFunc(app.serveNotAllowed)

		req := httptest.NewRequest(http.MethodGet, "/abc", nil)
		req.Header.Set("Accept", contenttype.JSON)

		rec := httptest.NewRecorder()

		h(rec, req)

		res := rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		assert.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
		assert.Contains(t, resString, "Method Not Allowed")
		assert.Contains(t, res.Header.Get("Content-Type"), "text/plain")
	})

	t.Run("Test Method Not Allowed", func(t *testing.T) {
		h := http.HandlerFunc(app.serveNotAllowed)

		req := httptest.NewRequest(http.MethodGet, "/abc", nil)
		req.Header.Set("Accept", contenttype.HTML)

		rec := httptest.NewRecorder()

		h(rec, req)

		res := rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		assert.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
		assert.Contains(t, resString, "Method Not Allowed")
		assert.Contains(t, res.Header.Get("Content-Type"), contenttype.HTML)
	})
}

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/alice"
	"github.com/stretchr/testify/assert"
)

func Test_privateMode(t *testing.T) {

	// Init

	app := &goBlog{
		cfg: &config{
			Db: &configDb{
				File: filepath.Join(t.TempDir(), "db.db"),
			},
			Server: &configServer{},
			PrivateMode: &configPrivateMode{
				Enabled: true,
			},
			User: &configUser{
				Name:     "Test",
				Nick:     "test",
				Password: "testpw",
				AppPasswords: []*configAppPassword{
					{
						Username: "testapp",
						Password: "pw",
					},
				},
			},
			DefaultBlog: "en",
			Blogs: map[string]*configBlog{
				"en": {
					Lang: "en",
				},
			},
		},
	}

	_ = app.initDatabase(false)
	app.initComponents(false)

	handler := alice.New(middleware.WithValue(blogKey, "en"), app.privateModeHandler).ThenFunc(func(rw http.ResponseWriter, r *http.Request) {
		_, _ = rw.Write([]byte("Awesome"))
	})

	// Test check

	assert.True(t, app.isPrivate())

	// Test successful request

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.SetBasicAuth("testapp", "pw")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	res := rec.Result()
	resBody, _ := io.ReadAll(res.Body)
	resBodyStr := string(resBody)

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "Awesome", resBodyStr)

	// Test unauthenticated request

	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	rec = httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	res = rec.Result()
	resBody, _ = io.ReadAll(res.Body)
	resBodyStr = string(resBody)

	assert.Equal(t, http.StatusOK, res.StatusCode) // Not 401, to be compatible with some apps
	assert.NotEqual(t, "Awesome", resBodyStr)
	assert.NotContains(t, resBodyStr, "Awesome")
	assert.Contains(t, resBodyStr, "Login")

	// Disable private mode

	app.cfg.PrivateMode.Enabled = false

	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	rec = httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	res = rec.Result()
	resBody, _ = io.ReadAll(res.Body)
	resBodyStr = string(resBody)

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "Awesome", resBodyStr)

}

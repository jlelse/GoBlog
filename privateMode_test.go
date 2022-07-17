package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/alice"
	"github.com/stretchr/testify/assert"
)

func Test_privateMode(t *testing.T) {

	// Init

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.PrivateMode = &configPrivateMode{Enabled: true}
	app.cfg.User =
		&configUser{
			Name:     "Test",
			Nick:     "test",
			Password: "testpw",
			AppPasswords: []*configAppPassword{
				{
					Username: "testapp",
					Password: "pw",
				},
			},
		}
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
		},
	}

	_ = app.initConfig(false)
	app.initSessions()
	_ = app.initTemplateStrings()

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

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Awesome", rec.Body.String())

	// Test unauthenticated request

	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	rec = httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code) // Not 401, to be compatible with some apps
	assert.NotEqual(t, "Awesome", rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Awesome")
	assert.Contains(t, rec.Body.String(), "Login")

	// Disable private mode

	app.cfg.PrivateMode.Enabled = false

	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	rec = httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Awesome", rec.Body.String())

}

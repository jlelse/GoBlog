package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.hacdias.com/indielib/indieauth"
)

func Test_checkIndieAuth(t *testing.T) {

	app := &goBlog{
		httpClient: newFakeHttpClient().Client,
		cfg:        createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)
	app.initMarkdown()
	app.initSessions()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	checked1 := false
	app.checkIndieAuth(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		checked1 = true
	})).ServeHTTP(rec, req)
	assert.False(t, checked1)

	token, err := app.db.indieAuthSaveToken(&indieauth.AuthenticationRequest{
		ClientID: "https://example.com/",
		Scopes:   strings.Split("create update delete", " "),
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	req.Header.Set("Authorization", "Bearer "+token)

	checked2 := false
	app.checkIndieAuth(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "create update delete", r.Context().Value(indieAuthScope).(string))
		checked2 = true
	})).ServeHTTP(rec, req)
	assert.True(t, checked2)

}

func Test_addAllScopes(t *testing.T) {

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	checked := false
	addAllScopes(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		scope := r.Context().Value(indieAuthScope).(string)
		assert.Contains(t, scope, "create")
		assert.Contains(t, scope, "update")
		assert.Contains(t, scope, "delete")
		assert.Contains(t, scope, "undelete")
		assert.Contains(t, scope, "media")
		checked = true
	})).ServeHTTP(rec, req)
	assert.True(t, checked)

}

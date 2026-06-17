package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_checkOAuth(t *testing.T) {

	app := &goBlog{
		httpClient: newFakeHttpClient().Client,
		cfg:        createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	checked1 := false
	app.checkOAuth(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		checked1 = true
	})).ServeHTTP(rec, req)
	assert.False(t, checked1)

	token := uuid.NewString()
	_, err := app.db.Exec("insert into indieauthtoken (time, token, client, scope) values (?, ?, ?, ?)", time.Now().UTC().Unix(), token, "https://example.com/", "create update delete")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	req.Header.Set("Authorization", "Bearer "+token)

	checked2 := false
	app.checkOAuth(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "create update delete", r.Context().Value(oauthScope).(string))
		checked2 = true
	})).ServeHTTP(rec, req)
	assert.True(t, checked2)

}

func Test_addAllOAuthScopes(t *testing.T) {

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	checked := false
	addAllOAuthScopes(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		scope := r.Context().Value(oauthScope).(string)
		assert.Contains(t, scope, "create")
		assert.Contains(t, scope, "update")
		assert.Contains(t, scope, "delete")
		assert.Contains(t, scope, "undelete")
		assert.Contains(t, scope, "media")
		checked = true
	})).ServeHTTP(rec, req)
	assert.True(t, checked)

}

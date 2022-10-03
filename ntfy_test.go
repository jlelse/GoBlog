package main

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ntfySending(t *testing.T) {
	fakeClient := newFakeHttpClient()
	fakeClient.setHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {}))

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: fakeClient.Client,
	}

	_ = app.initConfig(false)

	t.Run("Default", func(t *testing.T) {
		app.cfg.Notifications = &configNotifications{
			Ntfy: &configNtfy{
				Enabled: true,
				Topic:   "topic",
			},
		}

		app.sendNotification("Test notification")

		req := fakeClient.req

		require.NotNil(t, req)
		assert.Equal(t, http.MethodPost, req.Method)
		assert.Equal(t, "https://ntfy.sh/topic", req.URL.String())

		reqBody, _ := req.GetBody()
		reqBodyByte, _ := io.ReadAll(reqBody)

		assert.Equal(t, "Test notification", string(reqBodyByte))

		res := fakeClient.res

		require.NotNil(t, res)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("Custom server with Basic Auth and Email", func(t *testing.T) {
		app.cfg.Notifications = &configNotifications{
			Ntfy: &configNtfy{
				Enabled: true,
				Topic:   "topic",
				Server:  "https://ntfy.example.com",
				User:    "user",
				Pass:    "pass",
				Email:   "test@example.com",
			},
		}

		app.sendNotification("Test notification")

		req := fakeClient.req

		require.NotNil(t, req)
		assert.Equal(t, http.MethodPost, req.Method)
		assert.Equal(t, "https://ntfy.example.com/topic", req.URL.String())
		assert.Equal(t, "test@example.com", req.Header.Get("X-Email"))

		user, pass, _ := req.BasicAuth()
		assert.Equal(t, "user", user)
		assert.Equal(t, "pass", pass)

		reqBody, _ := req.GetBody()
		reqBodyByte, _ := io.ReadAll(reqBody)

		assert.Equal(t, "Test notification", string(reqBodyByte))

		res := fakeClient.res

		require.NotNil(t, res)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})

}

func Test_ntfyConfig(t *testing.T) {
	var cfg *configNtfy

	assert.False(t, cfg.enabled())

	cfg = &configNtfy{}

	assert.False(t, cfg.enabled())

	cfg.Enabled = true

	assert.False(t, cfg.enabled())

	cfg.Topic = "topic"

	assert.True(t, cfg.enabled())
}

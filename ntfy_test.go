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
	app.cfg.Notifications = &configNotifications{
		Ntfy: &configNtfy{
			Enabled: true,
			Topic:   "example.com/topic",
		},
	}

	_ = app.initConfig()
	_ = app.initDatabase(false)
	app.initComponents(false)

	app.sendNotification("Test notification")

	req := fakeClient.req

	require.NotNil(t, req)
	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, "https://example.com/topic", req.URL.String())

	reqBody, _ := req.GetBody()
	reqBodyByte, _ := io.ReadAll(reqBody)

	assert.Equal(t, "Test notification", string(reqBodyByte))

	res := fakeClient.res

	require.NotNil(t, res)
	assert.Equal(t, http.StatusOK, res.StatusCode)
}

func Test_ntfyConfig(t *testing.T) {
	var cfg *configNtfy

	assert.False(t, cfg.enabled())

	cfg = &configNtfy{}

	assert.False(t, cfg.enabled())

	cfg.Enabled = true

	assert.False(t, cfg.enabled())

	cfg.Topic = "example.com/topic"

	assert.True(t, cfg.enabled())
}

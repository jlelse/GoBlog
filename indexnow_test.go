package main

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_indexNow(t *testing.T) {
	fc := newFakeHttpClient()
	fc.setFakeResponse(http.StatusOK, "OK")

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: fc.Client,
	}
	app.cfg.IndexNow = &configIndexNow{Enabled: true}

	_ = app.initConfig(false)
	_ = app.initCache()
	app.initIndexNow()

	// Create http router
	app.d = app.buildRouter()

	// Check key
	require.NotEmpty(t, app.inKey)
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://localhost:8080/"+string(app.inKey)+".txt", nil)
	res, err := doHandlerRequest(req, app.d)
	require.NoError(t, err)
	require.Equal(t, 200, res.StatusCode)
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	require.Equal(t, app.inKey, body)

	// Test publish post
	_ = app.createPost(&post{
		Section:   "posts",
		Path:      "/testpost",
		Published: "2022-01-01",
	})

	// Wait for hooks to run
	fc.mu.Lock()
	for fc.req == nil {
		fc.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		fc.mu.Lock()
	}
	fc.mu.Unlock()

	// Check fake http client
	require.Equal(t, "https://api.indexnow.org/indexnow?key="+string(app.inKey)+"&url=http%3A%2F%2Flocalhost%3A8080%2Ftestpost", fc.req.URL.String())
}

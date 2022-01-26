package main

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_indexNow(t *testing.T) {
	fc := newFakeHttpClient()
	fc.setFakeResponse(200, "OK")

	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: fc.Client,
	}
	app.cfg.IndexNow = &configIndexNow{Enabled: true}
	_ = app.initConfig()
	_ = app.initDatabase(false)
	defer app.db.close()
	app.initComponents(false)

	// Create http router
	app.d, _ = app.buildRouter()

	// Check key
	require.NotEmpty(t, app.inKey)
	req, _ := http.NewRequest("GET", "http://localhost:8080/"+app.inKey+".txt", nil)
	res, err := doHandlerRequest(req, app.d)
	require.NoError(t, err)
	require.Equal(t, 200, res.StatusCode)
	body, _ := io.ReadAll(res.Body)
	require.Equal(t, app.inKey, string(body))

	// Test publish post
	_ = app.createPost(&post{
		Section:   "posts",
		Path:      "/testpost",
		Published: "2022-01-01",
	})

	// Wait for hooks to run
	time.Sleep(300 * time.Millisecond)

	// Check fake http client
	require.NotNil(t, fc.req)
	require.Equal(t, "https://api.indexnow.org/indexnow?key="+app.inKey+"&url=http%3A%2F%2Flocalhost%3A8080%2Ftestpost", fc.req.URL.String())
}

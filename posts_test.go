package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/carlmjohnson/requests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_serveDate(t *testing.T) {
	var err error

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig()
	_ = app.initDatabase(false)
	app.initComponents(false)

	app.d, err = app.buildRouter()
	require.NoError(t, err)

	err = app.createPost(&post{
		Path:       "/testpost",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post"}},
		Content:    "Test Content",
	})

	require.NoError(t, err)

	client := &http.Client{
		Transport: &handlerRoundTripper{
			handler: app.d,
		},
	}

	var resString string

	err = requests.
		URL("http://localhost:8080/2020/10/15").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "Test Post")
	assert.Contains(t, resString, "<h1 class=p-name>2020-10-15</h1>")

	err = requests.
		URL("http://localhost:8080/2020/10").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "Test Post")
	assert.Contains(t, resString, "<h1 class=p-name>2020-10</h1>")

	err = requests.
		URL("http://localhost:8080/2020").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "Test Post")
	assert.Contains(t, resString, "<h1 class=p-name>2020</h1>")

	err = requests.
		URL("http://localhost:8080/x/10").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "Test Post")
	assert.Contains(t, resString, "<h1 class=p-name>XXXX-10</h1>")

	err = requests.
		URL("http://localhost:8080/x/x/15").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "Test Post")
	assert.Contains(t, resString, "<h1 class=p-name>XXXX-XX-15</h1>")

	err = requests.
		URL("http://localhost:8080/x").
		CheckStatus(http.StatusNotFound).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)
}

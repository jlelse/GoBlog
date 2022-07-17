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

	_ = app.initConfig(false)
	app.initMarkdown()
	_ = app.initCache()
	app.initSessions()
	_ = app.initTemplateStrings()

	app.d = app.buildRouter()

	err = app.createPost(&post{
		Path:       "/testpost",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post"}},
		Content:    "Test Content",
	})

	require.NoError(t, err)

	client := newHandlerClient(app.d)

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

func Test_servePost(t *testing.T) {
	var err error

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.User.AppPasswords = append(app.cfg.User.AppPasswords, &configAppPassword{
		Username: "test",
		Password: "test",
	})

	_ = app.initConfig(false)
	app.initMarkdown()
	_ = app.initCache()
	app.initSessions()
	_ = app.initTemplateStrings()

	app.d = app.buildRouter()

	// Create a post
	err = app.createPost(&post{
		Path:       "/testpost",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post"}},
		Content:    "Test Content",
	})
	require.NoError(t, err)

	client := newHandlerClient(app.d)

	var resString string

	// Check if the post is served
	err = requests.
		URL("http://localhost:8080/testpost").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "<h1 class=p-name>Test Post</h1>")

	// Delete the post
	err = app.deletePost("/testpost")
	require.NoError(t, err)

	// Check if the post is no longer served
	err = requests.
		URL("http://localhost:8080/testpost").
		CheckStatus(http.StatusGone).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "410 Gone")

	// Check if the post is still served for logged in user
	err = requests.
		URL("http://localhost:8080/testpost").
		BasicAuth("test", "test").
		CheckStatus(http.StatusGone).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "<h1 class=p-name>Test Post</h1>")

	// Undelete the post
	err = app.undeletePost("/testpost")
	require.NoError(t, err)

	// Check if the post is served
	err = requests.
		URL("http://localhost:8080/testpost").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "<h1 class=p-name>Test Post</h1>")

	// Delete the post completely
	err = app.deletePost("/testpost")
	require.NoError(t, err)
	err = app.deletePost("/testpost")
	require.NoError(t, err)

	// Check if the post is no longer served
	err = requests.
		URL("http://localhost:8080/testpost").
		BasicAuth("test", "test").
		CheckStatus(http.StatusGone).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.NotContains(t, resString, "<h1 class=p-name>Test Post</h1>")

}

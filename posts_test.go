package main

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/gophertools/requests/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexPagination(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initTemplateStrings())
	app.cfg.Blogs[app.cfg.DefaultBlog].Pagination = 1
	client := newHandlerClient(app.buildRouter())
	for i, tag := range []string{"notes", "notes", "notes", "other"} {
		require.NoError(t, app.createPost(&post{
			Path: fmt.Sprintf("/post-%d", i), Section: "posts", Status: statusPublished,
			Published: "2020-10-15T10:00:00Z", Content: "A note",
			Parameters: map[string][]string{"tags": {tag}},
		}))
	}
	const query = "?p%3Atags=notes"
	for _, tc := range []struct{ path, count, prev, next string }{
		{"/" + query, "Page 1 of 3", "", "/page/2" + query},
		{"/page/2" + query, "Page 2 of 3", "/" + query, "/page/3" + query},
		{"/page/99" + query, "Page 3 of 3", "/page/2" + query, ""},
		{"/?p%3Atags=other", "", "", ""},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res, err := client.Get("http://localhost:8080" + tc.path)
			require.NoError(t, err)
			defer res.Body.Close()
			require.Equal(t, http.StatusOK, res.StatusCode)
			doc, err := goquery.NewDocumentFromReader(res.Body)
			require.NoError(t, err)
			nav := doc.Find("nav.pagination")
			require.Equal(t, tc.count != "", nav.Length() == 1)
			require.Equal(t, tc.count, nav.Find("[aria-current=page]").Text())
			require.Equal(t, tc.prev, nav.Find("[rel=prev]").AttrOr("href", ""))
			require.Equal(t, tc.next, nav.Find("[rel=next]").AttrOr("href", ""))
		})
	}
}

func Test_serveDate(t *testing.T) {
	var err error

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)
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

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/gophertools/requests/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/contenttype"
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

func Test_serveUntitledPost(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.d = app.buildRouter()

	require.NoError(t, app.createPost(&post{
		Path:      "/untitled",
		Section:   "posts",
		Status:    "published",
		Published: "2020-10-15T10:00:00Z",
		Content:   "This untitled post contains more than thirty characters.",
	}))

	var resString string
	require.NoError(t, requests.
		URL("http://localhost:8080/untitled").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(newHandlerClient(app.d)).Fetch(context.Background()))

	assert.Contains(t, resString, "<h1 class=visually-hidden aria-hidden=true>This untitled post contains…</h1>")
}

func TestPublicPostCDNCache(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.ActivityPub.Enabled = true
	require.NoError(t, app.initConfig(false))
	app.cfg.Blogs[app.cfg.DefaultBlog].PostAsHome = true
	require.NoError(t, app.initTemplateStrings())
	handler := app.buildRouter()

	for path, visibility := range map[string]postVisibility{
		"/":         visibilityPublic,
		"/public":   visibilityPublic,
		"/unlisted": visibilityUnlisted,
		"/private":  visibilityPrivate,
	} {
		require.NoError(t, app.createPost(&post{
			Path: path, Section: "posts", Status: statusPublished, Visibility: visibility,
			Published: "2020-10-15T10:00:00Z", Content: "Test post",
		}))
	}

	request := func(method, path string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		for name, value := range headers {
			req.Header.Set(name, value)
		}
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}

	public := request(http.MethodGet, "/public?source=test", nil)
	require.Equal(t, http.StatusOK, public.Code)
	require.Equal(t, cdnCacheableHTML, public.Header().Get(cdnCacheableHeader))
	tag := public.Header().Get("ETag")
	require.NotEmpty(t, tag)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		res := request(method, "/public?source=test", map[string]string{"If-None-Match": "W/" + tag})
		assert.Equal(t, http.StatusNotModified, res.Code)
		assert.Empty(t, res.Body.String())
		assert.Equal(t, tag, res.Header().Get("ETag"))
	}
	assert.Equal(t, http.StatusOK, request(http.MethodGet, "/public?source=test",
		map[string]string{"If-None-Match": `W/"old"`}).Code)

	for _, tc := range []struct {
		name    string
		path    string
		headers map[string]string
	}{
		{name: "static homepage", path: "/"},
		{name: "unlisted post", path: "/unlisted"},
		{name: "private post", path: "/private"},
		{name: "missing post", path: "/missing"},
		{name: "cache bypass", path: "/public?cache=0"},
		{name: "cookie", path: "/public?source=test", headers: map[string]string{"Cookie": "c=probe"}},
		{name: "authorization", path: "/public?source=test", headers: map[string]string{"Authorization": "Bearer probe"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := request(http.MethodGet, tc.path, tc.headers)
			assert.Empty(t, res.Header().Values(cdnCacheableHeader))
		})
	}

	activity := request(http.MethodGet, "/public?source=test", map[string]string{"Accept": contenttype.AS})
	assert.Equal(t, http.StatusOK, activity.Code)
	assert.Contains(t, activity.Header().Get(contentType), contenttype.AS)
	assert.Empty(t, activity.Header().Values(cdnCacheableHeader))
}

func TestPublicIndexAndFeedCDNCache(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	require.NoError(t, app.initConfig(false))
	app.cfg.Blogs[app.cfg.DefaultBlog].Search = &configSearch{Enabled: true}
	require.NoError(t, app.initTemplateStrings())
	handler := app.buildRouter()

	require.NoError(t, app.createPost(&post{
		Path: "/hello", Section: "posts", Status: statusPublished, Visibility: visibilityPublic,
		Published: "2020-10-15T10:00:00Z", Content: "Test post",
		Parameters: map[string][]string{"tags": {"go"}},
	}))

	request := func(path string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		for name, value := range headers {
			req.Header.Set(name, value)
		}
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		return res
	}

	for _, tc := range []struct {
		name, path, marker string
	}{
		{name: "home", path: "/", marker: cdnCacheableHTML},
		{name: "home feed", path: "/.rss", marker: cdnCacheableFeed},
		{name: "section", path: "/posts", marker: cdnCacheableHTML},
		{name: "section feed", path: "/posts.rss", marker: cdnCacheableFeed},
		{name: "section atom feed", path: "/posts.atom", marker: cdnCacheableFeed},
		{name: "section json feed", path: "/posts.json", marker: cdnCacheableFeed},
		{name: "section min feed", path: "/posts.min.rss", marker: cdnCacheableFeed},
		{name: "taxonomy", path: "/tags/go", marker: cdnCacheableHTML},
		{name: "taxonomy feed", path: "/tags/go.rss", marker: cdnCacheableFeed},
		{name: "date", path: "/2020/10/15", marker: cdnCacheableHTML},
		{name: "date feed", path: "/2020/10/15.rss", marker: cdnCacheableFeed},
		{name: "search", path: "/search/" + searchEncode("Test"), marker: cdnCacheableHTML},
		{name: "search feed", path: "/search/" + searchEncode("Test") + ".rss", marker: cdnCacheableFeed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := request(tc.path, nil)
			require.Equal(t, http.StatusOK, res.Code)
			assert.Equal(t, tc.marker, res.Header().Get(cdnCacheableHeader))
		})
	}

	feed := request("/posts.rss", nil)
	require.Equal(t, http.StatusOK, feed.Code)
	feedTag := feed.Header().Get("ETag")
	require.NotEmpty(t, feedTag)
	for _, ifNoneMatch := range []string{"W/" + feedTag, "*"} {
		notModified := request("/posts.rss", map[string]string{"If-None-Match": ifNoneMatch})
		assert.Equal(t, http.StatusNotModified, notModified.Code)
		assert.Empty(t, notModified.Body.String())
		assert.Equal(t, feedTag, notModified.Header().Get("ETag"))
		assert.Equal(t, cdnCacheableFeed, notModified.Header().Get(cdnCacheableHeader))
	}

	for _, tc := range []struct {
		name    string
		path    string
		headers map[string]string
	}{
		{name: "cache bypass", path: "/posts.rss?cache=0"},
		{name: "cookie", path: "/posts.rss", headers: map[string]string{"Cookie": "c=probe"}},
		{name: "authorization", path: "/posts.rss", headers: map[string]string{"Authorization": "Bearer probe"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Empty(t, request(tc.path, tc.headers).Header().Values(cdnCacheableHeader))
		})
	}
}

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_collectExternalDomains(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.Server.PublicAddress = "https://example.com"
	must.NoError(app.initConfig(false))
	must.NoError(app.initTemplateStrings())
	app.d = app.buildRouter()

	defaultBlog := app.cfg.DefaultBlog

	must.NoError(app.createPost(&post{
		Path:       "/post-a",
		Blog:       defaultBlog,
		Status:     statusPublished,
		Visibility: visibilityPublic,
		Section:    "posts",
		Published:  "2024-01-01T00:00:00Z",
		Content:    "Visit [external](https://external.example.org/page) and [target](https://target.example.net/x).",
		Parameters: map[string][]string{"title": {"Post A"}},
	}))
	must.NoError(app.createPost(&post{
		Path:       "/post-b",
		Blog:       defaultBlog,
		Status:     statusPublished,
		Visibility: visibilityPublic,
		Section:    "posts",
		Published:  "2024-01-02T00:00:00Z",
		Content:    "Another link to [external](https://external.example.org/another) and an internal one to [self](https://example.com/x).",
		Parameters: map[string][]string{"title": {"Post B"}},
	}))
	must.NoError(app.createPost(&post{
		Path:       "/post-c",
		Blog:       defaultBlog,
		Status:     statusPublished,
		Visibility: visibilityPublic,
		Section:    "posts",
		Published:  "2024-01-03T00:00:00Z",
		Content:    "No external links here.",
		Parameters: map[string][]string{"title": {"Post C"}},
	}))

	stats, err := app.collectExternalDomains(defaultBlog)
	must.NoError(err)
	byDomain := map[string]*linkDomainStat{}
	for _, s := range stats {
		byDomain[s.Domain] = s
	}

	// external.example.org is linked from /post-a and /post-b (2 links)
	ex := byDomain["external.example.org"]
	must.NotNil(ex)
	is.Equal(2, len(ex.Posts))
	is.Equal(2, ex.LinkCount)

	// target.example.net is linked from /post-a only
	tx := byDomain["target.example.net"]
	must.NotNil(tx)
	is.Equal(1, len(tx.Posts))
	is.Equal(1, tx.LinkCount)

	// Public host is excluded
	_, internalSeen := byDomain["example.com"]
	is.False(internalSeen)
}

func Test_serveEditorLinks(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.Server.PublicAddress = "https://example.com"
	must.NoError(app.initConfig(false))
	must.NoError(app.initTemplateStrings())
	app.d = app.buildRouter()

	defaultBlog := app.cfg.DefaultBlog

	must.NoError(app.createPost(&post{
		Path:       "/post-a",
		Blog:       defaultBlog,
		Status:     statusPublished,
		Visibility: visibilityPublic,
		Section:    "posts",
		Published:  "2024-01-01T00:00:00Z",
		Content:    "Visit [external](https://external.example.org/page).",
		Parameters: map[string][]string{"title": {"Post A"}},
	}))

	t.Run("domain list page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/editor/links", nil)
		req = req.WithContext(context.WithValue(req.Context(), blogKey, defaultBlog))
		rec := httptest.NewRecorder()
		app.serveEditorLinks(rec, req)
		is.Equal(http.StatusOK, rec.Code)
		body := rec.Body.String()
		is.Contains(body, "external.example.org")
	})

	t.Run("domain detail page lists posts", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/editor/links?domain=external.example.org", nil)
		req = req.WithContext(context.WithValue(req.Context(), blogKey, defaultBlog))
		rec := httptest.NewRecorder()
		app.serveEditorLinks(rec, req)
		is.Equal(http.StatusOK, rec.Code)
		body := rec.Body.String()
		is.Contains(body, "Post A")
		is.Contains(body, "/post-a")
	})
}

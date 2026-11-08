package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

var _ plugintypes.App = &goBlog{}
var _ plugintypes.Database = &database{}
var _ plugintypes.Post = &post{}
var _ plugintypes.RenderContext = &pluginRenderContext{}

func TestDemoPlugin(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{
			Path:   "embedded:demo",
			Import: "demo",
			Config: map[string]any{
				"prio": 99,
			},
		},
	}

	err := app.initConfig(false)
	require.NoError(t, err)
	err = app.initPlugins()
	require.NoError(t, err)

	middlewarePlugins := app.getPlugins(pluginMiddlewareType)
	if assert.Len(t, middlewarePlugins, 1) {
		mdw := middlewarePlugins[0].(plugintypes.Middleware)
		assert.Equal(t, 99, mdw.Prio())
	}
}

func TestDemoPluginUIRendering(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{
			Path:   "embedded:demo",
			Import: "demo",
		},
	}

	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initPlugins())

	req := httptest.NewRequest(http.MethodGet, "/demo-path", nil)
	rr := httptest.NewRecorder()

	app.render(rr, req, func(hb *htmlbuilder.HTMLBuilder, _ *renderData) {
		hb.WriteElementOpen("main", "class", "h-entry")
		hb.WriteElementOpen("article")
		hb.WriteElementOpen("div", "class", "e-content")
		hb.WriteEscaped("Original content")
		hb.WriteElementClose("div")
		hb.WriteElementClose("article")
		hb.WriteElementClose("main")
	}, &renderData{
		BlogString: app.cfg.DefaultBlog,
	})

	body := rr.Body.String()
	assert.Contains(t, body, "Original content")
	assert.Contains(t, body, "End of post content")
	assert.Contains(t, body, "Second end of post content")
}

func TestDemoPluginUISummaryAndExec(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{
			Path:   "embedded:demo",
			Import: "demo",
		},
	}

	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initPlugins())

	// Verify UISummary modifies document
	summaryPlugins := app.getPlugins(pluginUISummaryType)
	require.Len(t, summaryPlugins, 1)
	sp := summaryPlugins[0].(plugintypes.UISummary)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<div class="h-entry"></div>`))
	require.NoError(t, err)

	sp.RenderSummaryForPost(&pluginRenderContext{
		blog: "default",
		path: "/demo-summary",
		url:  "http://example.com/demo-summary",
	}, &post{Path: "/demo-summary"}, doc)

	assert.Contains(t, doc.Find(".h-entry").Text(), "/demo-summary")

	// Exec should be present and callable
	execPlugins := app.getPlugins(pluginExecType)
	require.NotEmpty(t, execPlugins)
	for _, ep := range execPlugins {
		ep.(plugintypes.Exec).Exec()
	}
}

func TestPluginInterfaceFunctionality(t *testing.T) {

	t.Run("Test create post", func(t *testing.T) {
		app := &goBlog{
			cfg: createDefaultTestConfig(t),
		}

		err := app.initConfig(false)
		require.NoError(t, err)

		p, err := app.CreatePost(`---
title: Test post
---
Test post content`)
		require.NoError(t, err)
		assert.Equal(t, "Test post", p.GetTitle())
		assert.Equal(t, "Test post content", p.GetContent())
	})

}

func TestEmbeddedPluginImageTooltipsAndCustomCSS(t *testing.T) {
	tmpDir := t.TempDir()
	cssFile := filepath.Join(tmpDir, "custom.css")
	require.NoError(t, os.WriteFile(cssFile, []byte(".x{color:red;}"), 0o644))

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{
			Path:   "embedded:imagetooltips",
			Import: "imagetooltips",
		},
		{
			Path:   "embedded:customcss",
			Import: "customcss",
			Config: map[string]any{"file": cssFile},
		},
	}

	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initTemplateAssets())
	require.NoError(t, app.initPlugins())
	app.assetFileNames = map[string]string{}
	app.assetFiles = map[string]*assetFile{}

	req := httptest.NewRequest(http.MethodGet, "/p/img", nil)
	rr := httptest.NewRecorder()

	app.render(rr, req, func(hb *htmlbuilder.HTMLBuilder, _ *renderData) {
		hb.WriteElementOpen("head")
		hb.WriteElementClose("head")
		hb.WriteElementOpen("main")
		hb.WriteElementOpen("div", "class", "e-content")
		hb.WriteElementOpen("a", "href", "#")
		hb.WriteElementOpen("img", "src", "/image.jpg", "title", "tooltip")
		hb.WriteElementClose("img")
		hb.WriteElementClose("a")
		hb.WriteElementClose("div")
		hb.WriteElementClose("main")
	}, &renderData{
		BlogString: app.cfg.DefaultBlog,
	})

	body := rr.Body.String()
	assert.NotContains(t, body, "<a href=\"#\"><img") // anchor removed
	assert.Contains(t, app.assetFileNames, "imagetooltips.js")
	assert.Contains(t, app.assetFileNames, "plugincustomcss.css")
}

func TestEmbeddedPluginSyndication(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{
			Path:   "embedded:syndication",
			Import: "syndication",
		},
	}

	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initPlugins())
	app.assetFileNames = map[string]string{}
	app.assetFiles = map[string]*assetFile{}

	createdPost, err := app.CreatePost(`---
title: Syndication Test
path: /syndicated
syndication:
  - https://example.com/post
---
content`)
	require.NoError(t, err)
	p := createdPost.(*post)

	req := httptest.NewRequest(http.MethodGet, p.Path, nil)
	rr := httptest.NewRecorder()

	app.render(rr, req, func(hb *htmlbuilder.HTMLBuilder, _ *renderData) {
		hb.WriteElementOpen("main", "class", "h-entry")
		hb.WriteElementOpen("article")
		hb.WriteElementOpen("div", "class", "e-content")
		hb.WriteEscaped(p.Content)
		hb.WriteElementClose("div")
		hb.WriteElementClose("article")
		hb.WriteElementClose("main")
	}, &renderData{
		BlogString: app.cfg.DefaultBlog,
	})

	body := rr.Body.String()
	assert.Contains(t, body, `class="u-syndication hide"`)
	assert.Contains(t, body, "https://example.com/post")
}

func TestEmbeddedPluginSocialSpec(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.DefaultBlog = "en"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
			Blogroll: &configBlogroll{
				Enabled: true,
				Opml:    "https://example.com/blogroll.opml",
			},
		},
	}
	app.cfg.Plugins = []*configPlugin{
		{
			Path:   "embedded:socialspec",
			Import: "socialspec",
			Config: map[string]any{
				"blog":      "en",
				"title":     "Jane Doe",
				"aboutpage": "/about-jane",
				"contact": map[string]any{
					"name":  "Jane Doe",
					"email": "jane@example.com",
					"url":   "https://example.com",
				},
			},
		},
	}

	require.NoError(t, app.initConfig(false))
	app.cfg.MediaOptimization.Enabled = true
	require.NoError(t, app.db.mediaOptimizedInsert(&mediaOptimizedRow{
		OriginalHash: "abc123", VariantType: "jpeg_800", OptimizedHash: "h2", Width: 800, Height: 600,
	}))
	_, err := app.CreatePost(`---
title: About Jane
path: /about-jane
summary: Eager explorer
---
### About Jane

Hello from Jane.

![Jane](/m/abc123.jpg)`)
	require.NoError(t, err)
	require.NoError(t, app.initPlugins())

	middlewarePlugins := app.getPlugins(pluginMiddlewareType)
	require.Len(t, middlewarePlugins, 1)
	handler := middlewarePlugins[0].(plugintypes.Middleware).Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	serve := func(method, path string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Origin", "https://example.com")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}

	t.Run("Following", func(t *testing.T) {
		rr := serve(http.MethodGet, "/.well-known/following.opml", nil)
		assert.Equal(t, http.StatusFound, rr.Code)
		assert.Equal(t, "/blogroll.opml", rr.Header().Get("Location"))
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("About", func(t *testing.T) {
		rr := serve(http.MethodGet, "/.well-known/about.md", nil)
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "text/markdown; charset=utf-8", rr.Header().Get("Content-Type"))
		assert.Equal(t, "public,no-cache", rr.Header().Get("Cache-Control"))
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
		body := rr.Body.String()
		assert.True(t, strings.HasPrefix(body, "---\nsummary: Eager explorer\n---\n\n"))
		assert.Contains(t, body, "### About Jane")
		assert.Contains(t, body, "![Jane](/m/h2.jpeg)")
		assert.NotContains(t, body, "/m/abc123.jpg")
	})

	t.Run("Contact", func(t *testing.T) {
		rr := serve(http.MethodGet, "/.well-known/contact.vcf", nil)
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "text/vcard; charset=utf-8", rr.Header().Get("Content-Type"))
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
		body := rr.Body.String()
		assert.Contains(t, body, "BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Jane Doe\r\n")
		assert.Contains(t, body, "EMAIL;PREF=1:jane@example.com\r\n")
		assert.Contains(t, body, "URL:https://example.com\r\n")
		assert.True(t, strings.HasSuffix(body, "END:VCARD\r\n"))
	})

	t.Run("FeedAliases", func(t *testing.T) {
		for path, target := range map[string]string{
			"/feed.xml":  "/.rss",
			"/feed.json": "/.json",
		} {
			rr := serve(http.MethodGet, path, nil)
			assert.Equal(t, http.StatusMovedPermanently, rr.Code)
			assert.Equal(t, target, rr.Header().Get("Location"))
			assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
		}

		rr := serve(http.MethodGet, "/feed.xml?fallbacktitle", nil)
		assert.Equal(t, "/.rss", rr.Header().Get("Location"))
	})

	t.Run("Cache", func(t *testing.T) {
		_, err := app.db.Exec("update posts set content = ? where path = ?", "### Changed", "/about-jane")
		require.NoError(t, err)
		rr := serve(http.MethodGet, "/.well-known/about.md", nil)
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Hello from Jane.")
		assert.NotContains(t, rr.Body.String(), "### Changed")

		app.purgeCache()
		rr = serve(http.MethodGet, "/.well-known/about.md", nil)
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "### Changed")
		assert.NotContains(t, rr.Body.String(), "Hello from Jane.")
	})

	t.Run("CacheWithoutOrigin", func(t *testing.T) {
		app.purgeCache()
		req := httptest.NewRequest(http.MethodGet, "/.well-known/about.md", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)

		rr = serve(http.MethodGet, "/.well-known/about.md", nil)
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("ETag", func(t *testing.T) {
		rr := serve(http.MethodGet, "/.well-known/about.md", nil)
		etag := rr.Header().Get("ETag")
		require.NotEmpty(t, etag)

		rr = serve(http.MethodGet, "/.well-known/about.md", map[string]string{"If-None-Match": etag})
		assert.Equal(t, http.StatusNotModified, rr.Code)
		assert.Empty(t, rr.Body.String())
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("HeadAndOptions", func(t *testing.T) {
		rr := serve(http.MethodHead, "/.well-known/contact.vcf", nil)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotEmpty(t, rr.Header().Get("ETag"))

		rr = serve(http.MethodOptions, "/.well-known/following.opml", map[string]string{"Access-Control-Request-Method": http.MethodGet})
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET", rr.Header().Get("Access-Control-Allow-Methods"))

		rr = serve(http.MethodPost, "/.well-known/following.opml", nil)
		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	})

	t.Run("UnknownPath", func(t *testing.T) {
		rr := serve(http.MethodGet, "/.well-known/unknown", nil)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestEmbeddedPluginWebringsFooter(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{
			Path:   "embedded:webrings",
			Import: "webrings",
			Config: map[string]any{
				"default": []any{
					map[string]any{
						"title": "My Ring",
						"link":  "https://ring.example.com",
						"prev":  "https://ring.example.com/prev",
						"next":  "https://ring.example.com/next",
					},
				},
			},
		},
	}

	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initPlugins())

	footerPlugins := app.getPlugins(pluginUIFooterType)
	require.Len(t, footerPlugins, 1)
	fp := footerPlugins[0].(plugintypes.UIFooter)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<footer></footer>"))
	require.NoError(t, err)

	fp.RenderFooter(&pluginRenderContext{
		blog: app.cfg.DefaultBlog,
	}, doc)

	html, err := doc.Find("footer").Html()
	require.NoError(t, err)
	assert.Contains(t, html, "My Ring")
	assert.Contains(t, html, "ring.example.com")
	assert.Contains(t, html, "→")
	assert.Contains(t, html, "←")
}

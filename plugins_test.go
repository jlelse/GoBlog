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

	app.render(rr, req, func(hb *htmlbuilder.HtmlBuilder, _ *renderData) {
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
	summaryPlugins := app.getPlugins(pluginUiSummaryType)
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
	require.NoError(t, app.initPlugins())
	app.assetFileNames = map[string]string{}
	app.assetFiles = map[string]*assetFile{}

	req := httptest.NewRequest(http.MethodGet, "/p/img", nil)
	rr := httptest.NewRecorder()

	app.render(rr, req, func(hb *htmlbuilder.HtmlBuilder, _ *renderData) {
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

	app.render(rr, req, func(hb *htmlbuilder.HtmlBuilder, _ *renderData) {
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

	footerPlugins := app.getPlugins(pluginUiFooterType)
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

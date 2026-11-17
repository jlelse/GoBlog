package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/htmlbuilder"
)

func writeTestPlugin(t *testing.T, root, importPath, source string) {
	t.Helper()
	pluginDir := filepath.Join(root, "src", importPath)
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, importPath+".go"), []byte(source), 0o600))
}

func TestPluginPanicIsolation(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "panicplugin", `package panicplugin

import (
	"io"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct{}

func GetPlugin() (
	plugintypes.SetApp,
	plugintypes.SetConfig,
	plugintypes.Exec,
	plugintypes.PostCreatedHook,
	plugintypes.BlockedBots,
	plugintypes.UIImgAttributes,
	plugintypes.UI,
	plugintypes.UI2,
	plugintypes.Routes,
) {
	p := &plugin{}
	return p, p, p, p, p, p, p, p, p
}

func (p *plugin) SetApp(plugintypes.App) {
	panic("set app panic")
}

func (p *plugin) SetConfig(map[string]any) {
	panic("set config panic")
}

func (p *plugin) Exec() {
	panic("exec panic")
}

func (p *plugin) PostCreated(plugintypes.Post) {
	panic("post created panic")
}

func (p *plugin) BlockedBots() []string {
	panic("blocked bots panic")
}

func (p *plugin) ImgAttributes(_, _, _, _ string) (string, string) {
	panic("img attributes panic")
}

func (p *plugin) Render(plugintypes.RenderContext, io.Reader, io.Writer) {
	panic("ui panic")
}

func (p *plugin) RenderWithDocument(plugintypes.RenderContext, *goquery.Document) {
	panic("ui2 panic")
}

func (p *plugin) Routes() []plugintypes.Route {
	return []plugintypes.Route{{
		Method: http.MethodGet,
		Path:   "/plugin-panic",
		Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("route panic")
		}),
	}}
}
`)

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{Path: root, Import: "panicplugin"},
	}
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initTemplateAssets())
	require.NoError(t, app.initTemplateStrings())
	require.NoError(t, app.initPlugins())
	app.reloadRouter()

	// Wait for the Exec goroutine to panic and get recovered.
	time.Sleep(100 * time.Millisecond)

	assert.Empty(t, app.getBlockedBots())

	var buf bytes.Buffer
	app.writePictureElement(htmlbuilder.NewHTMLBuilder(&buf), "https://example.com/image.jpg", "alt text", "title text", "", "/post", false)
	assert.Contains(t, buf.String(), `alt="alt text"`)
	assert.Contains(t, buf.String(), `title="title text"`)

	app.postPostHooks(&post{Path: "/plugin-panic-post"})
	// Wait for the post hook goroutine to panic and get recovered.
	time.Sleep(100 * time.Millisecond)

	t.Run("UIRender", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "text/html")
		rr := httptest.NewRecorder()
		app.getAppRouter().ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotEmpty(t, rr.Body.String())
	})

	t.Run("RouteHandler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/plugin-panic", nil)
		req.Header.Set("Accept", "text/html")
		rr := httptest.NewRecorder()
		app.getAppRouter().ServeHTTP(rr, req)
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Contains(t, rr.Body.String(), "500 Internal Server Error")
	})
}

func TestPluginRoutesPanicIsolation(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "routespanic", `package routespanic

import (
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct{}

func GetPlugin() plugintypes.Routes { return plugin{} }

func (plugin) Routes() []plugintypes.Route { panic("routes panic") }
`)

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{Path: root, Import: "routespanic"},
	}
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initPlugins())
	assert.Empty(t, app.pluginRoutes)
}

func TestPluginMiddlewarePanicIsolation(t *testing.T) {
	root := t.TempDir()
	writeTestPlugin(t, root, "middlewareprio", `package middlewareprio

import (
	"net/http"

	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct{}

func GetPlugin() plugintypes.Middleware { return plugin{} }

func (plugin) Prio() int { panic("prio panic") }

func (plugin) Handler(next http.Handler) http.Handler { return next }
`)
	writeTestPlugin(t, root, "middlewarehandler", `package middlewarehandler

import (
	"net/http"

	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct{}

func GetPlugin() plugintypes.Middleware { return plugin{} }

func (plugin) Prio() int { return 100 }

func (plugin) Handler(http.Handler) http.Handler { panic("handler panic") }
`)

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{Path: root, Import: "middlewareprio"},
		{Path: root, Import: "middlewarehandler"},
	}
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initPlugins())

	middlewares := app.getPluginMiddlewares()
	require.Len(t, middlewares, 1)

	handler := app.wrapPluginMiddlewares(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginErrorPage(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "src", "errorplugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "errorplugin.go"), []byte(`package errorplugin

import (
	"net/http"

	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
	app plugintypes.App
}

func GetPlugin() (plugintypes.SetApp, plugintypes.Routes) {
	p := &plugin{}
	return p, p
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
}

func (p *plugin) Routes() []plugintypes.Route {
	return []plugintypes.Route{{
		Method: http.MethodGet,
		Path:   "/plugin-error",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p.app.ServeError(w, r, "plugin error", http.StatusNotFound)
		}),
	}}
}
`), 0o600))

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{Path: root, Import: "errorplugin"},
	}
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initTemplateAssets())
	require.NoError(t, app.initTemplateStrings())
	require.NoError(t, app.initPlugins())
	app.reloadRouter()

	t.Run("HTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/plugin-error", nil)
		req.Header.Set("Accept", "text/html")
		rr := httptest.NewRecorder()
		app.getAppRouter().ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "404 Not Found")
		assert.Contains(t, rr.Body.String(), "plugin error")
	})

	t.Run("PlainText", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/plugin-error", nil)
		req.Header.Set("Accept", "application/json")
		rr := httptest.NewRecorder()
		app.getAppRouter().ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Equal(t, "plugin error\n", rr.Body.String())
	})
}

func TestEmbeddedPluginMCPRoute(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{
			Path:   "embedded:mcp",
			Import: "mcp",
			Config: map[string]any{"path": "/custom-mcp"},
		},
	}
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initPlugins())
	app.reloadRouter()

	req := httptest.NewRequest(http.MethodDelete, "/custom-mcp", nil)
	rr := httptest.NewRecorder()
	app.getAppRouter().ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestPluginRouteValidation(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "src", "invalidroute")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "invalidroute.go"), []byte(`package invalidroute

import (
	"net/http"

	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct{}

func GetPlugin() plugintypes.Routes { return plugin{} }

func (plugin) Routes() []plugintypes.Route {
	return []plugintypes.Route{{
		Path:    "missing-leading-slash",
		Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	}}
}
`), 0o600))

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{Path: root, Import: "invalidroute"},
	}
	require.NoError(t, app.initConfig(false))
	require.Error(t, app.initPlugins())
}

func TestPluginRoutes(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "src", "routeplugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "routeplugin.go"), []byte(`package routeplugin

import (
	"net/http"

	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct{}

func GetPlugin() plugintypes.Routes { return plugin{} }

func (plugin) Routes() []plugintypes.Route {
	return []plugintypes.Route{
		{
			Method: http.MethodGet,
			Path:   "/plugin-test",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("global"))
			}),
		},
		{
			Method: http.MethodGet,
			Path:   "/plugin-blog-test",
			Blog:   "default",
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("blog"))
			}),
		},
	}
}
`), 0o600))

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{Path: root, Import: "routeplugin"},
	}
	require.NoError(t, app.initConfig(false))
	app.cfg.Blogs["default"].Path = "/blog"
	require.NoError(t, app.initPlugins())

	serve := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		app.getAppRouter().ServeHTTP(rr, req)
		return rr
	}

	for i := 0; i < 2; i++ {
		app.reloadRouter()
		rr := serve("/plugin-test")
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "global", rr.Body.String())

		rr = serve("/blog/plugin-blog-test")
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "blog", rr.Body.String())
	}
}

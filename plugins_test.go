package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/plugintypes"
)

func TestExecPlugin(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{
			Path:   "./plugins/demo",
			Type:   "exec",
			Import: "demoexec",
		},
	}

	err := app.initConfig(false)
	require.NoError(t, err)
	err = app.initPlugins()
	require.NoError(t, err)
}

func TestMiddlewarePlugin(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Plugins = []*configPlugin{
		{
			Path:   "./plugins/demo",
			Type:   "middleware",
			Import: "demomiddleware",
			Config: map[string]any{
				"prio": 99,
			},
		},
	}

	err := app.initConfig(false)
	require.NoError(t, err)
	err = app.initPlugins()
	require.NoError(t, err)

	middlewarePlugins := getPluginsForType[plugintypes.Middleware](app, "middleware")
	if assert.Len(t, middlewarePlugins, 1) {
		mdw := middlewarePlugins[0]
		assert.Equal(t, 99, mdw.Prio())
	}

}

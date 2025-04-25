package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

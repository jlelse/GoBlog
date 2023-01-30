package main

import (
	"embed"
	"io/fs"
	"net/http"
	"reflect"

	"go.goblog.app/app/pkgs/plugins"
	"go.goblog.app/app/pkgs/plugintypes"
	"go.goblog.app/app/pkgs/yaegiwrappers"
)

//go:embed plugins/*
var pluginsFS embed.FS

const (
	pluginSetAppType     = "setapp"
	pluginSetConfigType  = "setconfig"
	pluginUiType         = "ui"
	pluginExecType       = "exec"
	pluginMiddlewareType = "middleware"
)

func (a *goBlog) initPlugins() error {
	subFS, err := fs.Sub(pluginsFS, "plugins")
	if err != nil {
		return err
	}
	a.pluginHost = plugins.NewPluginHost(
		map[string]reflect.Type{
			pluginSetAppType:     reflect.TypeOf((*plugintypes.SetApp)(nil)).Elem(),
			pluginSetConfigType:  reflect.TypeOf((*plugintypes.SetConfig)(nil)).Elem(),
			pluginUiType:         reflect.TypeOf((*plugintypes.UI)(nil)).Elem(),
			pluginExecType:       reflect.TypeOf((*plugintypes.Exec)(nil)).Elem(),
			pluginMiddlewareType: reflect.TypeOf((*plugintypes.Middleware)(nil)).Elem(),
		},
		yaegiwrappers.Symbols,
		subFS,
	)

	for _, pc := range a.cfg.Plugins {
		plugins, err := a.pluginHost.LoadPlugin(&plugins.PluginConfig{
			Path:       pc.Path,
			ImportPath: pc.Import,
		})
		if err != nil {
			return err
		}
		if p, ok := plugins[pluginSetConfigType]; ok {
			p.(plugintypes.SetConfig).SetConfig(pc.Config)
		}
		if p, ok := plugins[pluginSetAppType]; ok {
			p.(plugintypes.SetApp).SetApp(a)
		}
	}

	for _, p := range a.getPlugins(pluginExecType) {
		go p.(plugintypes.Exec).Exec()
	}

	return nil
}

func (a *goBlog) getPlugins(typ string) []any {
	if a.pluginHost == nil {
		return []any{}
	}
	return a.pluginHost.GetPlugins(typ)
}

// Implement all needed interfaces

func (a *goBlog) GetDatabase() plugintypes.Database {
	return a.db
}

func (a *goBlog) GetPost(path string) (plugintypes.Post, error) {
	return a.getPost(path)
}

func (a *goBlog) PurgeCache() {
	a.cache.purge()
}

func (a *goBlog) GetHTTPClient() *http.Client {
	return a.httpClient
}

func (p *post) GetPath() string {
	return p.Path
}

func (p *post) GetParameters() map[string][]string {
	return p.Parameters
}

func (p *post) GetSection() string {
	return p.Section
}

func (p *post) GetPublished() string {
	return p.Published
}

func (p *post) GetUpdated() string {
	return p.Updated
}

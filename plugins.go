package main

import (
	"go.goblog.app/app/pkgs/plugins"
	"go.goblog.app/app/pkgs/plugintypes"
	"go.goblog.app/app/pkgs/yaegiwrappers"
)

const (
	execPlugin       = "exec"
	middlewarePlugin = "middleware"
	uiPlugin         = "ui"
)

func (a *goBlog) initPlugins() error {
	a.pluginHost = plugins.NewPluginHost(yaegiwrappers.Symbols)

	a.pluginHost.AddPluginType(execPlugin, (*plugintypes.Exec)(nil))
	a.pluginHost.AddPluginType(middlewarePlugin, (*plugintypes.Middleware)(nil))
	a.pluginHost.AddPluginType(uiPlugin, (*plugintypes.UI)(nil))

	for _, pc := range a.cfg.Plugins {
		if pluginInterface, err := a.pluginHost.LoadPlugin(&plugins.PluginConfig{
			Path:       pc.Path,
			ImportPath: pc.Import,
			PluginType: pc.Type,
		}); err != nil {
			return err
		} else if pluginInterface != nil {
			if setAppPlugin, ok := pluginInterface.(plugintypes.SetApp); ok {
				setAppPlugin.SetApp(a)
			}
			if setConfigPlugin, ok := pluginInterface.(plugintypes.SetConfig); ok {
				setConfigPlugin.SetConfig(pc.Config)
			}
		}
	}

	execs := getPluginsForType[plugintypes.Exec](a, execPlugin)
	for _, p := range execs {
		go p.Exec()
	}

	return nil
}

func getPluginsForType[T any](a *goBlog, pluginType string) (list []T) {
	if a == nil || a.pluginHost == nil {
		return nil
	}
	return plugins.GetPluginsForType[T](a.pluginHost, pluginType)
}

// Implement all needed interfaces

func (a *goBlog) GetDatabase() plugintypes.Database {
	return a.db
}

func (p *post) GetParameters() map[string][]string {
	return p.Parameters
}

type pluginPostRenderData struct {
	p *post
}

func (d *pluginPostRenderData) GetPost() plugintypes.Post {
	return d.p
}

func (p *post) pluginRenderData() plugintypes.PostRenderData {
	return &pluginPostRenderData{p: p}
}

func (b *configBlog) GetBlog() string {
	return b.name
}

type pluginBlogRenderData struct {
	b *configBlog
}

func (d *pluginBlogRenderData) GetBlog() plugintypes.Blog {
	return d.b
}

func (b *configBlog) pluginRenderData() plugintypes.BlogRenderData {
	return &pluginBlogRenderData{b: b}
}

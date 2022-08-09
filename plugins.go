package main

import (
	"go.goblog.app/app/pkgs/plugins"
	"go.goblog.app/app/pkgs/plugintypes"
	"go.goblog.app/app/pkgs/yaegiwrappers"
)

func (a *goBlog) initPlugins() error {
	a.pluginHost = plugins.NewPluginHost(yaegiwrappers.Symbols)

	a.pluginHost.AddPluginType("exec", (*plugintypes.Exec)(nil))
	a.pluginHost.AddPluginType("middleware", (*plugintypes.Middleware)(nil))

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

	execs := getPluginsForType[plugintypes.Exec](a, "exec")
	for _, p := range execs {
		go p.Exec()
	}

	return nil
}

func getPluginsForType[T any](a *goBlog, pluginType string) (list []T) {
	return plugins.GetPluginsForType[T](a.pluginHost, pluginType)
}

// Implement all needed interfaces for goblog

var _ plugintypes.App = &goBlog{}

func (a *goBlog) GetDatabase() plugintypes.Database {
	return a.db
}

var _ plugintypes.Database = &database{}

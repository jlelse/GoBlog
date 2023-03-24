package main

import (
	"embed"
	"io"
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
	pluginSetAppType          = "setapp"
	pluginSetConfigType       = "setconfig"
	pluginUiType              = "ui"
	pluginUi2Type             = "ui2"
	pluginExecType            = "exec"
	pluginMiddlewareType      = "middleware"
	pluginUiSummaryType       = "uisummary"
	pluginUiPostType          = "uiPost"
	pluginUiFooterType        = "uifooter"
	pluginPostCreatedHookType = "postcreatedhook"
	pluginPostUpdatedHookType = "postupdatedhook"
	pluginPostDeletedHookType = "postdeletedhook"
)

func (a *goBlog) initPlugins() error {
	subFS, err := fs.Sub(pluginsFS, "plugins")
	if err != nil {
		return err
	}
	a.pluginHost = plugins.NewPluginHost(
		map[string]reflect.Type{
			pluginSetAppType:          reflect.TypeOf((*plugintypes.SetApp)(nil)).Elem(),
			pluginSetConfigType:       reflect.TypeOf((*plugintypes.SetConfig)(nil)).Elem(),
			pluginUiType:              reflect.TypeOf((*plugintypes.UI)(nil)).Elem(),
			pluginUi2Type:             reflect.TypeOf((*plugintypes.UI2)(nil)).Elem(),
			pluginExecType:            reflect.TypeOf((*plugintypes.Exec)(nil)).Elem(),
			pluginMiddlewareType:      reflect.TypeOf((*plugintypes.Middleware)(nil)).Elem(),
			pluginUiSummaryType:       reflect.TypeOf((*plugintypes.UISummary)(nil)).Elem(),
			pluginUiPostType:          reflect.TypeOf((*plugintypes.UIPost)(nil)).Elem(),
			pluginUiFooterType:        reflect.TypeOf((*plugintypes.UIFooter)(nil)).Elem(),
			pluginPostCreatedHookType: reflect.TypeOf((*plugintypes.PostCreatedHook)(nil)).Elem(),
			pluginPostUpdatedHookType: reflect.TypeOf((*plugintypes.PostUpdatedHook)(nil)).Elem(),
			pluginPostDeletedHookType: reflect.TypeOf((*plugintypes.PostDeletedHook)(nil)).Elem(),
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

func (a *goBlog) GetBlog(name string) (plugintypes.Blog, bool) {
	blog, ok := a.cfg.Blogs[name]
	return blog, ok
}

func (a *goBlog) PurgeCache() {
	a.cache.purge()
}

func (a *goBlog) GetHTTPClient() *http.Client {
	return a.httpClient
}

func (a *goBlog) CompileAsset(filename string, reader io.Reader) error {
	return a.compileAsset(filename, reader)
}

func (a *goBlog) AssetPath(filename string) string {
	return a.assetFileName(filename)
}

func (a *goBlog) SetPostParameter(path string, parameter string, values []string) error {
	return a.db.replacePostParam(path, parameter, values)
}

func (a *goBlog) RenderMarkdownAsText(markdown string) (text string, err error) {
	return a.renderText(markdown)
}

func (p *post) GetPath() string {
	return p.Path
}

func (p *post) GetParameters() map[string][]string {
	return p.Parameters
}

func (p *post) GetParameter(parameter string) []string {
	return p.Parameters[parameter]
}

func (p *post) GetFirstParameterValue(parameter string) string {
	return p.firstParameter(parameter)
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

func (p *post) GetContent() string {
	return p.Content
}

func (p *post) GetTitle() string {
	return p.Title()
}

func (p *post) GetBlog() string {
	return p.Blog
}

func (b *configBlog) GetLanguage() string {
	return b.Lang
}

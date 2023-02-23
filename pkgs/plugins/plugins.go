package plugins

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// PluginHost manages the plugins.
type PluginHost struct {
	plugins         map[string][]*plugin
	pluginTypes     map[string]reflect.Type
	symbols         interp.Exports
	embeddedPlugins fs.FS
}

// PluginConfig is the configuration of the plugin.
type PluginConfig struct {
	// Path is the storage path of the plugin.
	Path string
	// ImportPath is the module path i.e. "github.com/user/module".
	ImportPath string
}

type plugin struct {
	Config *PluginConfig
	plugin any
}

const (
	embeddedPrefix = "embedded:"
)

// NewPluginHost initializes a PluginHost.
func NewPluginHost(pluginTypes map[string]reflect.Type, symbols interp.Exports, embeddedPlugins fs.FS) *PluginHost {
	return &PluginHost{
		plugins:         map[string][]*plugin{},
		pluginTypes:     pluginTypes,
		symbols:         symbols,
		embeddedPlugins: embeddedPlugins,
	}
}

// LoadPlugin loads a new plugin to the host.
func (h *PluginHost) LoadPlugin(config *PluginConfig) (map[string]any, error) {
	p := &plugin{
		Config: config,
	}
	plugins, err := p.initPlugin(h)
	if err != nil {
		return nil, err
	}
	return plugins, nil
}

// GetPlugins returns a list of all plugins.
func (h *PluginHost) GetPlugins(typ string) (list []any) {
	for _, p := range h.plugins[typ] {
		list = append(list, p.plugin)
	}
	return
}

func (p *plugin) initPlugin(host *PluginHost) (plugins map[string]any, err error) {
	const errText = "initPlugin: %w"

	plugins = map[string]any{}

	var filesystem fs.FS = nil
	if strings.HasPrefix(p.Config.Path, embeddedPrefix) {
		filesystem = host.embeddedPlugins
	}

	interpreter := interp.New(interp.Options{
		GoPath:               strings.TrimPrefix(p.Config.Path, embeddedPrefix),
		SourcecodeFilesystem: filesystem,
		Unrestricted:         true,
	})

	if err := interpreter.Use(stdlib.Symbols); err != nil {
		return nil, fmt.Errorf(errText, err)
	}

	if err := interpreter.Use(host.symbols); err != nil {
		return nil, fmt.Errorf(errText, err)
	}

	if _, err := interpreter.Eval(fmt.Sprintf(`import "%s"`, p.Config.ImportPath)); err != nil {
		return nil, fmt.Errorf(errText, err)
	}

	v, err := interpreter.Eval(filepath.Base(p.Config.ImportPath) + ".GetPlugin")
	if err != nil {
		return nil, fmt.Errorf(errText, err)
	}

	resultArray := v.Call([]reflect.Value{})
	for _, result := range resultArray {
		newPlugin := &plugin{
			Config: p.Config,
			plugin: result.Interface(),
		}
		for name, reflectType := range host.pluginTypes {
			if result.Type().Implements(reflectType) {
				host.plugins[name] = append(host.plugins[name], newPlugin)
				plugins[name] = newPlugin.plugin
			}
		}
	}

	return
}

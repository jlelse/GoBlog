package plugins

import (
	"fmt"
	"reflect"

	"github.com/traefik/yaegi/interp"
)

// NewPluginHost initializes a PluginHost.
func NewPluginHost(symbols interp.Exports) *PluginHost {
	return &PluginHost{
		Plugins:     []*plugin{},
		PluginTypes: map[string]reflect.Type{},
		Symbols:     symbols,
	}
}

// AddPluginType adds a plugin type to the list.
// The interface for the pluginType parameter should be a nil of the plugin type interface:
//
//	(*PluginInterface)(nil)
func (h *PluginHost) AddPluginType(name string, pluginType interface{}) {
	h.PluginTypes[name] = reflect.TypeOf(pluginType).Elem()
}

// LoadPlugin loads a new plugin to the host.
func (h *PluginHost) LoadPlugin(config *PluginConfig) (any, error) {
	p := &plugin{
		Config: config,
	}
	err := p.initPlugin(h)
	if err != nil {
		return nil, err
	}
	err = h.validatePlugin(p)
	if err != nil {
		return nil, err
	}
	h.Plugins = append(h.Plugins, p)
	return p.plugin.Interface(), nil
}

func (h *PluginHost) validatePlugin(p *plugin) error {
	pType := reflect.TypeOf(p.plugin.Interface())

	if _, ok := h.PluginTypes[p.Config.PluginType]; !ok {
		return fmt.Errorf("validatePlugin: %v: %w", p.Config.PluginType, ErrInvalidType)
	}

	if !pType.Implements(h.PluginTypes[p.Config.PluginType]) {
		return fmt.Errorf("validatePlugin:%v: %w %v", p, ErrValidatingPlugin, p.Config.PluginType)
	}

	return nil
}

// GetPlugins returns a list of all plugins.
func (h *PluginHost) GetPlugins() (list []any) {
	for _, p := range h.Plugins {
		list = append(list, p.plugin.Interface())
	}
	return
}

// GetPluginsForType returns all the plugins that are of type pluginType or empty if the pluginType doesn't exist.
func GetPluginsForType[T any](h *PluginHost, pluginType string) (list []T) {
	if _, ok := h.PluginTypes[pluginType]; !ok {
		return
	}
	for _, p := range h.Plugins {
		if p.Config.PluginType != pluginType {
			continue
		}
		if t, ok := p.plugin.Interface().(T); ok {
			list = append(list, t)
		}
	}
	return
}

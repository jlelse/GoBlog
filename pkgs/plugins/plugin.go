package plugins

import (
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

type plugin struct {
	Config *PluginConfig
	plugin reflect.Value
}

// PluginConfig is the configuration of the plugin.
type PluginConfig struct {
	// Path is the storage path of the plugin.
	Path string
	// ImportPath is the module path i.e. "github.com/user/module".
	ImportPath string
	// PluginType is the type of plugin, this plugin is checked against that type.
	// The available types are specified by the implementor of this package.
	PluginType string
}

func (p *plugin) initPlugin(host *PluginHost) error {
	const errText = "initPlugin: %w"

	interpreter := interp.New(interp.Options{
		GoPath: p.Config.Path,
	})

	if err := interpreter.Use(stdlib.Symbols); err != nil {
		return fmt.Errorf(errText, err)
	}

	if err := interpreter.Use(host.Symbols); err != nil {
		return fmt.Errorf(errText, err)
	}

	if _, err := interpreter.Eval(fmt.Sprintf(`import "%s"`, p.Config.ImportPath)); err != nil {
		return fmt.Errorf(errText, err)
	}

	v, err := interpreter.Eval(filepath.Base(p.Config.ImportPath) + ".GetPlugin")
	if err != nil {
		return fmt.Errorf(errText, err)
	}

	result := v.Call([]reflect.Value{})
	if len(result) > 1 {
		return fmt.Errorf(errText+": function GetPlugin has more than one return value", ErrValidatingPlugin)
	}
	p.plugin = result[0]

	return nil
}

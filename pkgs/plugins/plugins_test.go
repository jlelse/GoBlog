package plugins

import (
	"embed"
	"fmt"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/yaegi/interp"
)

//go:embed sample/*
var sampleSourceFS embed.FS

func TestLoadPluginEmbeddedSuccess(t *testing.T) {
	host := NewPluginHost(
		map[string]reflect.Type{
			"stringer": reflect.TypeOf((*fmt.Stringer)(nil)).Elem(),
		},
		interp.Exports{},
		sampleSourceFS,
	)

	_, err := host.LoadPlugin(&PluginConfig{
		Path:       "embedded:sample",
		ImportPath: "sample",
	})
	require.NoError(t, err)

	plugins := host.GetPlugins("stringer")
	require.Len(t, plugins, 1)
	assert.Equal(t, "ok", plugins[0].(fmt.Stringer).String())
}

func TestLoadPluginFailsOnMissingSource(t *testing.T) {
	host := NewPluginHost(map[string]reflect.Type{}, interp.Exports{}, fstest.MapFS{})
	_, err := host.LoadPlugin(&PluginConfig{
		Path:       "embedded:missing",
		ImportPath: "missing",
	})
	assert.Error(t, err)
}

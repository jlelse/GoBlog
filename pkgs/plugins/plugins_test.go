package plugins

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/plugintypes"
	"golang.org/x/tools/go/ssa"
)

//go:embed sample/*
var sampleSourceFS embed.FS

func TestLoadPluginEmbeddedSuccess(t *testing.T) {
	host := NewPluginHost(
		map[string]reflect.Type{
			"stringer": reflect.TypeFor[fmt.Stringer](),
		},
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

func TestLoadPluginFromFilesystem(t *testing.T) {
	root := t.TempDir()
	pkgDir := filepath.Join(root, "src", "fsplugin")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "fsplugin.go"), []byte(`package fsplugin

import "fmt"

type plugin struct{}

func GetPlugin() fmt.Stringer { return plugin{} }

func (plugin) String() string { return "from-filesystem" }
`), 0o600))

	host := NewPluginHost(
		map[string]reflect.Type{
			"stringer": reflect.TypeFor[fmt.Stringer](),
		},
		fstest.MapFS{},
	)

	_, err := host.LoadPlugin(&PluginConfig{
		Path:       root,
		ImportPath: "fsplugin",
	})
	require.NoError(t, err)

	plugins := host.GetPlugins("stringer")
	require.Len(t, plugins, 1)
	assert.Equal(t, "from-filesystem", plugins[0].(fmt.Stringer).String())
}

func TestGetPluginsNoAllocation(t *testing.T) {
	host := NewPluginHost(
		map[string]reflect.Type{
			"stringer": reflect.TypeFor[fmt.Stringer](),
		},
		sampleSourceFS,
	)
	_, err := host.LoadPlugin(&PluginConfig{Path: "embedded:sample", ImportPath: "sample"})
	require.NoError(t, err)

	allocs := testing.AllocsPerRun(1000, func() {
		_ = host.GetPlugins("stringer")
	})
	assert.Zero(t, allocs)
}

func BenchmarkGetPlugins(b *testing.B) {
	host := NewPluginHost(
		map[string]reflect.Type{
			"stringer": reflect.TypeFor[fmt.Stringer](),
		},
		sampleSourceFS,
	)
	_, err := host.LoadPlugin(&PluginConfig{Path: "embedded:sample", ImportPath: "sample"})
	require.NoError(b, err)

	b.ReportAllocs()
	for b.Loop() {
		_ = host.GetPlugins("stringer")
	}
}

func TestLoadPluginFailsOnMissingSource(t *testing.T) {
	host := NewPluginHost(map[string]reflect.Type{}, fstest.MapFS{})
	_, err := host.LoadPlugin(&PluginConfig{
		Path:       "embedded:missing",
		ImportPath: "missing",
	})
	assert.Error(t, err)
}

func TestLoadPluginFailsOnInitPanic(t *testing.T) {
	source := fstest.MapFS{
		"panic/src/panicplugin/panicplugin.go": {Data: []byte(`package panicplugin

import "fmt"

func init() { panic("init panic") }

func GetPlugin() fmt.Stringer { return nil }
`)},
	}
	host := NewPluginHost(map[string]reflect.Type{"stringer": reflect.TypeFor[fmt.Stringer]()}, source)
	_, err := host.LoadPlugin(&PluginConfig{
		Path:       "embedded:panic",
		ImportPath: "panicplugin",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init panic")
}

func TestLoadPluginFailsOnGetPluginPanic(t *testing.T) {
	source := fstest.MapFS{
		"panic/src/panicplugin/panicplugin.go": {Data: []byte(`package panicplugin

import "fmt"

func GetPlugin() fmt.Stringer { panic("get plugin panic") }
`)},
	}
	host := NewPluginHost(map[string]reflect.Type{"stringer": reflect.TypeFor[fmt.Stringer]()}, source)
	_, err := host.LoadPlugin(&PluginConfig{
		Path:       "embedded:panic",
		ImportPath: "panicplugin",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GetPlugin panicked")
}

func TestPluginsShareSSAProgram(t *testing.T) {
	source := fstest.MapFS{
		"a/src/a/a.go": {Data: []byte(`package a

import "fmt"

type s string

func (v s) String() string { return string(v) }

func GetPlugin() fmt.Stringer { return s("a") }
`)},
		"b/src/b/b.go": {Data: []byte(`package b

import "fmt"

type s string

func (v s) String() string { return string(v) }

func GetPlugin() fmt.Stringer { return s("b") }
`)},
	}
	host := NewPluginHost(map[string]reflect.Type{"stringer": reflect.TypeFor[fmt.Stringer]()}, source)
	_, err := host.LoadPlugin(&PluginConfig{Path: "embedded:a", ImportPath: "a"})
	require.NoError(t, err)
	_, err = host.LoadPlugin(&PluginConfig{Path: "embedded:b", ImportPath: "b"})
	require.NoError(t, err)

	programs := map[*ssa.Program]struct{}{}
	for _, p := range host.plugins["stringer"] {
		programs[p.interp.MainPkg().Prog] = struct{}{}
	}
	assert.Len(t, programs, 1)
}

func TestLoadPluginRegistersHooks(t *testing.T) {
	source := fstest.MapFS{
		"hooktest/src/hooktest/hooktest.go": {
			Data: []byte(`package hooktest

import "go.goblog.app/app/pkgs/plugintypes"

type plugin struct{}

func GetPlugin() (plugintypes.PostCreatedHook, plugintypes.PostUpdatedHook) {
	return plugin{}, plugin{}
}

func (plugin) PostCreated(_ plugintypes.Post) {}
func (plugin) PostUpdated(_ plugintypes.Post) {}
`),
		},
	}

	host := NewPluginHost(
		map[string]reflect.Type{
			"postcreated": reflect.TypeFor[plugintypes.PostCreatedHook](),
			"postupdated": reflect.TypeFor[plugintypes.PostUpdatedHook](),
		},
		source,
	)

	plugins, err := host.LoadPlugin(&PluginConfig{
		Path:       "embedded:hooktest",
		ImportPath: "hooktest",
	})
	require.NoError(t, err)
	assert.Contains(t, plugins, "postcreated")
	assert.Contains(t, plugins, "postupdated")

	assert.Len(t, host.GetPlugins("postcreated"), 1)
	assert.Len(t, host.GetPlugins("postupdated"), 1)
}

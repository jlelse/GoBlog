// Package plugins provides plugin loading functionality for GoBlog.
package plugins

import (
	"fmt"
	"go/ast"
	"go/types"
	"io/fs"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/goplus/ixgo"
	"golang.org/x/tools/go/ssa"

	// Register the host packages available to plugins.
	_ "go.goblog.app/app/pkgs/ixgowrappers" //nolint:revive
)

// PluginHost manages the plugins.
type PluginHost struct {
	plugins         map[string][]*plugin
	pluginLists     map[string][]any // read-only, returned by GetPlugins without allocating
	pluginTypes     map[string]reflect.Type
	embeddedPlugins fs.FS
	ctx             *ixgo.Context
	loadMu          sync.Mutex
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
	interp *ixgo.Interp
}

const (
	embeddedPrefix = "embedded:"
)

// NewPluginHost initializes a PluginHost.
func NewPluginHost(pluginTypes map[string]reflect.Type, embeddedPlugins fs.FS) *PluginHost {
	return &PluginHost{
		plugins:         map[string][]*plugin{},
		pluginLists:     map[string][]any{},
		pluginTypes:     pluginTypes,
		embeddedPlugins: embeddedPlugins,
		ctx:             ixgo.NewContext(ixgo.SupportMultipleInterp),
	}
}

// LoadPlugin loads a new plugin to the host.
func (h *PluginHost) LoadPlugin(config *PluginConfig) (map[string]any, error) {
	h.loadMu.Lock()
	defer h.loadMu.Unlock()
	p := &plugin{
		Config: config,
	}
	plugins, err := p.initPlugin(h)
	if err != nil {
		return nil, err
	}
	return plugins, nil
}

// GetPlugins returns all plugins of the given type. The returned slice must
// not be modified.
func (h *PluginHost) GetPlugins(typ string) []any {
	return h.pluginLists[typ]
}

func (p *plugin) initPlugin(host *PluginHost) (plugins map[string]any, err error) {
	const errText = "initPlugin: %w"

	plugins = map[string]any{}

	interp, err := p.loadInterp(host.ctx, host.embeddedPlugins)
	if err != nil {
		return nil, fmt.Errorf(errText, err)
	}
	if interp.MainPkg().Func("init") != nil {
		if err = callPluginInit(interp); err != nil {
			return nil, fmt.Errorf(errText, err)
		}
	}

	v, ok := interp.GetFunc("GetPlugin")
	if !ok {
		return nil, fmt.Errorf(errText, fmt.Errorf("plugin %q has no GetPlugin function", p.Config.ImportPath))
	}

	resultArray, err := callGetPlugin(v)
	if err != nil {
		return nil, fmt.Errorf(errText, err)
	}
	for _, result := range resultArray {
		newPlugin := &plugin{
			Config: p.Config,
			plugin: result.Interface(),
			interp: interp,
		}
		for name, reflectType := range host.pluginTypes {
			if result.Type().Implements(reflectType) {
				host.plugins[name] = append(host.plugins[name], newPlugin)
				host.pluginLists[name] = append(host.pluginLists[name], newPlugin.plugin)
				plugins[name] = newPlugin.plugin
			}
		}
	}

	return
}

func callPluginInit(interp *ixgo.Interp) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("plugin init panicked: %v", r)
		}
	}()
	return interp.RunInit()
}

func callGetPlugin(fn any) (results []reflect.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("GetPlugin panicked: %v", r)
		}
	}()
	return reflect.ValueOf(fn).Call(nil), nil
}

func (p *plugin) loadInterp(ctx *ixgo.Context, embeddedPlugins fs.FS) (*ixgo.Interp, error) {
	pkg, err := p.loadPackage(ctx, embeddedPlugins)
	if err != nil {
		return nil, err
	}
	return ctx.NewInterp(pkg)
}

func (p *plugin) loadPackage(ctx *ixgo.Context, embeddedPlugins fs.FS) (*ssa.Package, error) {
	if !strings.HasPrefix(p.Config.Path, embeddedPrefix) {
		dir := filepath.Join(p.Config.Path, "src", filepath.FromSlash(p.Config.ImportPath))
		if err := ctx.AddImport(p.Config.ImportPath, dir); err != nil {
			return nil, err
		}
		return buildPackage(ctx, ctx.SourcePackage(p.Config.ImportPath))
	}

	apkg, err := p.embeddedPackage(ctx, embeddedPlugins)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, file := range apkg.Files {
		files = append(files, file)
	}
	if f, err := ixgo.ParseBuiltin(ctx.FileSet, apkg.Name); err == nil {
		files = append([]*ast.File{f}, files...)
	}
	sp := &ixgo.SourcePackage{
		Context: ctx,
		Package: types.NewPackage(p.Config.ImportPath, apkg.Name),
		Files:   files,
	}
	return buildPackage(ctx, sp)
}

// buildPackage builds the plugin into the host's shared SSA program. Unlike
// ixgo's LoadAstPackage and LoadDirEx helpers, it does not reset the builder,
// so imports shared between plugins are compiled only once.
func buildPackage(ctx *ixgo.Context, sp *ixgo.SourcePackage) (pkg *ssa.Package, err error) {
	if sp == nil || sp.Package == nil {
		return nil, fmt.Errorf("package not found")
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("build package %q: %v", sp.Package.Path(), r)
		}
	}()
	if err := sp.Load(); err != nil {
		return nil, err
	}
	return ctx.Builder.BuildMain(sp)
}

//nolint:staticcheck // ast.Package groups the parsed files of an embedded plugin.
func (p *plugin) embeddedPackage(ctx *ixgo.Context, embeddedPlugins fs.FS) (*ast.Package, error) {
	pluginRoot := strings.TrimPrefix(p.Config.Path, embeddedPrefix)
	pkgDir := path.Join(pluginRoot, "src", p.Config.ImportPath)
	entries, err := fs.ReadDir(embeddedPlugins, pkgDir)
	if err != nil {
		return nil, err
	}
	apkg := &ast.Package{Files: map[string]*ast.File{}}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		filename := path.Join(pkgDir, entry.Name())
		src, err := fs.ReadFile(embeddedPlugins, filename)
		if err != nil {
			return nil, err
		}
		file, err := ctx.ParseFile(filename, src)
		if err != nil {
			return nil, err
		}
		apkg.Name = file.Name.Name
		apkg.Files[filename] = file
	}
	if len(apkg.Files) == 0 {
		return nil, fmt.Errorf("no Go files found in %s", pkgDir)
	}
	return apkg, nil
}

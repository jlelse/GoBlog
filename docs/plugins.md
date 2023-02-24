# GoBlog Plugins

GoBlog has a (still experimental) plugin system, that allows adding new functionality to GoBlog without adding anything to the GoBlog source and recompiling GoBlog. Plugins work using the [Yaegi](https://github.com/traefik/yaegi) package by Traefik, are written in Go and are interpreted at run time.

## Configuration

Plugins can be added to GoBlog by adding a "plugins" section to the configuration.

```yaml
plugins:
  - path: embedded:syndication # Use a Plugin provided by GoBlog using the "embedded:" prefix
    import: syndication
    config: # Provide configuration for the plugin
      parameter: syndication
  - path: embedded:demo
    import: demo
  - path: ./plugins/mycustomplugin
    import: mycustompluginpackage
    config:
      abc:
        def:
          one: 1
          two: 2
```

You need to specify the path to the plugin (remember to mount the path to your GoBlog container when using Docker) and the Go packakge and you can additionally provide configuration for the plugin.

## Types of plugins

- `SetApp` (Access more GoBlog functionalities like the database) - see https://pkg.go.dev/go.goblog.app/app/pkgs/plugintypes#SetApp
- `SetConfig` (Access the configuration provided for the plugin) - see https://pkg.go.dev/go.goblog.app/app/pkgs/plugintypes#SetConfig

- `Exec` (Command that is executed in a Go routine when starting GoBlog) - see https://pkg.go.dev/go.goblog.app/app/pkgs/plugintypes#Exec
- `Middleware` (HTTP middleware to intercept or modify HTTP requests) - see https://pkg.go.dev/go.goblog.app/app/pkgs/plugintypes#Middleware
- `UI` (Modify rendered HTML) - see https://pkg.go.dev/go.goblog.app/app/pkgs/plugintypes#UI
- `UI2` (Modify rendered HTML using a goquery document which improves performance and avoids multiple HTML parsing and rendering when using multiple plugins) - see https://pkg.go.dev/go.goblog.app/app/pkgs/plugintypes#UI2

More types will be added later. Any plugin can implement multiple types, see the demo plugin as example.

## Plugin implementation

All you need to do is creating a Go-file that has a `GetPlugin` function that returns the interface implementation of the desired GoBlog plugin types.

So if you want to create a plugin that implements the `Exec` and `UI` plugin types, you need this:

```go
package yourpluginpackage

import "go.goblog.app/app/pkgs/plugintypes"

type plugin struct {}

func GetPlugin() (plugintypes.Exec,	plugintypes.UI) {
	p := &plugin{}
	return p, p
}
```

Of course, the plugin Go type also needs to have the required functions and methods:

```go
// Exec
func (p *plugin) Exec() {
  // Do something
}

// UI
func (p *plugin) Render(rc plugintypes.RenderContext, rendered io.Reader, modified io.Writer) {
  // Do something, but at least write something to modified, otherwise, the page will stay blank
}
```

If you want to access the configuration that is provided for your plugin, you need to implement the `SetConfig` plugin type. To access some more functions of GoBlog, implement the `SetApp` plugin type that allows you, for example, to access the database or get posts and their parameters.


### Packages provided

Several go modules are already provided by GoBlog, so you don't have to vendor them.

GoBlog modules:

- `go.goblog.app/app/pkgs/plugintypes` (Needed for every plugin)
- `go.goblog.app/app/pkgs/htmlbuilder` (Can be used to generate HTML)
- `go.goblog.app/app/pkgs/bufferpool` (Can be used to manage `bytes.Buffer`s more efficiently)

Third-party modules

- `github.com/PuerkitoBio/goquery` (Can be used to *manipulate* HTML in a jquery-like way)
- `github.com/carlmjohnson/requests` (Can be used to do HTTP requests more easily)

## Plugins

Some simple plugins are included in the main GoBlog repository. Some can be found elsewhere.

### Custom CSS (Path `embedded:customcss`, Import `customcss`)

A plugin that can add custom CSS to every HTML page. Just specify a CSS file and it will minify the file and append it to the rendered HTML head.

#### Config

`file` (string): Path to the custom CSS file.

### Demo (Path `embedded:demo`, Import `demo`)

A simple demo plugin showcasing some of the features plugins can implement. Take a look at the source code, if you want to implement your own plugin.

### Syndication links (Path `embedded:syndication`, Import `syndication`)

Adds hidden `u-syndication` `data` elements to post page when the configured post parameter (default: "syndication") is available.

#### Config

`parameter` (string): Name for the post parameter containing the syndication links.

### Webrings (Path `embedded:webrings`, Import `webrings`)

Adds webring links to the bottom of the blog footer to easily participate in webrings.

#### Config

You can add webring links like this:

```yaml
config:
  default: # Name of the blog
    - title: Webring # Title to show for the webring (required)
      # At least one of link, prev or next is required
      link: https://example.org/ # Link to the webring
      prev: https://example.com/ # Link to previous webring site
      next: https://example.net/ # Link to next webring site
```
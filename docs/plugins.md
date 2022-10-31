# GoBlog Plugins

GoBlog has a (still experimental) plugin system, that allows adding new functionality to GoBlog without adding anything to the GoBlog source and recompiling GoBlog. Plugins work using the [Yaegi](https://github.com/traefik/yaegi) package by Traefik and are interpreted at run time.

## Configuration

Plugins can be added to GoBlog by adding a "plugins" section to the configuration.

```yaml
plugins:
  - path: ./plugins/syndication
    type: ui
    import: syndication
    config:
      parameter: syndication
  - path: ./plugins/demo
    type: ui
    import: demoui
  - path: ./plugins/demo
    type: middleware
    import: demomiddleware
    config:
      prio: 99
```

You need to specify the path to the plugin (remember to mount the path to your GoBlog container when using Docker), the type of the plugin, the import (the Go packakge) and you can additionally provide configuration for the plugin.

## Types of plugins

- `exec` (Command that is executed in a Go routine when starting GoBlog) - see https://pkg.go.dev/go.goblog.app/app/pkgs/plugintypes#Exec
- `middleware` (HTTP middleware to intercept or modify HTTP requests) - see https://pkg.go.dev/go.goblog.app/app/pkgs/plugintypes#Middleware
- `ui` (Render additional HTML) - see https://pkg.go.dev/go.goblog.app/app/pkgs/plugintypes#UI

## Plugins

Some simple plugins are included in the main GoBlog repository. Some can be found elsewhere.

### Syndication links (plugins/syndication)

Adds hidden `u-syndication` `data` elements to post page when the configured post parameter (default: "syndication") is available.

#### Config

`parameter` (string): Name for the post parameter containing the syndication links.

### Webrings (plugins/webrings)

Adds webring links to the bottom of the blog footer to easily participate in webrings.

#### Config

You can add webring links like this:

```yaml
config:
  default: # Name of the blog
    - title: Webring # Title to show for the webring
      prev: https://example.com/ # Link to previous webring site
      next: https://example.net/ # Link to next webring site
```
# How to configure GoBlog

Most settings for GoBlog (still) have to be configured using a YAML configuration file. See `example-config.yaml` for the available options. It's recommended to just set the settings you really need. There are defaults for most settings. Some settings can be configured using a web UI (with the path of `/settings`).

## Sections

You can add, remove or update sections and the default section using the web UI.

Sections can have a title, description (with support for markdown) and a path template which gets used when creating a new post without a pre-setted path. The syntax for the path template is gets parsed as a [Go template](https://pkg.go.dev/text/template#pkg-overview). Available variables are: `.Section`, `.Slug`, `.Year`, `.Month`, `.Day` and `.BlogPath`.

Example for the path template:

```
{{printf \"/%v/%v\" .Section .Slug}}
```
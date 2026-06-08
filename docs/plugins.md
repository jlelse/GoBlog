# Plugins

GoBlog has a plugin system that allows extending functionality without modifying the source code. Plugins are written in Go and interpreted at runtime using [Yaegi](https://github.com/traefik/yaegi).

## Plugin Configuration

Plugins are configured in the `plugins` section of your config file:

```yaml
plugins:
  - path: embedded:pluginname  # Use embedded plugin
    import: pluginname         # Go package name
    config:                    # Plugin-specific config
      key: value

  - path: /path/to/plugin/src  # Use filesystem plugin
    import: example.com/myplugin
    config:
      key: value
```

**Plugin paths:**
- `embedded:pluginname` - Load from GoBlog's embedded plugins
- `/path/to/src` - Load from filesystem (must be Go source code, not compiled)
  - When using Docker, mount the plugin directory to the container

## Built-in Plugins

### Custom CSS

**Path:** `embedded:customcss` | **Import:** `customcss`

Add custom CSS to every HTML page. The CSS file is minified and appended to the HTML head.

```yaml
plugins:
  - path: embedded:customcss
    import: customcss
    config:
      file: ./custom.css  # Path to your CSS file
```

### Syndication Links

**Path:** `embedded:syndication` | **Import:** `syndication`

Adds hidden `u-syndication` data elements to post pages when the configured post parameter is available. Useful for POSSE (Publish on Own Site, Syndicate Elsewhere).

```yaml
plugins:
  - path: embedded:syndication
    import: syndication
    config:
      parameter: syndication  # Post parameter name (default: "syndication")
```

**Usage:** Add `syndication: https://twitter.com/user/status/123` to your post's front matter.

### Webrings

**Path:** `embedded:webrings` | **Import:** `webrings`

Adds webring links to the bottom of the blog footer.

```yaml
plugins:
  - path: embedded:webrings
    import: webrings
    config:
      default:  # Blog name
        - title: Webring Name  # Required
          link: https://webring.example.com/  # Optional
          prev: https://prev.example.com/     # Optional
          next: https://next.example.com/     # Optional
```

At least one of `link`, `prev`, or `next` is required.

### AI Summary (TL;DR)

**Path:** `embedded:aitldr` | **Import:** `aitldr`

Uses the OpenAI API to generate a short one-sentence summary for blog posts (after creating or updating).

```yaml
plugins:
  - path: embedded:aitldr
    import: aitldr
    config:
      apikey: YOUR_OPENAI_API_KEY  # Required
      model: gpt-4o                # Optional, default is gpt-4o
      endpoint: https://api.scaleway.ai/.../chat/completions  # Optional
      default:  # Blog name
        title: "AI Summary:"  # Optional, custom title
```

**Disable per post:** Add `noaitldr: "true"` to front matter.

### AI Image Captions

**Path:** `embedded:aiimage` | **Import:** `aiimage`

Uses the OpenAI API to generate short image captions for images in blog posts. Adds a button to the post for (re-)generating captions.

```yaml
plugins:
  - path: embedded:aiimage
    import: aiimage
    config:
      apikey: YOUR_OPENAI_API_KEY  # Required
      model: gpt-4o                # Optional, default is gpt-4o
      endpoint: https://api.scaleway.ai/.../chat/completions  # Optional
      default:  # Blog name
        title: "Caption:"  # Optional, custom prefix
```

**Disable per post:** Add `noaiimage: "true"` to front matter.

### Snow Animation

**Path:** `embedded:snow` | **Import:** `snow`

Adds a snow animation to pages using CSS and JavaScript.

```yaml
plugins:
  - path: embedded:snow
    import: snow
```

No configuration options.

### AI Bot Block

**Path:** `embedded:aibotblock` | **Import:** `aibotblock`

Enhances `/robots.txt` with AI bot user-agents and blocks requests from them. User-agents are sourced from [ai-robots-txt](https://github.com/ai-robots-txt/ai.robots.txt).

```yaml
plugins:
  - path: embedded:aibotblock
    import: aibotblock
```

No configuration options.

### Image Tooltips

**Path:** `embedded:imagetooltips` | **Import:** `imagetooltips`

Shows image titles in an alert when clicking on images. Improves mobile UX where hovering isn't possible.

```yaml
plugins:
  - path: embedded:imagetooltips
    import: imagetooltips
```

No configuration options.

### Telegram Bot

**Path:** `embedded:telegrambot` | **Import:** `telegrambot`

Post to your blog via a Telegram bot. Send the bot your post content and it will publish it and respond with the link. You can use front matter syntax in your message. Also supports file uploads.

```yaml
plugins:
  - path: embedded:telegrambot
    import: telegrambot
    config:
      token: YOUR_TELEGRAM_BOT_TOKEN    # Required
      allowed: YOUR_TELEGRAM_USERNAME   # Required (username or numeric user ID)
```

The bot responds to `/start` and `/help` commands. The `allowed` field accepts both Telegram usernames and numeric user IDs.

### MCP Server

**Path:** `embedded:mcp` | **Import:** `mcp`

Exposes your blog as an [MCP (Model Context Protocol)](https://modelcontextprotocol.io) server, allowing AI assistants and LLM applications to query your blog posts. Implements the MCP 2025-11-25 specification with Streamable HTTP transport. Read-only access: no editing or creating posts.

**Available tools:**
- `list_blogs`: List configured blogs with metadata
- `list_posts`: List posts with optional filtering by blog, section, status, visibility, with pagination
- `get_post`: Get a single post by path with full content and parameters
- `search_posts`: Full-text search across all posts
- `count_posts`: Count posts matching filters
- `list_sections`: List all blog sections with title and description
- `list_tags`: List all tags (taxonomy values) for a blog, optionally specify a taxonomy name
- `get_blog_stats`: Get statistics including post counts, characters, words, and words per post by year/month
- `list_webmentions`: List webmentions with filtering by target URL and status
- `list_comments`: List comments on posts, optionally filter by target path

```yaml
plugins:
  - path: embedded:mcp
    import: mcp
    config:
      path: /mcp  # Optional: endpoint path, default is /mcp
```

**Authentication:** Uses GoBlog's app passwords. Create an app password in Settings, then use it as a Bearer token (`Authorization: Bearer <app-password>`) or via HTTP Basic Auth.

**Testing:** Install the [MCP Inspector](https://github.com/modelcontextprotocol/inspector) with `npx @modelcontextprotocol/inspector`. Select "Streamable HTTP" as transport type, enter `https://yourblog.example.com/mcp` as the URL, add header `Authorization: Bearer <app-password>`, and click "Connect".

### Demo Plugin

**Path:** `embedded:demo` | **Import:** `demo`

A demo plugin showcasing various plugin features. Useful for learning how to create your own plugins.

```yaml
plugins:
  - path: embedded:demo
    import: demo
```

## Creating Custom Plugins

Plugins are Go packages with a `GetPlugin` function that returns implementations of plugin interfaces.

**Minimal plugin example:**

```go
package myplugin

import (
    "go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
    app    plugintypes.App
    config map[string]any
}

// GetPlugin returns the plugin implementations
func GetPlugin() (plugintypes.SetApp, plugintypes.SetConfig, plugintypes.Exec) {
    p := &plugin{}
    return p, p, p
}

func (p *plugin) SetApp(app plugintypes.App) {
    p.app = app
}

func (p *plugin) SetConfig(config map[string]any) {
    p.config = config
}

func (p *plugin) Exec() {
    // Background task - runs in a goroutine
}
```

### Plugin Interfaces

Plugins can implement multiple interfaces:

- **SetApp** - Receive app instance (database, HTTP client, etc.)
- **SetConfig** - Receive plugin configuration
- **Exec** - Run background tasks (runs in goroutine)
- **Middleware** - HTTP middleware with priority
- **UI** - Modify rendered HTML (stream-based)
- **UI2** - Modify rendered HTML (DOM-based with goquery)
- **UIImgAttributes** - Modify alt/title attributes on `<img>` elements
- **UISummary** - Modify post summaries on index pages
- **UIPost** - Modify post HTML on post pages
- **UIPostContent** - Modify post content HTML (all outputs)
- **UIFooter** - Modify footer HTML
- **PostCreatedHook** - React to post creation
- **PostUpdatedHook** - React to post updates
- **PostDeletedHook** - React to post deletion
- **BlockedBots** - Provide bot user-agent names to block in robots.txt

See [plugintypes documentation](https://pkg.go.dev/go.goblog.app/app/pkgs/plugintypes) for details.

### Available Packages for Plugins

**GoBlog packages:**
- `go.goblog.app/app/pkgs/plugintypes` - Plugin interfaces (required)
- `go.goblog.app/app/pkgs/htmlbuilder` - HTML generation
- `go.goblog.app/app/pkgs/bufferpool` - Efficient buffer management
- `go.goblog.app/app/pkgs/builderpool` - Efficient string builder management

**Third-party packages (available without vendoring):**
- `github.com/PuerkitoBio/goquery` - HTML manipulation (jQuery-like)
- `github.com/carlmjohnson/requests` - HTTP requests
- `github.com/araddon/dateparse` - Date parsing

For more examples, see the [embedded plugins](/plugins/) in the repository.

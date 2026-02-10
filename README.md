# GoBlog

**GoBlog** is a single-user, multi-blog platform written in Go. It's designed to be simple, fast, and extensible while giving you full control over your content through IndieWeb and ActivityPub protocols.

---

## Table of Contents

- [What is GoBlog?](#what-is-goblog)
- [Key Features](#key-features)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Configuration](#configuration)
- [Writing Posts](#writing-posts)
- [Editor & Publishing](#editor--publishing)
- [IndieWeb & Fediverse](#indieweb--fediverse)
- [Optional Features](#optional-features)
- [Plugins](#plugins)
- [Administration](#administration)
- [CLI Commands](#cli-commands)
- [Troubleshooting](#troubleshooting)
- [Advanced Topics](#advanced-topics)

---

## What is GoBlog?

GoBlog is a blogging engine built for people who want:

- **Full ownership** of their content with IndieWeb and ActivityPub support
- **Simplicity**: One binary, SQLite database, minimal dependencies
- **Performance**: Fast rendering with built-in caching
- **Flexibility**: Multiple blogs, sections, taxonomies, and custom plugins
- **Privacy**: Optional private mode to restrict access

### Philosophy

- **Opt-in features**: Most features are disabled by default - enable only what you need
- **Markdown-first**: Write in Markdown with optional front matter
- **No themes**: Customize through configuration and plugins, not themes
- **Docker-friendly**: Designed to run in containers with simple volume mounts

---

## Key Features

### Core Functionality
- ✅ **Markdown posts** with front matter support
- ✅ **Multiple blogs** under one installation (e.g., for different languages)
- ✅ **Sections and taxonomies** (tags, categories, etc.)
- ✅ **Web editor** with live preview
- ✅ **Media uploads** with optional automatic compression
- ✅ **Full-text search** (SQLite FTS5)
- ✅ **RSS, Atom, and JSON feeds**
- ✅ **Sitemap and robots.txt**

### IndieWeb & Fediverse
- ✅ **IndieAuth**: Use your blog as your identity
- ✅ **Micropub**: Create/update/delete posts via API
- ✅ **Webmention**: Send and receive webmentions
- ✅ **ActivityPub**: Publish to Mastodon and the Fediverse
- ✅ **Microformats2**: Proper h-entry markup

### Optional Features (Opt-in)
- 📝 **Comments** (via Webmention or ActivityPub)
- 👍 **Reactions** (emoji reactions on posts)
- 🗺️ **Maps** (show post locations and GPX tracks)
- 📊 **Statistics** (posts per year)
- 🔍 **Blogroll** (OPML-based)
- 🎲 **Random post** redirect
- 📅 **On this day** archive
- 📧 **Contact form** (SMTP-based)
- 🔊 **Text-to-Speech** (Google Cloud TTS)
- 🔔 **Notifications** (Ntfy, Telegram, Matrix)
- 🔗 **Short URLs** with custom domain
- 🌐 **Tor Hidden Service**
- 🔒 **Private mode** (login-only access)

### Extensibility
- 🔌 **Plugin system** (runtime-loaded Go plugins)
- 🪝 **Hooks** (shell commands on events)
- 🎨 **Custom CSS** via plugins
- 🔄 **Regex redirects**

---

## Quick Start

### Prerequisites

- **Docker** (recommended) or Go 1.25+
- Basic knowledge of **Markdown**
- Basic knowledge of **YAML** for configuration

### 1. Using Docker (Recommended)

Create a `docker-compose.yml`:

```yaml
services:
  goblog:
    container_name: goblog
    image: ghcr.io/jlelse/goblog:latest
    restart: unless-stopped
    volumes:
      - ./config:/app/config
      - ./data:/app/data
    ports:
      - "8080:8080"
    environment:
      - TZ=Europe/Berlin
```

Create minimal config at `./config/config.yml`:

```yaml
server:
  publicAddress: http://localhost:8080
```

Start the container:

```bash
docker-compose up -d
```

### 2. Using Go (Build from Source)

```bash
# Clone repository
git clone https://github.com/jlelse/GoBlog.git
cd GoBlog

# Build (with system libsqlite3)
go build -tags=linux,libsqlite3,sqlite_fts5 -o GoBlog

# Or build with embedded SQLite (slower build, no system dependency)
go build -tags=linux,sqlite_fts5 -o GoBlog

# Create directories and config
mkdir -p config data
cat > config/config.yml << 'EOF'
server:
  publicAddress: http://localhost:8080
EOF

# Run
./GoBlog --config ./config/config.yml
```

### 3. First Steps

1. **Check the logs**: On first startup, GoBlog generates a secure random password and logs it to the console. Look for a log line like:
   ```
   Generated initial password for first-time setup. Please change it via Settings or CLI. username=admin password=AbCdEfGhIjKlMnOpQrSt
   ```
2. **Open your browser**: Navigate to `http://localhost:8080`
3. **Log in**: Go to `/login` and use the username (`admin` by default) and the generated password from the logs
4. **Change your password**: Go to `/settings` and update your password to something memorable (or set up passkeys for passwordless login)
5. **Create a post**: Go to `/editor`

**Alternative**: You can also set credentials before first run using the CLI:
```bash
./GoBlog --config ./config/config.yml setup --username admin --password "your-secure-password"
```

---

## Installation

### Docker Installation (Recommended)

GoBlog provides two Docker images:

- `ghcr.io/jlelse/goblog:latest` - Base image
- `ghcr.io/jlelse/goblog:tools` - Includes `sqlite3`, `bash`, `curl` for hook commands

**Basic setup:**

```yaml
services:
  goblog:
    container_name: goblog
    image: ghcr.io/jlelse/goblog:latest
    restart: unless-stopped
    volumes:
      - ./config:/app/config  # Configuration files
      - ./data:/app/data      # Database and uploads
      - ./static:/app/static  # Optional: static files
    environment:
      - TZ=Europe/Berlin
```

**With built-in HTTPS (Let's Encrypt):**

```yaml
services:
  goblog:
    container_name: goblog
    image: ghcr.io/jlelse/goblog:latest
    restart: unless-stopped
    volumes:
      - ./config:/app/config
      - ./data:/app/data
    ports:
      - "80:80"
      - "443:443"
    environment:
      - TZ=Europe/Berlin
```

Config for built-in HTTPS:

```yaml
server:
  publicAddress: https://yourdomain.com
  publicHttps: true  # Enable Let's Encrypt
```

### Reverse Proxy Setup

If you prefer using a reverse proxy, here's a Caddy example:

```
yourdomain.com {
    reverse_proxy localhost:8080
}
```

For other reverse proxies (nginx, Traefik, etc.), configure them to proxy requests to GoBlog's port (default 8080).

### Building from Source

**Requirements:**
- Go 1.25 or later
- SQLite 3.38+ with FTS5 and JSON support (or use embedded SQLite)
- Linux (primary target, others may work)

**Build commands:**

```bash
# With system libsqlite3
go build -tags=linux,libsqlite3,sqlite_fts5 -o GoBlog

# With embedded SQLite (no system dependency)
go build -tags=linux,sqlite_fts5 -o GoBlog
```

### Data Storage

GoBlog stores all data in the `data` directory:

- `data/db.sqlite` - SQLite database (posts, comments, sessions, etc.)
- `data/media/` - Uploaded media files (served at `/m/`)
- `data/profileImage` - Your profile image
- `data/access.log` - HTTP access logs (if enabled)

**Important**: Always backup the `data` directory regularly!

---

## Configuration

Configuration is done via a YAML file (default: `./config/config.yml`).

### Minimal Configuration

```yaml
server:
  publicAddress: http://localhost:8080
```

That's it! GoBlog uses sensible defaults for everything else.

### Configuration Reference

For all available configuration options with detailed explanations, see [`example-config.yml`](/example-config.yml) in the repository.

**Key configuration sections:**

- `server` - HTTP server, HTTPS, domains, logging, Tor
- `database` - SQLite file path, dump, debug
- `cache` - Enable/disable caching and TTL
- `user` - Credentials, profile, 2FA, app passwords
- `blogs` - Multiple blog configuration
- `hooks` - Shell commands on events
- `micropub` - Micropub parameters and media storage
- `activityPub` - ActivityPub/Fediverse settings
- `webmention` - Webmention settings
- `notifications` - Ntfy, Telegram, Matrix
- `privateMode` - Restrict public access
- `indexNow` - Search engine notifications
- `tts` - Text-to-speech settings
- `reactions` - Emoji reactions
- `pathRedirects` - Regex-based redirects
- `mapTiles` - Custom map tile source
- `robotstxt` - Block specific bots
- `pprof` - Developer profiling
- `debug` - Verbose logging

### Important Notes

- **Blog title and description**: These are now configured via the `/settings` UI. Any title/description in the YAML config will be migrated to the database on first run.
- **Sections**: Sections are now configured via the `/settings` UI. Any sections in the YAML config will be migrated to the database on first run.
- **Default values**: Most settings have sensible defaults. Only configure what you need to change.
- **Reload**: After changing the config file, restart GoBlog or use `/reload` (only rebuilds router, doesn't re-read YAML).

---

## Writing Posts

### Post Format

Posts are written in **Markdown** with optional **front matter**. For Markdown syntax, see the [Markdown Guide](https://www.markdownguide.org/).

**GoBlog-specific Markdown features include:**
- Syntax highlighting in code blocks
- Automatic link detection
- `<mark>` blogs by using `==Text==`

**Example post:**

```markdown
---
title: My First Post
section: posts
tags:
  - introduction
  - blogging
published: 2025-01-15T10:00:00Z
---

This is my first post on GoBlog!

## Markdown Support

You can use all standard Markdown features.
```

### Front Matter

Front matter uses YAML syntax and can be delimited by `---`, `+++`, or any repeated character (e.g., `xxx`).

**Common parameters:**

```yaml
---
# Important
section: posts              # Which section (posts, notes, etc.)
status: published           # published, draft, scheduled
visibility: public          # public, unlisted, private

# Optional
title: Post Title           # Yes, titles are optional
slug: custom-slug           # Custom URL slug
path: /custom/path          # Full custom path
published: 2025-01-15T10:00:00Z
updated: 2025-01-16T12:00:00Z
priority: 0                 # Higher = appears first

# Taxonomies
tags:
  - tag1
  - tag2

# Media
images:
  - https://example.com/image.jpg
imagealts:
  - Image description

# Location
location: geo:51.5074,-0.1278  # Latitude,Longitude

# GPX Track (paste GPX file content as parameter value)
# Tip: Optimize the GPX file using the tool in the editor
gpx: |
  <?xml version="1.0"?>
  <gpx version="1.1">
    <!-- GPX content here -->
  </gpx>

# IndieWeb
replylink: https://example.com/post
likelink: https://example.com/post
link: https://example.com      # For bookmarks

# Custom parameters
mycustom: value
---
```

### Post Status

- `published` - Visible according to visibility setting
- `draft` - Only visible when logged in
- `scheduled` - Will be published at the `published` date/time

**Note**: The `-deleted` suffix (e.g., `published-deleted`) is automatically added by GoBlog when you delete a post. You don't need to set this manually.

### Visibility

- `public` - Visible to everyone, included in feeds, indexed by search engines
- `unlisted` - Visible to anyone with the link, but not in feeds or indexes
- `private` - Only visible when logged in

### Path Templates

Sections can have custom path templates. Configure these in the `/settings` UI under the sections management.

Available variables: `.Section`, `.Slug`, `.Year`, `.Month`, `.Day`, `.BlogPath`

Example: `{{printf "/%v/%v/%v/%v" .Section .Year .Month .Slug}}` results in `/posts/2025/01/my-post`

### Scheduling Posts

To schedule a post for future publication:

```yaml
---
title: Future Post
status: scheduled
published: 2025-12-25T09:00:00Z
---
```

GoBlog checks every 30 seconds and automatically publishes scheduled posts when their time comes.

---

## Editor & Publishing

### Web Editor

Access the editor at `/editor` (requires login).

**Features:**
- Live preview via WebSocket
- Markdown syntax highlighting
- Media upload
- Post management (create, update, delete, undelete)
- Visibility and status controls

**Creating a post:**

1. Go to `/editor`
2. Write your content (Markdown with optional front matter)
3. Click "Create" or "Update"
4. Post is immediately published (or saved as draft/scheduled)

### Micropub API

GoBlog supports the [Micropub](https://micropub.spec.indieweb.org/) protocol for creating/updating posts via API.

**Endpoints:**
- `/micropub` - Main endpoint
- `/micropub/media` - Media uploads

**Authentication:** Use IndieAuth to obtain a token.

**Example (create post):**

```bash
curl -X POST https://yourblog.com/micropub \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": ["h-entry"],
    "properties": {
      "content": ["Hello from Micropub!"],
      "category": ["test"]
    }
  }'
```

**Supported Micropub clients:**
- Indigenous (iOS/Android)
- Quill
- Micropublish
- And many others

---

## IndieWeb & Fediverse

GoBlog has first-class support for IndieWeb and ActivityPub protocols.

### IndieAuth

Use your blog as your identity on the web.

**Endpoints:**
- `/.well-known/oauth-authorization-server` - Server metadata
- `/indieauth` - Authorization endpoint
- `/indieauth/token` - Token endpoint

**How it works:**

1. Your blog automatically advertises IndieAuth endpoints
2. Use your blog URL (`https://yourblog.com/`) to sign in to IndieAuth-enabled sites
3. Approve the authorization request on your blog
4. You're logged in!

**Custom IndieAuth Address:**

If you're migrating domains and want to keep using your old domain for IndieAuth (to preserve existing app authorizations), you can configure an alternative IndieAuth address:

```yaml
server:
  publicAddress: https://new.example.com
  altAddresses:
    - https://old.example.com
  indieAuthAddress: https://old.example.com  # Must be one of altAddresses
```

This will advertise the IndieAuth endpoints on the old domain while serving all other content from the new domain.

### Webmention

Send and receive webmentions automatically.

**Receiving:**
- Endpoint: `/webmention`
- Webmentions are queued and verified asynchronously
- Approve/delete via `/webmention` admin UI

**Sending:**
- Automatic when you link to other sites in your posts
- Disabled in private mode for external targets

**Disable:**

```yaml
webmention:
  disableSending: true
  disableReceiving: true
```

### ActivityPub (Fediverse)

Publish your blog to Mastodon, Pleroma, and other ActivityPub platforms.

**Enable ActivityPub:**

```yaml
activityPub:
  enabled: true
  tagsTaxonomies:
    - tags  # Use tags as hashtags
```

**Your Fediverse address:** `@blogname@yourdomain.com`

**Features:**
- ✅ Publish posts to followers
- ✅ Receive replies as comments
- ✅ Receive likes and boosts (notifications)
- ✅ Followers collection
- ✅ Webfinger discovery
- ✅ Account migration (Move activity support)
- ❌ Following others (not supported - publish only)

**Endpoints:**
- `/.well-known/webfinger` - Webfinger
- `/activitypub/inbox/{blog}` - Inbox
- `/activitypub/followers/{blog}` - Followers

**Migration from another Fediverse server to GoBlog:**

If you're moving from another Fediverse server and want to migrate your followers to GoBlog:

1. Add your old account URL to the `alsoKnownAs` config:

```yaml
activityPub:
  enabled: true
  alsoKnownAs:
    - https://mastodon.example.com/users/oldusername
```

2. On your old Fediverse account, initiate the move to your GoBlog account using your old server's migration feature.

**Migration from GoBlog to another Fediverse server:**

If you're moving away from GoBlog to another Fediverse server:

1. Set up your new account on the target Fediverse server
2. Add your GoBlog account URL to the new account's "Also Known As" aliases (e.g., `https://yourblog.com`)
3. Run the CLI command to send Move activities to all followers:

```bash
./GoBlog activitypub move-followers blogname https://newserver.example.com/users/newusername
```

This sends a Move activity to all your followers, notifying them that your account has moved. Fediverse servers that support account migration will automatically update the follow to your new account.

**Domain change (moving GoBlog to a new domain):**

If you're changing your GoBlog domain (e.g., from `old.example.com` to `new.example.com`):

1. If you are using a reverse proxy, configure it to serve both domains pointing to your GoBlog instance
2. Add the old domain to the `altAddresses` config:

```yaml
server:
  publicAddress: https://new.example.com
  altAddresses:
    - https://old.example.com
```

3. Restart GoBlog to apply the configuration
4. Run the CLI command to send Move activities to all followers:

```bash
./GoBlog activitypub domainmove https://old.example.com https://new.example.com
```

This sends a Move activity from the old domain's actor to all followers, notifying them that the account has moved to the new domain. The old domain will continue to serve ActivityPub/Webfinger requests with the actor showing `movedTo` pointing to the new domain, while all other requests will be redirected to the new domain.

### Bluesky / ATProto

GoBlog can post links to new posts on Bluesky:

```yaml
# Global or per-blog
atproto:
  enabled: true
  pds: https://bsky.social
  handle: yourdomain.com
  password: YOUR_APP_PASSWORD  # Create at bsky.app/settings
  tagsTaxonomies:
    - tags
```

---

## Optional Features

Most features are opt-in. Enable only what you need. See [`example-config.yml`](/example-config.yml) for detailed configuration options for each feature.

### Comments

Enable comments via Webmention or ActivityPub:

```yaml
blogs:
  main:
    comments:
      enabled: true
```

**Admin UI:** `/comment`

**Disable per post:** Add `comments: false` to front matter

### Reactions

Enable emoji reactions on posts:

```yaml
reactions:
  enabled: true
```

Hardcoded reactions: ❤️ 👍 👎 😂 😱

**Disable per post:** Add `reactions: false` to front matter

### Search

Enable full-text search (SQLite FTS5):

```yaml
blogs:
  main:
    search:
      enabled: true
      path: /search
      title: Search
      placeholder: Search this blog
```

### Photos Index

Show all posts with images:

```yaml
blogs:
  main:
    photos:
      enabled: true
      path: /photos
      title: Photos
      description: My photo gallery
```

### Map

Show posts with locations and GPX tracks on a map:

```yaml
blogs:
  main:
    map:
      enabled: true
      path: /map
      allBlogs: false  # Show all blogs or just this one
```

**Add location to posts:** `location: geo:51.5074,-0.1278`

**Add GPX track:** Paste the entire GPX file content into the `gpx` parameter in your post's front matter

### Blogroll

Display blogs you follow (OPML):

```yaml
blogs:
  main:
    blogroll:
      enabled: true
      path: /blogroll
      opml: https://example.com/subscriptions.opml
      authHeader: X-Auth  # Optional
      authValue: secret   # Optional
      categories:         # Optional filter
        - Blogs
```

### Other Optional Features

See [`example-config.yml`](/example-config.yml) for configuration of:

- **Statistics** - Posts per year
- **Random post** - Redirect to random post
- **On this day** - Posts from this day in previous years
- **Contact form** - SMTP-based contact form
- **Text-to-Speech** - Google Cloud TTS audio generation
- **Notifications** - Ntfy, Telegram, Matrix
- **Short URLs** - Custom short domain
- **Tor Hidden Service** - .onion address
- **Private mode** - Login-only access
- **IndexNow** - Search engine notifications

### Media Storage

By default, media is stored locally in `data/media/` and served at `/m/`.

**Local compression:**

When enabled, uploaded images are automatically compressed to reduce file size and bandwidth:

```yaml
micropub:
  mediaStorage:
    localCompressionEnabled: true
```

This compresses images while maintaining reasonable quality. **Note**: Local compression is automatically disabled when private mode is enabled.

**External storage:**

You can use BunnyCDN or FTP for media storage instead of local storage. See [`example-config.yml`](/example-config.yml) for configuration details.

---

## Plugins

GoBlog has a plugin system that allows extending functionality without modifying the source code. Plugins are written in Go and interpreted at runtime using [Yaegi](https://github.com/traefik/yaegi).

### Plugin Configuration

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

### Built-in Plugins

GoBlog includes several plugins that you can use immediately.

#### Custom CSS

**Path:** `embedded:customcss` | **Import:** `customcss`

Add custom CSS to every HTML page. The CSS file is minified and appended to the HTML head.

**Config:**
```yaml
plugins:
  - path: embedded:customcss
    import: customcss
    config:
      file: ./custom.css  # Path to your CSS file
```

#### Syndication Links

**Path:** `embedded:syndication` | **Import:** `syndication`

Adds hidden `u-syndication` data elements to post pages when the configured post parameter is available. Useful for POSSE (Publish on Own Site, Syndicate Elsewhere).

**Config:**
```yaml
plugins:
  - path: embedded:syndication
    import: syndication
    config:
      parameter: syndication  # Post parameter name (default: "syndication")
```

**Usage:** Add `syndication: https://twitter.com/user/status/123` to your post's front matter.

#### Webrings

**Path:** `embedded:webrings` | **Import:** `webrings`

Adds webring links to the bottom of the blog footer.

**Config:**
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

#### AI Summary (TL;DR)

**Path:** `embedded:aitldr` | **Import:** `aitldr`

Uses the OpenAI API to generate a short one-sentence summary for blog posts (after creating or updating).

**Config:**
```yaml
plugins:
  - path: embedded:aitldr
    import: aitldr
    config:
      apikey: YOUR_OPENAI_API_KEY  # Required
      model: gpt-4o                # Optional, default is gpt-4o
      endpoint: https://api.scaleway.ai/.../chat/completions  # Optional, for OpenAI-compatible APIs
      default:  # Blog name
        title: "AI Summary:"  # Optional, custom title
```

**Disable per post:** Add `noaitldr: "true"` to front matter.

#### AI Image Captions

**Path:** `embedded:aiimage` | **Import:** `aiimage`

Uses the OpenAI API to generate short image captions for images in blog posts. Triggered by a button on the post.

**Config:**
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

#### Snow Animation

**Path:** `embedded:snow` | **Import:** `snow`

Adds a snow animation to pages using CSS and JavaScript. Perfect for winter holidays!

**Config:**
```yaml
plugins:
  - path: embedded:snow
    import: snow
```

No configuration options.

#### AI Bot Block

**Path:** `embedded:aibotblock` | **Import:** `aibotblock`

Enhances `/robots.txt` with AI bot user-agents and blocks requests from them. User-agents are sourced from [ai-robots-txt](https://github.com/ai-robots-txt/ai.robots.txt).

**Config:**
```yaml
plugins:
  - path: embedded:aibotblock
    import: aibotblock
```

No configuration options.

#### Image Tooltips

**Path:** `embedded:imagetooltips` | **Import:** `imagetooltips`

Shows image titles in an alert when clicking on images. Improves mobile UX where hovering isn't possible.

**Config:**
```yaml
plugins:
  - path: embedded:imagetooltips
    import: imagetooltips
```

No configuration options.

#### Telegram Bot

**Path:** `embedded:telegrambot` | **Import:** `telegrambot`

Post to your blog via a Telegram bot. Send the bot your post content and it will publish it and respond with the link. You can use front matter syntax in your message. Also supports file uploads.

**Config:**
```yaml
plugins:
  - path: embedded:telegrambot
    import: telegrambot
    config:
      token: YOUR_TELEGRAM_BOT_TOKEN    # Required
      allowed: YOUR_TELEGRAM_USERNAME   # Required
```

#### Demo Plugin

**Path:** `embedded:demo` | **Import:** `demo`

A demo plugin showcasing various plugin features. Useful for learning how to create your own plugins.

**Config:**
```yaml
plugins:
  - path: embedded:demo
    import: demo
```

### Creating Custom Plugins

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

### Plugin Types

Plugins can implement multiple interfaces:

- **SetApp** - Receive app instance (database, HTTP client, etc.)
- **SetConfig** - Receive plugin configuration
- **Exec** - Run background tasks (runs in goroutine)
- **Middleware** - HTTP middleware with priority
- **UI** - Modify rendered HTML (stream-based)
- **UI2** - Modify rendered HTML (DOM-based with goquery)
- **UISummary** - Modify post summaries on index pages
- **UIPost** - Modify post HTML on post pages
- **UIPostContent** - Modify post content HTML (all outputs)
- **UIFooter** - Modify footer HTML
- **PostCreatedHook** - React to post creation
- **PostUpdatedHook** - React to post updates
- **PostDeletedHook** - React to post deletion

See [plugintypes documentation](https://pkg.go.dev/go.goblog.app/app/pkgs/plugintypes) for details.

### Available Packages for Plugins

GoBlog provides several packages that plugins can use without vendoring:

**GoBlog packages:**
- `go.goblog.app/app/pkgs/plugintypes` - Plugin interfaces (required)
- `go.goblog.app/app/pkgs/htmlbuilder` - HTML generation
- `go.goblog.app/app/pkgs/bufferpool` - Efficient buffer management
- `go.goblog.app/app/pkgs/builderpool` - Efficient string builder management

**Third-party packages:**
- `github.com/PuerkitoBio/goquery` - HTML manipulation (jQuery-like)
- `github.com/carlmjohnson/requests` - HTTP requests
- `github.com/araddon/dateparse` - Date parsing

For more examples, see the [embedded plugins](/plugins/) in the repository.

---

## Administration

### Admin Paths

**Global paths (accessible from any blog):**
- `/login` - Login page
- `/logout` - Logout
- `/notifications` - View notifications
- `/reload` - Reload router (after config changes)

**Blog-relative paths:**
- `{blog}/settings` - User and blog settings
- `{blog}/editor` - Post editor (e.g., `/editor` for default blog)
- `{blog}/webmention` - Manage webmentions
- `{blog}/comment` - Manage comments

### Settings UI

Access `/settings` to configure:

- **User profile** - Name, username, profile image
- **Blog settings** - Blog title and description (subtitle)
- **Blog sections** - Add, edit, delete sections; configure path templates
- **Default section** - Set default section per blog
- **UI preferences** - Hide buttons, add reply context, etc.

### Managing Posts

**Via web editor:**
- Create: Go to `/editor`
- Edit: Go to `/editor?path=/your/post`
- Delete: Click delete button in editor
- Undelete: Click undelete button for deleted posts

**Via Micropub:**
- Use any Micropub client
- Or use the API directly with your IndieAuth token

### Managing Comments

1. Go to `/webmention` to view all webmentions and ActivityPub interactions
2. Approve or delete webmentions
3. Go to `/comment` to edit or permanently delete comments

### Database Backup

**Automatic SQL dump:**

```yaml
database:
  dumpFile: data/db.sql
```

This creates an hourly SQL dump of your database.

**Manual backup:**

```bash
# Stop GoBlog
docker-compose down

# Backup the data directory
cp -r data data-backup-$(date +%Y%m%d)

# Restart
docker-compose up -d
```

---

## CLI Commands

GoBlog provides several CLI commands for administration and maintenance.

### Start Server (Default)

```bash
./GoBlog --config ./config/config.yml
```

### Set Up User Credentials

```bash
./GoBlog --config ./config/config.yml setup --username admin --password "your-secure-password"
```

Set up the user credentials (username, password, and optionally TOTP). The password is securely hashed using bcrypt before storage.

**Options:**
- `--username` (required) - Login username
- `--password` (required) - Login password (stored as bcrypt hash)
- `--totp` - Enable TOTP two-factor authentication

**Example with TOTP (two-factor authentication):**

```bash
./GoBlog --config ./config/config.yml setup --username admin --password "your-secure-password" --totp
```

This will output the TOTP secret which you should add to your authenticator app.

### Health Check

```bash
./GoBlog --config ./config/config.yml healthcheck
# Exit code: 0 = healthy, 1 = unhealthy
```

Useful for container health checks and monitoring.

### Check External Links

```bash
./GoBlog --config ./config/config.yml check
```

Checks all external links in published posts and reports broken links.

### Export Posts

```bash
./GoBlog --config ./config/config.yml export ./exported
```

Exports all posts as Markdown files with front matter to the specified directory.

### Refetch ActivityPub Followers

```bash
./GoBlog --config ./config/config.yml activitypub refetch-followers blogname
```

Updates follower information from remote ActivityPub servers.

### Check and Clean ActivityPub Followers

```bash
./GoBlog --config ./config/config.yml activitypub check-followers blogname
```

Checks all ActivityPub followers by contacting each follower's home server. Reports which followers are still active, which accounts no longer exist (gone), and which have moved to a new account. After the check, displays a summary and prompts for confirmation before removing gone and moved followers from the database.

### Add ActivityPub Follower

```bash
./GoBlog --config ./config/config.yml activitypub add-follower blogname https://mastodon.example.com/users/alice
./GoBlog --config ./config/config.yml activitypub add-follower blogname @alice@mastodon.example.com
```

Manually adds an ActivityPub follower by actor IRI or `@user@instance` handle. When a handle is provided, it is resolved via WebFinger to find the actor's IRI. The remote actor profile is then fetched and stored in the follower database. Useful for re-adding accidentally removed followers.

### Move ActivityPub Followers

```bash
./GoBlog --config ./config/config.yml activitypub move-followers blogname https://newserver.example.com/users/newaccount
```

Sends Move activities to all followers, instructing them that your account has moved to a new Fediverse server. The blog's ActivityPub profile will also be updated with a `movedTo` field pointing to the new account.

**Requirements before running:**
1. Create your new account on the target Fediverse server
2. Add your GoBlog account URL (e.g., `https://yourblog.com`) to the new account's "Also Known As" aliases

**Note:** Most ActivityPub implementations will automatically trigger a follow for the new account when they receive the Move activity.

### Clear Moved Status

```bash
./GoBlog --config ./config/config.yml activitypub clear-moved blogname
```

Clears the `movedTo` setting from a blog's ActivityPub profile. Use this if you need to undo a migration or if you accidentally set the wrong target.

### Domain Move

```bash
./GoBlog --config ./config/config.yml activitypub domainmove https://old.example.com https://new.example.com
```

Sends Move activities to all followers when changing your GoBlog domain. This is used when you're moving GoBlog from one domain to another (e.g., `old.example.com` to `new.example.com`).

**Requirements before running:**
1. Configure both domains to point to your GoBlog instance
2. Add the old domain to `altAddresses` in your server config
3. Update `publicAddress` to the new domain
4. Restart GoBlog

**How it works:**
- The old domain's actor will serve with `movedTo` pointing to the new domain
- The new domain's actor will have `alsoKnownAs` including the old domain
- Move activities are sent from the old domain's actor to all followers
- Non-ActivityPub requests to the old domain will be redirected to the new domain

### Profiling

```bash
./GoBlog --config ./config/config.yml \
  --cpuprofile cpu.prof \
  --memprofile mem.prof
```

Generates CPU and memory profiles for performance analysis.

---

## Troubleshooting

### Common Issues

**"sqlite not compiled with FTS5"**

Your system's SQLite doesn't have FTS5 support. Use embedded SQLite:

```bash
go build -tags=linux,sqlite_fts5 -o GoBlog
```

**"no public address configured"**

Add to config:

```yaml
server:
  publicAddress: http://localhost:8080
```

**"default blog does not exist"**

Ensure `defaultBlog` matches a key in `blogs`:

```yaml
defaultBlog: main
blogs:
  main:
    path: /
```

**Posts not publishing**

- Check `status: published` in front matter
- Check `visibility: public` (or `unlisted`)
- For scheduled posts, ensure `published` date is in the past

**ActivityPub not working**

- Ensure `activityPub.enabled: true`
- Ensure `privateMode.enabled: false`
- Verify DNS resolves to your server
- Verify HTTPS is working properly

**Media uploads failing**

- Check available disk space
- Check file permissions on `data/media/`
- For external storage, verify credentials

### Logs

**Application logs:**

```bash
# Docker
docker-compose logs -f goblog

# Binary
./GoBlog --config ./config/config.yml
```

**Access logs:**

```yaml
server:
  logging: true
  logFile: data/access.log
```

**Debug mode:**

```yaml
debug: true
```

**Database debug:**

```yaml
database:
  debug: true  # Log all SQL queries
```

---

## Advanced Topics

### Hooks

Execute shell commands on specific events:

```yaml
hooks:
  shell: /bin/bash
  
  hourly:
    - echo "Hourly task"
  
  prestart:
    - echo "Starting GoBlog"
  
  postpost:
    - echo "New post at {{.URL}}"
  
  postupdate:
    - echo "Updated {{.URL}}"
  
  postdelete:
    - echo "Deleted {{.URL}}"
  
  postundelete:
    - echo "Undeleted {{.URL}}"
```

Post hooks receive `.URL` (string) and `.Post` (object with post data) as template variables.

### Regex Redirects

Create custom redirects using regular expressions:

```yaml
pathRedirects:
  - from: "\\/index\\.xml"
    to: ".rss"
    type: 302
  
  - from: "^\\/(old|legacy)\\/(.*)$"
    to: "/new/$2"
    type: 301
```

### Post Aliases

Redirect old URLs to new posts:

```yaml
---
path: /new-path
aliases:
  - /old-path
  - /another-old-path
---
```

### Custom Domains

**Short domain:**

```yaml
server:
  shortPublicAddress: https://short.example.com
```

**Media domain:**

```yaml
server:
  mediaAddress: https://media.example.com
  cspDomains:
    - media.example.com  # Add to Content Security Policy
```

**Important**: Custom domains (short URLs and media URLs) are global for the entire GoBlog instance. You cannot use different domains for different blogs on the same instance.

### Multiple Blogs

Run multiple blogs under one GoBlog installation. Common use cases:

- **Multiple languages** - Separate blogs for different languages
- **Different content types** - Personal vs. professional content
- **Multi-author scenarios** - Different blogs for different authors

**Example configuration:**

```yaml
defaultBlog: en

blogs:
  en:
    path: /
    lang: en
    # title and description are configured via the settings UI
  
  de:
    path: /de
    lang: de
  
  fr:
    path: /fr
    lang: fr
```

Each blog has its own:
- Title and description (configurable via settings UI)
- Sections and taxonomies
- Menus and navigation
- Settings and preferences
- ActivityPub actor (`@blogname@yourdomain.com`)

**Note**: All blogs share the same user account and global settings (server, database, etc.).

### Performance Tuning

**Enable caching:**

```yaml
cache:
  enable: true
  expiration: 600  # 10 minutes
```

**Optimize database:**

```bash
sqlite3 data/db.sqlite "VACUUM;"
sqlite3 data/db.sqlite "ANALYZE;"
```

**Use CDN for media:**

Configure external media storage (BunnyCDN, FTP) to offload media serving.

### Security

**Enable security headers:**

```yaml
server:
  securityHeaders: true  # Auto-enabled with HTTPS
```

**Use HTTPS:**

```yaml
server:
  publicHttps: true  # Let's Encrypt
```

**Initial password:**

On first startup, GoBlog automatically generates a secure random password and logs it to the console. Look for a log line like:

```
Generated initial password for first-time setup. Please change it via Settings or CLI. username=admin password=AbCdEfGhIjKlMnOpQrSt
```

You should change this password immediately via the Settings UI or CLI.

**Set password via CLI:**

```bash
./GoBlog --config ./config/config.yml setup --username admin --password "your-secure-password"
```

Passwords are securely hashed using bcrypt before storage in the database.

**Change password via Settings UI:**

After initial setup, you can change your password through the `/settings` UI under the "Security" section.

**Enable 2FA (TOTP):**

TOTP can be configured via the Settings UI or during initialization:

```bash
# Via CLI
./GoBlog --config ./config/config.yml setup --username admin --password "secure-password" --totp
```

Then add the secret to your authenticator app (Google Authenticator, Authy, etc.).

**Passkeys (WebAuthn):**

GoBlog supports passwordless authentication via Passkeys (WebAuthn/FIDO2). You can register multiple passkeys (e.g., for different devices) through the `/settings` UI under the "Security" section.

Features:
- Register multiple passkeys with custom names
- Rename passkeys for easy identification
- Delete passkeys you no longer need

**App passwords for API access:**

App passwords provide secure access for third-party applications and scripts. They can be managed entirely through the Settings UI:

1. Go to `/settings`
2. Under "Security" → "App Passwords", enter a name for the app password
3. Click "Create app password"
4. Copy the generated token (shown only once!)

Use the token with Basic Authentication (any username works):

```bash
curl -u "anyuser:YOUR_APP_PASSWORD_TOKEN" https://yourblog.com/micropub
```

**Private mode:**

```yaml
privateMode:
  enabled: true
```

---

## Getting Help

- **Repository**: [GitHub](https://github.com/jlelse/GoBlog)
- **Issues**: Report bugs on GitHub
- **Example config**: See [`example-config.yml`](/example-config.yml)
- **Matrix chatroom**: Join the [GoBlog Matrix chat](https://matrix.to/#/#goblog:matrix.org)

---

## License

GoBlog is licensed under the MIT License. See [LICENSE](/LICENSE) for details.

---

**Happy blogging! 🎉**
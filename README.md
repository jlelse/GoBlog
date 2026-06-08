# GoBlog

**GoBlog** is a single-user, multi-blog platform written in Go. One binary, SQLite database, minimal dependencies. IndieWeb (IndieAuth, Micropub, Webmention) and ActivityPub support for Fediverse integration.

### Philosophy

- **Opt-in features**: most features disabled by default, enable only what you need
- **Markdown-first**: write in Markdown with optional YAML front matter
- **No themes**: customize through configuration, plugins, and custom CSS
- **Docker-friendly**: designed to run in containers with simple volume mounts
- **Full ownership**: your content, your database, your server

---

## Quick Start

### Docker (Recommended)

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

Start:

```bash
docker-compose up -d
```

### Build from Source

```bash
git clone https://github.com/jlelse/GoBlog.git
cd GoBlog
go build -tags=linux,libsqlite3,sqlite_fts5,gomailnotpl -o GoBlog
mkdir -p config data
echo 'server:
  publicAddress: http://localhost:8080' > config/config.yml
./GoBlog --config ./config/config.yml
```

### First Steps

1. **Check the logs**: On first startup, GoBlog generates a random password:
   ```
   Generated initial password for first-time setup. username=admin password=AbCdEfGhIjKlMnOpQrSt
   ```
2. **Open your browser**: `http://localhost:8080`
3. **Log in**: `/login` with username `admin` and the generated password
4. **Change your password**: Go to `/settings`
5. **Create a post**: Go to `/editor`

Alternatively, set credentials before first run:

```bash
./GoBlog --config ./config/config.yml setup --username admin --password "your-secure-password"
```

### Data Storage

All data lives in the `data` directory:
- `data/db.sqlite`: SQLite database
- `data/media/`: Uploaded media (served at `/m/`)
- `data/profileImage`: Profile image

**Backup the `data` directory regularly.**

---

## Documentation

- **[Configuration](docs/configuration.md)**: YAML configuration, HTTPS, Docker, data storage
- **[Settings UI](docs/settings-ui.md)**: Blog, user, and security settings configurable via the web UI
- **[Features](docs/features.md)**: Writing posts, editor, comments, reactions, search, media, feeds, and more
- **[IndieWeb & Fediverse](docs/indieweb-fediverse.md)**: IndieAuth, Webmention, ActivityPub, Bluesky
- **[Plugins](docs/plugins.md)**: Plugin system, built-in plugins, creating custom plugins
- **[CLI Commands](docs/cli.md)**: Server, setup, health check, export, follower management
- **[Advanced Topics](docs/advanced.md)**: Hooks, redirects, multiple blogs, performance tuning, troubleshooting
- **[Testing](docs/testing.md)**: Test suite, integration tests, benchmarks

---

## Key Features

### Core

- Markdown posts with YAML front matter
- Multiple blogs under one installation
- Sections, taxonomies, and custom path templates
- Web editor with live preview and formatting toolbar
- Media uploads with optional image optimization (imgproxy)
- Full-text search (SQLite FTS5)
- RSS, Atom, and JSON feeds
- XML sitemap and robots.txt

### IndieWeb & Fediverse

- **IndieAuth**: Use your blog as your identity
- **Micropub**: Create/update/delete posts via API
- **Webmention**: Send and receive webmentions automatically
- **ActivityPub**: Publish to Mastodon, Pleroma, and other Fediverse platforms
- **Bluesky/ATProto**: Post links to Bluesky

### Optional Features (Opt-in)

| Feature | Description |
|---------|-------------|
| Comments | Via Webmention or ActivityPub |
| Reactions | Emoji reactions on posts |
| Search | Full-text search (SQLite FTS5) |
| Photos | Gallery of all posts with images |
| Map | Posts with locations/GPX tracks on a map |
| Blogroll | Display blogs you follow via OPML |
| Statistics | Posts per year and monthly chart |
| Random Post | Redirect to a random post |
| On This Day | Posts from this day in previous years |
| Contact Form | SMTP-based contact form |
| Text-to-Speech | Google Cloud TTS or Mistral audio generation |
| Notifications | Ntfy, Telegram, or Matrix push notifications |
| Short URLs | Custom short domain for compact links |
| Image Optimization | Automatic AVIF/JPEG/PNG variants via imgproxy |
| Tor Hidden Service | .onion address |
| IndexNow | Notify search engines of new content |
| Private Mode | Login-only access |
| Announcement Banner | Site-wide banner for announcements |

Built-in: RSS, Atom, JSON feeds, XML sitemap, robots.txt, microformats2 markup.

---

## Configuration

Configuration is done via a YAML file (default: `./config/config.yml`).

```yaml
server:
  publicAddress: http://localhost:8080
```

All other settings have sensible defaults. See [`example-config.yml`](/example-config.yml) for the full reference and [`docs/configuration.md`](docs/configuration.md) for detailed explanations.

Blog title, description, sections, and many other settings are configured via the [Settings UI](docs/settings-ui.md) (`/settings`), not YAML.

---

## Plugins

GoBlog has a runtime plugin system (based on Yaegi). See [`docs/plugins.md`](docs/plugins.md) for full details.

**Built-in plugins:** Custom CSS, Syndication Links, Webrings, AI Summary, AI Image Captions, Snow Animation, AI Bot Block, Image Tooltips, Telegram Bot, MCP Server.

---

## Administration

### Quick Reference

| Path | Purpose |
|------|---------|
| `/login`, `/logout` | Authentication |
| `/settings` | User and blog settings |
| `/editor` | Post editor |
| `/webmention` | Manage webmentions |
| `/comment` | Manage comments |
| `/notifications` | View notifications |
| `/reload` | Reload router |

### CLI Commands

```bash
./GoBlog --config ./config/config.yml                              # Start server
./GoBlog --config ./config/config.yml setup --username admin --password "pass"  # Setup credentials
./GoBlog --config ./config/config.yml healthcheck                  # Health check
./GoBlog --config ./config/config.yml check --ignore-403           # Check external links
./GoBlog --config ./config/config.yml export ./exported            # Export posts
```

See [`docs/cli.md`](docs/cli.md) for full reference.

---

## Getting Help

- **Repository**: [github.com/jlelse/GoBlog](https://github.com/jlelse/GoBlog)
- **Example config**: [`example-config.yml`](/example-config.yml)
- **Matrix chat**: [GoBlog Matrix room](https://matrix.to/#/#goblog:matrix.org)
- **Issues**: Report bugs on GitHub

---

## License

MIT License. See [LICENSE](/LICENSE) for details.

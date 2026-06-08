# Advanced Topics

## Hooks

Execute shell commands on specific events. Configure globally in YAML.

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

Post hooks receive template variables:
- `.URL` (string): The post's URL
- `.Post` (object): The post object with all post data

Use hooks for tasks like:
- Sending notifications to external services
- Triggering static site generators
- Backing up posts after changes
- Updating search indexes

## Regex Redirects

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

The `from` field is a Go regular expression. The `to` field can use capture groups (`$1`, `$2`, etc.). The `type` field is optional (defaults to 302).

## Post Aliases

Redirect old URLs to new posts using the `aliases` front matter parameter:

```yaml
---
path: /new-path
aliases:
  - /old-path
  - /another-old-path
---
```

Old URLs redirect permanently (301) to the new post.

## Custom Domains

### Short Domain

Use a custom short domain for compact post URLs:

```yaml
server:
  shortPublicAddress: https://short.example.com
```

Every post gets a short URL at `/s/{hex-id}` which redirects to the full post path.

### Media Domain

Use a custom domain for serving media files:

```yaml
server:
  mediaAddress: https://media.example.com
  cspDomains:
    - media.example.com  # Add to Content Security Policy
```

Custom domains (short URLs and media URLs) are global for the entire GoBlog instance. You cannot use different domains for different blogs on the same instance.

## Multiple Blogs

Run multiple blogs under one GoBlog installation. Common use cases:

- **Multiple languages**: Separate blogs for different languages
- **Different content types**: Personal vs. professional content

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

All blogs share the same user account and global settings (server, database, etc.).

## Performance Tuning

### Enable Caching

Response caching is enabled by default with a 6-hour TTL. Configure in YAML (see [`example-config.yml`](/example-config.yml) for options):

```yaml
cache:
  enable: true
  expiration: 600  # Per-route override in seconds
```

Append `?cache=0` or `?cache=false` to any URL to skip the cache for that request.

### Optimize Database

Run SQLite maintenance commands:

```bash
sqlite3 data/db.sqlite "VACUUM;"
sqlite3 data/db.sqlite "ANALYZE;"
```

### Use CDN for Media

Configure external media storage (BunnyCDN, FTP) to offload media serving. See [`example-config.yml`](/example-config.yml) for configuration.

## Database Backup

### Automatic SQL Dump

Enable hourly SQL dumps:

```yaml
database:
  dumpFile: data/db.sql
```

### Manual Backup

Stop GoBlog, copy the `data` directory, restart:

```bash
# Docker
docker-compose down
cp -r data data-backup-$(date +%Y%m%d)
docker-compose up -d

# Binary
# Stop GoBlog, then:
cp -r data data-backup-$(date +%Y%m%d)
# Restart GoBlog
```

## Tor Hidden Service

GoBlog can serve as a Tor `.onion` hidden service. Set `server.tor: true` in your YAML config. Tor must be installed and available in `$PATH`.

When enabled, GoBlog:
- Generates and persists an Ed25519 private key at `data/tor/onion.pk` (the `.onion` address is stable across restarts)
- Starts an onion service on port 80
- Adds an `Onion-Location` HTTP header to all responses, directing Tor browsers to the `.onion` address
- Logs the `.onion` address at startup

The `.onion` address is derived from the persistent key, so it remains the same across restarts. The cache is purged when Tor starts.

## Docker Images

Two Docker images are available:

- `ghcr.io/jlelse/goblog:latest` - Base image
- `ghcr.io/jlelse/goblog:tools` - Includes `sqlite3`, `bash`, `curl` and other tools for hook commands or advanced users

## Troubleshooting

### Health Check Endpoint

GoBlog serves a `/ping` endpoint that returns HTTP 200 when the server is running. Use this for external monitoring, load balancers, or container health checks.

```bash
# CLI health check
./GoBlog --config ./config/config.yml healthcheck

# Manual check
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/ping
```

### Common Issues

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

## Security

### Private Mode

Restrict all public access. When enabled, every page requires authentication and `X-Robots-Tag: noindex` is set on all responses:

```yaml
privateMode:
  enabled: true
```

### Security Headers

When `securityHeaders: true` (auto-enabled with HTTPS), GoBlog sets: `Strict-Transport-Security` (1 year), `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff`, and a Content Security Policy. The CSP includes `frame-ancestors 'none'` (blocks all framing). See [`configuration.md`](configuration.md#security-headers) for CSP domain configuration.

Cookies are set as `Secure` when security headers are enabled or the public address uses HTTPS. A 100 MB body size limit is enforced on all requests.

### Access Logs

Enable HTTP access logging:

```yaml
server:
  logging: true
  logFile: data/access.log
```

Access logs are rotated daily and include timestamp, method, path, status code, and response time, but do not log client IP addresses for privacy reasons.

### Debug Logging

Enable verbose logging:

```yaml
debug: true
```

Enable SQL query logging:

```yaml
database:
  debug: true  # Log all SQL queries
```

### Profiling (pprof)

Enable Go's built-in profiling endpoints for performance analysis:

```yaml
pprof:
  enabled: true
  address: localhost:6060  # Optional, defaults to random port on localhost
```

When enabled, GoBlog starts a separate HTTP server with pprof endpoints:
- `/debug/pprof/` - Profiling index page
- `/debug/pprof/profile` - CPU profile (30-second sample)

The pprof server runs on a separate listener (not the main server) and is opt-in only. For CLI-based profiling, use the `--cpuprofile` and `--memprofile` flags.

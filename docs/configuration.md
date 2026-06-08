# Configuration

GoBlog is configured via a single YAML file (default: `./config/config.yml`).

## Minimal Configuration

```yaml
server:
  publicAddress: http://localhost:8080
```

That's it. GoBlog uses sensible defaults for everything else.

## Configuration Reference

For every available option with detailed explanations, see [`example-config.yml`](/example-config.yml) in the repository.

**Key configuration sections:**

| Section | Description |
|---------|-------------|
| `server` | HTTP server, HTTPS, domains, logging, Tor |
| `database` | SQLite file path, dump, debug |
| `cache` | Enable/disable caching and TTL (enabled by default, 6-hour TTL) |
| `user` | Credentials, profile, 2FA, app passwords, identity |
| `blogs` | Multiple blog configuration |
| `hooks` | Shell commands on events |
| `plugins` | Runtime plugin system (Yaegi) |
| `micropub` | Micropub parameters and media storage |
| `activityPub` | ActivityPub/Fediverse settings |
| `webmention` | Webmention settings |
| `notifications` | Ntfy, Telegram, Matrix |
| `privateMode` | Restrict public access |
| `indexNow` | Search engine notifications |
| `tts` | Text-to-speech settings |
| `pathRedirects` | Regex-based redirects |
| `mapTiles` | Custom map tile source |
| `robotstxt` | Block specific bots |
| `pprof` | Developer profiling |
| `debug` | Verbose logging |

## YAML vs UI Settings

Some settings are configured via YAML, others via the Settings UI (`/settings`):

**YAML only** (requires restart to change):
- Server settings (port, HTTPS, domains)
- Database settings
- Cache settings
- Hooks
- Plugin configuration
- ActivityPub and ATProto configuration
- Notification channels
- Media storage (BunnyCDN, FTP)
- Image optimization (imgproxy)
- Map tiles
- Path redirects
- Tor, IndexNow, private mode

**Settings UI** (no restart needed):
- Blog title and description
- Sections (create, edit, delete, path templates)
- User profile (name, username, profile image)
- Password, TOTP, passkeys, app passwords
- Reactions (enable/disable, configure emojis)
- Webmention settings (sending, receiving, block list)
- UI toggles (hide buttons, auto-fetch reply context)

## User Configuration

The `user` section in YAML configures identity and authentication:

```yaml
user:
  nick: admin                # Initial login username (editable via Settings UI after first run)
  name: Your Name            # Initial display name (editable via Settings UI after first run)
  email: contact@example.com # Used in feeds (RSS/Atom/JSON)
  link: https://example.net  # Optional homepage link (defaults to blog root)
  identities:                # rel=me links for identity verification
    - https://micro.blog/yourusername
    - https://github.com/yourusername
```

- **`nick`** and **`name`**: Seed the database on first run. After that, editable via Settings UI (`/settings`). YAML values are ignored once the database has values.
- **`email`**: Only used in feed author fields. YAML-only, no Settings UI.
- **`link`**: Optional URL for the author's h-card link (falls back to `/` if empty). YAML-only, no Settings UI.
- **`identities`**: Rendered as `<link rel="me" href="...">` tags in HTML headers for IndieWeb identity verification. YAML-only, no Settings UI.

**Authentication** (password, TOTP, app passwords, passkeys) is configured via the Settings UI or CLI setup command. Do not set these in YAML.

## Alternative Addresses

For domain migration or multiple domains:

```yaml
server:
  altAddresses:
    - https://old.example.com
  indieAuthAddress: https://old.example.com  # Must be one of altAddresses
```

- **`altAddresses`**: Old domains during migration. ActivityPub gets `alsoKnownAs`/`movedTo` entries. WebFinger and WebAuthn work on alt addresses. All non-ActivityPub/IndieAuth requests redirect to the main domain.
- **`indieAuthAddress`**: Override which domain IndieAuth endpoints are advertised on. Must be one of `altAddresses`. Falls back to `publicAddress` if unset.

## HTTPS and ACME Certificates

When `publicHttps` is enabled, GoBlog automatically:
- Obtains and renews TLS certificates via ACME TLS-ALPN-01 challenges (no port 80 required)
- Optionally starts an HTTP server (configurable via `httpsRedirectPort`, default port 80) to redirect HTTP to HTTPS

You can configure any ACME-compatible CA:

```yaml
server:
  publicAddress: https://yourdomain.com
  publicHttps: true
  acmeDir: https://acme.zerossl.com/v2/DV90  # Use ZeroSSL instead of Let's Encrypt
  acmeEabKid: "your-key-id"                  # External Account Binding key ID
  acmeEabKey: "your-key"                     # External Account Binding key (base64url)
```

For manual TLS (with your own certificate files), use `httpsCert` and `httpsKey` instead of `publicHttps`.

## Security Headers

When `securityHeaders: true` (auto-enabled with HTTPS), GoBlog sets these HTTP headers:

| Header | Value |
|--------|-------|
| `Strict-Transport-Security` | `max-age=31536000;` (1 year) |
| `Referrer-Policy` | `no-referrer` |
| `X-Content-Type-Options` | `nosniff` |
| `Content-Security-Policy` | Dynamic CSP policy |

### Content Security Policy (CSP)

The CSP allows:
- **`default-src`**: `'self'`, `blob:`, plus configured domains
- **`img-src`**: `'self'`, `data:`, plus configured domains
- **`style-src`**: `'self'`, SHA-256 hashes for inlined CSS, plus configured domains
- **`frame-ancestors`**: `'none'` (blocks all framing)

Domains are automatically included from: `publicAddress`, `shortPublicAddress`, `mediaAddress`, `altAddresses`, and media storage URL. Add additional domains via `cspDomains`:

```yaml
server:
  securityHeaders: true
  cspDomains:
    - media.example.com
    - cdn.example.com
```

## Reverse Proxy Setup

If you prefer a reverse proxy (nginx, Caddy, Traefik), configure it to proxy to GoBlog's port (default 8080) and disable `publicHttps`. Example Caddy config:

```
yourdomain.com {
    reverse_proxy localhost:8080
}
```

## Data Storage

GoBlog stores all data in the `data` directory:

- `data/db.sqlite` - SQLite database (posts, comments, sessions, etc.)
- `data/media/` - Uploaded media files (served at `/m/`)
- `data/profileImage` - Your profile image
- `data/access.log` - HTTP access logs (if enabled)
- `data/media-migrate.json` - Media migration perceptual hash cache

Always backup the `data` directory regularly.

## Important Notes

- **Reload**: After changing the config file, restart GoBlog or use `/reload` (only rebuilds router, doesn't re-read YAML).
- **Profile image**: Stored at the path specified by `user.profileImageFile` (default: `data/profileImage`). Configured via the Settings UI.
- **Default values**: Most settings have sensible defaults. Only configure what you need to change.
- **postAsHome**: Set `postAsHome: true` on a blog to use a post as the homepage instead of the post index. See [`features.md`](features.md#static-homepage) for details.

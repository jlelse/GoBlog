# Features

Most features are opt-in. Enable only what you need in your blog's YAML configuration.

## Writing Posts

Posts are written in **Markdown** with optional YAML **front matter**.

### Markdown Features

GoBlog-specific Markdown features:
- Syntax highlighting in code blocks
- Automatic link detection
- Highlight text using `==marked text==` (renders as `<mark>`)
- Image optimization (if enabled) generates optimized variants automatically

### Front Matter

Front matter uses YAML syntax and can be delimited by `---`, `+++`, or any repeated character (e.g., `xxx`).

**Example:**

```markdown
---
title: My First Post
section: posts
tags:
  - introduction
status: published
visibility: public
published: 2025-01-15T10:00:00Z
---

This is my first post on GoBlog!
```

### Front Matter Parameters

| Parameter | Description |
|-----------|-------------|
| `title` | Optional post title |
| `section` | Which section (posts, notes, etc.) |
| `status` | published, draft, scheduled |
| `visibility` | public, unlisted, private |
| `slug` | Custom URL slug |
| `path` | Full custom path |
| `priority` | Higher value = appears first in listings |
| `published` | Publication date (ISO 8601) |
| `updated` | Last modification date |
| `tags` | List of tags |
| `location` | Geo coordinates (`geo:lat,lon`) |
| `gpx` | GPX track content (paste or upload): statistics (distance, time, elevation) displayed on post |
| `showroute` | Set to `false` to hide GPX track from the map (statistics remain visible) |
| `videoplaylist` | HLS `.m3u8` stream URL for embedded video player |
| `images` | Image URLs for the post |
| `imagealts` | Alt text for images |
| `comments` | Set to `false` to disable per post |
| `reactions` | Set to `false` to disable per post |
| `webmention` | Set to `false` to disable outgoing webmentions for this post |
| `aliases` | List of old URLs to redirect |
| `replylink` | URL this post replies to |
| `likelink` | URL this post likes |
| `link` | URL this post bookmarks |
| `translationkey` | Links translated versions of the same post across blogs |
| `original` | Overrides the canonical URL for a post |
| `summary` | Custom post summary text (overrides auto-generated) |
| `audio` | Embeds an HTML audio player with the specified URL |
| `+<param>` | Prefix with `+` to append values instead of replacing (e.g., `+tags: newtag`) |
| [any key] | Custom parameters are preserved and accessible |

### Post Status

- `published`: Visible according to visibility setting
- `draft`: Only visible when logged in
- `scheduled`: Auto-publishes at the `published` date/time

When you delete a post, GoBlog appends the `-deleted` suffix to its status (e.g., `published-deleted`). You don't need to set this manually; use the undelete button in the editor to restore. A `deleted` timestamp is set; posts with a `deleted` date older than 7 days are automatically permanently purged.

### Visibility

- `public`: Visible to everyone, in feeds, indexed
- `unlisted`: Visible with link, not in feeds or indexes
- `private`: Only visible when logged in

### Scheduling

```yaml
---
status: scheduled
published: 2025-12-25T09:00:00Z
---
```

GoBlog checks every 30 seconds and publishes scheduled posts when their time comes.

### Path Templates

Sections can have custom path templates configured in the Settings UI. Available variables: `.Section`, `.Slug`, `.Year`, `.Month`, `.Day`, `.BlogPath`.

### Static Homepage

Set `postAsHome: true` in a blog's YAML config to use a specific post as the homepage instead of the post index. The post at the blog's base path (e.g., `/` for the default blog) is rendered as a simplified static page (no post parameters, taxonomies, or sharing buttons). Useful for a static "about" page as the landing page. See [`example-config.yml`](/example-config.yml).

### Index Page Query Filters

Filter any index page by post parameters using the `?p:` query prefix:

```
/posts?p:tags=golang                  # posts with tag "golang"
/posts?p:tags=golang&p:tags=indieweb  # posts with either tag (OR)
/posts?p:tags                         # posts that have any tags at all
```

Multiple values for the same key are combined with OR; different keys with AND. An empty value (`?p:key` or `?p:key=`) matches posts where the parameter exists with a non-empty value.

### Cache Bypass

Append `?cache=0` or `?cache=false` to any URL to skip the response cache.

## Editor & Publishing

### Web Editor

Access at `/editor` (requires login).

**Features:**
- **Formatting toolbar** with buttons for bold, italic, strikethrough, and links
- **Undo/Redo** buttons
- **Live preview** via WebSocket: auto-updates as you type, renders HTML in real time
- **Auto-save and multi-tab sync** via WebSocket: editor state is synchronized across all open browser tabs and persists across server restarts
- **Markdown syntax highlighting**
- **Media upload** via file picker
- **Template button** to pre-fill a configured template
- **Geo-location button**: query the browser location API to get the current coordinates in the right format (`geo:lat,lon`)
- **GPX helper**: merge multiple GPX files, returns minified YAML to paste into post front matter
- **File uses tracking**: shows which posts reference each uploaded file
- **File optimization**: manually trigger imgproxy optimization on a file
- **View file variants**: see all optimized variants generated from an uploaded file

#### Special Editor Pages

Accessible from the editor page (requires login):

| Path | Shows |
|------|-------|
| `/editor/drafts` | All draft posts |
| `/editor/private` | All private posts |
| `/editor/unlisted` | All unlisted posts |
| `/editor/scheduled` | All scheduled posts |
| `/editor/deleted` | All deleted posts (with undelete option) |
| `/editor/links` | All external links across the blog, with usage counts and drill-down per domain |
| `/editor/files` | All uploaded media files, with options to view their usage or optimized variants, delete them and optimize images using imgproxy |

### Post Interactions

Each post page includes interactive buttons (individually hideable via Settings UI):

- **Share**: Modal with multiple share targets (currently Email, Mastodon, Bluesky, LinkedIn, Micro.blog, Reddit, Hacker News, SMS), copy-to-clipboard, and native Web Share API. Text auto-truncated per platform.
- **Read Aloud**: Browser speech synthesis (Web Speech API) or pre-generated cloud TTS audio (using Google Cloud or Mistral).
- **Translate**: Links to Google Translate for the post.

### Micropub API

Create/update/delete posts via the [Micropub](https://micropub.spec.indieweb.org/) protocol.

**Endpoints:** `/micropub` (posts), `/micropub/media` (media uploads)

**Authentication:** IndieAuth token or app password.

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

Compatible with Indigenous, Quill, Micropublish, and other Micropub clients.

## Comments

Comments are received via Webmention or ActivityPub replies. Enable per blog in YAML (see [`example-config.yml`](/example-config.yml)).

- **Admin UI**: `/comment` to list or delete comments
- **Disable per post**: Add `comments: false` to front matter
- **Comment form fields**: `target`, `comment`, `name`, `website`

Comments trigger webmentions to the post's URL, notifying linked pages.

### Comment CAPTCHA

Anonymous visitors submitting comments must solve a numeric image CAPTCHA challenge. This is always active and not configurable. Logged-in users bypass the CAPTCHA. The CAPTCHA uses digit-based images (500x250 pixels) and expires after 10 minutes. Once solved, the session remembers the solution for 24 hours.

## Reactions

Emoji reactions on posts. Enable and configure via the Settings UI. The available emoji reactions are customizable (comma-separated list of actual emoji characters). Default reactions: ❤️, 👍, 🎉, 😂, 😱.

- **Disable per post**: Add `reactions: false` to front matter

## Search

Full-text search using SQLite FTS5. Enable per blog in YAML (see [`example-config.yml`](/example-config.yml)).

When search is enabled, an [OpenSearch](https://developer.mozilla.org/en-US/docs/Web/OpenSearch) descriptor is served at `/search/opensearch.xml`, allowing browsers to add the blog as a search engine.

## Photos Index

Gallery of all posts with images. Posts with the `images` front matter parameter are automatically included. Enable per blog in YAML (see [`example-config.yml`](/example-config.yml)).

## Profile Image

Set and update your profile image in the Settings UI. It is automatically used for favicons, the Fediverse actor image, and feed icons.

## Map

Interactive map showing posts with locations and GPX tracks. Enable per blog in YAML (see [`example-config.yml`](/example-config.yml)).

- **Add location to posts**: `location: geo:51.5074,-0.1278`
- **Add GPX track**: Upload via the editor's file picker, or paste GPX content into the `gpx` front matter parameter. Use the GPX helper to merge multiple files.

For privacy, map tiles are proxied through GoBlog. OpenStreetMap tiles are used by default; configure custom tile providers via `mapTiles` in YAML.

## Blogroll

Display blogs you follow via an OPML file. Enable per blog in YAML (see [`example-config.yml`](/example-config.yml)). The OPML file can be hosted externally (e.g., from an RSS reader export). Buttons on the blogroll page allow downloading the OPML and refreshing the cache.

## Announcement Banner

Display a Markdown banner on all pages of a blog. Useful for temporary notices, scheduled maintenance, or important updates. Configure via YAML.

## Text-to-Speech (TTS)

Generate audio versions of posts using Google Cloud TTS or Mistral TTS. Audio files are auto-generated after publishing public posts with a section. Users can play them via the Read Aloud button on post pages.

Configure globally in YAML. Supports Google Cloud TTS (API key) or Mistral TTS (API key + voice ID). Mistral takes precedence when both are configured.

## Statistics

Blog statistics at a configurable path. Displays total posts (with and without dates), yearly breakdowns, and monthly aggregates with word/character counts and words-per-post averages.

## Random Post

Redirects to a random post at a configured path.

## On This Day

Redirects to the date archive page showing posts from this day across all years.

## Contact Form

SMTP-based contact form with optional privacy policy checkbox. Accepted fields: `name`, `email`, `website`, `message` (all optional except `message`). Configure per blog in YAML.

## Date Archives

Date archives are auto-generated at these paths:

| Pattern | Example | Shows |
|---------|---------|-------|
| `/{year}/` | `/2025/` | Posts from that year |
| `/{year}/{month}/` | `/2025/06/` | Posts from that month |
| `/{year}/{month}/{day}/` | `/2025/06/08/` | Posts from that day |
| `/x/{month}/` | `/x/06/` | Posts in June across years |
| `/x/{month}/{day}/` | `/x/06/08/` | Posts on June 8 across years |
| `/x/x/{day}/` | `/x/x/08/` | Posts on the 8th across months and years |

## Notifications

Send notifications via Ntfy, Telegram, or Matrix. Configure globally in YAML. Notifications are sent for new posts, updates, and other events.

## Telegram Cross-Posting

Automatically publish new posts to a Telegram channel. Sends a formatted message with the post title and link after publishing a public post. Updates, deletions, and undeletions are also synced.

**Per-post control**: After a post is sent, the Telegram chat and message IDs are stored in the post parameters (`telegramchat`, `telegrammsg`). Editing and republishing updates the existing message instead of sending a new one.

This is separate from notification-based Telegram, which sends push notifications to you; this cross-posts to channels for your audience.

## Feeds

RSS, Atom, and JSON feeds are always available at any index URL by appending the feed extension (e.g., `/posts.rss`, `/posts.atom`, `/posts.json`). Min variants (`/posts.min.rss`, `/posts.min.atom`, `/posts.min.json`) render the same content but omit TTS audio and the comments/interactions link for cleaner output. All feeds send `Access-Control-Allow-Origin: *`.

Add `?fallbacktitle` to feed URLs to generate computed titles for untitled posts.

## Short URLs

Custom short domain for compact post URLs. Every post also gets an auto-generated short URL at `/s/{hex-id}` which 301-redirects to the full post path.

## Posts with HLS Video

Posts can embed HLS video streams using the `videoplaylist` front matter parameter. An hls.js video player is rendered on the post page.

## Media Storage

By default, media is stored locally in `data/media/` and served at `/m/`.

**External storage**: You can use BunnyCDN or FTP for media storage instead. See [`example-config.yml`](/example-config.yml) for configuration details.

**Custom media domain**: See [`example-config.yml`](/example-config.yml) for configuration.

## Image Optimization (imgproxy)

GoBlog can automatically generate optimized image variants (AVIF, JPEG/PNG) using [imgproxy](https://imgproxy.net), a standalone image processing service.

### How it works

1. On upload, GoBlog saves the original and sends it to imgproxy to generate variants (AVIF, JPEG/PNG at configured sizes)
2. Variants are saved alongside the original in media storage
3. When rendering a page, GoBlog emits a `<picture>` element (AVIF preferred, JPEG/PNG as fallback) with responsive `srcset` `w` descriptors
4. Non-image uploads (audio, video, etc.) are never sent to imgproxy

### Benefits

- Automatically serves next-gen format (AVIF) without manual conversion
- `<picture>` markup is fully cacheable: same HTML for all browsers, no `Vary` header
- Originals are preserved unchanged

All combinations of `formats` x `widths` are generated. Quality and encoding options are set via imgproxy environment variables (see [imgproxy configuration](https://docs.imgproxy.net/configuration/options#compression)).

## Media Migration

If you have existing media files that are duplicated (e.g., original uploads alongside compressed copies), the media migration tool can consolidate them. It detects perceptually identical images using perceptual hashing (dhash), groups them together, and replaces all references to compressed copies with the original; then deletes the redundant files.

### How it works

1. **Discovery**: Scans all media files, computes perceptual hashes (with EXIF-aware and raw variants), and groups files with similar aspect ratios and low hash distance
2. **Original identification**: Picks the original based on EXIF presence, then image dimensions, then file size
3. **Verification**: Displays groups and asks for confirmation (or use `--yes` to auto-confirm)
4. **Optimization**: Optionally triggers imgproxy optimization on the originals
5. **Replacement**: Updates all post content and parameters to reference the original file instead of compressed copies
6. **Cleanup**: Deletes the redundant compressed files

### CLI flags

| Flag | Description |
|------|-------------|
| `--yes` | Auto-confirm all groups (skip interactive prompts) |
| `--dry-run` | Show what would be changed without making any changes |
| `--discover-only` | Only discover and group files, don't process them |
| `--threshold N` | Maximum perceptual hash distance to consider files similar (default: 10) |
| `--limit N` | Process at most N groups |
| `--preview` | Show image previews using timg (requires timg in PATH) |

### Notes

- A state cache (`data/media-migrate.json`) stores computed hashes to avoid re-processing unchanged files
- Only JPEG and PNG files are considered
- Cross-extension duplicates (e.g., original JPG + compressed PNG) are handled correctly
- Files with optimized variants (via imgproxy) are skipped
- The tool is idempotent: safe to run multiple times

## Other Features

### Tor Hidden Service

GoBlog can serve as a Tor `.onion` hidden service when Tor is installed. See [`advanced.md`](advanced.md#tor-hidden-service) for configuration and details.

### IndexNow

Notify search engines (Bing, Yandex) of new and updated content. GoBlog auto-generates a random 128-character key persisted in the database. The key is served at `/<key>.txt` for search engine verification. Pings are sent on both post creation and updates.

### Private Mode

Restrict all public access: users must be logged in to view any content. Enable via YAML (see [`example-config.yml`](/example-config.yml)).

### NodeInfo

GoBlog implements the [NodeInfo 2.1](https://nodeinfo.diaspora.software/) protocol for federation discovery. Two endpoints are always served:

- `/.well-known/nodeinfo` -- Discovery endpoint returning a JSON link document
- `/nodeinfo` -- NodeInfo 2.1 document reporting software name, version, user/post counts, and supported protocols (ActivityPub, Micropub, Webmention)

### robots.txt

GoBlog serves a dynamic `robots.txt` at `/robots.txt`. It automatically includes Sitemap directives and blocks all crawlers when Private Mode is enabled. Use the `robotstxt` YAML config to block specific bots (see [`example-config.yml`](/example-config.yml)). The `aibotblock` plugin can also contribute blocked bot names dynamically.

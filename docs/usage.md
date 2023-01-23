# How to use GoBlog

This section of the documentation is a **work in progress**!

## Posting

### Scheduling posts

To schedule a post, create a post with `status: scheduled` and set the `published` field to the desired date. A scheduler runs in the background and checks every 30 seconds if a scheduled post should be published. If there's a post to publish, the post status is changed to `published`. That will also trigger configured hooks. Scheduled posts are only visible when logged in.

### Bookmarklets

You can preset post parameters in the editor template by adding query parameters with the prefix `p:`. So `/editor?p:title=Title` will set the title post parameter in the editor template to `Title`. This way you can create yourself bookmarklets to, for example, like posts or reply to them more easily.

## Media storage

By default, GoBlog stores all uploaded files in the `media` subdirectory of the current working directory. It is possible to change this by configuring the `micropub.mediaStorage` setting. Currently it is possible to use BunnyCDN or any FTP storage as an alternative to the local filesystem.

### Media compression

To reduce the data transfer for blog visitors, GoBlog can compress the media files after they have been uploaded. If configured, media files with supported file extensions get compressed and the compressed file gets stored as well.

GoBlog currently supports the following media compression providers:

- [Cloudflare](https://cloudflare.com/) (no API key required)
- [Tinify](https://tinify.com/) (API key required)

Take a look at the `example-config.yml` on how to configure the compression providers.

It is possible to configure multiple compression providers. If one fails, the next one is tried. The current priority is as follows:

1. Tinify
2. Cloudflare
3. Local compression

## Text-to-Speech

GoBlog features a button on each post that allows you to read the post's content aloud. By default, that uses an API from the browser to generate the speech. But it's not available on all browsers and on some operating systems it sounds horrible.

There's also the possibility to configure GoBlog to use Google Cloud's Text-to-Speech API. For that take a look at the `example-config.yml` file. If configured and enabled, after publishing a post, GoBlog will automatically generate an audio file, save it to the configured media storage (local file storage by default) and safe the audio file URL to the post's `tts` parameter. After updating a post, you can manually regenerate the audio file by using the button on the post. When deleting a post or regenerating the audio, GoBlog tries to delete the old audio file as well.

## Notifications

On receiving a webmention, a new comment or a contact form submission, GoBlog will create a new notification. Notifications are displayed on `/notifications` and can be deleted by the user.

If configured, GoBlog will also send a notification using a Telegram bot, a Matrix user and an *unencrypted* Matrix channel, or [Ntfy.sh](https://ntfy.sh/). 

### Setting up Notifications with Ntfy

1.  Create a "Topic" in the [Ntfy.sh Webapp](https://ntfy.sh/app) or another Ntfy instance. Using a randomly generated string as “Topic” is recommended.
2.  Add a "notifications" section to your configuration file with the following configuration:

```yaml
notifications:
  ntfy: # Receive notifications using Ntfy.sh
    enabled: true # Enable it
    topic: randomlyGeneratedTopicString # The topic for the notifications
    server: https://ntfy.sh # The server to use (default is https://ntfy.sh)
    user: myusername # The username to use (optional)
    password: mypassword # The password to use (optional)
    email: notifications@yourdomain.tld # Email address for Ntfy Email Notifications
```

### Setting up Notifications with Matrix

1.  Set up a new Matrix account that will act as the Bot.
2.  Create a new *unencrypted* room.
3.  Add a "notifications" section to your configuration file with the following configuration:

```yaml
notifications:
  matrix: # Receive notifications via Matrix
    enabled: true # Enable it
    homeserver: https://matrix.org # The bot's homeserver
    username: botUsername # The bot's username
    password: botPassword # The bot's password
    room: "#roomsName:matrix.org" # The Matrix chat room for the notifications
    deviceid: TestBlogNotifications # A unique device ID (to not clutter your login sessions) (optional)
```

See the `example-config.yml` file for how to configure other notification providers.

## Tor Hidden Services

GoBlog can be configured to provide a Tor Hidden Service. This is useful if you want to offer your visitors a way to connect to your blog from censored networks or countries. See the `example-config.yml` file for how to enable the Tor Hidden Service. If you don't need to hide your server, you can enable the Single Hop mode.

## Reactions

It's possible to enable post reactions. GoBlog currently has a hardcoded list of reactions: "❤️", "👍", "👎", "😂" and "😱". If enabled, users can react to a post by clicking on the reaction button below the post. If you want to disable reactions for a single post, you can set the `reactions` parameter to `false` in the post's metadata.

## Comments and interactions

GoBlog has a comment system. That can be enable using the configuration. See the `example-config.yml` file for how to configure it.

All comments and interactions (Webmentions) have to be approved manually using the UI at `/webmention`. To completely delete a comment, delete the entry from the Webmention UI and also delete the comment from `/comment`.

To disable showing comments and interactions on a single post, add the parameter `comments` with the value `false` to the post's metadata.

## ActivityPub Support

Publish and comment to the Fediverse by adding an "activitypub" section to your configuration file:

```yaml
# ActivityPub
activityPub:
  enabled: true # Enable ActivityPub
  tagsTaxonomies: # Post taxonomies to use as "Hashtags"
    - tags
```

This configuration creates a Fediverse account at `@blogname@yourdomain.tld` with the following features:

✅ Publishing  
✅ Replying (Unlisted/Public)  
✅ Converting incoming replies to blog comments  
✅ Incoming Likes/Reposts  
❌ Outgoing Likes/Reposts  
✅ Incoming @-mention  
❌ Outgoing @-mention  
✅ Followers  
❌ Following

## Redirects & Aliases

Activate redirects by adding a `pathRedirects` section to your configuration file:

```yaml
# Redirects
pathRedirects:
  # Simple 302 redirect from /index.xml to .rss
  - from: "\\/index\\.xml"
    to: ".rss"
  # Redirect using regular expressions
  - from: "^\\/(writings|dev)\\/posts(.*)$"
    to: "/$1$2"
    type: 301 # custom redirect type
```

Individual posts can also have redirects by adding redirection paths using the `aliases` post parameter:

```text
---
path: /about
title: About me
aliases: 
- /info
- /me
---

This is an about me page located at /about and it redirects from /info and /me
```

## Plugins

There's a [seperate documentation section](./plugins.md) on how to use and implement plugins.

## Extra notes

### Export content to Markdown

Use the export command to export all posts as Markdown with the post parameters as frontmatter: 

```bash
$goblogpath export ./$exportpath
```

### Fixing a GoBlog corrupted database

While the GoBlog binary runs, next to the main SQLite database file some accompanying files (Write-Ahead-Log and shared memory for SQLite) are created in the data folder, these files are essential for the integrity of the database. If the database gets corrupted.

Stop the GoBlog process, backup the database files and try to recover the database with sqlite:

```bash
sqlite3 data/db.sqlite ".recover" | sqlite3 data/newdb.sqlite
```

If this doesn't work look for more clues running: 

```bash
sqlite3 data/db.sqlite “PRAGMA integrity_check”
```

### Cleaning up the GoBlog database

At the moment some options can't be modified via the UI, certain changes can be applied by accessing the database directly using sqlite.

```bash
sqlite3 data/db.sqlite
```

#### Revoking unused IndieAuth Tokens

Tokens can be manually deleted:

```sql
DELETE FROM indieauthtoken WHERE $condition;
```

But they can also be revoked [using the IndieAuth API](https://www.w3.org/TR/indieauth/#token-revocation).

#### Erasing deleted posts

GoBlog returns a 410 HTTP error for deleted posts, to stop that:

```sql
DELETE FROM deleted WHERE path = '/deletedpost';
```
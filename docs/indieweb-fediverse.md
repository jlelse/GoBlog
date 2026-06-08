# IndieWeb & Fediverse

GoBlog has first-class support for IndieWeb and ActivityPub protocols.

## IndieAuth

Use your blog as your identity on the web.

**Endpoints:**
- `/.well-known/oauth-authorization-server` - Server metadata
- `/indieauth` - Authorization endpoint
- `/indieauth/token` - Token endpoint (also supports GET for token introspection)
- `/indieauth/revoke` - Token revocation endpoint

**Supported scopes:** `create`, `update`, `delete`, `undelete`, `media`

**PKCE:** The server supports Proof Key for Code Exchange (`code_challenge_methods_supported`).

**How it works:**

1. Your blog automatically advertises IndieAuth endpoints
2. Use your blog URL (`https://yourblog.com/`) to sign in to IndieAuth-enabled sites
3. Approve the authorization request on your blog
4. You're logged in!

Authorization codes expire after 10 minutes. Access tokens can be verified via GET to the token endpoint (returns `active`, `me`, `client_id`, `scope`).

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

## Webmention

Send and receive webmentions automatically.

**Receiving:**
- Endpoint: `/webmention`
- Webmentions are queued and verified asynchronously
- Approve, delete, or reverify via `/webmention` admin UI

**Sending:**
- Automatic when you link to other sites in your posts
- Disabled in private mode for external targets

**Configuration (Settings UI):**

The following webmention controls are available in each blog's Settings page (they apply globally to the whole GoBlog instance):

- **Disable sending webmentions**: stop sending webmentions for all posts
- **Disable receiving webmentions**: disable the receiving endpoint, comments, and ActivityPub replies for all blogs
- **Disable inter-GoBlog mentions**: prevent posts from sending webmentions to other posts on the same GoBlog instance
- **Webmention block list**: block specific hosts from sending or receiving webmentions (incoming, outgoing, or both)

## ActivityPub (Fediverse)

Publish your blog to Mastodon, Pleroma, and other ActivityPub platforms.

### Remote Follow

Visitors can follow your blog from their own Fediverse instance via the remote follow page at `/activitypub/remote_follow/{blog}`. Supports both GET (shows the form) and POST (submits form data). Enter your Fediverse handle and you'll be redirected to your instance's follow confirmation page.

### NodeInfo

GoBlog implements [NodeInfo 2.1](https://nodeinfo.diaspora.software/) at `/.well-known/nodeinfo` and `/nodeinfo`, exposing software name (`goblog`), repository link, blog count, local post count, and supported protocols (activitypub, micropub, webmention).

### Setup

```yaml
activityPub:
  enabled: true
  tagsTaxonomies:
    - tags  # Use tags as hashtags
  attributionDomains:
    - yourblog.com  # Domains allowed to use fediverse:creator meta tag
```

The `attributionDomains` list controls which external domains may set `<meta name="fediverse:creator" content="..." />` to attribute shared articles to your ActivityPub actor.

**Your Fediverse address:** `@blogname@yourdomain.com`

### Features

- Publish posts to followers
- Receive replies as comments
- Receive likes and boosts (notifications)
- Followers collection
- Webfinger discovery
- Account migration (Move activity support)
- Following others is not supported (publish only)
- Post undelete re-posts as new (due to Mastodon limitations, there is no "Undo Delete" activity)
- Supported HTTP signature algorithms for verification: RSA-SHA256, ECDSA-SHA256, Ed25519

### Endpoints

- `/.well-known/webfinger` - Webfinger
- `/.well-known/host-meta` - WebFinger RFC 6415 host-meta discovery
- `/activitypub/inbox/{blog}` - Inbox
- `/activitypub/followers/{blog}` - Followers

### Account Migration

**From another Fediverse server to GoBlog:**

1. Add your old account URL to the `alsoKnownAs` config:

```yaml
activityPub:
  enabled: true
  alsoKnownAs:
    - https://mastodon.example.com/users/oldusername
```

2. On your old Fediverse account, initiate the move to your GoBlog account using your old server's migration feature.

**From GoBlog to another Fediverse server:**

1. Set up your new account on the target Fediverse server
2. Add your GoBlog account URL to the new account's "Also Known As" aliases (e.g., `https://yourblog.com`)
3. Run the CLI command to send Move activities to all followers:

```bash
./GoBlog activitypub move-followers blogname https://newserver.example.com/users/newusername
```

This sends a Move activity to all your followers. Servers that support account migration will automatically update the follow to your new account.

**Domain change (moving GoBlog to a new domain):**

1. If using a reverse proxy, configure it to serve both domains
2. Add the old domain to `altAddresses`:

```yaml
server:
  publicAddress: https://new.example.com
  altAddresses:
    - https://old.example.com
```

3. Restart GoBlog
4. Run:

```bash
./GoBlog activitypub domainmove https://old.example.com https://new.example.com
```

Sends Move activities from the old domain's actor to all followers. The old domain continues serving ActivityPub/Webfinger with `movedTo` pointing to the new domain, while all other requests are redirected.

### Follower Management CLI

```bash
# Refresh follower profiles from remote servers
./GoBlog activitypub refetch-followers blogname

# Check followers (reports ok/gone/moved, interactive cleanup)
./GoBlog activitypub check-followers blogname

# Manually add a follower by IRI or @handle
./GoBlog activitypub add-follower blogname https://mastodon.example.com/users/alice
./GoBlog activitypub add-follower blogname @alice@mastodon.example.com

# Clear the movedTo setting
./GoBlog activitypub clear-moved blogname
```

## Bluesky / ATProto

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

**Behavior:**
- New public posts are cross-posted to Bluesky
- Deleting a GoBlog post deletes the corresponding ATProto record
- Undeleting a GoBlog post re-publishes it to Bluesky
- Editing a post after initial publication does **not** update the ATProto record (unlike ActivityPub)

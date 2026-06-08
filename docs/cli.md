# CLI Commands

GoBlog provides several CLI commands for administration and maintenance. You can run any command with `--help` to see usage instructions and available options.

## Start Server (Default)

```bash
./GoBlog --config ./config/config.yml
```

## Setup User Credentials

```bash
./GoBlog --config ./config/config.yml setup --username admin --password "your-secure-password"
```

Set up the user credentials (username, password, and optionally TOTP). The password is securely hashed using bcrypt before storage.

**Options:**
- `--username` (required) - Login username
- `--password` (required) - Login password (stored as bcrypt hash)
- `--totp` - Enable TOTP two-factor authentication

**Example with TOTP:**

```bash
./GoBlog --config ./config/config.yml setup --username admin --password "your-secure-password" --totp
```

This will output the TOTP secret which you should add to your authenticator app.

## Health Check

```bash
./GoBlog --config ./config/config.yml healthcheck
# Exit code: 0 = healthy, 1 = unhealthy
```

Useful for container health checks and monitoring.

## Check External Links

```bash
./GoBlog --config ./config/config.yml check
```

Checks all external links in published posts and reports broken links.

**Flags:**

- `--ignore-403`: Skip reporting HTTP 403 responses. Sites behind Cloudflare and similar bot-protection services frequently respond with 403 to non-browser clients, producing noisy false positives.
- `--check-dnsbl`: After the link check, query public DNS-based blocklists for each unique linked hostname and report any listings. This helps spot links to domains that have been taken over (e.g., expired domain repurposed as spam/malware).

  Lookups go through the system DNS resolver. Spamhaus prohibits queries through large public resolvers (such as Google `8.8.8.8` or Cloudflare `1.1.1.1`): make sure your machine uses your ISP's resolver or your own. Free use is limited to low-volume, non-commercial users; check the [Spamhaus DNSBL usage terms](https://www.spamhaus.org/blocklists/dnsbl-fair-use-policy/).

```bash
./GoBlog --config ./config/config.yml check --ignore-403 --check-dnsbl
```

## Export Posts

```bash
./GoBlog --config ./config/config.yml export ./exported
```

Exports all posts as Markdown files with front matter to the specified directory.

## ActivityPub Follower Management

```bash
# Refresh follower profiles from remote servers
./GoBlog --config ./config/config.yml activitypub refetch-followers blogname

# Check followers (ok/gone/moved), interactive cleanup
./GoBlog --config ./config/config.yml activitypub check-followers blogname

# Manually add a follower by actor IRI or @handle
./GoBlog --config ./config/config.yml activitypub add-follower blogname https://mastodon.example.com/users/alice
./GoBlog --config ./config/config.yml activitypub add-follower blogname @alice@mastodon.example.com

# Send Move activities for account migration
./GoBlog --config ./config/config.yml activitypub move-followers blogname https://newserver.example.com/users/newaccount

# Clear the movedTo setting
./GoBlog --config ./config/config.yml activitypub clear-moved blogname

# Send Move activities when changing domains
./GoBlog --config ./config/config.yml activitypub domainmove https://old.example.com https://new.example.com
```

### Domain Move Details

When changing your GoBlog domain:

1. Configure both domains to point to your GoBlog instance
2. Add the old domain to `altAddresses` in your server config
3. Update `publicAddress` to the new domain
4. Restart GoBlog
5. Run `activitypub domainmove`

The old domain's actor will serve with `movedTo` pointing to the new domain, and non-ActivityPub requests to the old domain will be redirected.

## Media Migration

```bash
./GoBlog --config ./config/config.yml media migrate --yes
```

Consolidates duplicate media files by detecting perceptually identical images and replacing references to compressed copies with the originals. See [Media Migration](features.md#media-migration) for details.

**Flags:** `--yes`, `--dry-run`, `--discover-only`, `--threshold N`, `--limit N`, `--preview`

## Profiling

```bash
./GoBlog --config ./config/config.yml \
  --cpuprofile cpu.prof \
  --memprofile mem.prof
```

Generates CPU and memory profiles for performance analysis.

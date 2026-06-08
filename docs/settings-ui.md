# Settings UI

Access `/settings` (requires login) to configure blog and user preferences without editing YAML. Settings are stored in the database and persist across restarts.

## Blog Settings

### Title and Description

Set the blog title and subtitle (description) that appear in the header, feeds, and metadata.

### Sections

Manage post sections (e.g., "posts", "notes", "photos"). Each section can have:

- **Title and description**: Displayed on the section index page
- **Path template**: Custom URL pattern for posts in this section. Uses Go template syntax with variables: `.Section`, `.Slug`, `.Year`, `.Month`, `.Day`, `.BlogPath`. Example: `{{printf "/%v/%v/%v/%v" .Section .Year .Month .Slug}}`
- **Show full content**: Display full post content instead of summaries on index pages
- **Hide on main index**: Exclude this section's posts from the blog homepage (still accessible at the section URL)

Set the default section for new posts using the dropdown.

### Reactions

Enable emoji reactions on posts. Configure which emoji are available (comma-separated list of actual emoji characters). Default reactions: ❤️, 👍, 🎉, 😂, 😱.

## User Settings

### Profile

- **Full name**: Displayed on posts and in feeds
- **Username**: Used for login
- **Profile image**: Upload or delete. Automatically used for favicons, Fediverse actor image, and feed icons.

### Password

Set, change, or delete your password. Passwords are hashed with bcrypt before storage. To delete a password, you must have at least one passkey registered.

### TOTP (Two-Factor Authentication)

Enable TOTP for additional security. When enabled:

1. A secret is displayed for manual entry into your authenticator app
2. Enter the 6-digit code to confirm setup
3. Disable TOTP at any time (with confirmation)

### Passkeys (WebAuthn)

Register passkeys for passwordless login. You can:

- Register multiple passkeys (e.g., for different devices)
- Rename passkeys for easy identification
- Delete passkeys you no longer need

### App Passwords

Create API tokens for third-party applications (Micropub clients, MCP servers, etc.). Each token is shown once on creation and never again. Use with Basic Authentication (any username works):

```bash
curl -u "anyuser:YOUR_APP_PASSWORD" https://yourblog.com/micropub
```

## General Settings

These are per-blog boolean toggles:

| Setting | Description |
|---------|-------------|
| Hide old content warning | Suppress the "1 year+" age warning banner on old posts |
| Hide share button | Remove the share button from post pages |
| Hide translate button | Remove the translate button from post pages |
| Hide read aloud button | Remove the read-aloud button from post pages |
| Auto-fetch reply title | Automatically fetch and insert the title of the page being replied to |
| Auto-fetch reply context | Automatically fetch and insert context content from the replied-to page |
| Auto-fetch like title | Automatically fetch and insert the title of the page being liked |
| Auto-fetch like context | Automatically fetch and insert context content from the liked page |

## Webmention Settings

These are global settings (not per-blog):

| Setting | Description |
|---------|-------------|
| Disable sending | Stop sending webmentions for all posts |
| Disable receiving | Disable the receiving webmention endpoint and comments for all blogs |
| Disable inter-GoBlog mentions | Prevent posts from sending webmentions to other posts on the same GoBlog instance |

### Block List

Block specific hosts from sending or receiving webmentions. Each entry has:
- **Host**: The domain to block
- **Incoming**: Block webmentions from this host
- **Outgoing**: Block webmentions to this host

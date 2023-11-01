# GoBlog

With GoBlog I ([Jan-Lukas Else](https://jlelse.blog)) created my own blogging system because it was too much work for me to implement all my ideas with an already existing system. GoBlog is a dynamic blogging system, but inspired by Hugo, it has goals like performance and flexibility. It also implements many IndieWeb functionalities like Webmentions or IndieAuth to use your own blog as an identity on the internet.

License: MIT License

[Blog](https://goblog.app/)  
[Documentation](https://docs.goblog.app)  
[Main repository](https://github.com/jlelse/GoBlog)  
[Mirror](https://git.jlel.se/jlelse/GoBlog)  
[Mirror on Codeberg](https://codeberg.org/jlelse/GoBlog)

## Features

Here's an (incomplete) list of features:

- Single user with multiple blogs
- Publish, edit and delete Markdown posts using Micropub or the web-based editor
    - Editor with live preview
    - Drafts, private and unlisted posts
- SQLite database for storing posts and data
    - Built-in full-text search
- Micropub with media endpoint for uploads
    - Local storage for uploads or remote storage via FTP or BunnyCDN
    - Automatic image resizing and compression
    - Uploads possible via the web-based editor
- Send and receive Webmentions
    - Webmention-based commenting
- IndieAuth
    - Login with your own blog as an identity on the internet
    - Two-factor authentication
- ActivityPub
    - Publish posts to the Fediverse (Mastodon etc.)
    - ActivityPub-based commenting
- Web feeds
    - Multiple feed formats (.rss, .atom, .json, .min.rss, .min.atom, .min.json)
    - Feeds on any archive page
- Sitemap
- Automatic HTTPS using Let's Encrypt
- Tor Hidden Service
- Tailscale integration for private blogs with HTTPS
- Fast in-memory caching for even faster performance
- Automatic asset minification of HTML, CSS and JavaScript
- Statistics page with information about posts
- Map page with a map of all posts with a location
- Posts can have a `gpx` paramter to include and show a GPX track
- Option to create post aliases for automatic redirects
- Redirects using regular expressions
- Hooks to execute custom commands on certain events
- Short URLs with option for a separate short domain
- Command to check for broken links
- Command to export all posts to Markdown files

## More information about GoBlog:

- [How to install and run GoBlog](./install.md)
- [How to build GoBlog](./build.md)
- [How to use GoBlog](./usage.md)
- [How to configure GoBlog](./config.md)
- [Administration paths](./admin-paths.md)
- [GoBlog's storage system](./storage.md)
- [GoBlog Plugins](./plugins.md)
- [Local Development with HTTPS and Tailscale Funnel](./local-dev-https.md)
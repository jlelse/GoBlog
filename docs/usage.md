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

If configured, GoBlog will also send a notification using a Telegram Bot or [Ntfy.sh](https://ntfy.sh/). See the `example-config.yml` file for how to configure the notification providers.

## Tor Hidden Services

GoBlog can be configured to provide a Tor Hidden Service. This is useful if you want to offer your visitors a way to connect to your blog from censored networks or countries. See the `example-config.yml` file for how to enable the Tor Hidden Service. If you don't need to hide your server, you can enable the Single Hop mode.

## Reactions

It's possible to enable post reactions. GoBlog currently has a hardcoded list of reactions: "❤️", "👍", "👎", "😂" and "😱". If enabled, users can react to a post by clicking on the reaction button below the post. If you want to disable reactions for a single post, you can set the `reactions` parameter to `false` in the post's metadata.

## Comments and interactions

GoBlog has a comment system. That can be enable using the configuration. See the `example-config.yml` file for how to configure it.

All comments and interactions (Webmentions) have to be approved manually using the UI at `/webmention`. To completely delete a comment, delete the entry from the Webmention UI and also delete the comment from `/comment`.

To disable showing comments and interactions on a single post, add the parameter `comments` with the value `false` to the post's metadata.
# How to use GoBlog

This section of the documentation is a work in progress!

## Posting

### Scheduling posts

To schedule a post, create a post with `status: scheduled` and set the `published` field to the desired date. A scheduler runs in the background and checks every 10 seconds if a scheduled post should be published. If there's a post to publish, the post status is changed to `published`. That will also trigger configured hooks. Scheduled posts are only visible when logged in.

## Text-to-Speech

GoBlog features a button on each post that allows you to read the post's content aloud. By default, that uses an API from the browser to generate the speech. But it's not available on all browsers and on some operating systems it sounds horrible.

There's also the possibility to configure GoBlog to use Google Cloud's Text-to-Speech API. For that take a look at the `example-config.yml` file. If configured and enabled, after publishing a post, GoBlog will automatically generate an audio file, save it to the configured media storage (local file storage by default) and safe the audio file URL to the post's `tts` parameter. After updating a post, you can manually regenerate the audio file by using the button on the post. When deleting a post or regenerating the audio, GoBlog tries to delete the old audio file as well.
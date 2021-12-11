# How to use GoBlog

This section of the documentation is a work in progress!

## Posting

### Scheduling posts

To schedule a post, create a post with `status: scheduled` and set the `published` field to the desired date. A scheduler runs in the background and checks every 10 seconds if a scheduled post should be published. If there's a post to publish, the post status is changed to `published`. That will also trigger configured hooks. Scheduled posts are only visible when logged in.
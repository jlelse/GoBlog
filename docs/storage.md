# GoBlog's storage system

## Database

GoBlog uses a SQLite database for storing most of the data (posts, comments, webmention, sessions, etc.). The database is accessed using the Go library [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3). With each startup it is checked if there are schema migrations to be performed on the database.

Currently there are the following database tables:

```
activitypub_followers
comments
deleted
indieauthauth
indieauthtoken
migrations
notifications
persistent_cache
post_parameters
posts
posts_fts
queue
sessions
shortpath
webmentions
```
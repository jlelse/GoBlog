# GoBlog's storage system

GoBlog stores all data that must be persistent in the subdirectory `data` of the current working directory. This directory contains the SQLite database file and the directory media that contains all uploaded files.

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
reactions
sessions
shortpath
webmentions
```

## Media files

To prevent data duplication, GoBlog stores files with the filename of the SHA-256 hash of the file.
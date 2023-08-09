# How to build GoBlog

There are two ways to build GoBlog:

## With Docker

(There are already pre-built images available at `ghcr.io/jlelse/goblog:latest` and `ghcr.io/jlelse/goblog:tools`)

- Linux
- git
- docker

Build command:

```bash
git clone https://git.jlel.se/jlelse/GoBlog.git
cd GoBlog
docker build -t ghcr.io/jlelse/goblog:latest . --target base
```

If you want to build and use the advanced image (with additional tools), execute:

```bash
docker build -t ghcr.io/jlelse/goblog:tools . --target tools
```

## With Go installed

Requirements:

- Linux
- git
- go >= 1.21
- libsqlite3 with FTS5 enabled >= 3.31 (the newer the better)

Build command:

```bash
git clone https://git.jlel.se/jlelse/GoBlog.git
cd GoBlog
go build -tags=linux,libsqlite3,sqlite_fts5 -o GoBlog
```

Alternatively you can also compile sqlite3 directly into GoBlog. This doesn't require libsqlite3, but takes more time.

```bash
git clone https://git.jlel.se/jlelse/GoBlog.git
cd GoBlog
go build -tags=linux,sqlite_fts5 -o GoBlog
```
# How to build GoBlog

There are two ways to build GoBlog:

## With Docker

(There are already pre-built images available at `rg.fr-par.scw.cloud/jlelse/goblog:latest` and `rg.fr-par.scw.cloud/jlelse/goblog:tools`)

- Linux
- git
- docker

Build command:

```bash
git clone https://git.jlel.se/jlelse/GoBlog.git
cd GoBlog
docker build -f Dockerfile -t rg.fr-par.scw.cloud/jlelse/goblog:latest .
```

If you want to build and use the advanced image (with additional tools), also execute:

```bash
docker build -f Dockerfile.Tools -t rg.fr-par.scw.cloud/jlelse/goblog:tools .
```

## With Go installed

Requirements:

- Linux
- git
- go >= 1.16
- libsqlite3 >= 3.31 (the newer the better)

Build command:

```bash
git clone https://git.jlel.se/jlelse/GoBlog.git
cd GoBlog
go build -tags=linux,libsqlite3,sqlite_fts5 -o GoBlog
```
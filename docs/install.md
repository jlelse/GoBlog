# How to install and run GoBlog

It's recommended to install GoBlog using Docker (Compose). You can simply pull the latest image from `ghcr.io/jlelse/goblog:latest` (basic image) or `ghcr.io/jlelse/goblog:tools` (for when you want to use `sqlite3`, `bash` or `curl` in hook commands) when there are updates. Those images are tested and contain all necessary libraries and tools.

Create your config file (`./config/config.yml`) with inspiration from `example-config.yml` and create a new data directory (`./data`). For static files, you can also create a directory at `./static`.

Then you can use Docker Compose to run GoBlog. Here's an example `docker-compose.yml` file:

```yaml
services:
    goblog:
        container_name: goblog
        image: ghcr.io/jlelse/goblog:latest # or :tools
        restart: unless-stopped # auto restart the container
        volumes:
            - ./config:/app/config # Config directory
            - ./data:/app/data # Data directory, used for database, keys, uploads etc.
            - ./static:/app/static # Static directory, if you want to publish static files
        environment:
            - TZ=Europe/Berlin # You timezone
```

If you don't want to use a reverse proxy (like Caddy or nginx) you can also publish the ports directly from the GoBlog container. Remember to enable public https in the config, so GoBlog gets Let's Encrypt certificates.

```yaml
goblog:
    container_name: goblog
    ...
    ports:
        - 80:80
        - 443:443
```

Start the container:

```bash
docker-compose up -d
```

Update the container with a newer image:

```bash
docker-compose pull
docker-compose up -d
```
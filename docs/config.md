# How to configure GoBlog

Most settings for GoBlog (still) have to be configured using a YAML configuration file. See `example-config.yaml` for the available options. **It's recommended to just set the settings you really need, so it's best to start from a blank configuration file.** There are defaults for most settings. Some settings can be configured using a web UI (with the path of `/settings`).

## Sections

You can add, remove or update sections and the default section using the web UI.

Sections can have a title, description (with support for markdown) and a path template which gets used when creating a new post without a pre-setted path. The syntax for the path template is gets parsed as a [Go template](https://pkg.go.dev/text/template#pkg-overview). Available variables are: `.Section`, `.Slug`, `.Year`, `.Month`, `.Day` and `.BlogPath`.

Example for the path template:

```
{{printf \"/%v/%v\" .Section .Slug}}
```

### Setting Up GoBlog with nginx

The following is a minimal example configuration for GoBlog running behind an nginx reverse proxy and using the certbot plugin to generate TLS certificates.

**config.yml**

```text-plain
# Web server
server:
  port: 8080 # GoBlog Port
  publicAddress: https://yourdomain.tld # Public address of your blog
  
# User
user:
  nick: user # Username for login (can be changed later using the settings web UI)
  password: password # Password for login

# Blogs
defaultBlog: main # Default blog name (needed because you can define multiple blogs)
blogs:
  main: # Blog name
    path: / # Path of blog
    lang: en # Language of blog
    title: My Cool Blog # Blog title
    description: "Welcome to this blog." # Blog description
    # Taxonomies
    taxonomies:
      - name: tags # Code of taxonomy (used via post parameters)
        title: Tags # Name
        description: "**Tags** on this blog" # Description
    # Menus
    menus:
      # Main menu
      main:
        items:
          - title: Home # Title
            link: / # Site-relative or absolute links
          - title: Posts
            link: /posts
```

**example-nginx.conf**

```text-plain
server {
    server_name yourdomain.tld;

    location / {
        proxy_set_header Host $http_host;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_max_temp_file_size 0;
        proxy_pass http://localhost:8080;
        proxy_redirect http:// https://;
        client_max_body_size 30M;
    }
}    
```

Generate SSL certificates with the nginx certbot plugin by running:

```text-plain
$ certbot --nginx -d yourdomain.tld -d www.yourdomain.tld
```
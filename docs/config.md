# How to configure GoBlog

Most settings for GoBlog (still) have to be configured using a YAML configuration file. See `example-config.yaml` for the available options. It's recommended to just set the settings you really need. There are defaults for most settings. Some settings can be configured using a web UI (with the path of `/settings`).

## Sections

You can add, remove or update sections and the default section using the web UI.

Sections can have a title, description (with support for markdown) and a path template which gets used when creating a new post without a pre-setted path. The syntax for the path template is gets parsed as a [Go template](https://pkg.go.dev/text/template#pkg-overview). Available variables are: `.Section`, `.Slug`, `.Year`, `.Month`, `.Day` and `.BlogPath`.

Example for the path template:

```
{{printf \"/%v/%v\" .Section .Slug}}
```

### Setting Up GoBlog with nginx

The following is a bare-minimum example configuration for GoBlog running behind an NGINX Reverse Proxy and using the certbot plugin to generate SSL certificates.

**config.yml**

```text-plain
# Web server
server:
  logging: true # Enable access log
  logFile: data/access.log # File path for the access log
  port: 8080 # GoBlog Port
  publicAddress: https://yourdomain.tld # Public address to 
  publicHttps: false # Use Let's Encrypt and serve site with HTTPS
  
# User
user:
  nick: user # Username
  password: password # Password for login
  email: user@email.tld # Your E-mail address  

# Blogs
defaultBlog: blogName # Default blog code (needed because you can define multiple blogs)
blogs:
  blogName: # Blog code
    path: / # Path of blog
    lang: en # Language of blog
    title: My Cool Blog # Blog title
    description: "Welcome to this blog." # Blog description
    pagination: 10 # Number of posts per page
    # Sections
    defaultsection: notes # Default section
    sections:
      notes:
        title: Notes
        description: "A section for notes." # Section description, can also use Markdown
        pathtemplate: "{{printf \"/%v/%02d/%02d/%v\" .Section .Year .Month .Slug}}"
        showFull: true # Show full post content instead of just the summary on index pages
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
          - title: Notes
            link: /notes
```

**example-nginx.conf**

```text-plain
server {
    server_name yourdomain.tld;

    location / {
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Forwarded-Host $http_host;
        proxy_set_header Host $http_host;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_max_temp_file_size 0;
        proxy_pass http://localhost:GoBlogPort;
        proxy_redirect http:// https://;
        client_max_body_size 30M;
    }
}    
```

Generate SSL certificates with the nginx certbot plugin by running:

```text-plain
$ certbot --nginx -d yourdomain.tld -d www.yourdomain.tld
```
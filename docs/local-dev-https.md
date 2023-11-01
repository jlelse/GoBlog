# Local Development with HTTPS and Tailscale Funnel

This guide will walk you through the process of exposing your locally running GoBlog instance to the public web using [Tailscale Funnel](https://tailscale.com/kb/1223/funnel/). Tailscale Funnel allows you to test GoBlog features, such as ActivityPub, without the need for complex port forwarding, proxy setups, or DNS configurations.

## Prerequisites

Before getting started, make sure you have the following prerequisites in place:

- A Tailscale account.
- Docker and Docker Compose installed on your local machine (or the machine you are using to develop on GoBlog).

## Step 1: Edit Docker Compose configuration

1. Edit the `docker-compose.dev.tailscale.yml` file with your [Tailscale Auth Key](https://login.tailscale.com/admin/settings/keys). Set the value to the `TS_AUTHKEY` environment variable.

## Step 2: Edit GoBlog configuration

2. In your GoBlog configuration file, set the following values:

```yaml
server:
  publicHttps: true # Enables Let's Encrypt integration
  publicAddress: https://goblog-dev.<your-tailnet-name>.ts.net
  port: 8080
```

## Step 3: Start Tailscale and Funnel

3. Run the following commands to start up Tailscale in a Docker container and set up the Funnel tunnels:

```sh
# Start Tailscale container
docker compose -f docker-compose.dev.tailscale.yml up -d

# Start Tailscale Funnel for HTTPS
docker compose -f docker-compose.dev.tailscale.yml exec tailscale tailscale --socket /tmp/tailscaled.sock funnel --bg --tcp=443 8080

# Start Tailscale Funnel for HTTP (to redirect to HTTPS)
docker compose -f docker-compose.dev.tailscale.yml exec tailscale tailscale --socket /tmp/tailscaled.sock funnel --bg --tcp=80 80
```

With these steps completed, your locally running GoBlog instance will be accessible over HTTPS through your Tailscale Funnel setup. This makes it easy to test GoBlog features in a public web environment without the hassle of configuring complex networking settings.

## Why running Tailscale in Docker?

It's totally possible to use Tailscale Funnel without Docker. But the advantage of this solution is being able to easily change the host name. You are free to use it without Docker.
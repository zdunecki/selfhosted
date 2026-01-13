# SelfHosted - Self-Hosted App Installer

> ⚠️ **Early Stage**: This project is in very early development. Features may be incomplete, APIs may change, and there may be bugs. Use at your own risk.

A modern CLI tool to deploy self-hosted applications to multiple cloud providers with a single click. Features a beautiful web-based wizard UI and native desktop app wrapper.

## Features

- **Multi-provider support**: Deploy to DigitalOcean, Scaleway, UpCloud, Vultr, and Google Cloud Platform
- **Multi-app support**: Deploy analytics, session replay, and more
- **Automatic**: DNS setup, SSL certificates, app installation
- **No dependencies**: Single binary, no Pulumi/Terraform needed
- **Beautiful UI**: Web-based wizard and native desktop app
- **Secure**: Encrypted credential handling

## Supported Applications

| App | Icon | Description |
|-----|------|-------------|
| **OpenReplay** | ![OpenReplay](web/public/openreplay.svg) | Open-source session replay and product analytics |
| **OpenPanel** | ![OpenPanel](web/public/openpanel.svg) | Open-source analytics (self-hosted) |
| **Plausible** | ![Plausible](web/public/plausible.svg) | Lightweight, privacy-friendly web analytics (Docker Compose) |
| **Umami** | ![Umami](web/public/umami.svg) | Simple, fast, privacy-focused web analytics (Postgres + HTTPS via Caddy) |
| **Swetrix** | ![Swetrix](web/public/swetrix.png) | Open-source, privacy-focused analytics (ClickHouse + Redis + HTTPS via Caddy) |
| **Rybbit** | ![Rybbit](web/public/rybbit.svg) | Open-source, privacy-friendly web & product analytics (ClickHouse + Postgres + HTTPS via Caddy) |

## Supported Cloud Providers

| Provider | Icon | Description |
|----------|------|-------------|
| **DigitalOcean** | ![DigitalOcean](web/public/digitalocean.svg) | Developer-friendly cloud hosting |
| **Scaleway** | ![Scaleway](web/public/scaleway.svg) | European cloud hosting (Go SDK) |
| **UpCloud** | ![UpCloud](web/public/upcloud.svg) | European cloud hosting (Go SDK) |
| **Vultr** | ![Vultr](web/public/vultr.svg) | Global cloud hosting (Go SDK) |
| **Google Cloud Platform** | ![GCP](web/public/gcloud.svg) | Enterprise cloud hosting (GCP) |

## Installation

```bash
# Build from source
go build -o selfhosted .
sudo mv selfhosted /usr/local/bin/

# Or download binary (when available)
curl -L https://github.com/zdunecki/selfhosted/releases/latest/download/selfhosted-linux-amd64 -o selfhosted
chmod +x selfhosted
sudo mv selfhosted /usr/local/bin/
```

> 💡 **Coming Soon**: SelfHosted will be available as an npm package! Install and run apps directly with:
> ```bash
> npx selfhosted <app>
> ```

## Quick Start

### Web UI (Default)

```bash
# Start the web UI server (opens in browser automatically)
./selfhosted
```

### Desktop App

```bash
# Launch the native desktop application
./selfhosted --desktop
```

The web UI provides an intuitive wizard interface to guide you through deploying your chosen application. Simply select your app, cloud provider, and follow the prompts!

For detailed commands, environment variables, and examples, see [DEVELOPMENT.md](DEVELOPMENT.md).

## License

MIT

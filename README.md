# SelfHosted - Self-Hosted App Installer

> ⚠️ **Early Stage**: This project is in very early development. Features may be incomplete, APIs may change, and there may be bugs. Use at your own risk.

A modern CLI tool to deploy self-hosted applications to multiple cloud providers with a single click. Features a beautiful web-based wizard UI and native desktop app wrapper.

![SelfHosted Installer](.github/gif1.gif)

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
| **OpenReplay** | <img src="web/public/openreplay.svg" width="20" height="20"> | Open-source session replay and product analytics |
| **OpenPanel** | <img src="web/public/openpanel.svg" width="20" height="20"> | Open-source analytics (self-hosted) |
| **Plausible** | <img src="web/public/plausible.svg" width="20" height="20"> | Lightweight, privacy-friendly web analytics (Docker Compose) |
| **Umami** | <img src="web/public/umami.svg" width="20" height="20"> | Simple, fast, privacy-focused web analytics (Postgres + HTTPS via Caddy) |
| **Swetrix** | <img src="web/public/swetrix.png" width="20" height="20"> | Open-source, privacy-focused analytics (ClickHouse + Redis + HTTPS via Caddy) |
| **Rybbit** | <img src="web/public/rybbit.svg" width="20" height="20"> | Open-source, privacy-friendly web & product analytics (ClickHouse + Postgres + HTTPS via Caddy) |

## Supported Cloud Providers

| Provider | Icon | Description |
|----------|------|-------------|
| **DigitalOcean** | <img src="web/public/digitalocean.svg" width="20" height="20"> | Developer-friendly cloud hosting |
| **Scaleway** | <img src="web/public/scaleway.svg" width="20" height="20"> | European cloud hosting|
| **UpCloud** | <img src="web/public/upcloud.svg" width="20" height="20"> | European cloud hosting|
| **Vultr** | <img src="web/public/vultr.svg" width="20" height="20"> | Global cloud hosting |
| **Google Cloud Platform** | <img src="web/public/gcloud.svg" width="20" height="20"> | High-performance infrastructure for cloud computing |

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

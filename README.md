# selfhost - Self-Hosted App Installer CLI

A CLI tool to deploy self-hosted applications to multiple cloud providers with a single command.

## Features

- **Multi-provider support**: DigitalOcean, Scaleway (OVH coming soon)
- **Multi-app support**: OpenReplay, Plausible (easily extensible)
- **Automatic**: DNS setup, SSL certificates, app installation
- **No dependencies**: Single binary, no Pulumi/Terraform needed

## Installation

```bash
# Build from source
make build
sudo make install

# Or download binary
curl -L https://github.com/yourname/selfhost/releases/latest/download/selfhost-linux-amd64 -o selfhost
chmod +x selfhost
sudo mv selfhost /usr/local/bin/
```

## Quick Start

```bash
# Set provider credentials
export DIGITALOCEAN_TOKEN="your-token"

# Deploy OpenReplay
selfhost deploy \
  --provider digitalocean \
  --app openreplay \
  --domain openreplay.example.com \
  --email admin@example.com

# Deploy Plausible
selfhost deploy \
  --provider digitalocean \
  --app plausible \
  --domain analytics.example.com \
  --email admin@example.com
```

## Commands

### Deploy an app
```bash
selfhost deploy [flags]

Flags:
  -p, --provider string   Cloud provider (digitalocean, scaleway)
  -a, --app string        Application to deploy (openreplay, plausible)
  -d, --domain string     Domain name for the app
  -r, --region string     Region/datacenter (optional, uses default)
  -s, --size string       VM size (optional, uses app minimum)
      --ssl               Enable Let's Encrypt SSL (default: true)
      --email string      Email for Let's Encrypt
      --ssh-key string    Path to SSH private key
      --ssh-pub string    Path to SSH public key
```

### List available providers
```bash
selfhost providers
```

### List available apps
```bash
selfhost apps
```

### List regions for a provider
```bash
selfhost regions digitalocean
selfhost regions scaleway
```

### List VM sizes for a provider
```bash
selfhost sizes digitalocean
```

### Destroy a server
```bash
selfhost destroy <server-id> --provider digitalocean
```

## Environment Variables

### DigitalOcean
```bash
export DIGITALOCEAN_TOKEN="your-api-token"
# or
export DO_TOKEN="your-api-token"
```

### Scaleway
```bash
export SCW_ACCESS_KEY="your-access-key"
export SCW_SECRET_KEY="your-secret-key"
export SCW_DEFAULT_PROJECT_ID="your-project-id"
export SCW_DEFAULT_ORGANIZATION_ID="your-org-id"
```

## Adding New Providers

Create a new file in `pkg/providers/`:

```go
package providers

type MyProvider struct {
    client *myapi.Client
}

func (p *MyProvider) Name() string { return "myprovider" }
func (p *MyProvider) Description() string { return "My Cloud Provider" }
func (p *MyProvider) DefaultRegion() string { return "us-east-1" }
func (p *MyProvider) ListRegions() ([]Region, error) { ... }
func (p *MyProvider) ListSizes() ([]Size, error) { ... }
func (p *MyProvider) GetSizeForSpecs(specs Specs) (string, error) { ... }
func (p *MyProvider) CreateServer(config *DeployConfig) (*Server, error) { ... }
func (p *MyProvider) WaitForServer(id string) (*Server, error) { ... }
func (p *MyProvider) DestroyServer(id string) error { ... }
func (p *MyProvider) SetupDNS(domain, ip string) error { ... }

func init() {
    Register(&MyProvider{})
}
```

## Adding New Apps

Create a new file in `pkg/apps/`:

```go
package apps

type MyApp struct{}

func (a *MyApp) Name() string { return "myapp" }
func (a *MyApp) Description() string { return "My Self-Hosted App" }
func (a *MyApp) MinSpecs() providers.Specs {
    return providers.Specs{CPUs: 2, MemoryMB: 4096, DiskGB: 40}
}
func (a *MyApp) Install(config *InstallConfig) error {
    runner := NewSSHRunner(config.ServerIP, config.SSHUser, config.SSHKey)
    defer runner.Close()
    
    commands := []string{
        "apt-get update -y",
        "curl -fsSL https://myapp.com/install.sh | bash",
    }
    return runner.RunMultiple(commands)
}
func (a *MyApp) SetupSSL(config *InstallConfig) error { ... }
func (a *MyApp) PrintSummary(ip, domain string) { ... }

func init() {
    Register(&MyApp{})
}
```

## Project Structure

```
selfhost-installer/
├── main.go                     # Entry point
├── cmd/
│   └── root.go                 # CLI commands (cobra)
├── pkg/
│   ├── providers/
│   │   ├── provider.go         # Provider interface
│   │   ├── digitalocean.go     # DO implementation
│   │   └── scaleway.go         # Scaleway implementation
│   └── apps/
│       ├── app.go              # App interface
│       ├── ssh.go              # SSH utilities
│       ├── openreplay.go       # OpenReplay app
│       └── plausible.go        # Plausible app
├── go.mod
├── Makefile
└── README.md
```

## Examples

### Deploy OpenReplay to DigitalOcean Frankfurt
```bash
selfhost deploy \
  -p digitalocean \
  -a openreplay \
  -d replay.mycompany.com \
  -r fra1 \
  --email devops@mycompany.com
```

### Deploy Plausible to Scaleway Paris
```bash
export SCW_ACCESS_KEY="xxx"
export SCW_SECRET_KEY="xxx"

selfhost deploy \
  -p scaleway \
  -a plausible \
  -d analytics.mycompany.com \
  -r fr-par-1 \
  --email devops@mycompany.com
```

### Check available sizes that meet OpenReplay requirements
```bash
selfhost sizes digitalocean | grep -E "s-[4-9]vcpu|s-[0-9]{2}vcpu"
```

## License

MIT

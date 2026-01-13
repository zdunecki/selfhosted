# Development

## Build

### Frontend
```bash
cd web
npm install
npm run build
```

### Backend
```bash
# Copy frontend assets to server package
mkdir -p pkg/server/dist
cp -r web/dist/* pkg/server/dist/

# Build the binary
go build -o selfhosted
```

### Desktop App
```bash
# Install dependencies
cd desktop/vite-src
pnpm install

# Build desktop app
pnpm run build

# Run desktop app (requires Neutralino CLI)
cd ../..
./selfhosted --desktop
```

## Project Structure

```
selfhosted/
├── main.go                     # Entry point
├── cmd/
│   ├── root.go                 # CLI commands (cobra)
│   ├── serve.go                # Web UI server command
│   └── desktop.go              # Desktop app launcher
├── pkg/
│   ├── providers/              # Cloud provider implementations
│   │   ├── provider.go         # Provider interface
│   │   ├── digitalocean.go
│   │   ├── scaleway.go
│   │   ├── upcloud.go
│   │   ├── vultr.go
│   │   └── gcp.go
│   ├── apps/                   # App registry and DSL
│   │   ├── app.go
│   │   ├── dsl_app.go
│   │   └── registry_from_yaml.go
│   ├── server/                 # Web UI backend
│   │   └── server.go
│   └── cli/                    # CLI deployment logic
├── marketplace/                # App definitions (YAML)
│   ├── apps.yaml
│   └── apps/
│       ├── openreplay.yaml
│       ├── openpanel.yaml
│       └── ...
├── app/                        # Shared frontend code
│   └── src/
├── web/                        # Web UI wrapper
│   └── src/
├── desktop/                    # Desktop app (Neutralino)
│   └── vite-src/
└── README.md
```

## Adding New Providers

Create a new file in `pkg/providers/` implementing the `Provider` interface:

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
func (p *MyProvider) Configure(config map[string]string) error { ... }

func init() {
    Register(NewMyProvider())
}
```

See existing implementations in `pkg/providers/` for reference:
- `digitalocean.go` - Simple API-based provider
- `scaleway.go` - SDK-based provider with config file support
- `gcp.go` - Complex provider with project/billing management

## Adding New Apps

Create a new YAML file in `marketplace/apps/`:

```yaml
app: myapp
description: My Self-Hosted App
os: ubuntu-24-04-x64
min_spec:
    cpu: 2
    ram: 4gib
    disk: 40gib
providers:
  - digitalocean
  - scaleway
# ... rest of configuration
```

Then add it to `marketplace/apps.yaml`:

```yaml
apps:
  - myapp.yaml
```

See existing app definitions in `marketplace/apps/` for examples:
- `openreplay.yaml` - Complex app with custom questions
- `plausible.yaml` - Simple Docker Compose app
- `umami.yaml` - App with database setup

## Frontend Development

The frontend uses a monorepo structure with shared code:

- `app/` - Shared React components, hooks, and utilities
- `web/` - Web UI wrapper (thin wrapper around `app/`)
- `desktop/vite-src/` - Desktop app wrapper (uses Neutralino)

### Development Setup

```bash
# Install dependencies
pnpm install

# Run web dev server
cd web
pnpm run dev

# Run desktop dev server
cd desktop/vite-src
pnpm run dev
```

### Shared Code

The `app/` package contains shared code used by both web and desktop:
- Components (`InstallerLayout`, `SelectCard`, etc.)
- Pages (`Wizard`, `Deploying`)
- Hooks (`useWizardData`, `useRegions`, `useSizes`)
- Utilities (`api.ts`, `crypto.ts`)

Both `web/` and `desktop/vite-src/` import from `@selfhosted/app` workspace package.

## Commands

### Start Web UI (Default)
```bash
./selfhosted
# or
./selfhosted serve [--port PORT] [--no-browser]
```

### Launch Desktop App
```bash
./selfhosted --desktop
```

### Deploy an app (CLI)
```bash
./selfhosted deploy [flags]

Flags:
  -p, --provider string   Cloud provider (digitalocean, scaleway, upcloud, vultr, gcp)
  -a, --app string        Application to deploy (openreplay, openpanel, plausible, umami, swetrix, rybbit)
  -d, --domain string     Domain name for the app
  -r, --region string     Region/datacenter (optional, uses default)
  -s, --size string       VM size (optional, uses app minimum)
      --ssl               Enable Let's Encrypt SSL (default: true)
      --email string       Email for Let's Encrypt
      --ssh-key string     Path to SSH private key
      --ssh-pub string     Path to SSH public key
```

### List available providers
```bash
./selfhosted providers
```

### List available apps
```bash
./selfhosted apps
```

### List regions for a provider
```bash
./selfhosted regions digitalocean
./selfhosted regions scaleway
```

### List VM sizes for a provider
```bash
./selfhosted sizes digitalocean
```

### Destroy a server
```bash
./selfhosted destroy <server-id> --provider digitalocean
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

### UpCloud
```bash
export UPCLOUD_TOKEN="your-api-token"
# or configure via ~/.config/upctl.yaml
```

### Vultr
```bash
export VULTR_API_KEY="your-api-key"
# or configure via ~/.vultr-cli.yaml
```

### Google Cloud Platform
```bash
# Use Application Default Credentials (ADC)
gcloud auth application-default login

# Or set service account JSON
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"
```

## Examples

### Deploy OpenReplay to DigitalOcean Frankfurt
```bash
export DIGITALOCEAN_TOKEN="your-token"

./selfhosted deploy \
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

./selfhosted deploy \
  -p scaleway \
  -a plausible \
  -d analytics.mycompany.com \
  -r fr-par-1 \
  --email devops@mycompany.com
```

### Deploy Umami to Google Cloud Platform
```bash
gcloud auth application-default login

./selfhosted deploy \
  -p gcp \
  -a umami \
  -d analytics.mycompany.com \
  --email devops@mycompany.com
```
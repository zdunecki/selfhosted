package apps

import (
	"fmt"

	"github.com/zdunecki/selfhosted/pkg/providers"
)

// App is the interface all installable applications must implement
type App interface {
	// Name returns the app identifier
	Name() string

	// Description returns a human-readable description
	Description() string

	// MinSpecs returns minimum hardware requirements
	MinSpecs() providers.Specs

	// Install runs the installation on the server
	Install(config *InstallConfig) error

	// SetupSSL configures SSL/TLS
	SetupSSL(config *InstallConfig) error

	// PrintSummary prints post-installation info
	PrintSummary(ip, domain string)

	// DomainHint returns a suggested domain format for the app
	DomainHint() string
}

// InstallConfig holds installation configuration
type InstallConfig struct {
	Domain                 string
	ServerIP               string
	SSHKey                 string
	SSHUser                string
	EnableSSL              bool
	Email                  string
	SSL                    bool
	SSLPrivateKeyFile      string
	SSLCertificateCrt      string
	HttpToHttpsRedirection bool
	ExtraVars              map[string]string
}

// Registry holds all registered apps
var Registry = make(map[string]App)

// Register adds an app to the registry
func Register(a App) {
	Registry[a.Name()] = a
}

// Get retrieves an app by name
func Get(name string) (App, error) {
	a, ok := Registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown app: %s", name)
	}
	return a, nil
}

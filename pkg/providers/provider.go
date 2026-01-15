package providers

import (
	"fmt"
)

// Provider is the interface all cloud providers must implement
type Provider interface {
	// Name returns the provider identifier
	Name() string

	// Description returns a human-readable description
	Description() string

	// DefaultRegion returns the default region
	DefaultRegion() string

	// ListRegions returns available regions
	ListRegions() ([]Region, error)

	// ListSizes returns available VM sizes
	ListSizes() ([]Size, error)

	// GetSizeForSpecs finds a size matching minimum specs
	GetSizeForSpecs(specs Specs) (string, error)

	// CreateServer creates a new server
	CreateServer(config *DeployConfig) (*Server, error)

	// WaitForServer waits for server to be ready
	WaitForServer(id string) (*Server, error)

	// DestroyServer deletes a server
	DestroyServer(id string) error

	// SetupDNS creates DNS records
	SetupDNS(domain, ip string) error

	// Configure updates provider settings (e.g. API tokens)
	Configure(config map[string]string) error
}

// Region represents a datacenter region
type Region struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// Size represents a VM size/plan
type Size struct {
	Slug         string  `json:"slug"`
	VCPUs        int     `json:"vcpus"`
	MemoryMB     int     `json:"memory"`
	DiskGB       int     `json:"disk"`
	PriceMonthly float64 `json:"price_monthly"`
	PriceHourly  float64 `json:"price_hourly"`
}

// Specs represents minimum hardware requirements
type Specs struct {
	CPUs     int
	MemoryMB int
	DiskGB   int
}

// Server represents a created server
type Server struct {
	ID     string
	Name   string
	IP     string
	Status string
}

// DeployConfig holds configuration for deploying a server
type DeployConfig struct {
	Name          string
	Region        string
	Size          string
	Image         string // OS image, defaults to Ubuntu 22.04
	SSHPublicKey  string
	SSHPrivateKey string
	Domain        string
	Tags          []string
}

// Registry holds all registered providers
var Registry = make(map[string]Provider)

// Register adds a provider to the registry
func Register(p Provider) {
	Registry[p.Name()] = p
}

// Get retrieves a provider by name
func Get(name string) (Provider, error) {
	p, ok := Registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return p, nil
}

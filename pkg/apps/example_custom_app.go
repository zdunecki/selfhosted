//go:build example_app
// +build example_app

// This file is an example showing how to add custom Go logic for an app.
// It is NOT compiled by default.
//
// To try it, build with:
//
//	go build -tags example_app ./...
//
// Note: This example is intentionally NOT referenced from pkg/apps/apps.yaml.
package apps

import (
	"fmt"

	"github.com/zdunecki/selfhosted/pkg/providers"
)

// ExampleCustomApp demonstrates overriding behavior in Go.
// You can embed or wrap DSLApp and override specific methods.
type ExampleCustomApp struct{}

func (a *ExampleCustomApp) Name() string { return "example-custom" }
func (a *ExampleCustomApp) Description() string {
	return "Example custom app implemented in Go (demo only)"
}
func (a *ExampleCustomApp) DomainHint() string { return "Example: demo.your-domain.com" }

func (a *ExampleCustomApp) MinSpecs() providers.Specs {
	return providers.Specs{CPUs: 1, MemoryMB: 512, DiskGB: 10}
}

func (a *ExampleCustomApp) Install(config *InstallConfig) error {
	if config.Logger != nil {
		config.Logger("👋 This is a custom Go app. Implement your own install logic here.\n")
	}
	// Return nil so it behaves like a no-op demo app.
	return nil
}

func (a *ExampleCustomApp) SetupSSL(config *InstallConfig) error { return nil }

func (a *ExampleCustomApp) PrintSummary(ip, domain string) {
	fmt.Printf("Example app deployed to %s on %s\n", domain, ip)
}

func (a *ExampleCustomApp) ShouldSetupDNS(dnsSetupMode, providerName, detectedDNSProvider string) bool {
	return false
}

func init() {
	// Registers an app without touching apps.yaml (only when built with -tags example_app).
	Register(&ExampleCustomApp{})
}

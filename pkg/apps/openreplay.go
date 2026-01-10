package apps

import (
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/zdunecki/selfhosted/pkg/dsl"
	"github.com/zdunecki/selfhosted/pkg/providers"
	"github.com/zdunecki/selfhosted/pkg/utils"
)

//go:embed openreplay.yaml
var openReplayDSLData []byte

type OpenReplayDSL struct {
	spec dsl.Spec
	err  error
}

func NewOpenReplayDSL(loader *dsl.Loader) *OpenReplayDSL {
	if loader == nil {
		loader = &dsl.Loader{}
	}
	spec, err := loader.Load(openReplayDSLData)
	if err != nil {
		return &OpenReplayDSL{spec: dsl.Spec{}, err: err}
	}
	return &OpenReplayDSL{spec: spec}
}

func (o *OpenReplayDSL) Name() string {
	if o.spec.App != "" {
		return o.spec.App
	}
	return "openreplay"
}

func (o *OpenReplayDSL) Description() string {
	if o.spec.Description != "" {
		return o.spec.Description
	}
	return "OpenReplay - Open-source session replay and product analytics"
}

func (o *OpenReplayDSL) MinSpecs() providers.Specs {
	ramMB := int(o.spec.MinSpec.RAM)
	if ramMB == 0 {
		ramMB = 8192
	}
	diskGB := int(o.spec.MinSpec.Disk)
	if diskGB == 0 {
		diskGB = 80
	}
	cpus := o.spec.MinSpec.CPU
	if cpus == 0 {
		cpus = 4
	}

	return providers.Specs{
		CPUs:     cpus,
		MemoryMB: ramMB,
		DiskGB:   diskGB,
	}
}

func (o *OpenReplayDSL) Install(config *InstallConfig) error {
	if o.err != nil {
		return o.err
	}
	runner := utils.NewSSHRunner(config.ServerIP, config.SSHUser, config.SSHKey)
	defer runner.Close()

	if err := runner.Connect(); err != nil {
		return err
	}

	return dsl.RunStepsWithConfig(dsl.Runner{
		Run:   runner.Run,
		Log:   func(msg string) { fmt.Println(msg) },
		Sleep: time.Sleep,
	}, o.spec.Steps, config, false)
}

func (o *OpenReplayDSL) SetupSSL(config *InstallConfig) error {
	if o.err != nil {
		return o.err
	}
	runner := utils.NewSSHRunner(config.ServerIP, config.SSHUser, config.SSHKey)
	defer runner.Close()

	if err := runner.Connect(); err != nil {
		return err
	}

	return dsl.RunStepsWithConfig(dsl.Runner{
		Run:   runner.Run,
		Log:   func(msg string) { fmt.Println(msg) },
		Sleep: time.Sleep,
	}, o.spec.Steps, config, true)
}

func (o *OpenReplayDSL) DomainHint() string {
	if strings.TrimSpace(o.spec.DomainHint) != "" {
		return strings.TrimSpace(o.spec.DomainHint)
	}
	return "Example: openreplay.your-domain.com"
}

func (o *OpenReplayDSL) PrintSummary(ip, domain string) {
	fmt.Println()
	fmt.Println(strings.Repeat("═", 70))
	fmt.Println("🎉 OpenReplay Deployment Complete!")
	fmt.Println(strings.Repeat("═", 70))
	fmt.Printf("\n📍 Server IP: %s\n", ip)
	fmt.Printf("🌐 Domain: %s\n", domain)
	fmt.Printf("🔗 OpenReplay URL: https://%s\n", domain)
	fmt.Printf("📝 Signup URL: https://%s/signup\n", domain)
	fmt.Println(strings.Repeat("═", 70))
}

func (o *OpenReplayDSL) ShouldSetupDNS(dnsSetupMode, providerName, detectedDNSProvider string) bool {
	// OpenReplay-specific logic:
	// Skip DNS setup if provider is DigitalOcean but DNS is managed elsewhere (e.g., Cloudflare)
	// This avoids conflicts when users want to use their existing DNS provider
	if strings.ToLower(providerName) == "digitalocean" &&
		detectedDNSProvider != "" &&
		strings.ToLower(detectedDNSProvider) != "digitalocean" {
		return false
	}

	// Default: setup DNS
	return true
}

package cmd

import (
	"strings"

	"github.com/zdunecki/selfhosted/pkg/cli"
	"github.com/zdunecki/selfhosted/pkg/utils"
)

func shouldSetupDNS(opts cli.DeployOptions, providerName string) bool {
	mode := strings.ToLower(strings.TrimSpace(opts.DNSSetupMode))
	if mode == "" {
		mode = "auto"
	}

	if opts.AppName != "openreplay" {
		return true
	}

	if mode == "skip" {
		return false
	}
	if mode == "force" {
		return true
	}

	if strings.ToLower(providerName) != "digitalocean" {
		return true
	}

	info := utils.DetectDNSProvider(opts.Domain)
	if info.Name == "" || info.Name == utils.DNSProviderDigitalOcean {
		return true
	}
	return false
}

package cmd

import (
	"net"
	"strings"
)

type dnsProviderInfo struct {
	Name string
	Host string
}

func detectDNSProvider(domain string) dnsProviderInfo {
	records := lookupNS(domain)
	if len(records) == 0 {
		root := getRootDomain(domain)
		if root != "" && root != domain {
			records = lookupNS(root)
		}
	}
	if len(records) == 0 {
		return dnsProviderInfo{}
	}

	host := strings.TrimSuffix(strings.ToLower(records[0].Host), ".")
	switch {
	case strings.Contains(host, "digitalocean.com"):
		return dnsProviderInfo{Name: "DigitalOcean", Host: host}
	case strings.Contains(host, "cloudflare.com"):
		return dnsProviderInfo{Name: "Cloudflare", Host: host}
	case strings.Contains(host, "awsdns"):
		return dnsProviderInfo{Name: "AWS Route 53", Host: host}
	case strings.Contains(host, "googledomains.com"):
		return dnsProviderInfo{Name: "Google Cloud DNS", Host: host}
	default:
		return dnsProviderInfo{Name: host, Host: host}
	}
}

func lookupNS(domain string) []*net.NS {
	records, err := net.LookupNS(domain)
	if err != nil {
		return nil
	}
	return records
}

func getRootDomain(domain string) string {
	parts := strings.Split(strings.TrimSpace(domain), ".")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func shouldSetupDNS(opts deployOptions, providerName string) bool {
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

	info := detectDNSProvider(opts.Domain)
	if info.Name == "" || info.Name == "DigitalOcean" {
		return true
	}
	return false
}

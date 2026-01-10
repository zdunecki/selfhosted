package dns

import (
	"net"
	"strings"
)

// DNSProvider represents a DNS provider
type DNSProvider string

// DNS Provider constants
const (
	DNSProviderDigitalOcean DNSProvider = "DigitalOcean"
	DNSProviderCloudflare   DNSProvider = "Cloudflare"
	DNSProviderAWS          DNSProvider = "AWS Route 53"
	DNSProviderGoogleCloud  DNSProvider = "Google Cloud DNS"
	DNSProviderAzure        DNSProvider = "Azure DNS"
	DNSProviderLinode       DNSProvider = "Linode"
	DNSProviderVultr        DNSProvider = "Vultr"
	DNSProviderHetzner      DNSProvider = "Hetzner"
	DNSProviderOVH          DNSProvider = "OVH"
	DNSProviderNamecheap    DNSProvider = "Namecheap"
	DNSProviderGoDaddy      DNSProvider = "GoDaddy"
	DNSProviderNameDotCom   DNSProvider = "Name.com"
	DNSProviderBluehost     DNSProvider = "Bluehost"
	DNSProviderHostGator    DNSProvider = "HostGator"
	DNSProviderDreamHost    DNSProvider = "DreamHost"
	DNSProviderHover        DNSProvider = "Hover"
	DNSProviderDNSimple     DNSProvider = "DNSimple"
	DNSProviderZoneEdit     DNSProvider = "ZoneEdit"
	DNSProviderNetlify      DNSProvider = "Netlify DNS"
	DNSProviderVercel       DNSProvider = "Vercel DNS"
	DNSProviderDyn          DNSProvider = "Dyn"
	DNSProviderNS1          DNSProvider = "NS1"
	DNSProviderDNSPark      DNSProvider = "DNSPark"
	DNSProviderEasyDNS      DNSProvider = "EasyDNS"
	DNSProviderFreeDNS      DNSProvider = "FreeDNS"
	DNSProviderUnknown      DNSProvider = "Unknown"
)

// DNSProviderInfo holds information about a detected DNS provider
type DNSProviderInfo struct {
	Name DNSProvider
	Host string
}

// DetectDNSProvider detects the DNS provider for a given domain
func DetectDNSProvider(domain string) DNSProviderInfo {
	records := lookupNS(domain)
	if len(records) == 0 {
		root := GetRootDomain(domain)
		if root != "" && root != domain {
			records = lookupNS(root)
		}
	}
	if len(records) == 0 {
		return DNSProviderInfo{}
	}

	host := strings.TrimSuffix(strings.ToLower(records[0].Host), ".")
	switch {
	case strings.Contains(host, "digitalocean.com"):
		return DNSProviderInfo{Name: DNSProviderDigitalOcean, Host: host}
	case strings.Contains(host, "cloudflare.com"):
		return DNSProviderInfo{Name: DNSProviderCloudflare, Host: host}
	case strings.Contains(host, "awsdns"):
		return DNSProviderInfo{Name: DNSProviderAWS, Host: host}
	case strings.Contains(host, "googledomains.com"), strings.Contains(host, "google.com"):
		return DNSProviderInfo{Name: DNSProviderGoogleCloud, Host: host}
	case strings.Contains(host, "azure-dns"):
		return DNSProviderInfo{Name: DNSProviderAzure, Host: host}
	case strings.Contains(host, "linode.com"):
		return DNSProviderInfo{Name: DNSProviderLinode, Host: host}
	case strings.Contains(host, "vultr.com"):
		return DNSProviderInfo{Name: DNSProviderVultr, Host: host}
	case strings.Contains(host, "hetzner.com"), strings.Contains(host, "hetzner-dns"):
		return DNSProviderInfo{Name: DNSProviderHetzner, Host: host}
	case strings.Contains(host, "ovh.net"), strings.Contains(host, "ovh.com"):
		return DNSProviderInfo{Name: DNSProviderOVH, Host: host}
	case strings.Contains(host, "namecheap.com"):
		return DNSProviderInfo{Name: DNSProviderNamecheap, Host: host}
	case strings.Contains(host, "godaddy.com"):
		return DNSProviderInfo{Name: DNSProviderGoDaddy, Host: host}
	case strings.Contains(host, "name.com"):
		return DNSProviderInfo{Name: DNSProviderNameDotCom, Host: host}
	case strings.Contains(host, "bluehost.com"):
		return DNSProviderInfo{Name: DNSProviderBluehost, Host: host}
	case strings.Contains(host, "hostgator.com"):
		return DNSProviderInfo{Name: DNSProviderHostGator, Host: host}
	case strings.Contains(host, "dreamhost.com"):
		return DNSProviderInfo{Name: DNSProviderDreamHost, Host: host}
	case strings.Contains(host, "hover.com"):
		return DNSProviderInfo{Name: DNSProviderHover, Host: host}
	case strings.Contains(host, "dnsimple.com"):
		return DNSProviderInfo{Name: DNSProviderDNSimple, Host: host}
	case strings.Contains(host, "zoneedit.com"):
		return DNSProviderInfo{Name: DNSProviderZoneEdit, Host: host}
	case strings.Contains(host, "netlify.com"):
		return DNSProviderInfo{Name: DNSProviderNetlify, Host: host}
	case strings.Contains(host, "vercel-dns.com"):
		return DNSProviderInfo{Name: DNSProviderVercel, Host: host}
	case strings.Contains(host, "dynect.net"):
		return DNSProviderInfo{Name: DNSProviderDyn, Host: host}
	case strings.Contains(host, "nsone.net"):
		return DNSProviderInfo{Name: DNSProviderNS1, Host: host}
	case strings.Contains(host, "dnspark.net"):
		return DNSProviderInfo{Name: DNSProviderDNSPark, Host: host}
	case strings.Contains(host, "easydns.com"):
		return DNSProviderInfo{Name: DNSProviderEasyDNS, Host: host}
	case strings.Contains(host, "afraid.org"):
		return DNSProviderInfo{Name: DNSProviderFreeDNS, Host: host}
	default:
		return DNSProviderInfo{Name: DNSProviderUnknown, Host: host}
	}
}

func lookupNS(domain string) []*net.NS {
	records, err := net.LookupNS(domain)
	if err != nil {
		return nil
	}
	return records
}

// GetRootDomain extracts the root domain from a domain string
func GetRootDomain(domain string) string {
	parts := strings.Split(strings.TrimSpace(domain), ".")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

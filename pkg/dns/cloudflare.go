package dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// CloudflareProvider handles Cloudflare DNS operations
type CloudflareProvider struct {
	apiToken string
}

// GetToken returns the API token (for internal use)
func (c *CloudflareProvider) GetToken() string {
	return c.apiToken
}

// CloudflareZone represents a Cloudflare zone
type CloudflareZone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// CloudflareZonesResponse represents the API response for zones
type CloudflareZonesResponse struct {
	Result  []CloudflareZone `json:"result"`
	Success bool             `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

// CloudflareDNSRecordRequest represents a DNS record creation request
type CloudflareDNSRecordRequest struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// CloudflareDNSRecordResponse represents the API response for DNS record creation
type CloudflareDNSRecordResponse struct {
	Success bool `json:"success"`
	Result  struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"result"`
	Errors []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

// NewCloudflareProvider creates a new Cloudflare DNS provider
// It attempts to get the API token from CLOUDFLARE_API_TOKEN env var first,
// then falls back to showing instructions for creating one
func NewCloudflareProvider() (*CloudflareProvider, error) {
	cf := &CloudflareProvider{}

	// Check for CLOUDFLARE_API_TOKEN environment variable first
	if token := os.Getenv("CLOUDFLARE_API_TOKEN"); token != "" {
		cf.apiToken = token
		return cf, nil
	}

	// No env var - need to guide user to create a token
	return nil, fmt.Errorf("CLOUDFLARE_API_TOKEN not found")
}

// NewCloudflareProviderWithToken creates a new Cloudflare DNS provider with a custom token
func NewCloudflareProviderWithToken(token string) (*CloudflareProvider, error) {
	if token == "" {
		return nil, fmt.Errorf("token cannot be empty")
	}
	return &CloudflareProvider{apiToken: token}, nil
}

// isWranglerAvailable checks if wrangler CLI is installed
func isWranglerAvailable() bool {
	cmd := exec.Command("wrangler", "--version")
	err := cmd.Run()
	return err == nil
}

// getAccountID retrieves the Cloudflare account ID from wrangler whoami
func getAccountID() (string, error) {
	cmd := exec.Command("wrangler", "whoami")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run wrangler whoami: %w", err)
	}

	// Parse output to extract account ID
	// Expected format contains lines like:
	// Account ID: 38506b7d3508e99f1165509b2344237b
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Account ID") || strings.Contains(line, "account_id") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				accountID := strings.TrimSpace(parts[1])
				if len(accountID) > 0 {
					return accountID, nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not extract account ID from wrangler whoami output")
}

// GetTokenCreationURL returns the URL to create a Cloudflare API token
// If wrangler is available, it uses the account ID from wrangler whoami
func GetTokenCreationURL() string {
	if !isWranglerAvailable() {
		return "https://dash.cloudflare.com/profile/api-tokens"
	}

	accountID, err := getAccountID()
	if err == nil && accountID != "" {
		return fmt.Sprintf("https://dash.cloudflare.com/%s/profile/api-tokens", accountID)
	}
	// Fallback to generic tokens page
	return "https://dash.cloudflare.com/profile/api-tokens"
}

// FindZoneForDomain finds the Cloudflare zone that matches the given domain
func (c *CloudflareProvider) FindZoneForDomain(domain string) (*CloudflareZone, error) {
	// Get root domain (e.g., xyz.livesession.io -> livesession.io)
	rootDomain := GetRootDomain(domain)

	url := "https://api.cloudflare.com/client/v4/zones"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query zones: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var zonesResp CloudflareZonesResponse
	if err := json.Unmarshal(body, &zonesResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !zonesResp.Success {
		if len(zonesResp.Errors) > 0 {
			return nil, fmt.Errorf("API error: %s", zonesResp.Errors[0].Message)
		}
		return nil, fmt.Errorf("API request failed")
	}

	// Find matching zone
	// First try exact match with root domain
	for _, zone := range zonesResp.Result {
		if strings.EqualFold(zone.Name, rootDomain) {
			return &zone, nil
		}
	}

	// If no exact match, try to find zone that the domain belongs to
	for _, zone := range zonesResp.Result {
		if strings.HasSuffix(strings.ToLower(domain), "."+strings.ToLower(zone.Name)) {
			return &zone, nil
		}
	}

	return nil, fmt.Errorf("no matching zone found for domain %s", domain)
}

// CreateDNSRecord creates a DNS record in Cloudflare.
func (c *CloudflareProvider) CreateDNSRecord(zoneID string, recordReq CloudflareDNSRecordRequest) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)

	jsonData, err := json.Marshal(recordReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create DNS record: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var dnsResp CloudflareDNSRecordResponse
	if err := json.Unmarshal(body, &dnsResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !dnsResp.Success {
		if len(dnsResp.Errors) > 0 {
			return fmt.Errorf("API error: %s", dnsResp.Errors[0].Message)
		}
		return fmt.Errorf("failed to create DNS record")
	}

	return nil
}

// SetupDNS creates a DNS A record for the domain pointing to the IP
func (c *CloudflareProvider) SetupDNS(domain, ip string, proxied bool) error {
	// Find the zone
	zone, err := c.FindZoneForDomain(domain)
	if err != nil {
		return err
	}

	// Create DNS record
	return c.CreateDNSRecord(zone.ID, CloudflareDNSRecordRequest{
		Type:    "A",
		Name:    domain,
		Content: ip,
		TTL:     3600,
		Proxied: proxied,
	})
}

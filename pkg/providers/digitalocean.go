package providers

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"github.com/zdunecki/selfhosted/pkg/terraform"
	"golang.org/x/oauth2"
)

type DigitalOcean struct {
	client    *godo.Client
	ctx       context.Context
	token     string
	tfServer  *Server
	tfWorkDir string
}

func NewDigitalOcean() *DigitalOcean {
	return &DigitalOcean{
		ctx: context.Background(),
	}
}

func (d *DigitalOcean) Configure(config map[string]string) error {
	token, ok := config["token"]
	if !ok || token == "" {
		return fmt.Errorf("token invalid or missing")
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	d.client = godo.NewClient(oauthClient)
	d.token = token
	return nil
}

func (d *DigitalOcean) Name() string {
	return "digitalocean"
}

func (d *DigitalOcean) Description() string {
	return "DigitalOcean - Simple cloud hosting"
}

// NeedsConfig indicates this provider typically requires user-supplied credentials.
// (It can also be configured via env vars, but the installer UI should prompt by default.)
func (d *DigitalOcean) NeedsConfig() bool {
	// If client already configured via Configure(), no config needed.
	if d.client != nil {
		return false
	}
	// If env vars exist, no config needed.
	if os.Getenv("DIGITALOCEAN_TOKEN") != "" || os.Getenv("DO_TOKEN") != "" {
		return false
	}
	return true
}

func (d *DigitalOcean) DefaultRegion() string {
	return "fra1"
}

func (d *DigitalOcean) ensureClient() error {
	if d.client != nil {
		return nil
	}

	// Try loading from env
	token := os.Getenv("DIGITALOCEAN_TOKEN")
	if token == "" {
		token = os.Getenv("DO_TOKEN")
	}

	if token != "" {
		tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		oauthClient := oauth2.NewClient(context.Background(), tokenSource)
		d.client = godo.NewClient(oauthClient)
		d.token = token
		return nil
	}

	return fmt.Errorf("DIGITALOCEAN_TOKEN or DO_TOKEN environment variable required")
}

func (d *DigitalOcean) ListRegions() ([]Region, error) {
	if err := d.ensureClient(); err != nil {
		return nil, err
	}

	regions, _, err := d.client.Regions.List(d.ctx, &godo.ListOptions{PerPage: 100})
	if err != nil {
		return nil, err
	}

	var result []Region
	for _, r := range regions {
		if r.Available {
			result = append(result, Region{
				Slug: r.Slug,
				Name: r.Name,
			})
		}
	}
	return result, nil
}

func (d *DigitalOcean) ListSizes() ([]Size, error) {
	if err := d.ensureClient(); err != nil {
		return nil, err
	}

	sizes, _, err := d.client.Sizes.List(d.ctx, &godo.ListOptions{PerPage: 100})
	if err != nil {
		return nil, err
	}

	var result []Size
	for _, s := range sizes {
		if s.Available {
			result = append(result, Size{
				Slug:         s.Slug,
				VCPUs:        s.Vcpus,
				MemoryMB:     s.Memory,
				DiskGB:       s.Disk,
				PriceMonthly: float64(s.PriceMonthly),
				PriceHourly:  float64(s.PriceHourly),
			})
		}
	}
	return result, nil
}

func (d *DigitalOcean) GetSizeForSpecs(specs Specs) (string, error) {
	sizes, err := d.ListSizes()
	if err != nil {
		return "", err
	}

	best, ok := pickBestSizeForSpecs(sizes, specs)
	if !ok {
		return "", fmt.Errorf("no size found matching specs: %d CPUs, %dMB RAM", specs.CPUs, specs.MemoryMB)
	}

	return best.Slug, nil
}

func (d *DigitalOcean) CreateServer(config *DeployConfig) (*Server, error) {
	if err := d.ensureClient(); err != nil {
		return nil, err
	}

	// Get profile from env var (default: "basic")
	profile := strings.TrimSpace(strings.ToLower(os.Getenv("SELFHOSTED_DO_PROFILE")))
	if profile == "" {
		profile = "basic"
	}

	moduleDir, err := terraform.FindModuleDir("digitalocean", profile)
	if err != nil {
		return nil, err
	}

	env := d.terraformEnv()
	if len(env) == 0 {
		return nil, fmt.Errorf("DIGITALOCEAN_TOKEN or DO_TOKEN environment variable required")
	}

	image := config.Image
	if image == "" {
		image = "ubuntu-22-04-x64"
	}

	fingerprint, err := sshPublicKeyFingerprint(config.SSHPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to compute SSH key fingerprint: %w", err)
	}

	vars := map[string]interface{}{
		"name":            config.Name,
		"region":          config.Region,
		"size":            config.Size,
		"image":           image,
		"ssh_public_key":  config.SSHPublicKey,
		"ssh_fingerprint": fingerprint,
		"tags":            config.Tags,
	}

	// Add volume_size for advanced profile
	if profile == "advanced" {
		volumeSize := 100 // default 100 GiB
		if sizeStr := os.Getenv("SELFHOSTED_DO_VOLUME_SIZE"); sizeStr != "" {
			if parsed, err := strconv.Atoi(sizeStr); err == nil && parsed > 0 {
				volumeSize = parsed
			}
		}
		vars["volume_size"] = volumeSize
	}

	runID := fmt.Sprintf("%s-%d", config.Name, time.Now().Unix())
	result, err := terraform.Apply(d.ctx, moduleDir, runID, env, vars)
	if err != nil {
		return nil, err
	}

	ip, _ := terraform.OutputString(result.Outputs, "droplet_ipv4")
	dropletID, _ := terraform.OutputString(result.Outputs, "droplet_id")

	server := &Server{
		ID:     dropletID,
		Name:   config.Name,
		IP:     ip,
		Status: "active",
	}

	d.tfServer = server
	d.tfWorkDir = result.WorkDir

	return server, nil
}

func (d *DigitalOcean) WaitForServer(id string) (*Server, error) {
	// Terraform creates servers synchronously, so the server is already ready
	if d.tfServer != nil {
		return d.tfServer, nil
	}
	// Fallback: return server with the provided ID (shouldn't happen in normal flow)
	return &Server{
		ID:     id,
		Status: "active",
	}, nil
}

func (d *DigitalOcean) DestroyServer(id string) error {
	if d.tfWorkDir == "" {
		return fmt.Errorf("terraform work directory not found for server %s", id)
	}

	env := d.terraformEnv()
	if len(env) == 0 {
		return fmt.Errorf("DIGITALOCEAN_TOKEN or DO_TOKEN environment variable required")
	}

	return terraform.Destroy(d.ctx, d.tfWorkDir, env)
}

func (d *DigitalOcean) terraformEnv() map[string]string {
	if d.token == "" {
		return nil
	}
	return map[string]string{
		"DIGITALOCEAN_TOKEN": d.token,
	}
}

func (d *DigitalOcean) SetupDNS(domain, ip string) error {
	if err := d.ensureClient(); err != nil {
		return err
	}

	rootDomain := getRootDomain(domain)
	subdomain := getSubdomain(domain)

	// Try to create domain first (this will work if user owns the domain and wants DO to manage DNS)
	domainCreated := false
	_, resp, err := d.client.Domains.Create(d.ctx, &godo.DomainCreateRequest{
		Name: rootDomain,
	})

	if err == nil {
		// Domain created successfully
		domainCreated = true
		fmt.Printf("✅ Domain '%s' added to DigitalOcean\n", rootDomain)
		fmt.Println("📋 Point your domain's nameservers to:")
		fmt.Println("   ns1.digitalocean.com")
		fmt.Println("   ns2.digitalocean.com")
		fmt.Println("   ns3.digitalocean.com")
	} else if resp != nil && resp.StatusCode == 422 {
		// Domain already exists in DigitalOcean (422 = unprocessable entity)
		domainCreated = true
	} else {
		// Check if domain already exists
		_, _, checkErr := d.client.Domains.Get(d.ctx, rootDomain)
		if checkErr == nil {
			// Domain exists
			domainCreated = true
		}
	}

	if !domainCreated {
		// Domain doesn't exist in DigitalOcean - provide instructions
		return fmt.Errorf(`domain '%s' not managed by DigitalOcean

To use DigitalOcean DNS, you have two options:

Option 1: Point your domain's nameservers to DigitalOcean (recommended)
   1. Go to your domain registrar (GoDaddy, Namecheap, etc.)
   2. Update nameservers to:
      - ns1.digitalocean.com
      - ns2.digitalocean.com
      - ns3.digitalocean.com
   3. Wait 24-48 hours for propagation
   4. Add domain at: https://cloud.digitalocean.com/networking/domains/new
   5. Re-run the deployment

Option 2: Manual DNS configuration
   1. Create an A record in your domain registrar:
      - Type: A
      - Name: %s
      - Value: %s
      - TTL: 300
   2. Skip this step and continue with the deployment`, rootDomain, subdomain, ip)
	}

	// Create A record
	recordRequest := &godo.DomainRecordEditRequest{
		Type: "A",
		Name: subdomain,
		Data: ip,
		TTL:  300,
	}

	_, _, err = d.client.Domains.CreateRecord(d.ctx, rootDomain, recordRequest)
	if err != nil {
		return fmt.Errorf("failed to create DNS record: %w\n\nManual configuration:\n  Create an A record for '%s' pointing to '%s' at https://cloud.digitalocean.com/networking/domains/%s", err, subdomain, ip, rootDomain)
	}

	return nil
}

// Helper functions
func getRootDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return domain
}

func getSubdomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) > 2 {
		return strings.Join(parts[:len(parts)-2], ".")
	}
	return "@"
}

// sshPublicKeyFingerprint computes the MD5 fingerprint of an OpenSSH public key.
// The format matches DigitalOcean's fingerprint format (e.g., "ab:cd:ef:...").
func sshPublicKeyFingerprint(pubKey string) (string, error) {
	pubKey = strings.TrimSpace(pubKey)
	parts := strings.Fields(pubKey)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid SSH public key format")
	}

	keyData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode SSH public key: %w", err)
	}

	hash := md5.Sum(keyData)
	fingerprint := make([]string, len(hash))
	for i, b := range hash {
		fingerprint[i] = fmt.Sprintf("%02x", b)
	}

	return strings.Join(fingerprint, ":"), nil
}

func init() {
	Register(NewDigitalOcean())
}

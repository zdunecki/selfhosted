package providers

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
)

type DigitalOcean struct {
	client *godo.Client
	ctx    context.Context
}

func NewDigitalOcean() *DigitalOcean {
	token := os.Getenv("DIGITALOCEAN_TOKEN")
	if token == "" {
		token = os.Getenv("DO_TOKEN")
	}

	var client *godo.Client
	if token != "" {
		tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		oauthClient := oauth2.NewClient(context.Background(), tokenSource)
		client = godo.NewClient(oauthClient)
	}

	return &DigitalOcean{
		client: client,
		ctx:    context.Background(),
	}
}

func (d *DigitalOcean) Name() string {
	return "digitalocean"
}

func (d *DigitalOcean) Description() string {
	return "DigitalOcean - Simple cloud hosting"
}

func (d *DigitalOcean) DefaultRegion() string {
	return "fra1"
}

func (d *DigitalOcean) ensureClient() error {
	if d.client == nil {
		return fmt.Errorf("DIGITALOCEAN_TOKEN or DO_TOKEN environment variable required")
	}
	return nil
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

	// Find the smallest size that meets the requirements
	var bestSize *Size
	for i, s := range sizes {
		if s.VCPUs >= specs.CPUs && s.MemoryMB >= specs.MemoryMB {
			if bestSize == nil || s.PriceMonthly < bestSize.PriceMonthly {
				bestSize = &sizes[i]
			}
		}
	}

	if bestSize == nil {
		return "", fmt.Errorf("no size found matching specs: %d CPUs, %dMB RAM", specs.CPUs, specs.MemoryMB)
	}

	return bestSize.Slug, nil
}

func (d *DigitalOcean) CreateServer(config *DeployConfig) (*Server, error) {
	if err := d.ensureClient(); err != nil {
		return nil, err
	}

	// Create or get SSH key
	sshKeyID, err := d.ensureSSHKey(config.SSHPublicKey)
	if err != nil {
		return nil, fmt.Errorf("SSH key setup failed: %w", err)
	}

	// Default image
	image := config.Image
	if image == "" {
		image = "ubuntu-22-04-x64"
	}

	createRequest := &godo.DropletCreateRequest{
		Name:   config.Name,
		Region: config.Region,
		Size:   config.Size,
		Image: godo.DropletCreateImage{
			Slug: image,
		},
		SSHKeys: []godo.DropletCreateSSHKey{
			{ID: sshKeyID},
		},
		Tags: config.Tags,
	}

	droplet, _, err := d.client.Droplets.Create(d.ctx, createRequest)
	if err != nil {
		return nil, err
	}

	return &Server{
		ID:     strconv.Itoa(droplet.ID),
		Name:   droplet.Name,
		Status: droplet.Status,
	}, nil
}

func (d *DigitalOcean) WaitForServer(id string) (*Server, error) {
	if err := d.ensureClient(); err != nil {
		return nil, err
	}

	dropletID, _ := strconv.Atoi(id)
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for server %s", id)
		case <-ticker.C:
			droplet, _, err := d.client.Droplets.Get(d.ctx, dropletID)
			if err != nil {
				return nil, err
			}

			if droplet.Status == "active" {
				ip := ""
				for _, network := range droplet.Networks.V4 {
					if network.Type == "public" {
						ip = network.IPAddress
						break
					}
				}

				return &Server{
					ID:     id,
					Name:   droplet.Name,
					IP:     ip,
					Status: droplet.Status,
				}, nil
			}
		}
	}
}

func (d *DigitalOcean) DestroyServer(id string) error {
	if err := d.ensureClient(); err != nil {
		return err
	}

	dropletID, err := strconv.Atoi(id)
	if err != nil {
		return fmt.Errorf("invalid server ID: %s", id)
	}

	_, err = d.client.Droplets.Delete(d.ctx, dropletID)
	return err
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

func (d *DigitalOcean) ensureSSHKey(publicKey string) (int, error) {
	// Check if key already exists
	keys, _, err := d.client.Keys.List(d.ctx, &godo.ListOptions{PerPage: 100})
	if err != nil {
		return 0, err
	}

	// Check by fingerprint or name
	for _, key := range keys {
		if key.Name == "selfhost-key" {
			return key.ID, nil
		}
	}

	// Create new key
	keyReq := &godo.KeyCreateRequest{
		Name:      "selfhost-key",
		PublicKey: publicKey,
	}

	key, _, err := d.client.Keys.Create(d.ctx, keyReq)
	if err != nil {
		return 0, err
	}

	return key.ID, nil
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

func init() {
	Register(NewDigitalOcean())
}

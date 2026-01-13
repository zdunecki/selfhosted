package providers

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vultr/govultr/v3"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

// Vultr implements Provider using Vultr's official Go SDK (govultr v3).
//
// Auth:
// - API key (PAT) via config or env var VULTR_API_KEY
//
// Env:
// - VULTR_API_KEY (recommended)
//
// Config fallback:
// - ~/.vultr-cli.yaml  (field: api-key)
// - XDG_CONFIG_HOME is respected for config lookup
type Vultr struct {
	client *govultr.Client
	ctx    context.Context

	apiKey string

	// cached OS id for Ubuntu
	ubuntuOSID int
}

func NewVultr() *Vultr {
	return &Vultr{ctx: context.Background()}
}

func (v *Vultr) Name() string { return "vultr" }

func (v *Vultr) Description() string { return "Vultr - Global cloud hosting (Go SDK)" }

func (v *Vultr) DefaultRegion() string { return "ewr" } // New Jersey (commonly available)

func (v *Vultr) NeedsConfig() bool {
	if v.client != nil {
		return false
	}
	if strings.TrimSpace(os.Getenv("VULTR_API_KEY")) != "" {
		return false
	}
	ok, _ := canLoadVultrCLIConfig()
	return !ok
}

func (v *Vultr) Configure(config map[string]string) error {
	// Accept common keys.
	if x := strings.TrimSpace(config["token"]); x != "" {
		v.apiKey = x
	}
	if x := strings.TrimSpace(config["api_key"]); x != "" {
		v.apiKey = x
	}
	if x := strings.TrimSpace(config["api-key"]); x != "" {
		v.apiKey = x
	}

	// reset cached client so ensureClient rebuilds.
	v.client = nil
	v.ubuntuOSID = 0

	_, err := v.ensureClient()
	return err
}

func (v *Vultr) ensureClient() (*govultr.Client, error) {
	if v.client != nil {
		return v.client, nil
	}

	if strings.TrimSpace(v.apiKey) == "" {
		v.apiKey = strings.TrimSpace(os.Getenv("VULTR_API_KEY"))
	}
	if strings.TrimSpace(v.apiKey) == "" {
		_ = v.loadFromVultrCLIConfigFile()
	}
	if strings.TrimSpace(v.apiKey) == "" {
		return nil, fmt.Errorf("VULTR_API_KEY (or ~/.vultr-cli.yaml api-key) is required")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: v.apiKey})
	httpClient := oauth2.NewClient(v.ctx, ts)
	c := govultr.NewClient(httpClient)
	c.SetRateLimit(500)
	v.client = c

	return v.client, nil
}

func (v *Vultr) ListRegions() ([]Region, error) {
	c, err := v.ensureClient()
	if err != nil {
		return nil, err
	}

	regions, _, _, err := c.Region.List(v.ctx, nil)
	if err != nil {
		return nil, err
	}

	out := make([]Region, 0, len(regions))
	for _, r := range regions {
		name := r.ID
		// e.g. City "Amsterdam", Country "NL"
		if strings.TrimSpace(r.City) != "" && strings.TrimSpace(r.Country) != "" {
			name = fmt.Sprintf("%s (%s)", r.City, r.Country)
		} else if strings.TrimSpace(r.City) != "" {
			name = r.City
		}
		out = append(out, Region{Slug: r.ID, Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

func (v *Vultr) ListSizes() ([]Size, error) {
	// Default: return all compute plans (vc2) and sort by price.
	c, err := v.ensureClient()
	if err != nil {
		return nil, err
	}

	plans, err := v.listPlans(c, "vc2")
	if err != nil {
		// Fallback to all plans if vc2 isn't available for the account.
		plans, err = v.listPlans(c, "")
		if err != nil {
			return nil, err
		}
	}

	out := make([]Size, 0, len(plans))
	for _, p := range plans {
		// Skip GPU plans; these typically have GPU fields populated.
		if p.GPUVRAM > 0 {
			continue
		}
		monthly := float64(p.MonthlyCost)
		hourly := 0.0
		if monthly > 0 {
			hourly = monthly / (24 * 30)
		}
		out = append(out, Size{
			Slug:         p.ID,
			VCPUs:        p.VCPUCount,
			MemoryMB:     p.RAM,
			DiskGB:       p.Disk,
			PriceMonthly: monthly,
			PriceHourly:  hourly,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PriceMonthly < out[j].PriceMonthly })
	return out, nil
}

// ListSizesForRegion filters plans by their allowed Locations field when available.
func (v *Vultr) ListSizesForRegion(region string) ([]Size, error) {
	region = strings.TrimSpace(region)
	if region == "" {
		return v.ListSizes()
	}

	c, err := v.ensureClient()
	if err != nil {
		return nil, err
	}

	plans, err := v.listPlans(c, "vc2")
	if err != nil {
		plans, err = v.listPlans(c, "")
		if err != nil {
			return nil, err
		}
	}

	allowed := map[string]struct{}{}
	// Use region availability endpoint when possible to filter.
	if avail, _, aerr := c.Region.Availability(v.ctx, region, ""); aerr == nil && avail != nil {
		for _, id := range avail.AvailablePlans {
			allowed[id] = struct{}{}
		}
	}

	out := make([]Size, 0, len(plans))
	for _, p := range plans {
		if p.GPUVRAM > 0 {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[p.ID]; !ok {
				continue
			}
		} else if len(p.Locations) > 0 {
			ok := false
			for _, loc := range p.Locations {
				if strings.EqualFold(loc, region) {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}

		monthly := float64(p.MonthlyCost)
		hourly := 0.0
		if monthly > 0 {
			hourly = monthly / (24 * 30)
		}
		out = append(out, Size{
			Slug:         p.ID,
			VCPUs:        p.VCPUCount,
			MemoryMB:     p.RAM,
			DiskGB:       p.Disk,
			PriceMonthly: monthly,
			PriceHourly:  hourly,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].PriceMonthly < out[j].PriceMonthly })
	return out, nil
}

func (v *Vultr) listPlans(c *govultr.Client, planType string) ([]govultr.Plan, error) {
	var out []govultr.Plan
	opts := &govultr.ListOptions{PerPage: 500}
	for {
		plans, meta, _, err := c.Plan.List(v.ctx, planType, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, plans...)
		if meta == nil || meta.Links == nil || meta.Links.Next == "" {
			break
		}
		opts.Cursor = meta.Links.Next
	}
	return out, nil
}

func (v *Vultr) GetSizeForSpecs(specs Specs) (string, error) {
	sizes, err := v.ListSizes()
	if err != nil {
		return "", err
	}

	best, ok := pickBestSizeForSpecs(sizes, specs)
	if !ok {
		return "", fmt.Errorf("no Vultr plan found matching specs: %d CPUs, %dMB RAM", specs.CPUs, specs.MemoryMB)
	}
	return best.Slug, nil
}

func (v *Vultr) CreateServer(config *DeployConfig) (*Server, error) {
	c, err := v.ensureClient()
	if err != nil {
		return nil, err
	}

	region := strings.TrimSpace(config.Region)
	if region == "" {
		region = v.DefaultRegion()
	}

	plan := strings.TrimSpace(config.Size)
	if plan == "" {
		return nil, fmt.Errorf("vultr: plan is required")
	}

	sshID, err := v.ensureSSHKey(c, config.SSHPublicKey)
	if err != nil {
		return nil, fmt.Errorf("vultr: ssh key setup failed: %w", err)
	}

	osID, err := v.ensureUbuntuOSID(c)
	if err != nil {
		return nil, err
	}

	label := strings.TrimSpace(config.Name)
	if label == "" {
		label = "selfhosted-server"
	}
	host := sanitizeHostname(label)
	if host == "" {
		host = "selfhosted"
	}

	req := &govultr.InstanceCreateReq{
		Label:    label,
		Hostname: host,
		Region:   region,
		Plan:     plan,
		OsID:     osID,
		// SSHKeys expects the ssh key IDs to attach.
		SSHKeys: []string{sshID},
		// Keep defaults; IPv6 is optional for our installer.
		EnableIPv6: govultr.BoolToBoolPtr(false),
		Tags:       config.Tags,
	}

	inst, _, err := c.Instance.Create(v.ctx, req)
	if err != nil {
		return nil, err
	}
	if inst == nil || strings.TrimSpace(inst.ID) == "" {
		return nil, fmt.Errorf("vultr: create instance returned empty id")
	}

	return &Server{
		ID:     inst.ID,
		Name:   label,
		Status: inst.Status,
	}, nil
}

func (v *Vultr) WaitForServer(id string) (*Server, error) {
	c, err := v.ensureClient()
	if err != nil {
		return nil, err
	}

	id = strings.TrimSpace(id)
	timeout := time.After(10 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for Vultr server %s", id)
		case <-ticker.C:
			inst, _, err := c.Instance.Get(v.ctx, id)
			if err != nil {
				return nil, err
			}
			if inst == nil {
				continue
			}
			ip := strings.TrimSpace(inst.MainIP)
			if ip != "" {
				_ = WaitForSSH(ip, 22)
				return &Server{
					ID:     id,
					Name:   inst.Label,
					IP:     ip,
					Status: inst.Status,
				}, nil
			}
		}
	}
}

func (v *Vultr) DestroyServer(id string) error {
	c, err := v.ensureClient()
	if err != nil {
		return err
	}
	return c.Instance.Delete(v.ctx, strings.TrimSpace(id))
}

func (v *Vultr) SetupDNS(domain, ip string) error {
	return fmt.Errorf("vultr DNS is not supported in this installer yet; please create an A record for %s -> %s at your DNS provider", domain, ip)
}

func (v *Vultr) ensureUbuntuOSID(c *govultr.Client) (int, error) {
	if v.ubuntuOSID != 0 {
		return v.ubuntuOSID, nil
	}

	oses, _, _, err := c.OS.List(v.ctx, &govultr.ListOptions{PerPage: 500})
	if err != nil {
		return 0, err
	}

	// Prefer newer Ubuntu.
	want := []string{"Ubuntu 24.04", "Ubuntu 22.04", "Ubuntu 20.04", "Ubuntu"}
	bestID := 0
	bestScore := -1
	for _, osx := range oses {
		name := strings.TrimSpace(osx.Name)
		if name == "" {
			continue
		}
		l := strings.ToLower(name)
		score := -1
		for i, w := range want {
			if strings.Contains(l, strings.ToLower(w)) {
				score = len(want) - i
				break
			}
		}
		if score > bestScore {
			bestScore = score
			bestID = osx.ID
		}
	}
	if bestID == 0 {
		return 0, fmt.Errorf("vultr: could not find an Ubuntu OS id")
	}
	v.ubuntuOSID = bestID
	return bestID, nil
}

func (v *Vultr) ensureSSHKey(c *govultr.Client, publicKey string) (string, error) {
	publicKey = strings.TrimSpace(publicKey)
	if publicKey == "" {
		return "", fmt.Errorf("empty ssh public key")
	}

	keys, _, _, err := c.SSHKey.List(v.ctx, &govultr.ListOptions{PerPage: 500})
	if err != nil {
		return "", err
	}

	for _, k := range keys {
		if strings.TrimSpace(k.SSHKey) == publicKey {
			return k.ID, nil
		}
	}

	// Create a stable-ish name based on key fingerprint.
	sum := sha1.Sum([]byte(publicKey))
	name := "selfhosted-" + hex.EncodeToString(sum[:4])

	key, _, err := c.SSHKey.Create(v.ctx, &govultr.SSHKeyReq{
		Name:   name,
		SSHKey: publicKey,
	})
	if err != nil {
		return "", err
	}
	if key == nil || strings.TrimSpace(key.ID) == "" {
		return "", fmt.Errorf("vultr: ssh key create returned empty id")
	}
	return key.ID, nil
}

type vultrCLIConfig struct {
	APIKey string `yaml:"api-key"`
}

func vultrCLIConfigPaths() []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}

	// XDG_CONFIG_HOME takes precedence if set (not documented by vultr-cli, but common convention).
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		add(filepath.Join(xdg, "vultr-cli.yaml"))
		add(filepath.Join(xdg, ".vultr-cli.yaml"))
	}

	if home := os.Getenv("HOME"); home != "" {
		// Some users keep config without the dot (~/vultr-cli.yaml).
		add(filepath.Join(home, "vultr-cli.yaml"))
		add(filepath.Join(home, ".vultr-cli.yaml"))
		add(filepath.Join(home, ".config", "vultr-cli.yaml"))
		add(filepath.Join(home, ".config", ".vultr-cli.yaml"))
	}

	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		add(filepath.Join(homeDir, "vultr-cli.yaml"))
		add(filepath.Join(homeDir, ".vultr-cli.yaml"))
		add(filepath.Join(homeDir, ".config", "vultr-cli.yaml"))
		add(filepath.Join(homeDir, ".config", ".vultr-cli.yaml"))
	}

	return out
}

func canLoadVultrCLIConfig() (bool, error) {
	for _, p := range vultrCLIConfigPaths() {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg vultrCLIConfig
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			continue
		}
		if strings.TrimSpace(cfg.APIKey) != "" {
			return true, nil
		}
	}
	return false, nil
}

func (v *Vultr) loadFromVultrCLIConfigFile() error {
	for _, p := range vultrCLIConfigPaths() {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg vultrCLIConfig
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			continue
		}
		if strings.TrimSpace(v.apiKey) == "" && strings.TrimSpace(cfg.APIKey) != "" {
			v.apiKey = strings.TrimSpace(cfg.APIKey)
		}
		return nil
	}
	return nil
}

func init() {
	Register(NewVultr())
}

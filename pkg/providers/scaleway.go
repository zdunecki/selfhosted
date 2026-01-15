package providers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"github.com/zdunecki/selfhosted/pkg/terraform"
	"gopkg.in/yaml.v3"
)

// Scaleway implements Provider using Scaleway's Go SDK.
//
// Auth expects:
// - access_key
// - secret_key
// - project_id (required for server create)
// - organization_id (optional; project-scoped keys are preferred)
// - zone (optional; defaults to fr-par-1)
//
// Env fallback:
// - SCW_ACCESS_KEY
// - SCW_SECRET_KEY
// - SCW_DEFAULT_PROJECT_ID
// - SCW_DEFAULT_ORGANIZATION_ID
// - SCW_DEFAULT_ZONE
type Scaleway struct {
	client *scw.Client
	api    *instance.API

	accessKey string
	secretKey string
	projectID string
	orgID     string
	zone      scw.Zone

	ctx context.Context

	// Terraform state
	tfServer  *Server
	tfWorkDir string
}

func NewScaleway() *Scaleway {
	return &Scaleway{
		ctx:  context.Background(),
		zone: scw.Zone("fr-par-1"),
	}
}

func (s *Scaleway) Name() string { return "scaleway" }

func (s *Scaleway) Description() string { return "Scaleway - European cloud hosting (Go SDK)" }

func (s *Scaleway) NeedsConfig() bool {
	// If we already have credentials, no config is needed.
	if strings.TrimSpace(s.accessKey) != "" && strings.TrimSpace(s.secretKey) != "" {
		return false
	}
	// Env vars also count.
	if strings.TrimSpace(os.Getenv("SCW_ACCESS_KEY")) != "" && strings.TrimSpace(os.Getenv("SCW_SECRET_KEY")) != "" {
		return false
	}
	// If ~/.config/scw/config.yaml exists and contains creds, don't force UI config.
	ok, _ := canLoadScalewayProfileFromConfigFile()
	return !ok
}

func (s *Scaleway) DefaultRegion() string { return string(s.zone) }

func (s *Scaleway) Configure(config map[string]string) error {
	// Accept a few common key names to make UI/JSON entry easier.
	if v := strings.TrimSpace(config["access_key"]); v != "" {
		s.accessKey = v
	}
	if v := strings.TrimSpace(config["secret_key"]); v != "" {
		s.secretKey = v
	}
	if v := strings.TrimSpace(config["project_id"]); v != "" {
		s.projectID = v
	}
	if v := strings.TrimSpace(config["organization_id"]); v != "" {
		s.orgID = v
	}
	if v := strings.TrimSpace(config["zone"]); v != "" {
		s.zone = scw.Zone(v)
	}

	// Reset cached client/api so next call uses new config.
	s.client = nil
	s.api = nil

	// Validate that we have at least credentials; project_id can come from env.
	if strings.TrimSpace(s.accessKey) == "" || strings.TrimSpace(s.secretKey) == "" {
		return fmt.Errorf("scaleway config missing access_key/secret_key")
	}

	// Attempt to create client now to validate credentials early.
	_, err := s.ensureAPI()
	return err
}

func (s *Scaleway) ensureAPI() (*instance.API, error) {
	if s.api != nil {
		return s.api, nil
	}

	// Fill from env if missing.
	if s.accessKey == "" {
		s.accessKey = os.Getenv("SCW_ACCESS_KEY")
	}
	if s.secretKey == "" {
		s.secretKey = os.Getenv("SCW_SECRET_KEY")
	}
	if s.projectID == "" {
		s.projectID = os.Getenv("SCW_DEFAULT_PROJECT_ID")
	}
	if s.orgID == "" {
		s.orgID = os.Getenv("SCW_DEFAULT_ORGANIZATION_ID")
	}
	if z := os.Getenv("SCW_DEFAULT_ZONE"); z != "" {
		s.zone = scw.Zone(z)
	}

	// Fallback: try Scaleway CLI config file (~/.config/scw/config.yaml) if still missing.
	if strings.TrimSpace(s.accessKey) == "" || strings.TrimSpace(s.secretKey) == "" {
		_ = s.loadFromScalewayConfigFile()
	}

	if strings.TrimSpace(s.accessKey) == "" || strings.TrimSpace(s.secretKey) == "" {
		return nil, fmt.Errorf("SCW_ACCESS_KEY and SCW_SECRET_KEY are required")
	}

	// Create client (project ID is optional for listing, but required for server creation).
	opts := []scw.ClientOption{
		scw.WithAuth(s.accessKey, s.secretKey),
		scw.WithDefaultZone(s.zone),
	}
	if strings.TrimSpace(s.projectID) != "" {
		opts = append(opts, scw.WithDefaultProjectID(s.projectID))
	}
	if strings.TrimSpace(s.orgID) != "" {
		opts = append(opts, scw.WithDefaultOrganizationID(s.orgID))
	}

	client, err := scw.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	s.client = client
	s.api = instance.NewAPI(client)
	return s.api, nil
}

// Scaleway CLI config file format (best-effort).
// Example the user shared:
// profiles:
//
//	newprofile:
//	  acces_key: ...
//	  secret_key: ...
//
// Real keys are usually access_key / secret_key; we accept both spellings.
type scalewayCLIConfig struct {
	Profiles       map[string]map[string]any `yaml:"profiles"`
	ActiveProfile  string                    `yaml:"active_profile"`
	DefaultProfile string                    `yaml:"default_profile"`
}

func scalewayConfigPaths() []string {
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

	// Explicit override (handy if the server runs under a different user).
	if p := os.Getenv("SCW_CONFIG_PATH"); p != "" {
		add(p)
	}

	// XDG_CONFIG_HOME takes precedence if set.
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		add(filepath.Join(xdg, "scw", "config.yaml"))
	}

	// HOME env (can differ from os.UserHomeDir in some launch contexts).
	if home := os.Getenv("HOME"); home != "" {
		add(filepath.Join(home, ".config", "scw", "config.yaml"))
	}

	// OS account home (best effort).
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		add(filepath.Join(homeDir, ".config", "scw", "config.yaml"))
	}

	// If launched under sudo, the original user's config is typically under SUDO_USER.
	if sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER")); sudoUser != "" {
		if runtime.GOOS == "darwin" {
			add(filepath.Join("/Users", sudoUser, ".config", "scw", "config.yaml"))
		} else {
			add(filepath.Join("/home", sudoUser, ".config", "scw", "config.yaml"))
		}
	}

	// Last resort: scan standard user homes to find a usable config without requiring UI paste.
	// This helps if the backend is started under a different user than the one who ran `scw init`.
	//
	// Note: this is appended last so current user/env remain preferred.
	var base string
	if runtime.GOOS == "darwin" {
		base = "/Users"
	} else {
		base = "/home"
	}
	if entries, err := os.ReadDir(base); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			add(filepath.Join(base, e.Name(), ".config", "scw", "config.yaml"))
		}
	}

	return out
}

func canLoadScalewayProfileFromConfigFile() (bool, error) {
	for _, path := range scalewayConfigPaths() {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg scalewayCLIConfig
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			continue
		}
		if len(cfg.Profiles) == 0 {
			continue
		}
		name := chooseScalewayProfileName(cfg)
		p := cfg.Profiles[name]
		if p == nil {
			continue
		}
		ak := getStringAny(p, "access_key", "acces_key")
		sk := getStringAny(p, "secret_key")
		if strings.TrimSpace(ak) != "" && strings.TrimSpace(sk) != "" {
			return true, nil
		}
	}
	return false, nil
}

func (s *Scaleway) loadFromScalewayConfigFile() error {
	var cfg scalewayCLIConfig
	loaded := false
	for _, path := range scalewayConfigPaths() {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			continue
		}
		if len(cfg.Profiles) == 0 {
			continue
		}
		loaded = true
		break
	}
	if !loaded {
		return nil
	}

	profile := os.Getenv("SCW_PROFILE")
	if strings.TrimSpace(profile) == "" {
		profile = chooseScalewayProfileName(cfg)
	}
	p := cfg.Profiles[profile]
	if p == nil {
		return nil
	}

	if strings.TrimSpace(s.accessKey) == "" {
		s.accessKey = getStringAny(p, "access_key", "acces_key")
	}
	if strings.TrimSpace(s.secretKey) == "" {
		s.secretKey = getStringAny(p, "secret_key")
	}
	if strings.TrimSpace(s.projectID) == "" {
		// Scaleway CLI sometimes stores default_project_id.
		s.projectID = getStringAny(p, "default_project_id", "project_id")
	}
	if strings.TrimSpace(s.orgID) == "" {
		s.orgID = getStringAny(p, "default_organization_id", "organization_id")
	}
	if z := getStringAny(p, "default_zone", "zone"); z != "" && strings.TrimSpace(string(s.zone)) == "" {
		s.zone = scw.Zone(z)
	}
	return nil
}

func chooseScalewayProfileName(cfg scalewayCLIConfig) string {
	if strings.TrimSpace(cfg.ActiveProfile) != "" {
		return cfg.ActiveProfile
	}
	if strings.TrimSpace(cfg.DefaultProfile) != "" {
		return cfg.DefaultProfile
	}
	// stable choice: lexicographically first key
	keys := make([]string, 0, len(cfg.Profiles))
	for k := range cfg.Profiles {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

func getStringAny(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			return t
		default:
			return fmt.Sprint(t)
		}
	}
	return ""
}

func (s *Scaleway) ListRegions() ([]Region, error) {
	api, err := s.ensureAPI()
	if err != nil {
		return nil, err
	}

	zones := api.Zones()
	out := make([]Region, 0, len(zones))
	for _, z := range zones {
		slug := string(z)
		out = append(out, Region{
			Slug: slug,
			Name: humanZoneName(slug),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

func (s *Scaleway) ListSizes() ([]Size, error) {
	return s.listSizesForZone(s.zone)
}

// ListSizesForRegion lists sizes available for a given Scaleway zone (the UI passes zones like nl-ams-3).
func (s *Scaleway) ListSizesForRegion(region string) ([]Size, error) {
	zone := scw.Zone(strings.TrimSpace(region))
	if zone == "" {
		zone = s.zone
	}
	return s.listSizesForZone(zone)
}

func (s *Scaleway) listSizesForZone(zone scw.Zone) ([]Size, error) {
	api, err := s.ensureAPI()
	if err != nil {
		return nil, err
	}

	typesResp, err := api.ListServersTypes(&instance.ListServersTypesRequest{
		Zone: zone,
	})
	if err != nil {
		return nil, err
	}

	// Availability is zone-specific; filter out "shortage".
	availResp, err := api.GetServerTypesAvailability(&instance.GetServerTypesAvailabilityRequest{
		Zone: zone,
	})
	if err != nil {
		// If this endpoint fails, still return the types list as a fallback.
		availResp = &instance.GetServerTypesAvailabilityResponse{Servers: map[string]*instance.GetServerTypesAvailabilityResponseAvailability{}}
	}

	var result []Size
	for slug, st := range typesResp.Servers {
		if st == nil {
			continue
		}
		if st.EndOfService {
			continue
		}
		// Keep only x86_64 offers for now (most self-hosted stacks assume x86_64).
		if st.Arch != instance.ArchX86_64 {
			continue
		}

		if a, ok := availResp.Servers[slug]; ok && a != nil {
			if a.Availability != instance.ServerTypesAvailabilityAvailable && a.Availability != instance.ServerTypesAvailabilityScarce {
				continue
			}
		}

		memMB := int(st.RAM / (1024 * 1024))
		diskGB := 0
		if st.VolumesConstraint != nil {
			// Min size is a scw.Size (bytes).
			diskGB = int(uint64(st.VolumesConstraint.MinSize) / (1024 * 1024 * 1024))
		}

		priceHourly := float64(st.HourlyPrice)
		priceMonthly := priceHourly * 24 * 30
		result = append(result, Size{
			Slug:         slug,
			VCPUs:        int(st.Ncpus),
			MemoryMB:     memMB,
			DiskGB:       diskGB,
			PriceHourly:  priceHourly,
			PriceMonthly: priceMonthly,
		})
	}

	// Sort stable for UX.
	sort.Slice(result, func(i, j int) bool { return result[i].PriceMonthly < result[j].PriceMonthly })
	return result, nil
}

func (s *Scaleway) GetSizeForSpecs(specs Specs) (string, error) {
	sizes, err := s.ListSizes()
	if err != nil {
		return "", err
	}

	best, ok := pickBestSizeForSpecs(sizes, specs)
	if !ok {
		return "", fmt.Errorf("no Scaleway instance type found matching specs: %d CPUs, %dMB RAM", specs.CPUs, specs.MemoryMB)
	}
	return best.Slug, nil
}

func (s *Scaleway) CreateServer(config *DeployConfig) (*Server, error) {
	api, err := s.ensureAPI()
	if err != nil {
		return nil, err
	}

	zone := scw.Zone(config.Region)
	if strings.TrimSpace(string(zone)) == "" {
		zone = s.zone
	}

	if strings.TrimSpace(s.projectID) == "" {
		return nil, fmt.Errorf("SCW_DEFAULT_PROJECT_ID (project_id) is required to create servers")
	}

	// Find image if not provided
	image := config.Image
	if strings.TrimSpace(image) == "" {
		image, err = s.findUbuntuImageLabelOrID(api, zone)
		if err != nil {
			return nil, err
		}
	}

	// Use Terraform to create the instance
	return s.createServerWithTerraform(config, string(zone), image)
}

func (s *Scaleway) WaitForServer(id string) (*Server, error) {
	// Terraform creates servers synchronously, so the server is already ready
	if s.tfServer != nil {
		return s.tfServer, nil
	}
	// Fallback: return server with the provided ID (shouldn't happen in normal flow)
	return &Server{
		ID:     id,
		Status: "active",
	}, nil
}

func (s *Scaleway) DestroyServer(id string) error {
	if s.tfWorkDir == "" {
		return fmt.Errorf("terraform work directory not found for server %s", id)
	}

	env := s.terraformEnv()
	if len(env) == 0 {
		return fmt.Errorf("Scaleway credentials not configured")
	}

	return terraform.Destroy(s.ctx, s.tfWorkDir, env)
}

func (s *Scaleway) SetupDNS(domain, ip string) error {
	return fmt.Errorf("scaleway DNS is not supported in this installer yet; please create an A record for %s -> %s at your DNS provider", domain, ip)
}

func (s *Scaleway) findUbuntuImageLabelOrID(api *instance.API, zone scw.Zone) (string, error) {
	// Prefer a public Ubuntu 24.04 image when available; fall back to 22.04.
	public := true
	perPage := uint32(100)

	resp, err := api.ListImages(&instance.ListImagesRequest{
		Zone:    zone,
		Public:  &public,
		Name:    scw.StringPtr("Ubuntu"),
		PerPage: &perPage,
	})
	if err != nil {
		return "", fmt.Errorf("scaleway: list images: %w", err)
	}

	// Try to derive image label from name, or use ID with zone prefix
	bestLabel := ""
	bestID := ""
	for _, img := range resp.Images {
		if img == nil {
			continue
		}
		name := strings.ToLower(img.Name)
		if strings.Contains(name, "24.04") || strings.Contains(name, "noble") {
			// Try to derive label from name (e.g., "Ubuntu 24.04" -> "ubuntu_noble")
			if strings.Contains(name, "noble") {
				bestLabel = "ubuntu_noble"
				break
			}
			if bestID == "" {
				// Use zone/id format for Terraform
				bestID = fmt.Sprintf("%s/%s", zone, img.ID)
			}
		}
		if bestLabel == "" && bestID == "" && (strings.Contains(name, "22.04") || strings.Contains(name, "jammy")) {
			if strings.Contains(name, "jammy") {
				bestLabel = "ubuntu_jammy"
			} else if bestID == "" {
				// Use zone/id format for Terraform
				bestID = fmt.Sprintf("%s/%s", zone, img.ID)
			}
		}
	}

	// Prefer label over ID for Terraform compatibility
	if bestLabel != "" {
		return bestLabel, nil
	}
	if bestID != "" {
		return bestID, nil
	}
	return "", fmt.Errorf("scaleway: could not find a public Ubuntu image in zone %s; set provider image explicitly", zone)
}

func humanZoneName(zone string) string {
	// zone format: <region>-<city>-<n> e.g. fr-par-1
	parts := strings.Split(zone, "-")
	if len(parts) < 3 {
		return zone
	}
	region := strings.Join(parts[:2], "-")
	num := parts[2]
	switch region {
	case "fr-par":
		return fmt.Sprintf("Paris %s", num)
	case "nl-ams":
		return fmt.Sprintf("Amsterdam %s", num)
	case "pl-waw":
		return fmt.Sprintf("Warsaw %s", num)
	default:
		return zone
	}
}

func (s *Scaleway) createServerWithTerraform(config *DeployConfig, zone, imageID string) (*Server, error) {
	// Get profile from env var (default: "basic")
	profile := strings.TrimSpace(strings.ToLower(os.Getenv("SELFHOSTED_SCW_PROFILE")))
	if profile == "" {
		profile = "basic"
	}

	moduleDir, err := terraform.FindModuleDir("scaleway", profile)
	if err != nil {
		return nil, fmt.Errorf("failed to find terraform module for scaleway/%s: %w", profile, err)
	}

	env := s.terraformEnv()
	if len(env) == 0 {
		return nil, fmt.Errorf("SCW_ACCESS_KEY and SCW_SECRET_KEY are required")
	}

	commercialType := config.Size
	if commercialType == "" {
		commercialType = "DEV1-S"
	}

	vars := map[string]interface{}{
		"name":            config.Name,
		"zone":            zone,
		"commercial_type": commercialType,
		"image_id":        imageID,
		"project_id":      s.projectID,
		"ssh_public_key":  config.SSHPublicKey,
		"tags":            config.Tags,
	}

	runID := fmt.Sprintf("%s-%d", config.Name, time.Now().Unix())
	result, err := terraform.Apply(s.ctx, moduleDir, runID, env, vars)
	if err != nil {
		return nil, fmt.Errorf("terraform apply failed: %w", err)
	}

	ip, _ := terraform.OutputString(result.Outputs, "server_ip")
	serverID, _ := terraform.OutputString(result.Outputs, "server_id")
	serverZone, _ := terraform.OutputString(result.Outputs, "server_zone")

	if ip == "" {
		return nil, fmt.Errorf("terraform apply succeeded but server_ip output is empty")
	}
	if serverID == "" {
		return nil, fmt.Errorf("terraform apply succeeded but server_id output is empty")
	}

	// Format ID as zone:server_id for consistency
	serverIDFormatted := fmt.Sprintf("%s:%s", serverZone, serverID)

	server := &Server{
		ID:     serverIDFormatted,
		Name:   config.Name,
		IP:     ip,
		Status: "active",
	}

	s.tfServer = server
	s.tfWorkDir = result.WorkDir

	return server, nil
}

func (s *Scaleway) terraformEnv() map[string]string {
	env := make(map[string]string)

	// Set credentials
	if s.accessKey != "" {
		env["SCW_ACCESS_KEY"] = s.accessKey
	}
	if s.secretKey != "" {
		env["SCW_SECRET_KEY"] = s.secretKey
	}
	if s.projectID != "" {
		env["SCW_DEFAULT_PROJECT_ID"] = s.projectID
	}
	if s.orgID != "" {
		env["SCW_DEFAULT_ORGANIZATION_ID"] = s.orgID
	}
	if string(s.zone) != "" {
		env["SCW_DEFAULT_ZONE"] = string(s.zone)
	}

	return env
}

func init() {
	Register(NewScaleway())
}

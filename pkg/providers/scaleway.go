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

	image := config.Image
	if strings.TrimSpace(image) == "" {
		image, err = s.findUbuntuImageLabelOrID(api, zone)
		if err != nil {
			return nil, err
		}
	}

	dynamic := true
	req := &instance.CreateServerRequest{
		Zone:              zone,
		Name:              config.Name,
		CommercialType:    config.Size,
		Image:             scw.StringPtr(image),
		DynamicIPRequired: &dynamic,
		Project:           scw.StringPtr(s.projectID),
		Tags:              config.Tags,
	}

	resp, err := api.CreateServer(req)
	if err != nil {
		return nil, err
	}

	server := resp.Server
	if server == nil {
		return nil, fmt.Errorf("scaleway: create server returned nil server")
	}

	// Inject SSH key via cloud-init user-data, then ensure the instance boots with it.
	if strings.TrimSpace(config.SSHPublicKey) != "" {
		cloudInit := buildCloudInitWithSSHKey(config.SSHPublicKey)
		if err := api.SetServerUserData(&instance.SetServerUserDataRequest{
			Zone:     zone,
			ServerID: server.ID,
			Key:      "cloud-init",
			Content:  strings.NewReader(cloudInit),
		}); err != nil {
			return nil, fmt.Errorf("scaleway: set cloud-init user-data: %w", err)
		}
	}

	// Ensure running (or reboot to apply cloud-init when it was already running).
	action := instance.ServerActionPoweron
	if server.State == instance.ServerStateRunning {
		action = instance.ServerActionReboot
	}
	dur := 5 * time.Minute
	_ = api.ServerActionAndWait(&instance.ServerActionAndWaitRequest{
		ServerID: server.ID,
		Zone:     zone,
		Action:   action,
		Timeout:  &dur,
	})

	return &Server{
		ID:     encodeScalewayID(zone, server.ID),
		Name:   server.Name,
		Status: server.State.String(),
	}, nil
}

func (s *Scaleway) WaitForServer(id string) (*Server, error) {
	api, err := s.ensureAPI()
	if err != nil {
		return nil, err
	}

	zone, serverID, err := decodeScalewayID(id, s.zone)
	if err != nil {
		return nil, err
	}

	timeout := time.After(10 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for Scaleway server %s", id)
		case <-ticker.C:
			resp, err := api.GetServer(&instance.GetServerRequest{
				Zone:     zone,
				ServerID: serverID,
			})
			if err != nil {
				return nil, err
			}
			if resp.Server == nil {
				continue
			}

			ip := scalewayPublicIPv4(resp.Server)
			if resp.Server.State == instance.ServerStateRunning && ip != "" {
				// Wait for SSH to be reachable
				_ = WaitForSSH(ip, 22)
				return &Server{
					ID:     id,
					Name:   resp.Server.Name,
					IP:     ip,
					Status: resp.Server.State.String(),
				}, nil
			}
		}
	}
}

func (s *Scaleway) DestroyServer(id string) error {
	api, err := s.ensureAPI()
	if err != nil {
		return err
	}
	zone, serverID, err := decodeScalewayID(id, s.zone)
	if err != nil {
		return err
	}
	return api.DeleteServer(&instance.DeleteServerRequest{
		Zone:     zone,
		ServerID: serverID,
	})
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

	best := ""
	for _, img := range resp.Images {
		if img == nil {
			continue
		}
		name := strings.ToLower(img.Name)
		if strings.Contains(name, "24.04") || strings.Contains(name, "noble") {
			best = img.ID
			break
		}
		if best == "" && (strings.Contains(name, "22.04") || strings.Contains(name, "jammy")) {
			best = img.ID
		}
	}
	if best == "" {
		return "", fmt.Errorf("scaleway: could not find a public Ubuntu image in zone %s; set provider image explicitly", zone)
	}
	return best, nil
}

func buildCloudInitWithSSHKey(pubKey string) string {
	pubKey = strings.TrimSpace(pubKey)
	// Simple cloud-init that adds the SSH key.
	return fmt.Sprintf(`#cloud-config
ssh_authorized_keys:
  - %s
`, pubKey)
}

func scalewayPublicIPv4(srv *instance.Server) string {
	if srv == nil {
		return ""
	}
	// Prefer the newer PublicIPs list.
	for _, ip := range srv.PublicIPs {
		if ip == nil {
			continue
		}
		if ip.Family == instance.ServerIPIPFamilyInet && ip.Address != nil {
			return ip.Address.String()
		}
	}
	// Fallback to deprecated PublicIP.
	if srv.PublicIP != nil && srv.PublicIP.Address != nil {
		return srv.PublicIP.Address.String()
	}
	return ""
}

func encodeScalewayID(zone scw.Zone, serverID string) string {
	return fmt.Sprintf("%s:%s", string(zone), serverID)
}

func decodeScalewayID(id string, defaultZone scw.Zone) (scw.Zone, string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", "", fmt.Errorf("empty server id")
	}
	parts := strings.SplitN(id, ":", 2)
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return scw.Zone(parts[0]), parts[1], nil
	}
	// Backward-compat: treat as raw server UUID, assume default zone.
	return defaultZone, id, nil
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

func init() {
	Register(NewScaleway())
}

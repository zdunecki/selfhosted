package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/client"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/service"
	"github.com/zdunecki/selfhosted/pkg/terraform"
	"gopkg.in/yaml.v3"
)

// UpCloud implements Provider using UpCloud's official Go SDK.
//
// Auth (recommended):
// - token (API token) via config or UPCLOUD_TOKEN env var
//
// Alternative auth:
// - username + password via config or UPCLOUD_USERNAME / UPCLOUD_PASSWORD env vars
type UpCloud struct {
	svc *service.Service
	ctx context.Context
	api *client.Client

	token    string
	username string
	password string

	// Terraform state
	tfServer  *Server
	tfWorkDir string
}

func NewUpCloud() *UpCloud {
	return &UpCloud{ctx: context.Background()}
}

func (u *UpCloud) Name() string { return "upcloud" }

func (u *UpCloud) Description() string { return "UpCloud - European cloud hosting (Go SDK)" }

func (u *UpCloud) DefaultRegion() string { return "de-fra1" }

// NeedsConfig indicates this provider typically requires user-supplied credentials.
// (It can also be configured via env vars, but the installer UI should prompt by default.)
func (u *UpCloud) NeedsConfig() bool {
	// If already configured, no config needed.
	if u.svc != nil {
		return false
	}

	// Env vars count as having credentials.
	if strings.TrimSpace(os.Getenv("UPCLOUD_TOKEN")) != "" {
		return false
	}
	if strings.TrimSpace(os.Getenv("UPCLOUD_USERNAME")) != "" && strings.TrimSpace(os.Getenv("UPCLOUD_PASSWORD")) != "" {
		return false
	}

	// If ~/.config/upctl.yaml exists and contains creds, don't force UI config.
	ok, _ := canLoadUpctlConfig()
	return !ok
}

func (u *UpCloud) Configure(config map[string]string) error {
	if v := strings.TrimSpace(config["token"]); v != "" {
		u.token = v
	}
	// Accept a few key aliases to make manual entry easier.
	if v := strings.TrimSpace(config["username"]); v != "" {
		u.username = v
	}
	if v := strings.TrimSpace(config["password"]); v != "" {
		u.password = v
	}

	// Reset cached service; ensureService will rebuild with new creds.
	u.svc = nil

	// Validate credentials early
	_, err := u.ensureService()
	return err
}

func (u *UpCloud) ensureService() (*service.Service, error) {
	if u.svc != nil {
		return u.svc, nil
	}

	// Fill from env if missing.
	if strings.TrimSpace(u.token) == "" {
		u.token = strings.TrimSpace(os.Getenv("UPCLOUD_TOKEN"))
	}
	if strings.TrimSpace(u.username) == "" {
		u.username = strings.TrimSpace(os.Getenv("UPCLOUD_USERNAME"))
	}
	if strings.TrimSpace(u.password) == "" {
		u.password = strings.TrimSpace(os.Getenv("UPCLOUD_PASSWORD"))
	}

	// Fallback: try UpCloud CLI config file (~/.config/upctl.yaml) if still missing.
	if strings.TrimSpace(u.token) == "" && (strings.TrimSpace(u.username) == "" || strings.TrimSpace(u.password) == "") {
		_ = u.loadFromUpctlConfigFile()
	}

	var authCfg client.ConfigFn
	if strings.TrimSpace(u.token) != "" {
		authCfg = client.WithBearerAuth(u.token)
	} else {
		if strings.TrimSpace(u.username) == "" || strings.TrimSpace(u.password) == "" {
			return nil, fmt.Errorf("UPCLOUD_TOKEN or UPCLOUD_USERNAME/UPCLOUD_PASSWORD required")
		}
		authCfg = client.WithBasicAuth(u.username, u.password)
	}

	clnt := client.New("", "", authCfg, client.WithTimeout(30*time.Second))
	u.api = clnt
	u.svc = service.New(clnt)

	// Quick sanity check (fail fast if creds are wrong).
	_, err := u.svc.GetAccount(u.ctx)
	if err != nil {
		u.svc = nil
		return nil, err
	}

	return u.svc, nil
}

// upctl config file format (best-effort):
// token: "ucat_..."
// username: "..."
// password: "..."
type upctlConfig struct {
	Token    string `yaml:"token"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func upctlConfigPaths() []string {
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

	// Explicit override (mirrors upctl --config).
	if p := os.Getenv("UPCTL_CONFIG"); p != "" {
		add(p)
	}

	// XDG_CONFIG_HOME takes precedence if set.
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		add(filepath.Join(xdg, "upctl.yaml"))
	}

	// HOME env (can differ from os.UserHomeDir in some launch contexts).
	if home := os.Getenv("HOME"); home != "" {
		add(filepath.Join(home, ".config", "upctl.yaml"))
	}

	// OS account home (best effort).
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		add(filepath.Join(homeDir, ".config", "upctl.yaml"))
	}

	return out
}

func canLoadUpctlConfig() (bool, error) {
	for _, path := range upctlConfigPaths() {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg upctlConfig
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			continue
		}
		if strings.TrimSpace(cfg.Token) != "" {
			return true, nil
		}
		if strings.TrimSpace(cfg.Username) != "" && strings.TrimSpace(cfg.Password) != "" {
			return true, nil
		}
	}
	return false, nil
}

func (u *UpCloud) loadFromUpctlConfigFile() error {
	for _, path := range upctlConfigPaths() {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg upctlConfig
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			continue
		}

		if strings.TrimSpace(u.token) == "" && strings.TrimSpace(cfg.Token) != "" {
			u.token = strings.TrimSpace(cfg.Token)
		}
		if strings.TrimSpace(u.username) == "" && strings.TrimSpace(cfg.Username) != "" {
			u.username = strings.TrimSpace(cfg.Username)
		}
		if strings.TrimSpace(u.password) == "" && strings.TrimSpace(cfg.Password) != "" {
			u.password = strings.TrimSpace(cfg.Password)
		}
		// Stop at the first config file we can parse.
		return nil
	}
	return nil
}

func (u *UpCloud) ListRegions() ([]Region, error) {
	svc, err := u.ensureService()
	if err != nil {
		return nil, err
	}

	zones, err := svc.GetZones(u.ctx)
	if err != nil {
		return nil, err
	}

	out := make([]Region, 0, len(zones.Zones))
	for _, z := range zones.Zones {
		// Only list public zones.
		if z.Public < upcloud.True {
			continue
		}
		out = append(out, Region{
			Slug: z.ID,
			Name: z.Description,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

func (u *UpCloud) ListSizes() ([]Size, error) {
	return u.ListSizesForRegion(u.DefaultRegion())
}

// ListSizesForRegion lists plans and approximated prices for a given UpCloud zone (e.g. de-fra1, fi-hel2).
func (u *UpCloud) ListSizesForRegion(region string) ([]Size, error) {
	svc, err := u.ensureService()
	if err != nil {
		return nil, err
	}

	plans, err := svc.GetPlans(u.ctx)
	if err != nil {
		return nil, err
	}

	var pz *upcloud.PriceZone
	if prices, perr := svc.GetPriceZones(u.ctx); perr == nil {
		for i := range prices.PriceZones {
			if strings.EqualFold(prices.PriceZones[i].Name, region) {
				pz = &prices.PriceZones[i]
				break
			}
		}
		// Fallback to any price zone if region not found (still better than 0).
		if pz == nil && len(prices.PriceZones) > 0 {
			pz = &prices.PriceZones[0]
		}
	}

	out := make([]Size, 0, len(plans.Plans))
	for _, p := range plans.Plans {
		// Skip GPU plans for now (installer assumes standard x86_64 stacks).
		if p.GPUAmount > 0 {
			continue
		}

		hourly := estimateUpCloudHourly(pz, p)
		out = append(out, Size{
			Slug:         p.Name,
			VCPUs:        p.CoreNumber,
			MemoryMB:     p.MemoryAmount,
			DiskGB:       p.StorageSize,
			PriceHourly:  hourly,
			PriceMonthly: hourly * 24 * 30,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].PriceMonthly < out[j].PriceMonthly })
	return out, nil
}

func estimateUpCloudHourly(pz *upcloud.PriceZone, p upcloud.Plan) float64 {
	if pz == nil || pz.ServerCore == nil || pz.ServerMemory == nil {
		return 0
	}

	// UpCloud /price values are expressed in "cents per hour" (e.g. 0.744 => 0.00744/h).
	// `amount` specifies the billing unit size (e.g. server_memory amount is 256MB).
	//
	// We normalize those into the UI's expected "currency units per hour".
	coreUnit := float64(pz.ServerCore.Amount)
	if coreUnit <= 0 {
		coreUnit = 1
	}
	memUnitMB := float64(pz.ServerMemory.Amount)
	if memUnitMB <= 0 {
		memUnitMB = 256
	}

	coresUnits := float64(p.CoreNumber) / coreUnit
	memUnits := float64(p.MemoryAmount) / memUnitMB

	// Convert from cents/hour -> currency/hour.
	return ((pz.ServerCore.Price * coresUnits) + (pz.ServerMemory.Price * memUnits)) / 100.0
}

func (u *UpCloud) GetSizeForSpecs(specs Specs) (string, error) {
	sizes, err := u.ListSizes()
	if err != nil {
		return "", err
	}

	best, ok := pickBestSizeForSpecs(sizes, specs)
	if !ok {
		return "", fmt.Errorf("no UpCloud plan found matching specs: %d CPUs, %dMB RAM", specs.CPUs, specs.MemoryMB)
	}
	return best.Slug, nil
}

func (u *UpCloud) CreateServer(config *DeployConfig) (*Server, error) {
	svc, err := u.ensureService()
	if err != nil {
		return nil, err
	}

	zone := strings.TrimSpace(config.Region)
	if zone == "" {
		zone = u.DefaultRegion()
	}

	// Get profile from env var (default: "basic")
	profile := strings.TrimSpace(strings.ToLower(os.Getenv("SELFHOSTED_UPCLOUD_PROFILE")))
	if profile == "" {
		profile = "basic"
	}

	moduleDir, err := terraform.FindModuleDir("upcloud", profile)
	if err != nil {
		return nil, fmt.Errorf("failed to find terraform module for upcloud/%s: %w", profile, err)
	}

	// Ensure credentials are loaded into struct before getting env
	_, err = u.ensureService()
	if err != nil {
		return nil, fmt.Errorf("failed to ensure service (credentials may be invalid): %w", err)
	}

	// Debug: Check what credentials we have loaded
	hasToken := strings.TrimSpace(u.token) != ""
	hasUsername := strings.TrimSpace(u.username) != ""
	hasPassword := strings.TrimSpace(u.password) != ""

	env := u.terraformEnv()
	if len(env) == 0 {
		return nil, fmt.Errorf("UPCLOUD_TOKEN or UPCLOUD_USERNAME/UPCLOUD_PASSWORD are required")
	}

	// Debug: Log what we're passing to Terraform (without exposing sensitive values)
	envToken, hasEnvToken := env["UPCLOUD_TOKEN"]
	envUsername, hasEnvUsername := env["UPCLOUD_USERNAME"]
	envPassword, hasEnvPassword := env["UPCLOUD_PASSWORD"]

	// Log debug info (mask sensitive values)
	tokenPreview := ""
	if hasEnvToken && envToken != "" {
		if len(envToken) > 8 {
			tokenPreview = envToken[:4] + "..." + envToken[len(envToken)-4:]
		} else {
			tokenPreview = "***"
		}
	}

	// This will help us see what's being passed
	fmt.Fprintf(os.Stderr, "[DEBUG] UpCloud terraform env - hasToken: %v, tokenPreview: %q, hasUsername: %v, hasPassword: %v, usernameLen: %d, passwordLen: %d\n",
		hasEnvToken && envToken != "", tokenPreview, hasEnvUsername && envUsername != "", hasEnvPassword && envPassword != "",
		len(envUsername), len(envPassword))

	// If we have a token, make absolutely sure username/password are not set
	// The UpCloud provider may check for these variables and try to use them even if token is set
	if hasEnvToken && strings.TrimSpace(envToken) != "" {
		// Remove username/password from env map entirely (don't set to empty, just don't include them)
		// This prevents the provider from seeing them at all
		delete(env, "UPCLOUD_USERNAME")
		delete(env, "UPCLOUD_PASSWORD")
		fmt.Fprintf(os.Stderr, "[DEBUG] UpCloud: Using token authentication, removed username/password from env map\n")
	} else {
		fmt.Fprintf(os.Stderr, "[DEBUG] UpCloud: No token found! hasToken in struct: %v, hasUsername: %v, hasPassword: %v\n", hasToken, hasUsername, hasPassword)
	}

	// Preflight: validate zone exists (helps avoid generic NOT_FOUND).
	if err := u.validateZone(svc, zone); err != nil {
		return nil, err
	}

	// Preflight: validate plan exists (helps avoid generic NOT_FOUND).
	if config.Size != "" {
		if err := u.validatePlan(svc, config.Size); err != nil {
			return nil, err
		}
	}

	// Find Ubuntu template name - template block accepts names
	templateName, err := u.findUbuntuTemplateName(svc, zone)
	if err != nil {
		return nil, err
	}

	diskGB := 25
	tier := "maxiops" // Default to maxiops (SSD)
	if config.Size != "" {
		if plan, ok := u.findPlanByName(svc, config.Size); ok {
			if plan.StorageSize > 0 {
				diskGB = plan.StorageSize
			}
			if strings.TrimSpace(plan.StorageTier) != "" {
				tier = strings.ToLower(plan.StorageTier)
			}
		}
	}

	hostname := sanitizeHostname(config.Name)
	if hostname == "" {
		hostname = "selfhosted"
	}

	plan := config.Size
	if plan == "" {
		plan = "1xCPU-1GB" // Default plan
	}

	vars := map[string]interface{}{
		"name":           config.Name,
		"hostname":       hostname,
		"zone":           zone,
		"plan":           plan,
		"template_name":  templateName,
		"disk_size_gb":   diskGB,
		"disk_tier":      tier,
		"ssh_public_key": config.SSHPublicKey,
		"tags":           config.Tags,
	}

	runID := fmt.Sprintf("%s-%d", config.Name, time.Now().Unix())
	result, err := terraform.Apply(u.ctx, moduleDir, runID, env, vars)
	if err != nil {
		return nil, fmt.Errorf("terraform apply failed: %w", err)
	}

	ip, _ := terraform.OutputString(result.Outputs, "server_ip")
	serverID, _ := terraform.OutputString(result.Outputs, "server_id")
	serverName, _ := terraform.OutputString(result.Outputs, "server_name")

	if ip == "" {
		return nil, fmt.Errorf("terraform apply succeeded but server_ip output is empty")
	}
	if serverID == "" {
		return nil, fmt.Errorf("terraform apply succeeded but server_id output is empty")
	}

	server := &Server{
		ID:     serverID,
		Name:   serverName,
		IP:     ip,
		Status: "active",
	}

	u.tfServer = server
	u.tfWorkDir = result.WorkDir

	return server, nil
}

func (u *UpCloud) validateZone(svc *service.Service, zone string) error {
	z, err := svc.GetZones(u.ctx)
	if err != nil {
		return err
	}
	for _, x := range z.Zones {
		if strings.EqualFold(strings.TrimSpace(x.ID), strings.TrimSpace(zone)) {
			if x.Public < upcloud.True {
				return fmt.Errorf("upcloud: zone %s is not public", zone)
			}
			return nil
		}
	}
	return fmt.Errorf("upcloud: unknown zone %s", zone)
}

func (u *UpCloud) validatePlan(svc *service.Service, plan string) error {
	plan = strings.TrimSpace(plan)
	if plan == "" {
		return nil
	}
	p, err := svc.GetPlans(u.ctx)
	if err != nil {
		return err
	}
	for _, x := range p.Plans {
		if strings.EqualFold(strings.TrimSpace(x.Name), plan) {
			return nil
		}
	}

	// Provide a small hint list (first 10) so user can see what's valid.
	names := make([]string, 0, len(p.Plans))
	for _, x := range p.Plans {
		if strings.TrimSpace(x.Name) == "" {
			continue
		}
		names = append(names, x.Name)
	}
	sort.Strings(names)
	if len(names) > 10 {
		names = names[:10]
	}
	return fmt.Errorf("upcloud: unknown plan %q (examples: %s)", plan, strings.Join(names, ", "))
}

func formatUpcloudError(err error) string {
	if err == nil {
		return ""
	}
	var prob *upcloud.Problem
	if errors.As(err, &prob) && prob != nil {
		typ := strings.TrimSpace(prob.ErrorCode())
		if typ == "" {
			typ = strings.TrimSpace(prob.Type)
		}
		if strings.TrimSpace(prob.CorrelationID) != "" {
			return fmt.Sprintf("%s (type=%s, status=%d, correlation_id=%s)", prob.Title, typ, prob.Status, prob.CorrelationID)
		}
		return fmt.Sprintf("%s (type=%s, status=%d)", prob.Title, typ, prob.Status)
	}
	return err.Error()
}

func (u *UpCloud) WaitForServer(id string) (*Server, error) {
	// Terraform creates servers synchronously, so the server is already ready
	if u.tfServer != nil {
		// Wait for SSH to be available
		if u.tfServer.IP != "" {
			_ = WaitForSSH(u.tfServer.IP, 22)
		}
		return u.tfServer, nil
	}
	// Fallback: return server with the provided ID (shouldn't happen in normal flow)
	return &Server{
		ID:     id,
		Status: "active",
	}, nil
}

func (u *UpCloud) DestroyServer(id string) error {
	if u.tfWorkDir == "" {
		return fmt.Errorf("terraform work directory not found for server %s", id)
	}

	env := u.terraformEnv()
	if len(env) == 0 {
		return fmt.Errorf("UpCloud credentials not configured")
	}

	return terraform.Destroy(u.ctx, u.tfWorkDir, env)
}

func (u *UpCloud) SetupDNS(domain, ip string) error {
	return fmt.Errorf("upcloud DNS is not supported in this installer yet; please create an A record for %s -> %s at your DNS provider", domain, ip)
}

func (u *UpCloud) findUbuntuTemplateUUID(svc *service.Service, zone string) (string, error) {
	storages, err := u.getTemplateStoragesForZone(zone)
	if err != nil {
		return "", err
	}

	type cand struct {
		uuid     string
		priority int
		title    string
		zone     string
	}
	var bestExact *cand
	var bestAny *cand

	want := []struct {
		substr   string
		priority int
	}{
		{"ubuntu 24.04", 100},
		{"ubuntu 22.04", 90},
		{"ubuntu 20.04", 80},
		{"ubuntu", 10},
	}

	for _, s := range storages.Storages {
		z := strings.TrimSpace(s.Zone)
		isExactZone := z != "" && strings.EqualFold(z, zone)
		if s.Type != upcloud.StorageTypeTemplate {
			continue
		}
		if s.State != upcloud.StorageStateOnline {
			continue
		}
		// Only use public templates - private templates may not be accessible
		// Check Access field - if it's not Public, skip it
		if s.Access != upcloud.StorageAccessPublic {
			fmt.Fprintf(os.Stderr, "[DEBUG] UpCloud: Skipping template %s (title: %q, access: %v, not public)\n", s.UUID, s.Title, s.Access)
			continue
		}

		title := strings.ToLower(strings.TrimSpace(s.Title))
		if title == "" {
			continue
		}

		p := 0
		for _, w := range want {
			if strings.Contains(title, w.substr) {
				p = w.priority
				break
			}
		}
		if p == 0 {
			continue
		}
		// Prefer cloud-init templates when available.
		if strings.EqualFold(strings.TrimSpace(s.TemplateType), upcloud.StorageTemplateTypeCloudInit) {
			p += 5
		}

		fmt.Fprintf(os.Stderr, "[DEBUG] UpCloud: Found candidate template %s (title: %q, zone: %q, access: %v, priority: %d)\n", s.UUID, s.Title, s.Zone, s.Access, p)

		c := cand{uuid: s.UUID, priority: p, title: s.Title, zone: s.Zone}
		if isExactZone {
			if bestExact == nil || c.priority > bestExact.priority {
				bestExact = &c
			}
			continue
		}
		// Many accounts/APIs return template storages without a zone or with a different zone.
		// Those templates are still cloneable into the desired zone, so keep them as a fallback.
		if bestAny == nil || c.priority > bestAny.priority {
			bestAny = &c
		}
	}

	best := bestExact
	if best == nil {
		best = bestAny
	}
	if best == nil || strings.TrimSpace(best.uuid) == "" {
		// Helpful hint: show a few templates we did see in that zone (any OS).
		hints := u.zoneTemplateHints(storages, zone, 10)
		if hints != "" {
			return "", fmt.Errorf("upcloud: could not find a public Ubuntu template in zone %s (templates in zone: %s)", zone, hints)
		}
		anyHints := u.anyTemplateHints(storages, 10)
		if anyHints != "" {
			return "", fmt.Errorf("upcloud: could not find any public Ubuntu templates (examples: %s)", anyHints)
		}
		return "", fmt.Errorf("upcloud: could not find any public templates")
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] UpCloud: Selected template UUID: %s (title: %q, zone: %q)\n", best.uuid, best.title, best.zone)
	return best.uuid, nil
}

func (u *UpCloud) findUbuntuTemplateName(svc *service.Service, zone string) (string, error) {
	storages, err := u.getTemplateStoragesForZone(zone)
	if err != nil {
		return "", err
	}

	type cand struct {
		name     string
		priority int
		title    string
		zone     string
	}
	var bestExact *cand
	var bestAny *cand

	want := []struct {
		substr   string
		priority int
	}{
		{"ubuntu 24.04", 100},
		{"ubuntu 22.04", 90},
		{"ubuntu 20.04", 80},
		{"ubuntu", 10},
	}

	for _, s := range storages.Storages {
		z := strings.TrimSpace(s.Zone)
		isExactZone := z != "" && strings.EqualFold(z, zone)
		if s.Type != upcloud.StorageTypeTemplate {
			continue
		}
		if s.State != upcloud.StorageStateOnline {
			continue
		}
		// Only use public templates - private templates may not be accessible
		if s.Access != upcloud.StorageAccessPublic {
			fmt.Fprintf(os.Stderr, "[DEBUG] UpCloud: Skipping template %s (title: %q, access: %v, not public)\n", s.UUID, s.Title, s.Access)
			continue
		}

		title := strings.TrimSpace(s.Title)
		if title == "" {
			continue
		}
		titleLower := strings.ToLower(title)

		p := 0
		for _, w := range want {
			if strings.Contains(titleLower, w.substr) {
				p = w.priority
				break
			}
		}
		if p == 0 {
			continue
		}
		// Prefer cloud-init templates when available.
		if strings.EqualFold(strings.TrimSpace(s.TemplateType), upcloud.StorageTemplateTypeCloudInit) {
			p += 5
		}

		fmt.Fprintf(os.Stderr, "[DEBUG] UpCloud: Found candidate template %s (title: %q, zone: %q, access: %v, priority: %d)\n", s.UUID, title, s.Zone, s.Access, p)

		c := cand{name: title, priority: p, title: title, zone: s.Zone}
		if isExactZone {
			if bestExact == nil || c.priority > bestExact.priority {
				bestExact = &c
			}
			continue
		}
		// Many accounts/APIs return template storages without a zone or with a different zone.
		// Those templates are still cloneable into the desired zone, so keep them as a fallback.
		if bestAny == nil || c.priority > bestAny.priority {
			bestAny = &c
		}
	}

	best := bestExact
	if best == nil {
		best = bestAny
	}
	if best == nil || strings.TrimSpace(best.name) == "" {
		// Helpful hint: show a few templates we did see in that zone (any OS).
		hints := u.zoneTemplateHints(storages, zone, 10)
		if hints != "" {
			return "", fmt.Errorf("upcloud: could not find a public Ubuntu template in zone %s (templates in zone: %s)", zone, hints)
		}
		anyHints := u.anyTemplateHints(storages, 10)
		if anyHints != "" {
			return "", fmt.Errorf("upcloud: could not find any public Ubuntu templates (examples: %s)", anyHints)
		}
		return "", fmt.Errorf("upcloud: could not find any public templates")
	}

	fmt.Fprintf(os.Stderr, "[DEBUG] UpCloud: Selected template name: %q (zone: %q)\n", best.name, best.zone)
	return best.name, nil
}

// getTemplateStoragesForZone lists template storages. UpCloud API endpoints for public templates vary across accounts,
// so we try a few known patterns. This avoids the generic 404 the user is seeing.
func (u *UpCloud) getTemplateStoragesForZone(zone string) (*upcloud.Storages, error) {
	// Ensure client is initialized.
	if _, err := u.ensureService(); err != nil {
		return nil, err
	}

	// Always request public templates first - private templates may not be accessible
	// Some accounts/APIs return 404 for /storage/public/template, so we try alternatives
	tryReqs := []*request.GetStoragesRequest{
		// Most intuitive, but may 404 depending on API behavior.
		{Access: upcloud.StorageAccessPublic, Type: upcloud.StorageTypeTemplate},
		// Alternative: omit access and just ask for templates (some APIs accept /storage/template).
		// We'll filter for public templates in findUbuntuTemplateUUID
		{Type: upcloud.StorageTypeTemplate},
	}
	for _, r := range tryReqs {
		st, err := u.svc.GetStorages(u.ctx, r)
		if err == nil {
			// If we got results, prefer the public-only request
			if r.Access == upcloud.StorageAccessPublic {
				return st, nil
			}
			// Otherwise, we'll filter for public templates later
			return st, nil
		}
		if isUpcloudNotFound(err) {
			continue
		}
		return nil, err
	}

	// Last resort: hit raw paths via the client (service doesn't expose arbitrary endpoints).
	paths := []string{
		"/storage/template",
		"/storage/public/template",
		"/storage/public",
	}
	for _, p := range paths {
		b, err := u.api.Get(u.ctx, p)
		if err != nil {
			if isUpcloudNotFound(err) {
				continue
			}
			return nil, err
		}
		var st upcloud.Storages
		if err := json.Unmarshal(b, &st); err != nil {
			continue
		}
		return &st, nil
	}

	return nil, fmt.Errorf("upcloud: could not list template storages (zone=%s): API returned 404 for template listing endpoints", zone)
}

func isUpcloudNotFound(err error) bool {
	var prob *upcloud.Problem
	if errors.As(err, &prob) && prob != nil {
		if prob.Status == 404 || strings.EqualFold(prob.ErrorCode(), "NOT_FOUND") || strings.EqualFold(prob.Type, "NOT_FOUND") {
			return true
		}
	}
	return false
}

func (u *UpCloud) zoneTemplateHints(storages *upcloud.Storages, zone string, limit int) string {
	if storages == nil {
		return ""
	}
	type item struct {
		title string
		size  int
		tier  string
		tt    string
	}
	var items []item
	for _, s := range storages.Storages {
		// Prefer showing templates that match the requested zone, but also include zone-less templates
		// since those are commonly returned for public templates.
		sz := strings.TrimSpace(s.Zone)
		if !(strings.EqualFold(sz, strings.TrimSpace(zone)) || sz == "") {
			continue
		}
		title := strings.TrimSpace(s.Title)
		if title == "" {
			continue
		}
		items = append(items, item{
			title: title,
			size:  s.Size,
			tier:  strings.TrimSpace(s.Tier),
			tt:    strings.TrimSpace(s.TemplateType),
		})
	}
	if len(items) == 0 {
		return ""
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].title) < strings.ToLower(items[j].title) })
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		meta := ""
		if it.size > 0 || it.tier != "" || it.tt != "" {
			parts := []string{}
			if it.size > 0 {
				parts = append(parts, strconv.Itoa(it.size)+"GB")
			}
			if it.tier != "" {
				parts = append(parts, it.tier)
			}
			if it.tt != "" {
				parts = append(parts, it.tt)
			}
			meta = " [" + strings.Join(parts, " ") + "]"
		}
		out = append(out, it.title+meta)
	}
	return strings.Join(out, " | ")
}

func (u *UpCloud) anyTemplateHints(storages *upcloud.Storages, limit int) string {
	if storages == nil {
		return ""
	}
	type item struct {
		title string
		zone  string
	}
	var items []item
	for _, s := range storages.Storages {
		if s.Type != upcloud.StorageTypeTemplate {
			continue
		}
		title := strings.TrimSpace(s.Title)
		if title == "" {
			continue
		}
		items = append(items, item{title: title, zone: strings.TrimSpace(s.Zone)})
	}
	if len(items) == 0 {
		return ""
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].title) < strings.ToLower(items[j].title) })
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if it.zone != "" {
			out = append(out, fmt.Sprintf("%s (%s)", it.title, it.zone))
		} else {
			out = append(out, it.title)
		}
	}
	return strings.Join(out, " | ")
}

func (u *UpCloud) findPlanByName(svc *service.Service, name string) (upcloud.Plan, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return upcloud.Plan{}, false
	}
	plans, err := svc.GetPlans(u.ctx)
	if err != nil {
		return upcloud.Plan{}, false
	}
	for _, p := range plans.Plans {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return upcloud.Plan{}, false
}

func (u *UpCloud) terraformEnv() map[string]string {
	env := make(map[string]string)

	// Prefer token over username/password
	// Check struct fields first (populated by ensureService/Configure, including from ~/.config/upctl.yaml)
	token := strings.TrimSpace(u.token)
	username := strings.TrimSpace(u.username)
	password := strings.TrimSpace(u.password)

	// Always set token if available, and explicitly unset username/password
	// The UpCloud provider may check for username/password first, so we need to ensure they're not set
	if token != "" {
		env["UPCLOUD_TOKEN"] = token
		// Critical: Set username/password to empty to prevent provider from trying username/password auth
		env["UPCLOUD_USERNAME"] = ""
		env["UPCLOUD_PASSWORD"] = ""
		return env
	}

	// Only use username/password if token is not available
	if username != "" && password != "" {
		env["UPCLOUD_USERNAME"] = username
		env["UPCLOUD_PASSWORD"] = password
		env["UPCLOUD_TOKEN"] = ""
		return env
	}

	// Fallback to environment variables if struct fields are empty
	if envToken := strings.TrimSpace(os.Getenv("UPCLOUD_TOKEN")); envToken != "" {
		env["UPCLOUD_TOKEN"] = envToken
		env["UPCLOUD_USERNAME"] = ""
		env["UPCLOUD_PASSWORD"] = ""
		return env
	}

	if envUsername := strings.TrimSpace(os.Getenv("UPCLOUD_USERNAME")); envUsername != "" {
		if envPassword := strings.TrimSpace(os.Getenv("UPCLOUD_PASSWORD")); envPassword != "" {
			env["UPCLOUD_USERNAME"] = envUsername
			env["UPCLOUD_PASSWORD"] = envPassword
			env["UPCLOUD_TOKEN"] = ""
			return env
		}
	}

	return env
}

func init() {
	Register(NewUpCloud())
}

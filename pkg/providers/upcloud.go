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

	templateUUID, err := u.findUbuntuTemplateUUID(svc, zone)
	if err != nil {
		return nil, err
	}

	diskGB := 25
	tier := upcloud.StorageTierStandard
	if config.Size != "" {
		if plan, ok := u.findPlanByName(svc, config.Size); ok {
			if plan.StorageSize > 0 {
				diskGB = plan.StorageSize
			}
			if strings.TrimSpace(plan.StorageTier) != "" {
				tier = plan.StorageTier
			}
		}
	}

	hostname := sanitizeHostname(config.Name)
	if hostname == "" {
		hostname = "selfhosted"
	}

	req := &request.CreateServerRequest{
		Zone:                zone,
		Title:               config.Name,
		Hostname:            hostname,
		Plan:                config.Size,
		PasswordDelivery:    request.PasswordDeliveryNone,
		Metadata:            upcloud.True,
		NICModel:            upcloud.NICModelVirtio,
		RemoteAccessEnabled: upcloud.False,
		LoginUser: &request.LoginUser{
			Username:       "root",
			CreatePassword: "no",
			SSHKeys:        []string{strings.TrimSpace(config.SSHPublicKey)},
		},
		Networking: &request.CreateServerNetworking{
			Interfaces: []request.CreateServerInterface{
				{
					IPAddresses: []request.CreateServerIPAddress{
						{Family: upcloud.IPAddressFamilyIPv4},
					},
					// Include utility NIC explicitly (matches UpCloud SDK integration tests / provisioning expectations).
					Type: upcloud.NetworkTypeUtility,
				},
				{
					IPAddresses: []request.CreateServerIPAddress{
						{Family: upcloud.IPAddressFamilyIPv4},
					},
					Type: upcloud.NetworkTypePublic,
				},
			},
		},
		StorageDevices: []request.CreateServerStorageDevice{
			{
				Action:  request.CreateServerStorageDeviceActionClone,
				Storage: templateUUID,
				Title:   "disk1",
				Size:    diskGB,
				Tier:    tier,
			},
		},
	}

	// Best-effort labels/tags.
	if len(config.Tags) > 0 {
		labels := make([]upcloud.Label, 0, len(config.Tags))
		for _, t := range config.Tags {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			labels = append(labels, upcloud.Label{Key: t, Value: "true"})
		}
		if len(labels) > 0 {
			req.Labels = (*upcloud.LabelSlice)(&labels)
		}
	}

	details, err := svc.CreateServer(u.ctx, req)
	if err != nil {
		// Add context: NOT_FOUND is most commonly a bad plan name, bad template UUID, or wrong zone.
		return nil, fmt.Errorf("upcloud create server failed (zone=%s plan=%s template=%s): %s", zone, config.Size, templateUUID, formatUpcloudError(err))
	}

	return &Server{
		ID:     details.UUID,
		Name:   details.Title,
		Status: details.State,
	}, nil
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
	svc, err := u.ensureService()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(u.ctx, 10*time.Minute)
	defer cancel()

	details, err := svc.WaitForServerState(ctx, &request.WaitForServerStateRequest{
		UUID:         strings.TrimSpace(id),
		DesiredState: upcloud.ServerStateStarted,
	})
	if err != nil {
		return nil, err
	}

	ip := upcloudPublicIPv4(details)
	if ip == "" {
		// Fallback to fresh details once more (IP assignment can lag behind state).
		d2, derr := svc.GetServerDetails(u.ctx, &request.GetServerDetailsRequest{UUID: strings.TrimSpace(id)})
		if derr == nil {
			ip = upcloudPublicIPv4(d2)
		}
	}
	if ip == "" {
		return nil, fmt.Errorf("upcloud: server started but no public IPv4 address found")
	}

	_ = WaitForSSH(ip, 22)
	return &Server{
		ID:     id,
		Name:   details.Title,
		IP:     ip,
		Status: details.State,
	}, nil
}

func (u *UpCloud) DestroyServer(id string) error {
	svc, err := u.ensureService()
	if err != nil {
		return err
	}

	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("upcloud: empty server id")
	}

	// UpCloud can reject operations while server is in "maintenance" (during deploy).
	// Wait for it to leave maintenance before attempting stop/delete.
	ctx, cancel := context.WithTimeout(u.ctx, 12*time.Minute)
	defer cancel()

	details, err := svc.GetServerDetails(ctx, &request.GetServerDetailsRequest{UUID: id})
	if err != nil {
		return err
	}

	if strings.EqualFold(details.State, upcloud.ServerStateMaintenance) {
		_, _ = svc.WaitForServerState(ctx, &request.WaitForServerStateRequest{
			UUID:           id,
			UndesiredState: upcloud.ServerStateMaintenance,
		})
		// Refresh details (best-effort).
		if d2, derr := svc.GetServerDetails(ctx, &request.GetServerDetailsRequest{UUID: id}); derr == nil {
			details = d2
		}
	}

	// If still running, stop first (delete often requires stopped).
	if details != nil && !strings.EqualFold(details.State, upcloud.ServerStateStopped) {
		_, _ = svc.StopServer(ctx, &request.StopServerRequest{
			UUID:     id,
			StopType: request.ServerStopTypeHard,
			Timeout:  5 * time.Minute,
		})
		_, _ = svc.WaitForServerState(ctx, &request.WaitForServerStateRequest{
			UUID:         id,
			DesiredState: upcloud.ServerStateStopped,
		})
	}

	return svc.DeleteServerAndStorages(ctx, &request.DeleteServerAndStoragesRequest{UUID: id})
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
	return best.uuid, nil
}

// getTemplateStoragesForZone lists template storages. UpCloud API endpoints for public templates vary across accounts,
// so we try a few known patterns. This avoids the generic 404 the user is seeing.
func (u *UpCloud) getTemplateStoragesForZone(zone string) (*upcloud.Storages, error) {
	// Ensure client is initialized.
	if _, err := u.ensureService(); err != nil {
		return nil, err
	}

	// First try the SDK's canonical request URL. Some accounts/APIs return 404 for /storage/public/template.
	tryReqs := []*request.GetStoragesRequest{
		// Most intuitive, but may 404 depending on API behavior.
		{Access: upcloud.StorageAccessPublic, Type: upcloud.StorageTypeTemplate},
		// Alternative: omit access and just ask for templates (some APIs accept /storage/template).
		{Type: upcloud.StorageTypeTemplate},
	}
	for _, r := range tryReqs {
		st, err := u.svc.GetStorages(u.ctx, r)
		if err == nil {
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

func upcloudPublicIPv4(details *upcloud.ServerDetails) string {
	if details == nil {
		return ""
	}
	for _, ip := range details.IPAddresses {
		if ip.Access == upcloud.IPAddressAccessPublic && ip.Family == upcloud.IPAddressFamilyIPv4 && strings.TrimSpace(ip.Address) != "" {
			return strings.TrimSpace(ip.Address)
		}
	}
	return ""
}

func init() {
	Register(NewUpCloud())
}

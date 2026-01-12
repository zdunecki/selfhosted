package providers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	billing "cloud.google.com/go/billing/apiv1"
	compute "cloud.google.com/go/compute/apiv1"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	serviceusage "cloud.google.com/go/serviceusage/apiv1"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	serviceusagepb "google.golang.org/genproto/googleapis/api/serviceusage/v1"
	billingpb "google.golang.org/genproto/googleapis/cloud/billing/v1"
	computepb "google.golang.org/genproto/googleapis/cloud/compute/v1"
	resmgrpb "google.golang.org/genproto/googleapis/cloud/resourcemanager/v3"
)

// GCP implements Provider for Google Cloud Platform.
//
// Auth:
// - Prefer Application Default Credentials (ADC) on the server (gcloud / workload identity)
// - Or paste Service Account JSON in the UI (stored in-memory for this process)
//
// Project handling:
// - By default CreateServer will create a new project (if no project_id is provided)
// - If a billing account is available, it will link billing to the new project
type GCP struct {
	ctx context.Context

	ts oauth2.TokenSource
	am GCPAuthMethod

	// optional config
	credentialsJSON string
	projectID       string
	parent          string // e.g. "organizations/123" or "folders/456" (optional)
	billingAccount  string // e.g. "billingAccounts/0123-4567-89AB" (optional)
	createProject   bool
}

func NewGCP() *GCP {
	return &GCP{
		ctx:           context.Background(),
		createProject: true,
	}
}

func (g *GCP) Name() string { return "gcp" }

func (g *GCP) Description() string { return "Google Cloud Platform - Enterprise cloud hosting (GCP)" }

func (g *GCP) DefaultRegion() string { return "europe-west1" }

func (g *GCP) NeedsConfig() bool {
	// If already configured, no config needed.
	if g.ts != nil {
		return false
	}
	// If we can resolve any token source (ADC or gcloud), no config needed.
	if _, _, err := ResolveGCPTokenSource(g.ctx, ""); err == nil {
		return false
	}
	return true
}

func (g *GCP) Configure(config map[string]string) error {
	// Accept raw SA JSON via UI.
	if v := strings.TrimSpace(config["credentials_json"]); v != "" {
		g.credentialsJSON = v
	}
	if v := strings.TrimSpace(config["service_account_json"]); v != "" {
		g.credentialsJSON = v
	}
	// Optional fields.
	if v := strings.TrimSpace(config["project_id"]); v != "" {
		g.projectID = v
	}
	if v := strings.TrimSpace(config["parent"]); v != "" {
		g.parent = v
	}
	if v := strings.TrimSpace(config["billing_account"]); v != "" {
		g.billingAccount = v
	}
	if v := strings.TrimSpace(config["create_project"]); v != "" {
		g.createProject = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
	}

	// Reset cached creds.
	g.ts = nil
	g.am = ""

	_, err := g.ensureTokenSource()
	return err
}

func (g *GCP) ensureTokenSource() (oauth2.TokenSource, error) {
	if g.ts != nil {
		return g.ts, nil
	}

	ts, method, err := ResolveGCPTokenSource(g.ctx, g.credentialsJSON)
	if err != nil {
		return nil, err
	}
	g.ts = ts
	g.am = method
	return g.ts, nil
}

// ResolveAuth returns the TokenSource and the chosen auth method for this provider instance.
// This is useful for debugging/diagnostics and for small utilities.
func (g *GCP) ResolveAuth() (oauth2.TokenSource, GCPAuthMethod, error) {
	if g.ts != nil {
		return g.ts, g.am, nil
	}
	ts, method, err := ResolveGCPTokenSource(g.ctx, g.credentialsJSON)
	if err != nil {
		return nil, "", err
	}
	g.ts = ts
	g.am = method
	return g.ts, g.am, nil
}

// AuthMethod returns the currently selected auth method (if already resolved).
func (g *GCP) AuthMethod() GCPAuthMethod { return g.am }

// ListProjects lists projects visible to the current credentials for this provider instance.
func (g *GCP) ListProjects() ([]GCPProject, error) {
	ts, _, err := g.ResolveAuth()
	if err != nil {
		return nil, err
	}
	return ListGCPProjects(g.ctx, ts)
}

func canUseGCPADC(ctx context.Context) (bool, error) {
	// Use the real Google auth resolver so "has creds" matches what the SDK can actually use.
	// Note: `gcloud auth login` alone is NOT ADC; users typically need:
	//   gcloud auth application-default login
	scopes := []string{"https://www.googleapis.com/auth/cloud-platform"}
	_, err := google.FindDefaultCredentials(ctx, scopes...)
	if err != nil {
		return false, nil
	}
	return true, nil
}

type GCPAuthMethod string

const (
	GCPAuthServiceAccountJSON GCPAuthMethod = "service_account_json"
	GCPAuthADC               GCPAuthMethod = "adc"
	GCPAuthGcloudToken       GCPAuthMethod = "gcloud_access_token"
)

// ResolveGCPTokenSource returns a TokenSource that can be used with GCP clients.
//
// Resolution order:
// - service account JSON (if provided)
// - ADC
// - gcloud user access token (gcloud auth print-access-token)
func ResolveGCPTokenSource(ctx context.Context, credentialsJSON string) (oauth2.TokenSource, GCPAuthMethod, error) {
	scopes := []string{"https://www.googleapis.com/auth/cloud-platform"}

	if strings.TrimSpace(credentialsJSON) != "" {
		creds, err := google.CredentialsFromJSON(ctx, []byte(credentialsJSON), scopes...)
		if err != nil {
			return nil, "", fmt.Errorf("gcp: invalid credentials_json: %w", err)
		}
		if creds.TokenSource == nil {
			return nil, "", fmt.Errorf("gcp: credentials_json produced nil token source")
		}
		return creds.TokenSource, GCPAuthServiceAccountJSON, nil
	}

	if creds, err := google.FindDefaultCredentials(ctx, scopes...); err == nil && creds != nil && creds.TokenSource != nil {
		return creds.TokenSource, GCPAuthADC, nil
	}

	// Last resort: use `gcloud auth print-access-token` if available.
	// This matches what the user sees via `gcloud projects list`.
	ts := oauth2.ReuseTokenSource(nil, &gcloudTokenSource{})
	if _, terr := ts.Token(); terr == nil {
		return ts, GCPAuthGcloudToken, nil
	}

	return nil, "", fmt.Errorf("gcp: no usable credentials found. Either run `gcloud auth application-default login`, or provide service account JSON")
}

type GCPProject struct {
	ProjectID   string
	DisplayName string
	State       string
}

func ListGCPProjects(ctx context.Context, ts oauth2.TokenSource) ([]GCPProject, error) {
	if ts == nil {
		return nil, fmt.Errorf("gcp: nil token source")
	}
	cli, err := resourcemanager.NewProjectsClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	it := cli.SearchProjects(ctx, &resmgrpb.SearchProjectsRequest{})
	var out []GCPProject
	for {
		p, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		if p == nil {
			continue
		}
		out = append(out, GCPProject{
			ProjectID:   p.ProjectId,
			DisplayName: p.DisplayName,
			State:       p.State.String(),
		})
	}
	return out, nil
}

func (g *GCP) ListRegions() ([]Region, error) {
	// Region listing doesn't require a project in the wizard; keep a static curated list.
	// (Compute API region listing requires a project.)
	regions := []Region{
		{Slug: "us-central1", Name: "Iowa (us-central1)"},
		{Slug: "us-east1", Name: "South Carolina (us-east1)"},
		{Slug: "us-west1", Name: "Oregon (us-west1)"},
		{Slug: "europe-west1", Name: "Belgium (europe-west1)"},
		{Slug: "europe-west2", Name: "London (europe-west2)"},
		{Slug: "europe-west3", Name: "Frankfurt (europe-west3)"},
		{Slug: "europe-west4", Name: "Netherlands (europe-west4)"},
		{Slug: "europe-central2", Name: "Warsaw (europe-central2)"},
		{Slug: "asia-southeast1", Name: "Singapore (asia-southeast1)"},
		{Slug: "asia-northeast1", Name: "Tokyo (asia-northeast1)"},
	}
	sort.Slice(regions, func(i, j int) bool { return regions[i].Slug < regions[j].Slug })
	return regions, nil
}

func (g *GCP) ListSizes() ([]Size, error) {
	// A small, safe subset (costs vary per region; we keep price 0 for now).
	return []Size{
		{Slug: "e2-medium", VCPUs: 2, MemoryMB: 4096, DiskGB: 10},
		{Slug: "e2-standard-2", VCPUs: 2, MemoryMB: 8192, DiskGB: 10},
		{Slug: "e2-standard-4", VCPUs: 4, MemoryMB: 16384, DiskGB: 10},
		{Slug: "n2-standard-2", VCPUs: 2, MemoryMB: 8192, DiskGB: 10},
		{Slug: "n2-standard-4", VCPUs: 4, MemoryMB: 16384, DiskGB: 10},
	}, nil
}

func (g *GCP) GetSizeForSpecs(specs Specs) (string, error) {
	sizes, _ := g.ListSizes()
	var best *Size
	for i := range sizes {
		sz := &sizes[i]
		if sz.VCPUs >= specs.CPUs && sz.MemoryMB >= specs.MemoryMB {
			if best == nil || sz.VCPUs < best.VCPUs || (sz.VCPUs == best.VCPUs && sz.MemoryMB < best.MemoryMB) {
				best = sz
			}
		}
	}
	if best == nil {
		return "", fmt.Errorf("no GCP machine type found matching specs: %d CPUs, %dMB RAM", specs.CPUs, specs.MemoryMB)
	}
	return best.Slug, nil
}

func (g *GCP) CreateServer(config *DeployConfig) (*Server, error) {
	ts, method, err := g.ResolveAuth()
	if err != nil {
		return nil, err
	}
	_ = method // reserved for future diagnostics/logging

	region := strings.TrimSpace(config.Region)
	if region == "" {
		region = g.DefaultRegion()
	}
	zone := region + "-a"

	machineType := strings.TrimSpace(config.Size)
	if machineType == "" {
		machineType, err = g.GetSizeForSpecs(Specs{CPUs: 2, MemoryMB: 2048})
		if err != nil {
			return nil, err
		}
	}

	projectID := strings.TrimSpace(g.projectID)
	createdProject := false
	if projectID == "" && g.createProject {
		projectID, err = g.createProjectAndBilling(ts, config.Name)
		if err != nil {
			return nil, err
		}
		createdProject = true
	}
	if projectID == "" {
		return nil, fmt.Errorf("gcp: project_id is required (or enable create_project)")
	}

	// Ensure Compute API is enabled.
	// For freshly-created projects it's typically disabled and may take time to propagate after enabling.
	if createdProject {
		if err := g.ensureServiceEnabled(ts, projectID, "compute.googleapis.com"); err != nil {
			return nil, fmt.Errorf("gcp: failed to enable compute API for new project %s: %w", projectID, err)
		}
	} else {
		// Best-effort for existing projects; if it's disabled we'll retry after a 403 below.
		_ = g.ensureServiceEnabled(ts, projectID, "compute.googleapis.com")
	}

	// Create firewall rule allowing SSH (best-effort; many projects already have it).
	_ = g.ensureAllowSSH(ts, projectID)

	instName := sanitizeHostname(config.Name)
	if instName == "" {
		instName = "selfhosted"
	}
	// Keep it short enough for GCE.
	if len(instName) > 55 {
		instName = instName[:55]
	}

	startup := buildGCPStartupScript(config.SSHPublicKey)

	instancesClient, err := compute.NewInstancesRESTClient(g.ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	defer instancesClient.Close()

	// Debian/Ubuntu images are referenced by another project.
	sourceImage := "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts"

	req := &computepb.InsertInstanceRequest{
		Project: projectID,
		Zone:    zone,
		InstanceResource: &computepb.Instance{
			Name:        &instName,
			MachineType: ptr(fmt.Sprintf("zones/%s/machineTypes/%s", zone, machineType)),
			Tags: &computepb.Tags{
				Items: []string{"selfhosted"},
			},
			Disks: []*computepb.AttachedDisk{
				{
					AutoDelete: ptr(true),
					Boot:       ptr(true),
					Type:       ptr("PERSISTENT"),
					InitializeParams: &computepb.AttachedDiskInitializeParams{
						SourceImage: &sourceImage,
						DiskSizeGb:  ptr(int64(25)),
					},
				},
			},
			NetworkInterfaces: []*computepb.NetworkInterface{
				{
					// Use default network.
					Network: ptr("global/networks/default"),
					AccessConfigs: []*computepb.AccessConfig{
						{
							Name: ptr("External NAT"),
							Type: ptr("ONE_TO_ONE_NAT"),
						},
					},
				},
			},
			Metadata: &computepb.Metadata{
				Items: []*computepb.Items{
					{Key: ptr("startup-script"), Value: &startup},
				},
			},
		},
	}

	op, err := instancesClient.Insert(g.ctx, req)
	if err != nil {
		// Common case for new projects: compute API enablement propagation lag.
		if isComputeAPIDisabledErr(err) {
			// Try enable + wait once, then retry insert.
			if e2 := g.ensureServiceEnabled(ts, projectID, "compute.googleapis.com"); e2 == nil {
				op, err = instancesClient.Insert(g.ctx, req)
			}
		}
		if err != nil {
		if createdProject {
			// Don't auto-delete project; too risky. Provide hint.
			return nil, fmt.Errorf("gcp: instance create failed (project=%s created=true): %w", projectID, err)
		}
		return nil, err
		}
	}
	_ = op

	return &Server{
		ID:     fmt.Sprintf("%s/%s/%s", projectID, zone, instName),
		Name:   config.Name,
		Status: "provisioning",
	}, nil
}

func (g *GCP) WaitForServer(id string) (*Server, error) {
	ts, method, err := g.ResolveAuth()
	if err != nil {
		return nil, err
	}
	_ = method

	projectID, zone, name, err := parseGCPServerID(id)
	if err != nil {
		return nil, err
	}

	instancesClient, err := compute.NewInstancesRESTClient(g.ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	defer instancesClient.Close()

	timeout := time.After(12 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for GCP instance %s", id)
		case <-ticker.C:
			inst, err := instancesClient.Get(g.ctx, &computepb.GetInstanceRequest{
				Project:  projectID,
				Zone:     zone,
				Instance: name,
			})
			if err != nil {
				return nil, err
			}
			ip := gcpExternalIPv4(inst)
			if ip != "" {
				_ = WaitForSSH(ip, 22)
				return &Server{
					ID:     id,
					Name:   name,
					IP:     ip,
					Status: "running",
				}, nil
			}
		}
	}
}

func (g *GCP) DestroyServer(id string) error {
	ts, method, err := g.ResolveAuth()
	if err != nil {
		return err
	}
	_ = method
	projectID, zone, name, err := parseGCPServerID(id)
	if err != nil {
		return err
	}
	instancesClient, err := compute.NewInstancesRESTClient(g.ctx, option.WithTokenSource(ts))
	if err != nil {
		return err
	}
	defer instancesClient.Close()
	_, err = instancesClient.Delete(g.ctx, &computepb.DeleteInstanceRequest{
		Project:  projectID,
		Zone:     zone,
		Instance: name,
	})
	return err
}

func (g *GCP) SetupDNS(domain, ip string) error {
	return fmt.Errorf("gcp DNS is not supported in this installer yet; please create an A record for %s -> %s at your DNS provider", domain, ip)
}

func (g *GCP) createProjectAndBilling(ts oauth2.TokenSource, displayName string) (string, error) {
	// Generate a random-ish project id.
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	projectID := fmt.Sprintf("selfhosted-%x", suffix)
	projectID = strings.ToLower(projectID)

	rmClient, err := resourcemanager.NewProjectsClient(g.ctx, option.WithTokenSource(ts))
	if err != nil {
		return "", err
	}
	defer rmClient.Close()

	req := &resmgrpb.CreateProjectRequest{
		Project: &resmgrpb.Project{
			DisplayName: strings.TrimSpace(displayName),
			ProjectId:   projectID,
		},
	}
	if strings.TrimSpace(g.parent) != "" {
		req.Project.Parent = g.parent
	}

	op, err := rmClient.CreateProject(g.ctx, req)
	if err != nil {
		return "", err
	}
	_, err = op.Wait(g.ctx)
	if err != nil {
		return "", err
	}

	// Pick billing account if not provided.
	ba := strings.TrimSpace(g.billingAccount)
	if ba == "" {
		if picked, err := g.pickBillingAccount(ts); err == nil && picked != "" {
			ba = picked
		}
	}
	if ba != "" {
		_ = g.linkBilling(ts, projectID, ba)
	}

	return projectID, nil
}

func (g *GCP) pickBillingAccount(ts oauth2.TokenSource) (string, error) {
	bc, err := billing.NewCloudBillingClient(g.ctx, option.WithTokenSource(ts))
	if err != nil {
		return "", err
	}
	defer bc.Close()

	var accounts []string
	it := bc.ListBillingAccounts(g.ctx, &billingpb.ListBillingAccountsRequest{})
	for {
		acc, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return "", err
		}
		if acc == nil {
			continue
		}
		if acc.Open {
			accounts = append(accounts, acc.Name)
		}
	}
	if len(accounts) == 1 {
		return accounts[0], nil
	}
	if len(accounts) > 1 {
		// Force user to choose explicitly (enterprise setups).
		return "", fmt.Errorf("gcp: multiple billing accounts available; specify billing_account in config (examples: %s)", strings.Join(accounts[:min(5, len(accounts))], ", "))
	}
	return "", nil
}

func (g *GCP) linkBilling(ts oauth2.TokenSource, projectID, billingAccount string) error {
	bc, err := billing.NewCloudBillingClient(g.ctx, option.WithTokenSource(ts))
	if err != nil {
		return err
	}
	defer bc.Close()

	_, err = bc.UpdateProjectBillingInfo(g.ctx, &billingpb.UpdateProjectBillingInfoRequest{
		Name: fmt.Sprintf("projects/%s/billingInfo", projectID),
		ProjectBillingInfo: &billingpb.ProjectBillingInfo{
			BillingAccountName: billingAccount,
			BillingEnabled:     true,
		},
	})
	return err
}

func (g *GCP) enableService(ts oauth2.TokenSource, projectID, svc string) error {
	return g.enableServiceWithCtx(g.ctx, ts, projectID, svc)
}

func (g *GCP) enableServiceWithCtx(ctx context.Context, ts oauth2.TokenSource, projectID, svc string) error {
	su, err := serviceusage.NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		return err
	}
	defer su.Close()
	name := fmt.Sprintf("projects/%s/services/%s", projectID, svc)
	op, err := su.EnableService(ctx, &serviceusagepb.EnableServiceRequest{Name: name})
	if err != nil {
		return err
	}
	_, err = op.Wait(ctx)
	return err
}

func (g *GCP) ensureServiceEnabled(ts oauth2.TokenSource, projectID, svc string) error {
	// Enabling APIs for a freshly-created project can take several minutes to propagate.
	// We intentionally wait longer here to avoid forcing users to click a console link manually.
	ctx, cancel := context.WithTimeout(g.ctx, 12*time.Minute)
	defer cancel()

	// First attempt to enable. If this errors, we'll still poll GetService (sometimes enable is in-flight),
	// but we keep the error for better diagnostics.
	enableErr := g.enableServiceWithCtx(ctx, ts, projectID, svc)

	// Then wait until it reports enabled (or timeout).
	return g.waitForServiceEnabled(ctx, ts, projectID, svc, enableErr)
}

func (g *GCP) waitForServiceEnabled(ctx context.Context, ts oauth2.TokenSource, projectID, svc string, enableErr error) error {
	su, err := serviceusage.NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		return err
	}
	defer su.Close()

	name := fmt.Sprintf("projects/%s/services/%s", projectID, svc)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastErr := enableErr
	lastState := "UNKNOWN"
	for {
		svcObj, err := su.GetService(ctx, &serviceusagepb.GetServiceRequest{Name: name})
		if err == nil && svcObj != nil && svcObj.State == serviceusagepb.State_ENABLED {
			return nil
		}
		if err != nil {
			lastErr = err
		} else if svcObj != nil {
			lastState = svcObj.State.String()
		}
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("service %s not enabled (last_state=%s last_error=%v): %w", svc, lastState, lastErr, ctx.Err())
			}
			return fmt.Errorf("service %s not enabled (last_state=%s): %w", svc, lastState, ctx.Err())
		case <-ticker.C:
		}
	}
}

func isComputeAPIDisabledErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	// Heuristic based on the error message users see:
	// "Compute Engine API has not been used in project ... before or it is disabled."
	if strings.Contains(s, "Compute Engine API") && strings.Contains(s, "is disabled") {
		return true
	}
	if strings.Contains(s, "compute.googleapis.com") && (strings.Contains(s, "has not been used") || strings.Contains(s, "is disabled")) {
		return true
	}
	return false
}

func (g *GCP) ensureAllowSSH(ts oauth2.TokenSource, projectID string) error {
	// Best-effort: create firewall rule allowing TCP/22 to instances tagged "selfhosted" in default network.
	fc, err := compute.NewFirewallsRESTClient(g.ctx, option.WithTokenSource(ts))
	if err != nil {
		return err
	}
	defer fc.Close()

	name := "selfhosted-allow-ssh"
	// Try get first; if exists, ok.
	_, err = fc.Get(g.ctx, &computepb.GetFirewallRequest{Project: projectID, Firewall: name})
	if err == nil {
		return nil
	}

	net := "global/networks/default"
	_, err = fc.Insert(g.ctx, &computepb.InsertFirewallRequest{
		Project: projectID,
		FirewallResource: &computepb.Firewall{
			Name:         &name,
			Network:      &net,
			Direction:    ptr("INGRESS"),
			SourceRanges: []string{"0.0.0.0/0"},
			TargetTags:   []string{"selfhosted"},
			Allowed: []*computepb.Allowed{
				{IPProtocol: ptr("tcp"), Ports: []string{"22"}},
			},
		},
	})
	return err
}

func buildGCPStartupScript(sshPub string) string {
	sshPub = strings.TrimSpace(sshPub)
	// This is intentionally defensive and idempotent.
	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail

mkdir -p /root/.ssh
chmod 700 /root/.ssh
touch /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys

grep -qF "%s" /root/.ssh/authorized_keys || echo "%s" >> /root/.ssh/authorized_keys

if [ -f /etc/ssh/sshd_config ]; then
  sed -i.bak 's/^#\?PermitRootLogin.*/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config || true
  systemctl restart ssh || systemctl restart sshd || true
fi
`, sshPub, sshPub)
}

func parseGCPServerID(id string) (project string, zone string, name string, err error) {
	id = strings.TrimSpace(id)
	parts := strings.Split(id, "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("gcp: invalid server id %q (expected project/zone/name)", id)
	}
	return parts[0], parts[1], parts[2], nil
}

func gcpExternalIPv4(inst *computepb.Instance) string {
	if inst == nil {
		return ""
	}
	for _, ni := range inst.NetworkInterfaces {
		for _, ac := range ni.AccessConfigs {
			if ac.NatIP != nil && strings.TrimSpace(*ac.NatIP) != "" {
				return strings.TrimSpace(*ac.NatIP)
			}
		}
	}
	return ""
}

func ptr[T any](v T) *T { return &v }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	Register(NewGCP())
}

// --- debug-only: validate credentials JSON structure early (helps UX) ---
// Some users paste service account JSON; ensure it looks like JSON.
func looksLikeJSON(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
		return false
	}
	var tmp map[string]any
	return json.Unmarshal([]byte(s), &tmp) == nil
}

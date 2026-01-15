package providers

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	billing "cloud.google.com/go/billing/apiv1"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	serviceusage "cloud.google.com/go/serviceusage/apiv1"
	"github.com/zdunecki/selfhosted/pkg/terraform"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	serviceusagepb "google.golang.org/genproto/googleapis/api/serviceusage/v1"
	billingpb "google.golang.org/genproto/googleapis/cloud/billing/v1"
	resmgrpb "google.golang.org/genproto/googleapis/cloud/resourcemanager/v3"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"
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

	// Terraform state
	tfServer  *Server
	tfWorkDir string
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
	GCPAuthADC                GCPAuthMethod = "adc"
	GCPAuthGcloudToken        GCPAuthMethod = "gcloud_access_token"
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
	ProjectID   string `json:"projectID"`
	DisplayName string `json:"displayName"`
	State       string `json:"state"`
}

type GCPBillingAccount struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Open        bool   `json:"open"`
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

func (g *GCP) ListBillingAccounts() ([]GCPBillingAccount, error) {
	ts, _, err := g.ResolveAuth()
	if err != nil {
		return nil, err
	}

	bc, err := billing.NewCloudBillingClient(g.ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	defer bc.Close()

	it := bc.ListBillingAccounts(g.ctx, &billingpb.ListBillingAccountsRequest{})
	var out []GCPBillingAccount
	for {
		acc, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		if acc == nil {
			continue
		}
		out = append(out, GCPBillingAccount{
			Name:        acc.Name,
			DisplayName: acc.DisplayName,
			Open:        acc.Open,
		})
	}
	// Prefer open accounts first.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Open != out[j].Open {
			return out[i].Open
		}
		return out[i].Name < out[j].Name
	})
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
	best, ok := pickBestSizeForSpecs(sizes, specs)
	if !ok {
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
	if projectID == "" && !g.createProject {
		return nil, fmt.Errorf("gcp: project_id is required when create_project=false (select an existing project in the wizard)")
	}
	if projectID == "" && g.createProject {
		// Create project via API first (Terraform can't create projects without a project)
		projectID, err = g.createProjectAndBilling(ts, config.Name)
		if err != nil {
			return nil, err
		}
		// Update the projectID in the struct for Terraform
		g.projectID = projectID
	}
	if projectID == "" {
		return nil, fmt.Errorf("gcp: project_id is required (or enable create_project)")
	}

	// Use Terraform to create the instance
	return g.createServerWithTerraform(config, projectID, zone, machineType, ts)
}

func (g *GCP) WaitForServer(id string) (*Server, error) {
	// Terraform creates instances synchronously, so the server is already ready
	if g.tfServer != nil {
		return g.tfServer, nil
	}
	// Fallback: return server with the provided ID (shouldn't happen in normal flow)
	return &Server{
		ID:     id,
		Status: "active",
	}, nil
}

func (g *GCP) DestroyServer(id string) error {
	if g.tfWorkDir == "" {
		return fmt.Errorf("terraform work directory not found for server %s", id)
	}

	ts, _, err := g.ResolveAuth()
	if err != nil {
		return err
	}

	env := g.terraformEnv(ts)
	if len(env) == 0 {
		return fmt.Errorf("GCP credentials not configured")
	}

	return terraform.Destroy(g.ctx, g.tfWorkDir, env)
}

func (g *GCP) SetupDNS(domain, ip string) error {
	return fmt.Errorf("gcp DNS is not supported in this installer yet; please create an A record for %s -> %s at your DNS provider", domain, ip)
}

func (g *GCP) createServerWithTerraform(config *DeployConfig, projectID, zone, machineType string, ts oauth2.TokenSource) (*Server, error) {
	// Get profile from env var (default: "basic")
	profile := strings.TrimSpace(strings.ToLower(os.Getenv("SELFHOSTED_GCP_PROFILE")))
	if profile == "" {
		profile = "basic"
	}

	moduleDir, err := terraform.FindModuleDir("gcp", profile)
	if err != nil {
		return nil, err
	}

	env := g.terraformEnv(ts)
	if len(env) == 0 {
		return nil, fmt.Errorf("GCP credentials not configured")
	}

	instName := sanitizeHostname(config.Name)
	if instName == "" {
		instName = "selfhosted"
	}
	// Keep it short enough for GCE.
	if len(instName) > 55 {
		instName = instName[:55]
	}

	image := config.Image
	if image == "" {
		image = "projects/ubuntu-os-cloud/global/images/family/ubuntu-2204-lts"
	}

	vars := map[string]interface{}{
		"project_id":     projectID,
		"name":           instName,
		"zone":           zone,
		"machine_type":   machineType,
		"image":          image,
		"ssh_public_key": config.SSHPublicKey,
		"tags":           config.Tags,
	}

	runID := fmt.Sprintf("%s-%d", instName, time.Now().Unix())
	result, err := terraform.Apply(g.ctx, moduleDir, runID, env, vars)
	if err != nil {
		return nil, err
	}

	ip, _ := terraform.OutputString(result.Outputs, "instance_ip")
	instanceZone, _ := terraform.OutputString(result.Outputs, "instance_zone")

	// Format ID as project/zone/name for consistency
	serverID := fmt.Sprintf("%s/%s/%s", projectID, instanceZone, instName)

	server := &Server{
		ID:     serverID,
		Name:   instName,
		IP:     ip,
		Status: "active",
	}

	g.tfServer = server
	g.tfWorkDir = result.WorkDir

	return server, nil
}

func (g *GCP) terraformEnv(ts oauth2.TokenSource) map[string]string {
	env := make(map[string]string)

	// Set project
	if g.projectID != "" {
		env["GOOGLE_PROJECT"] = g.projectID
		env["GOOGLE_CLOUD_PROJECT"] = g.projectID
	}

	// Handle credentials
	if g.credentialsJSON != "" {
		// Write credentials to a temp file and set GOOGLE_APPLICATION_CREDENTIALS
		// For now, we'll pass it via env var (Terraform supports GOOGLE_CREDENTIALS)
		env["GOOGLE_CREDENTIALS"] = g.credentialsJSON
	} else {
		// Use Application Default Credentials (ADC)
		// Terraform will automatically use ADC if GOOGLE_APPLICATION_CREDENTIALS is not set
		// and GOOGLE_CREDENTIALS is not set
	}

	return env
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
	if ba == "" {
		// Without billing enabled, enabling Compute API will fail for most accounts.
		return "", fmt.Errorf("gcp: billing is not configured for new project %s. Provide billing_account in GCP config (example: billingAccounts/XXXX-XXXX-XXXX) or ensure your credentials can list/select an open billing account", projectID)
	}

	if err := g.linkBilling(ts, projectID, ba); err != nil {
		return "", fmt.Errorf("gcp: failed to link billing account %s to project %s: %s", ba, projectID, formatGCPError(err))
	}

	if err := g.ensureProjectBillingEnabled(ts, projectID); err != nil {
		return "", err
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
		// Per proto, this is `projects/{project_id}` (not the billingInfo subresource).
		Name: fmt.Sprintf("projects/%s", projectID),
		ProjectBillingInfo: &billingpb.ProjectBillingInfo{
			BillingAccountName: billingAccount,
		},
	})
	return err
}

func (g *GCP) ensureProjectBillingEnabled(ts oauth2.TokenSource, projectID string) error {
	bc, err := billing.NewCloudBillingClient(g.ctx, option.WithTokenSource(ts))
	if err != nil {
		return err
	}
	defer bc.Close()

	info, err := bc.GetProjectBillingInfo(g.ctx, &billingpb.GetProjectBillingInfoRequest{
		Name: fmt.Sprintf("projects/%s/billingInfo", projectID),
	})
	if err != nil {
		return fmt.Errorf("gcp: could not fetch billing status for project %s: %s", projectID, formatGCPError(err))
	}
	if info == nil || !info.BillingEnabled {
		return fmt.Errorf("gcp: billing is not enabled for project %s (billing_account=%s). Enable billing for the project or provide a valid billing_account in config", projectID, strings.TrimSpace(g.billingAccount))
	}
	return nil
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

	// If enable fails due to billing not enabled, surface it immediately (polling won't help).
	if enableErr != nil && strings.Contains(formatGCPError(enableErr), "billing-enabled") {
		return fmt.Errorf("failed to enable %s for project %s: %s", svc, projectID, formatGCPError(enableErr))
	}

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

func formatGCPError(err error) string {
	if err == nil {
		return ""
	}
	st, ok := status.FromError(err)
	if !ok {
		return err.Error()
	}

	msg := fmt.Sprintf("%s (code=%s)", st.Message(), st.Code().String())
	for _, d := range st.Details() {
		switch x := d.(type) {
		case *errdetails.ErrorInfo:
			// Commonly includes reasons like UREQ_PROJECT_BILLING_NOT_FOUND
			msg += fmt.Sprintf(" | ErrorInfo(reason=%s domain=%s metadata=%v)", x.Reason, x.Domain, x.Metadata)
		case *errdetails.PreconditionFailure:
			msg += " | PreconditionFailure("
			for i, v := range x.Violations {
				if i > 0 {
					msg += ", "
				}
				msg += fmt.Sprintf("type=%s subject=%s description=%s", v.Type, v.Subject, v.Description)
			}
			msg += ")"
		case *errdetails.QuotaFailure:
			msg += " | QuotaFailure("
			for i, v := range x.Violations {
				if i > 0 {
					msg += ", "
				}
				msg += fmt.Sprintf("subject=%s description=%s", v.Subject, v.Description)
			}
			msg += ")"
		default:
			// Keep something visible even for unknown detail types.
			msg += fmt.Sprintf(" | detail=%T", d)
		}
	}
	return msg
}

// gcloudTokenSource shells out to `gcloud auth print-access-token`.
// This allows using a developer's gcloud user login without ADC.
//
// Notes:
// - Tokens are short-lived; we assume ~1h and refresh proactively.
// - This is a convenience fallback; production should prefer ADC or service accounts.
type gcloudTokenSource struct {
	mu  sync.Mutex
	tok *oauth2.Token
}

func (s *gcloudTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reuse cached token if it’s still valid (with a small safety window).
	if s.tok != nil && s.tok.Expiry.After(time.Now().Add(2*time.Minute)) {
		return s.tok, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gcloud", "auth", "print-access-token")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("gcloud auth print-access-token failed: %s", msg)
	}

	token := strings.TrimSpace(stdout.String())
	if token == "" {
		return nil, fmt.Errorf("gcloud returned empty access token")
	}

	// gcloud access tokens are typically valid for 1 hour.
	s.tok = &oauth2.Token{
		AccessToken: token,
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(55 * time.Minute),
	}
	return s.tok, nil
}

func init() {
	Register(NewGCP())
}

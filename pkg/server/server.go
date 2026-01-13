package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zdunecki/selfhosted/pkg/apps"
	github_com_zdunecki_selfhosted_pkg_cli "github.com/zdunecki/selfhosted/pkg/cli"
	"github.com/zdunecki/selfhosted/pkg/providers"
	"github.com/zdunecki/selfhosted/pkg/utils"
)

//go:embed dist
var frontendDist embed.FS

func Start(port int) error {
	if err := initSecureKeypair(); err != nil {
		return fmt.Errorf("init secure keypair: %w", err)
	}

	// Serve frontend
	dist, err := fs.Sub(frontendDist, "dist")
	if err != nil {
		return err
	}

	// Serve frontend with SPA fallback
	fsHandler := http.FileServer(http.FS(dist))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// If the request is for an API endpoint, let it pass (it will be handled by specific handlers)
		// actually, specific handlers are registered on ServeMux, so they take precedence if exact match.
		// But "/" is a catch-all.

		// Check if file exists in dist
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else if path[0] == '/' {
			path = path[1:]
		}

		_, err := dist.Open(path)
		if err != nil {
			// File not found, serve index.html
			r.URL.Path = "/"
		}

		fsHandler.ServeHTTP(w, r)
	})

	// API Endpoints
	http.HandleFunc("/api/apps", handleListApps)
	http.HandleFunc("/api/pty/input", handlePTYInput)
	http.HandleFunc("/api/providers", handleListProviders)
	http.HandleFunc("/api/providers/check", handleCheckProviderCredentials)
	http.HandleFunc("/api/providers/gcp/billing-accounts", handleGCPBillingAccounts)
	http.HandleFunc("/api/providers/gcp/projects", handleGCPProjects)
	http.HandleFunc("/api/regions", handleListRegions)
	http.HandleFunc("/api/sizes", handleListSizes)
	http.HandleFunc("/api/deploy", handleDeploy)
	http.HandleFunc("/api/providers/config", handleProviderConfig)
	http.HandleFunc("/api/domains/check", handleDomainCheck)
	http.HandleFunc("/api/cloudflare/verify", handleCloudflareVerify)
	http.HandleFunc("/api/crypto/public-key", handlePublicKey)

	url := fmt.Sprintf("http://localhost:%d", port)
	log.Printf("Starting web interface at %s\n", url)

	openBrowser(url)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatal(err)
	}
	return nil // Return nil as log.Fatal will exit the program on error
}

func handleCheckProviderCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	providerName := r.URL.Query().Get("provider")
	if providerName == "" {
		http.Error(w, "provider is required", http.StatusBadRequest)
		return
	}

	p, err := providers.Get(providerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	needsConfig := false
	if np, ok := p.(interface{ NeedsConfig() bool }); ok {
		needsConfig = np.NeedsConfig()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"provider":       providerName,
		"hasCredentials": !needsConfig,
		"needsConfig":    needsConfig,
	})
}

func handlePTYInput(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"sessionId"`
		DataB64   string `json:"dataB64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.SessionID == "" || req.DataB64 == "" {
		http.Error(w, "sessionId and dataB64 are required", http.StatusBadRequest)
		return
	}

	if err := utils.WritePTYBase64(req.SessionID, req.DataB64); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleListApps(w http.ResponseWriter, r *http.Request) {
	type AppResponse struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		MinCPUs     int    `json:"min_cpus"`
		MinMemory   int    `json:"min_memory"`
		DomainHint  string `json:"domain_hint"`
		Wizard      struct {
			Application struct {
				CustomQuestions []apps.WizardQuestion `json:"custom_questions,omitempty"`
			} `json:"application"`
		} `json:"wizard,omitempty"`
	}
	var res []AppResponse
	for name, app := range apps.Registry {
		specs := app.MinSpecs()
		ar := AppResponse{
			Name:        name,
			Description: app.Description(),
			MinCPUs:     specs.CPUs,
			MinMemory:   specs.MemoryMB,
			DomainHint:  app.DomainHint(),
		}
		if wp, ok := app.(apps.WizardProvider); ok {
			ar.Wizard.Application.CustomQuestions = wp.WizardQuestions()
		}
		res = append(res, ar)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Name < res[j].Name })
	json.NewEncoder(w).Encode(res)
}

func handleListProviders(w http.ResponseWriter, r *http.Request) {
	type ProviderResponse struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		NeedsConfig bool   `json:"needs_config,omitempty"`
	}
	var res []ProviderResponse
	for name, p := range providers.Registry {
		needsConfig := false
		if np, ok := p.(interface{ NeedsConfig() bool }); ok {
			needsConfig = np.NeedsConfig()
		}
		res = append(res, ProviderResponse{
			Name:        name,
			Description: p.Description(),
			NeedsConfig: needsConfig,
		})
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Name < res[j].Name })
	json.NewEncoder(w).Encode(res)
}

func handleGCPBillingAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	p, err := providers.Get("gcp")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	g, ok := p.(*providers.GCP)
	if !ok {
		http.Error(w, "gcp provider not available", http.StatusInternalServerError)
		return
	}

	accounts, err := g.ListBillingAccounts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(accounts)
}

func handleGCPProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	p, err := providers.Get("gcp")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	g, ok := p.(*providers.GCP)
	if !ok {
		http.Error(w, "gcp provider not available", http.StatusInternalServerError)
		return
	}

	projects, err := g.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Only show ACTIVE projects (keeps dropdown usable).
	active := make([]providers.GCPProject, 0, len(projects))
	for _, p := range projects {
		if p.State == "ACTIVE" {
			active = append(active, p)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(active)
}

func handleListRegions(w http.ResponseWriter, r *http.Request) {
	providerName := r.URL.Query().Get("provider")
	p, err := providers.Get(providerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	regions, err := p.ListRegions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(regions)
}

func handleListSizes(w http.ResponseWriter, r *http.Request) {
	providerName := r.URL.Query().Get("provider")
	region := r.URL.Query().Get("region")
	p, err := providers.Get(providerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Optional: if provider supports region/zone-specific sizes, use them.
	type sizesByRegion interface {
		ListSizesForRegion(region string) ([]providers.Size, error)
	}

	var sizes []providers.Size
	if region != "" {
		if sp, ok := p.(sizesByRegion); ok {
			sizes, err = sp.ListSizesForRegion(region)
		} else {
			sizes, err = p.ListSizes()
		}
	} else {
		sizes, err = p.ListSizes()
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(sizes)
}

func handleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse options
	var opts struct {
		App                  string                 `json:"app"`
		Provider             string                 `json:"provider"`
		Region               string                 `json:"region"`
		Size                 string                 `json:"size"`
		Domain               string                 `json:"domain"`
		Name                 string                 `json:"serverName"`
		DNSMode              string                 `json:"dnsMode"`
		CloudflareToken      string                 `json:"cloudflareToken" secure:"rsa_oaep_b64" secure_key:"CloudflareTokenKeyID"`
		CloudflareTokenKeyID string                 `json:"cloudflareTokenKeyId"`
		CloudflareAccountId  string                 `json:"cloudflareAccountId"`
		CloudflareProxied    *bool                  `json:"cloudflareProxied"` // Optional, defaults to true
		WizardAnswers        map[string]interface{} `json:"wizardAnswers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// If frontend sent encrypted Cloudflare token, decrypt into opts.CloudflareToken.
	_ = decryptSecureFields(&opts)

	// Default CloudflareProxied to true if not specified and Cloudflare is being used
	cloudflareProxied := true
	if opts.CloudflareProxied != nil {
		cloudflareProxied = *opts.CloudflareProxied
	} else if opts.DNSMode == "auto" && opts.CloudflareToken != "" {
		// Auto-detect: if using Cloudflare, default to proxied
		cloudflareProxied = true
	}

	// Map to CLI options
	deployOpts := github_com_zdunecki_selfhosted_pkg_cli.DeployOptions{
		AppName:           opts.App,
		ProviderName:      opts.Provider,
		Region:            opts.Region,
		Size:              opts.Size,
		Domain:            opts.Domain,
		DeployName:        opts.Name,
		EnableSSL:         true,
		SSHKeyPath:        "", // Will use default
		SSHPubKey:         "", // Will use default
		DNSSetupMode:      opts.DNSMode,
		CloudflareToken:   opts.CloudflareToken,
		CloudflareProxied: cloudflareProxied,
		WizardAnswers:     opts.WizardAnswers,
	}

	// Set headers for streaming (must be set before writing status)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Set status code before writing body
	w.WriteHeader(http.StatusOK)

	// Create a flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial connection message
	fmt.Fprintf(w, "data: Connected\n\n")
	flusher.Flush()

	// Get request context to detect client disconnects
	ctx := r.Context()

	// Deploy with logging
	var writeErr error
	var keepAliveStop = make(chan struct{})
	var writeMutex sync.Mutex // Protect writes from race conditions

	// Helper function to safely write SSE message (thread-safe)
	writeSSE := func(data string) bool {
		writeMutex.Lock()
		defer writeMutex.Unlock()

		if writeErr != nil {
			return false
		}
		select {
		case <-ctx.Done():
			writeErr = ctx.Err()
			log.Printf("SSE connection closed by client: %v", writeErr)
			return false
		default:
		}

		// Use fmt.Fprintf directly - it's more reliable for HTTP responses
		// and will fail immediately if connection is closed
		message := fmt.Sprintf("data: %s\n\n", data)
		n, err := fmt.Fprint(w, message)
		if err != nil {
			writeErr = err
			log.Printf("SSE write error (connection likely closed): %v (wrote %d bytes)", err, n)
			return false
		}
		if n == 0 {
			// This shouldn't happen, but if we wrote 0 bytes, something is wrong
			writeErr = fmt.Errorf("wrote 0 bytes to response")
			log.Printf("SSE write error: wrote 0 bytes")
			return false
		}

		// Flush immediately - this will also detect if connection is closed
		// Note: Flush() doesn't return an error, but if connection is closed,
		// the next write will fail
		flusher.Flush()
		return true
	}

	// Start keep-alive goroutine to prevent connection timeouts
	// SSE comments (lines starting with :) are ignored by clients but keep the connection alive
	go func() {
		ticker := time.NewTicker(30 * time.Second) // Send keep-alive every 30 seconds
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-keepAliveStop:
				return
			case <-ticker.C:
				writeMutex.Lock()
				shouldStop := writeErr != nil
				writeMutex.Unlock()

				if shouldStop {
					return
				}

				select {
				case <-ctx.Done():
					return
				case <-keepAliveStop:
					return
				default:
					// Send a keep-alive comment (SSE comments start with :)
					// This keeps the connection alive without triggering events on the client
					writeMutex.Lock()
					if writeErr == nil {
						if _, err := fmt.Fprintf(w, ": keep-alive\n\n"); err != nil {
							writeErr = err
							log.Printf("Keep-alive write error: %v", err)
							writeMutex.Unlock()
							return
						}
						flusher.Flush()
					}
					writeMutex.Unlock()
				}
			}
		}
	}()

	// Ensure keep-alive stops when deployment completes
	defer close(keepAliveStop)

	err := github_com_zdunecki_selfhosted_pkg_cli.Deploy(deployOpts, func(format string, a ...interface{}) {
		// Check if we already have a write error (thread-safe)
		writeMutex.Lock()
		hasError := writeErr != nil
		writeMutex.Unlock()

		if hasError {
			return
		}

		msg := fmt.Sprintf(format, a...)
		// Split by newlines and send each non-empty line as a separate SSE message
		lines := strings.Split(msg, "\n")

		for _, line := range lines {
			line = strings.TrimRight(line, "\r")
			line = strings.TrimRight(line, "\n")

			// Skip empty lines
			if strings.TrimSpace(line) == "" {
				continue
			}

			// Write the line - function handles error checking
			if !writeSSE(line) {
				// Log that we stopped sending logs due to connection issue
				writeMutex.Lock()
				err := writeErr
				writeMutex.Unlock()
				if err != nil {
					log.Printf("Stopped sending SSE logs due to error: %v. Deployment continues on backend.", err)
				}
				return
			}
		}
	})

	if err != nil {
		// Check if client is still connected before sending error
		select {
		case <-ctx.Done():
			// Client disconnected, don't send error
			return
		default:
			if _, writeErr := fmt.Fprintf(w, "data: [SELFHOSTED::ERROR] %v\n\n", err); writeErr == nil {
				flusher.Flush()
			}
		}
	} else {
		// Check if client is still connected before sending completion
		select {
		case <-ctx.Done():
			// Client disconnected, don't send completion
			return
		default:
			// Send completion message
			if _, writeErr := fmt.Fprintf(w, "data: [SELFHOSTED::DONE]\n\n"); writeErr == nil {
				flusher.Flush()
			}
		}
	}
}

func handleProviderConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Provider string            `json:"provider"`
		Config   map[string]string `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p, err := providers.Get(req.Provider)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := p.Configure(req.Config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Printf("Failed to open browser: %v\n", err)
	}
}

func handleDomainCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	domain := r.URL.Query().Get("domain")
	if domain == "" {
		http.Error(w, "Domain is required", http.StatusBadRequest)
		return
	}

	// Helper to lookup NS recursively
	var lookupNS func(d string) ([]*net.NS, error)
	lookupNS = func(d string) ([]*net.NS, error) {
		ns, err := net.LookupNS(d)
		if (err == nil && len(ns) > 0) || !strings.Contains(d, ".") {
			return ns, err
		}
		// Try parent
		parts := strings.SplitN(d, ".", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("no NS records found")
		}
		return lookupNS(parts[1])
	}

	ns, err := lookupNS(domain)
	if err != nil {
		// Just return unknown if we can't lookup, don't fail hard
		json.NewEncoder(w).Encode(map[string]interface{}{
			"provider":    "other",
			"nameservers": []string{},
		})
		return
	}

	var nameservers []string
	isCloudflare := false
	for _, n := range ns {
		nameservers = append(nameservers, n.Host)
		if strings.Contains(strings.ToLower(n.Host), "cloudflare.com") {
			isCloudflare = true
		}
	}

	provider := "other"
	if isCloudflare {
		provider = "cloudflare"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"provider":    provider,
		"nameservers": nameservers,
	})
}

func handleCloudflareVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Token     string `json:"token" secure:"rsa_oaep_b64" secure_key:"KeyID"`
		KeyID     string `json:"keyId"`
		AccountID string `json:"accountId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// If frontend sent encrypted token, decrypt into req.Token.
	_ = decryptSecureFields(&req)

	if req.Token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	// Proxy the request to Cloudflare API
	// Use account-specific endpoint if account ID is provided, otherwise use user endpoint
	var cfURL string
	if req.AccountID != "" {
		cfURL = fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/tokens/verify", req.AccountID)
	} else {
		cfURL = "https://api.cloudflare.com/client/v4/user/tokens/verify"
	}
	cfReq, err := http.NewRequest("GET", cfURL, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}

	cfReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", req.Token))
	cfReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	cfResp, err := client.Do(cfReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to verify token: %v", err), http.StatusInternalServerError)
		return
	}
	defer cfResp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(cfResp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read response: %v", err), http.StatusInternalServerError)
		return
	}

	// Forward the status code and response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(cfResp.StatusCode)
	w.Write(body)
}

func handlePublicKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	keyID, spkiB64, err := currentPublicKeySPKIB64()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"alg":     "RSA-OAEP-256",
		"keyId":   keyID,
		"spkiB64": spkiB64,
	})
}

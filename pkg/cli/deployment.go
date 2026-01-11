package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/zdunecki/selfhosted/pkg/apps"
	"github.com/zdunecki/selfhosted/pkg/dns"
	"github.com/zdunecki/selfhosted/pkg/providers"
)

// DeployOptions holds all deployment configuration
type DeployOptions struct {
	ProviderName           string `json:"provider"`
	AppName                string `json:"app"`
	Region                 string `json:"region"`
	Size                   string `json:"size"`
	Domain                 string `json:"domain"`
	DeployName             string `json:"deploy_name"`
	SSHKeyPath             string `json:"ssh_key_path"`
	SSHPubKey              string `json:"ssh_pub_key"`
	EnableSSL              bool   `json:"enable_ssl"`
	Email                  string `json:"email"`
	SSLPrivateKeyFile      string `json:"ssl_private_key_file"`
	SSLCertificateCrt      string `json:"ssl_certificate_crt"`
	HttpToHttpsRedirection bool   `json:"http_to_https_redirection"`
	DNSSetupMode           string `json:"dns_setup_mode"`
	CloudflareToken        string `json:"cloudflare_token"`     // Cloudflare API token (if provided by user)
	CloudflareZoneName     string `json:"cloudflare_zone_name"` // Cloudflare zone name if using Cloudflare DNS
	CloudflareProxied      bool   `json:"cloudflare_proxied"`   // Whether to enable Cloudflare proxy
}

// Deploy executes a deployment with the given options
func Deploy(opts DeployOptions, logf func(string, ...interface{})) error {
	// Get provider
	provider, err := providers.Get(opts.ProviderName)
	if err != nil {
		return fmt.Errorf("provider error: %w", err)
	}

	// Get app
	app, err := apps.Get(opts.AppName)
	if err != nil {
		return fmt.Errorf("app error: %w", err)
	}

	// Load SSH keys
	sshPrivate, sshPublic, err := LoadSSHKeys(opts.SSHKeyPath, opts.SSHPubKey)
	if err != nil {
		return fmt.Errorf("SSH key error: %w", err)
	}

	// Determine size (use app minimum if not specified)
	vmSize := opts.Size
	if vmSize == "" {
		vmSize, err = provider.GetSizeForSpecs(app.MinSpecs())
		if err != nil {
			return fmt.Errorf("could not find suitable size: %w", err)
		}
	}

	// Determine region
	vmRegion := opts.Region
	if vmRegion == "" {
		vmRegion = provider.DefaultRegion()
	}

	logf("🚀 Deploying %s to %s\n", opts.AppName, opts.ProviderName)
	logf("   Region: %s\n", vmRegion)
	logf("   Size: %s\n", vmSize)
	logf("   Domain: %s\n", opts.Domain)
	logf("\n")

	// Create deployment config
	serverName := opts.DeployName
	if serverName == "" {
		serverName = fmt.Sprintf("%s-server", opts.AppName)
	}
	config := &providers.DeployConfig{
		Name:          serverName,
		Region:        vmRegion,
		Size:          vmSize,
		SSHPublicKey:  sshPublic,
		SSHPrivateKey: sshPrivate,
		Domain:        opts.Domain,
		Tags:          []string{opts.AppName, "selfhost"},
	}

	// Step 1: Create server
	logf("⏳ Creating server...\n")
	server, err := provider.CreateServer(config)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	logf("✅ Server created: %s (ID: %s)\n", server.Name, server.ID)

	// Step 2: Wait for server
	logf("⏳ Waiting for server to be ready...\n")
	server, err = provider.WaitForServer(server.ID)
	if err != nil {
		return fmt.Errorf("server not ready: %w", err)
	}
	logf("✅ Server ready with IP: %s\n", server.IP)

	// Step 3: Setup DNS
	detectedDNS := dns.DetectDNSProvider(opts.Domain)
	detectedProvider := string(detectedDNS.Name)

	// Debug logging for DNS setup decision
	logf("🔍 DNS Setup Debug:\n")
	logf("   DNS Mode: %s\n", opts.DNSSetupMode)
	logf("   Detected DNS Provider: %s\n", detectedProvider)
	logf("   Cloudflare Token Provided: %v\n", opts.CloudflareToken != "")
	logf("   Cloudflare Zone Name: %s\n", opts.CloudflareZoneName)

	// Check if we should use Cloudflare DNS (even if ShouldSetupDNS returns false)
	// Use case-insensitive comparison for detected provider
	detectedProviderLower := strings.ToLower(detectedProvider)
	shouldUseCloudflare := (opts.DNSSetupMode == "cloudflare" && opts.CloudflareZoneName != "") ||
		(opts.DNSSetupMode == "auto" && detectedProviderLower == "cloudflare" && opts.CloudflareToken != "")

	// If Cloudflare token is provided, we should set up DNS even if ShouldSetupDNS returns false
	// (e.g., when provider is DigitalOcean but DNS is Cloudflare)
	shouldSetupDNSFromApp := apps.ShouldSetupDNS(app, opts.DNSSetupMode, provider.Name(), detectedProviderLower)
	shouldSetupDNS := shouldSetupDNSFromApp || shouldUseCloudflare

	logf("   Should Setup DNS (from app): %v\n", shouldSetupDNSFromApp)
	logf("   Should Use Cloudflare: %v\n", shouldUseCloudflare)
	logf("   Final Should Setup DNS: %v\n", shouldSetupDNS)

	if shouldSetupDNS {
		if shouldUseCloudflare {
			logf("⏳ Setting up Cloudflare DNS...\n")

			// Use custom token if provided, otherwise try env var
			var cfProvider *dns.CloudflareProvider
			var err error
			if opts.CloudflareToken != "" {
				cfProvider, err = dns.NewCloudflareProviderWithToken(opts.CloudflareToken)
			} else {
				cfProvider, err = dns.NewCloudflareProvider()
			}

			if err != nil {
				logf("⚠️  Could not initialize Cloudflare provider: %v\n", err)
				logf("ℹ️  Please configure DNS manually at your Cloudflare dashboard\n")
			} else {
				// App-defined DNS records (optional). If none provided, fall back to a single record for opts.Domain.
				var customRecords []apps.DNSRecord
				if rp, ok := app.(apps.DNSRecordProvider); ok {
					customRecords = rp.DNSRecords(opts.Domain, server.IP)
				}

				if len(customRecords) == 0 {
					err = cfProvider.SetupDNS(opts.Domain, server.IP, opts.CloudflareProxied)
					if err != nil {
						logf("⚠️  Cloudflare DNS setup failed: %v\n", err)
						logf("ℹ️  Please configure DNS manually at your Cloudflare dashboard\n")
					} else {
						if opts.CloudflareProxied {
							logf("✅ DNS configured with Cloudflare proxy enabled\n")
						} else {
							logf("✅ DNS configured (DNS only mode)\n")
						}
					}
				} else {
					zone, zerr := cfProvider.FindZoneForDomain(opts.Domain)
					if zerr != nil {
						logf("⚠️  Cloudflare DNS setup failed: %v\n", zerr)
						logf("ℹ️  Please configure DNS manually at your Cloudflare dashboard\n")
					} else {
						for _, rec := range customRecords {
							proxied := opts.CloudflareProxied
							if rec.Proxied != nil {
								proxied = *rec.Proxied
							}
							ttl := rec.TTL
							if ttl == 0 {
								ttl = 3600
							}
							rerr := cfProvider.CreateDNSRecord(zone.ID, dns.CloudflareDNSRecordRequest{
								Type:    rec.Type,
								Name:    rec.Name,
								Content: rec.Content,
								TTL:     ttl,
								Proxied: proxied,
							})
							if rerr != nil {
								logf("⚠️  Cloudflare DNS record failed (%s %s): %v\n", rec.Type, rec.Name, rerr)
							} else {
								if proxied {
									logf("✅ DNS record created (proxied): %s %s\n", rec.Type, rec.Name)
								} else {
									logf("✅ DNS record created: %s %s\n", rec.Type, rec.Name)
								}
							}
						}
					}
				}
			}
		} else {
			// Try provider's native DNS setup
			err = provider.SetupDNS(opts.Domain, server.IP)
			if err != nil {
				logf("⚠️  DNS setup failed (manual setup may be needed): %v\n", err)
			} else {
				logf("✅ DNS configured\n")
			}
		}
	} else {
		logf("ℹ️  Skipping DNS setup. Configure DNS at your provider.\n")
	}

	// Step 4: Wait for SSH
	logf("⏳ Waiting for SSH...\n")
	err = providers.WaitForSSH(server.IP, 22)
	if err != nil {
		return fmt.Errorf("SSH not ready: %w", err)
	}
	logf("✅ SSH ready\n")

	// Step 5: Install app
	logf("⏳ Installing %s (this may take 10-15 minutes)...\n", opts.AppName)
	installConfig := &apps.InstallConfig{
		Domain:                 opts.Domain,
		ServerIP:               server.IP,
		SSHKey:                 sshPrivate,
		SSHUser:                "root",
		EnableSSL:              opts.EnableSSL,
		Email:                  opts.Email,
		SSL:                    opts.EnableSSL,
		SSLPrivateKeyFile:      opts.SSLPrivateKeyFile,
		SSLCertificateCrt:      opts.SSLCertificateCrt,
		HttpToHttpsRedirection: opts.HttpToHttpsRedirection,
		Logger:                 logf, // Pass logger to capture all installation logs
	}

	err = app.Install(installConfig)
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}
	logf("✅ %s installed\n", opts.AppName)

	// Step 6: Setup SSL (if enabled)
	if (opts.EnableSSL && opts.Email != "") || opts.SSLPrivateKeyFile != "" || opts.SSLCertificateCrt != "" || opts.HttpToHttpsRedirection {
		logf("⏳ Setting up SSL...\n")
		// Logger is already set in installConfig
		err = app.SetupSSL(installConfig)
		if err != nil {
			logf("⚠️  SSL setup failed: %v\n", err)
		} else {
			logf("✅ SSL configured\n")
		}
	}

	// Print summary
	// Capturing summary output is tricky because PresentSummary likely prints to stdout.
	// For now, we accept that app.PrintSummary might still go to stdout, or we can check if it supports a writer.
	// Assuming PrintSummary prints to stdout. We might want to replicate it logic or update apps interface later.
	// For the wizard purposes, we can just log the final success message.
	logf("\n")
	logf("🎉 Deployment Complete!\n")
	logf("🔗 URL: https://%s\n", opts.Domain)
	logf("🔑 SSH: ssh root@%s\n", server.IP)

	return nil
}

func LoadSSHKeys(privatePath, publicPath string) (privateKey, publicKey string, err error) {
	// Try to load from flags first
	if privatePath != "" {
		data, err := os.ReadFile(privatePath)
		if err != nil {
			return "", "", fmt.Errorf("failed to read private key: %w", err)
		}
		privateKey = string(data)
	}

	if publicPath != "" {
		data, err := os.ReadFile(publicPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to read public key: %w", err)
		}
		publicKey = string(data)
	}

	// Fallback to default paths
	if privateKey == "" {
		home, _ := os.UserHomeDir()
		defaultPaths := []string{
			home + "/.ssh/id_rsa",
			home + "/.ssh/id_ed25519",
		}
		for _, p := range defaultPaths {
			if data, err := os.ReadFile(p); err == nil {
				privateKey = string(data)
				break
			}
		}
	}

	if publicKey == "" {
		home, _ := os.UserHomeDir()
		defaultPaths := []string{
			home + "/.ssh/id_rsa.pub",
			home + "/.ssh/id_ed25519.pub",
		}
		for _, p := range defaultPaths {
			if data, err := os.ReadFile(p); err == nil {
				publicKey = string(data)
				break
			}
		}
	}

	if privateKey == "" || publicKey == "" {
		return "", "", fmt.Errorf("SSH keys not found. Use --ssh-key and --ssh-pub flags")
	}

	return privateKey, publicKey, nil
}

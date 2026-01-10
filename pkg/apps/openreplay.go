package apps

import (
	"fmt"
	"strings"

	"github.com/zdunecki/selfhosted/pkg/providers"
	"github.com/zdunecki/selfhosted/pkg/utils"
)

type OpenReplay struct{}

func NewOpenReplay() *OpenReplay {
	return &OpenReplay{}
}

func (o *OpenReplay) Name() string {
	return "openreplay-legacy"
}

func (o *OpenReplay) Description() string {
	return "OpenReplay (legacy Go installer)"
}

func (o *OpenReplay) MinSpecs() providers.Specs {
	return providers.Specs{
		CPUs:     4,
		MemoryMB: 8192, // 8GB
		DiskGB:   80,
	}
}

func (o *OpenReplay) Install(config *InstallConfig) error {
	runner := NewSSHRunner(config.ServerIP, config.SSHUser, config.SSHKey)
	defer runner.Close()

	if err := runner.Connect(); err != nil {
		return err
	}

	fmt.Println("\n📦 Installing OpenReplay (this takes 10-15 minutes)...")
	fmt.Println("   - Updating system packages")
	fmt.Println("   - Downloading OpenReplay CLI")
	fmt.Println("   - Installing Kubernetes (k3s)")
	fmt.Println("   - Deploying OpenReplay services\n")

	commands := []string{
		// Wait for cloud-init to complete
		"cloud-init status --wait || true",

		// Update system
		"apt-get update -y",
		"DEBIAN_FRONTEND=noninteractive apt-get upgrade -y",

		// Install OpenReplay CLI
		"wget https://raw.githubusercontent.com/openreplay/openreplay/main/scripts/helmcharts/openreplay-cli -O /bin/openreplay",
		"chmod +x /bin/openreplay",

		// Install OpenReplay with domain
		fmt.Sprintf("/bin/openreplay -i %s", config.Domain),
	}

	if err := runner.RunMultiple(commands); err != nil {
		return fmt.Errorf("installation failed: %w\n\nTroubleshooting:\n  SSH into server: ssh root@%s\n  Check status: openreplay -s\n  View logs: journalctl -xeu k3s", err, config.ServerIP)
	}

	// Verify installation
	fmt.Println("\n✅ Verifying installation...")
	output, err := runner.RunWithOutput("openreplay -s 2>&1 || echo 'STATUS_CHECK_FAILED'")
	if err != nil || strings.Contains(output, "STATUS_CHECK_FAILED") {
		fmt.Println("⚠️  Warning: Could not verify OpenReplay status")
		fmt.Println("   The installation may still be completing in the background")
		fmt.Printf("   SSH to check: ssh root@%s\n", config.ServerIP)
		fmt.Println("   Then run: openreplay -s")
	}

	return nil
}

func (o *OpenReplay) SetupSSL(config *InstallConfig) error {
	runner := NewSSHRunner(config.ServerIP, config.SSHUser, config.SSHKey)
	defer runner.Close()

	if err := runner.Connect(); err != nil {
		return err
	}

	// Verify DNS is resolving correctly before attempting SSL
	fmt.Println("🔍 Verifying DNS configuration...")
	dnsCheckCmd := utils.GetDNSCheckCommand(config.Domain, config.ServerIP)

	output, err := runner.RunWithOutput(dnsCheckCmd)

	// Parse DNS check results
	isResolved, resolvedIP, dnsErr := utils.ParseDNSCheckOutput(output)

	if !isResolved {
		if resolvedIP != "" {
			// DNS is resolving to wrong IP
			return utils.FormatDNSMismatchError(config.Domain, resolvedIP, config.ServerIP)
		}
		// DNS is not resolving at all
		return utils.FormatDNSNotResolvedError(config.Domain, config.ServerIP, config.Email)
	}

	if dnsErr != nil && err != nil {
		// Unexpected error
		return fmt.Errorf("DNS verification failed: %w", err)
	}

	fmt.Println("✅ DNS verified - proceeding with SSL setup")

	// OpenReplay specific configuration
	const (
		openreplayConfigDir  = "/var/lib/openreplay"
		openreplayScriptsDir = "/var/lib/openreplay/openreplay/scripts/helmcharts"
		openreplayConfigFile = "/var/lib/openreplay/vars.yaml"
	)

	// First try the OpenReplay cert-manager script
	fmt.Println("📝 Configuring SSL with cert-manager...")

	commands := []string{
		"sleep 30",
		utils.GetAppendSSLConfigCommand(openreplayConfigFile),
		utils.GetCertManagerCommand(config.Email, config.Domain, openreplayScriptsDir),
	}

	// Try the OpenReplay script first
	scriptErr := runner.RunMultiple(commands)

	if scriptErr != nil {
		fmt.Println("⚠️  OpenReplay cert-manager script failed, using direct cert-manager setup...")

		// Fallback to direct cert-manager setup
		directCommands := utils.GetDirectCertManagerSetup(config.Email, config.Domain)
		if err := runner.RunMultiple(directCommands); err != nil {
			return fmt.Errorf("both cert-manager methods failed: %w", err)
		}
	}

	fmt.Println("✅ Certificate configuration complete")

	// Update ingress to use the certificate
	fmt.Println("🔄 Updating ingress for SSL...")
	updateIngressCmd := fmt.Sprintf(`kubectl patch ingress -n app frontend --type='json' -p='[
		{"op": "add", "path": "/spec/tls", "value": [{"hosts": ["%s"], "secretName": "%s-tls"}]}
	]'`, config.Domain, config.Domain)

	if err := runner.Run(updateIngressCmd); err != nil {
		fmt.Printf("⚠️  Warning: Could not update ingress: %v\n", err)
		fmt.Println("   The certificate will be created but ingress needs manual update")
	}

	// Reinstall/restart OpenReplay
	fmt.Println("🔄 Restarting OpenReplay services...")
	if err := runner.Run("/bin/openreplay -R"); err != nil {
		fmt.Printf("⚠️  Warning: Could not restart OpenReplay: %v\n", err)
	}

	return nil
}

func (o *OpenReplay) PrintSummary(ip, domain string) {
	fmt.Println()
	fmt.Println(strings.Repeat("═", 70))
	fmt.Println("🎉 OpenReplay Deployment Complete!")
	fmt.Println(strings.Repeat("═", 70))
	fmt.Printf("\n📍 Server IP: %s\n", ip)
	fmt.Printf("🌐 Domain: %s\n", domain)
	fmt.Printf("🔗 OpenReplay URL: https://%s\n", domain)
	fmt.Printf("📝 Signup URL: https://%s/signup\n", domain)

	fmt.Println("\n⚠️  IMPORTANT:")
	fmt.Println("   OpenReplay is configured for domain-based access only")
	fmt.Printf("   Accessing http://%s directly will show 404\n", ip)
	fmt.Println("   You MUST access it via the domain name above")

	fmt.Println("\n📋 Next Steps:")
	fmt.Println("   1. Ensure DNS is pointing to the server IP:")
	fmt.Printf("      dig %s (should return %s)\n", domain, ip)
	fmt.Println("   2. If DNS is not set up, add an A record:")
	fmt.Printf("      - Type: A\n")
	fmt.Printf("      - Name: %s\n", domain)
	fmt.Printf("      - Value: %s\n", ip)
	fmt.Println("   3. Wait a few minutes for SSL certificate to provision")
	fmt.Println("   4. Visit https://" + domain + " to access OpenReplay")
	fmt.Println("   5. Create your account at the signup page")

	fmt.Printf("\n🔑 SSH Access: ssh root@%s\n", ip)

	fmt.Println("\n📚 Troubleshooting Commands:")
	fmt.Println("   openreplay -s                    # Check service status")
	fmt.Println("   openreplay -R                    # Reinstall/restart services")
	fmt.Println("   kubectl get pods -A              # Check all pods")
	fmt.Println("   kubectl get ingress -A           # Check ingress configuration")
	fmt.Println("   journalctl -xeu k3s              # View k3s logs")
	fmt.Println("   curl -I http://localhost         # Test local nginx")

	fmt.Println("\n🐛 If you see 404:")
	fmt.Println("   - Make sure you're accessing via domain, not IP")
	fmt.Println("   - Check DNS is resolving correctly: nslookup " + domain)
	fmt.Println("   - Verify pods are running: kubectl get pods -A")
	fmt.Println("   - Check ingress: kubectl describe ingress -n app")

	fmt.Println(strings.Repeat("═", 70))
}

func (o *OpenReplay) DomainHint() string {
	return "Example: openreplay.your-domain.com"
}

func init() {
	Register(NewOpenReplay())
}

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zdunecki/selfhosted/pkg/apps"
	"github.com/zdunecki/selfhosted/pkg/apps/dsl"
	"github.com/zdunecki/selfhosted/pkg/providers"
)

var (
	// Global flags
	providerName           string
	appName                string
	region                 string
	size                   string
	domain                 string
	deployName             string
	sshKeyPath             string
	sshPubKey              string
	enableSSL              bool
	email                  string
	sslPrivateKeyFile      string
	sslCertificateCrt      string
	httpToHttpsRedirection bool
	configFile             string
	dnsSetupMode           string
)

type deployOptions struct {
	ProviderName           string
	AppName                string
	Region                 string
	Size                   string
	Domain                 string
	DeployName             string
	SSHKeyPath             string
	SSHPubKey              string
	EnableSSL              bool
	Email                  string
	SSLPrivateKeyFile      string
	SSLCertificateCrt      string
	HttpToHttpsRedirection bool
	DNSSetupMode           string
}

var rootCmd = &cobra.Command{
	Use:   "selfhost",
	Short: "Self-hosted app installer for multiple cloud providers",
	Long: `A CLI tool to deploy self-hosted applications like OpenReplay, 
OpenPanel, Plausible, and more to cloud providers like DigitalOcean, 
Scaleway, and OVH with a single command.`,
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy an application",
	Long:  `Deploy a self-hosted application to a cloud provider.`,
	RunE:  runDeploy,
}

var listProvidersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List available cloud providers",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Available providers:")
		for name, p := range providers.Registry {
			fmt.Printf("  - %s: %s\n", name, p.Description())
		}
	},
}

var listAppsCmd = &cobra.Command{
	Use:   "apps",
	Short: "List available applications",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Available applications:")
		for name, a := range apps.Registry {
			specs := a.MinSpecs()
			fmt.Printf("  - %s: %s (min: %d vCPUs, %dMB RAM)\n",
				name, a.Description(), specs.CPUs, specs.MemoryMB)
		}
	},
}

var listRegionsCmd = &cobra.Command{
	Use:   "regions [provider]",
	Short: "List available regions for a provider",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := providers.Get(args[0])
		if err != nil {
			return err
		}

		regions, err := p.ListRegions()
		if err != nil {
			return err
		}

		fmt.Printf("Available regions for %s:\n", args[0])
		for _, r := range regions {
			fmt.Printf("  - %s: %s\n", r.Slug, r.Name)
		}
		return nil
	},
}

var listSizesCmd = &cobra.Command{
	Use:   "sizes [provider]",
	Short: "List available VM sizes for a provider",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := providers.Get(args[0])
		if err != nil {
			return err
		}

		sizes, err := p.ListSizes()
		if err != nil {
			return err
		}

		fmt.Printf("Available sizes for %s:\n", args[0])
		fmt.Printf("  %-20s %6s %8s %12s\n", "SLUG", "VCPUS", "MEMORY", "PRICE/MO")
		fmt.Println(strings.Repeat("-", 50))
		for _, s := range sizes {
			fmt.Printf("  %-20s %6d %6dMB %10.2f$\n",
				s.Slug, s.VCPUs, s.MemoryMB, s.PriceMonthly)
		}
		return nil
	},
}

var destroyCmd = &cobra.Command{
	Use:   "destroy [server-id]",
	Short: "Destroy a deployed server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := providers.Get(providerName)
		if err != nil {
			return err
		}
		return p.DestroyServer(args[0])
	},
}

var setupSSLCmd = &cobra.Command{
	Use:   "setup-ssl",
	Short: "Setup SSL for an existing deployment",
	Long:  `Configure Let's Encrypt SSL for an already deployed application. Use this if SSL setup failed during initial deployment or DNS wasn't ready.`,
	RunE:  runSetupSSL,
}

func init() {
	apps.Register(apps.NewOpenReplayDSL(&dsl.Loader{}))

	// Deploy command flags
	deployCmd.Flags().StringVarP(&providerName, "provider", "p", "", "Cloud provider (digitalocean, scaleway, ovh)")
	deployCmd.Flags().StringVarP(&appName, "app", "a", "", "Application to deploy (openreplay, openpanel, plausible)")
	deployCmd.Flags().StringVarP(&region, "region", "r", "", "Region/datacenter")
	deployCmd.Flags().StringVarP(&size, "size", "s", "", "VM size (optional, will use app minimum)")
	deployCmd.Flags().StringVarP(&domain, "domain", "d", "", "Domain name for the app")
	deployCmd.Flags().StringVar(&deployName, "name", "", "Server name (defaults to <app>-server)")
	deployCmd.Flags().StringVar(&sshKeyPath, "ssh-key", "", "Path to SSH private key")
	deployCmd.Flags().StringVar(&sshPubKey, "ssh-pub", "", "Path to SSH public key")
	deployCmd.Flags().BoolVar(&enableSSL, "ssl", true, "Enable Let's Encrypt SSL")
	deployCmd.Flags().StringVar(&email, "email", "", "Email for Let's Encrypt")
	deployCmd.Flags().StringVar(&sslPrivateKeyFile, "ssl-key-file", "", "Path to SSL private key file for app-managed TLS")
	deployCmd.Flags().StringVar(&sslCertificateCrt, "ssl-cert-crt", "", "Path to SSL certificate CRT for app-managed TLS")
	deployCmd.Flags().BoolVar(&httpToHttpsRedirection, "http-to-https", false, "Enable HTTP to HTTPS redirection in the app")
	deployCmd.Flags().StringVarP(&configFile, "config", "c", "", "Config file path")
	deployCmd.Flags().StringVar(&dnsSetupMode, "dns-setup", "auto", "DNS setup mode for openreplay (auto, skip, force)")

	deployCmd.MarkFlagRequired("provider")
	deployCmd.MarkFlagRequired("app")
	deployCmd.MarkFlagRequired("domain")

	// Destroy command flags
	destroyCmd.Flags().StringVarP(&providerName, "provider", "p", "", "Cloud provider")
	destroyCmd.MarkFlagRequired("provider")

	// Setup SSL command flags
	setupSSLCmd.Flags().StringVarP(&appName, "app", "a", "", "Application name (openreplay, openpanel, plausible)")
	setupSSLCmd.Flags().StringVarP(&domain, "domain", "d", "", "Domain name")
	setupSSLCmd.Flags().StringVar(&email, "email", "", "Email for Let's Encrypt")
	setupSSLCmd.Flags().StringVar(&sshKeyPath, "ssh-key", "", "Path to SSH private key")
	setupSSLCmd.Flags().String("server-ip", "", "Server IP address")
	setupSSLCmd.MarkFlagRequired("app")
	setupSSLCmd.MarkFlagRequired("domain")
	setupSSLCmd.MarkFlagRequired("email")
	setupSSLCmd.MarkFlagRequired("server-ip")

	// Add commands
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(listProvidersCmd)
	rootCmd.AddCommand(listAppsCmd)
	rootCmd.AddCommand(listRegionsCmd)
	rootCmd.AddCommand(listSizesCmd)
	rootCmd.AddCommand(destroyCmd)
	rootCmd.AddCommand(setupSSLCmd)
}

func Execute() error {
	if len(os.Args) == 1 {
		return runWizard()
	}
	return rootCmd.Execute()
}

func runDeploy(cmd *cobra.Command, args []string) error {
	opts := deployOptions{
		ProviderName:           providerName,
		AppName:                appName,
		Region:                 region,
		Size:                   size,
		Domain:                 domain,
		DeployName:             deployName,
		SSHKeyPath:             sshKeyPath,
		SSHPubKey:              sshPubKey,
		EnableSSL:              enableSSL,
		Email:                  email,
		SSLPrivateKeyFile:      sslPrivateKeyFile,
		SSLCertificateCrt:      sslCertificateCrt,
		HttpToHttpsRedirection: httpToHttpsRedirection,
		DNSSetupMode:           dnsSetupMode,
	}
	return deployWithOptions(opts)
}

func deployWithOptions(opts deployOptions) error {
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
	sshPrivate, sshPublic, err := loadSSHKeys(opts.SSHKeyPath, opts.SSHPubKey)
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

	fmt.Printf("🚀 Deploying %s to %s\n", opts.AppName, opts.ProviderName)
	fmt.Printf("   Region: %s\n", vmRegion)
	fmt.Printf("   Size: %s\n", vmSize)
	fmt.Printf("   Domain: %s\n", opts.Domain)
	fmt.Println()

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
	fmt.Println("⏳ Creating server...")
	server, err := provider.CreateServer(config)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	fmt.Printf("✅ Server created: %s (ID: %s)\n", server.Name, server.ID)

	// Step 2: Wait for server
	fmt.Println("⏳ Waiting for server to be ready...")
	server, err = provider.WaitForServer(server.ID)
	if err != nil {
		return fmt.Errorf("server not ready: %w", err)
	}
	fmt.Printf("✅ Server ready with IP: %s\n", server.IP)

	// Step 3: Setup DNS
	if shouldSetupDNS(opts, provider.Name()) {
		fmt.Println("⏳ Setting up DNS...")
		err = provider.SetupDNS(opts.Domain, server.IP)
		if err != nil {
			fmt.Printf("⚠️  DNS setup failed (manual setup may be needed): %v\n", err)
		} else {
			fmt.Println("✅ DNS configured")
		}
	} else {
		fmt.Println("ℹ️  Skipping DNS setup. Configure DNS at your provider.")
	}

	// Step 4: Wait for SSH
	fmt.Println("⏳ Waiting for SSH...")
	err = providers.WaitForSSH(server.IP, 22)
	if err != nil {
		return fmt.Errorf("SSH not ready: %w", err)
	}
	fmt.Println("✅ SSH ready")

	// Step 5: Install app
	fmt.Printf("⏳ Installing %s (this may take 10-15 minutes)...\n", opts.AppName)
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
	}

	err = app.Install(installConfig)
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}
	fmt.Printf("✅ %s installed\n", opts.AppName)

	// Step 6: Setup SSL (if enabled)
	if (opts.EnableSSL && opts.Email != "") || opts.SSLPrivateKeyFile != "" || opts.SSLCertificateCrt != "" || opts.HttpToHttpsRedirection {
		fmt.Println("⏳ Setting up SSL...")
		err = app.SetupSSL(installConfig)
		if err != nil {
			fmt.Printf("⚠️  SSL setup failed: %v\n", err)
		} else {
			fmt.Println("✅ SSL configured")
		}
	}

	// Print summary
	app.PrintSummary(server.IP, opts.Domain)

	return nil
}

func loadSSHKeys(privatePath, publicPath string) (privateKey, publicKey string, err error) {
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

func runSetupSSL(cmd *cobra.Command, args []string) error {
	// Get server IP from flag
	serverIP, _ := cmd.Flags().GetString("server-ip")

	// Get app
	app, err := apps.Get(appName)
	if err != nil {
		return fmt.Errorf("app error: %w", err)
	}

	// Load SSH keys
	sshPrivate, _, err := loadSSHKeys(sshKeyPath, "")
	if err != nil {
		return fmt.Errorf("SSH key error: %w", err)
	}

	fmt.Printf("🔐 Setting up SSL for %s\n", domain)
	fmt.Printf("   Server: %s\n", serverIP)
	fmt.Printf("   Email: %s\n", email)
	fmt.Println()

	// Create install config for SSL setup
	installConfig := &apps.InstallConfig{
		Domain:                 domain,
		ServerIP:               serverIP,
		SSHKey:                 sshPrivate,
		SSHUser:                "root",
		EnableSSL:              true,
		Email:                  email,
		SSL:                    true,
		SSLPrivateKeyFile:      sslPrivateKeyFile,
		SSLCertificateCrt:      sslCertificateCrt,
		HttpToHttpsRedirection: httpToHttpsRedirection,
	}

	// Setup SSL
	fmt.Println("⏳ Configuring SSL certificate...")
	err = app.SetupSSL(installConfig)
	if err != nil {
		return fmt.Errorf("SSL setup failed: %w", err)
	}

	fmt.Println()
	fmt.Println("✅ SSL configured successfully!")
	fmt.Printf("🔗 Your app should now be accessible at: https://%s\n", domain)
	fmt.Println()

	return nil
}

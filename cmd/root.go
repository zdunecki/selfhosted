package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zdunecki/selfhosted/pkg/server"

	"github.com/zdunecki/selfhosted/pkg/apps"
	"github.com/zdunecki/selfhosted/pkg/cli"
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
	desktopMode            bool
)

var rootCmd = &cobra.Command{
	Use:   "selfhost",
	Short: "Self-hosted app installer for multiple cloud providers",
	Long: `A CLI tool to deploy self-hosted applications like OpenReplay, 
OpenPanel, Plausible, and more to cloud providers like DigitalOcean, 
Scaleway, and OVH with a single command.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If --desktop flag is set, launch desktop app
		if desktopMode {
			return launchDesktop()
		}
		// Default: start web UI in browser
		return server.Start(8080)
	},
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
	// Root command flags
	rootCmd.Flags().BoolVar(&desktopMode, "desktop", false, "Launch desktop application instead of web UI")

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
	return rootCmd.Execute()
}

func runDeploy(cmd *cobra.Command, args []string) error {
	opts := cli.DeployOptions{
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

// deployWithOptions executes a deployment with the given options
func deployWithOptions(opts cli.DeployOptions) error {
	return cli.Deploy(opts, func(format string, a ...interface{}) {
		fmt.Printf(format, a...)
	})
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
	sshPrivate, _, err := cli.LoadSSHKeys(sshKeyPath, "")
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

package cli

// DeployOptions holds all deployment configuration
type DeployOptions struct {
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
	CloudflareToken        string // Cloudflare API token (if provided by user)
	CloudflareZoneName     string // Cloudflare zone name if using Cloudflare DNS
	CloudflareProxied      bool   // Whether to enable Cloudflare proxy
}

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
}

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zdunecki/selfhosted/pkg/dns"

	// "github.com/zdunecki/selfhosted/pkg/server" // Removed to break import cycle

	"github.com/zdunecki/selfhosted/pkg/apps"
	"github.com/zdunecki/selfhosted/pkg/providers"
)

type wizardStep int

const (
	stepMode wizardStep = iota
	stepApp
	stepProvider
	stepRegion
	stepSize
	stepDomain
	stepDNSProviderChoice
	stepCloudflareTokenChoice
	stepCloudflareTokenInput
	stepCloudflareSetup
	stepDNSSetup
	stepSSL
	stepEmail
	stepSSHPrivate
	stepSSHPublic
	stepDeployName
	stepConfirm
	stepDone
)

type optionItem struct {
	title string
	desc  string
	value string
}

func (i optionItem) Title() string       { return i.title }
func (i optionItem) Description() string { return i.desc }
func (i optionItem) FilterValue() string { return i.title }

type wizardModel struct {
	step               wizardStep
	list               list.Model
	input              textinput.Model
	opts               DeployOptions
	validationErr      string
	cancelled          bool
	err                error
	width              int
	height             int
	cloudflareToken    string              // Cloudflare API token
	cloudflareTokenURL string              // Cached Cloudflare token creation URL
	cloudflareZoneName string              // For Cloudflare setup flow
	cloudflareProxied  bool                // User's proxy preference
	detectedDNS        dns.DNSProviderInfo // Detected DNS provider from domain
	startWebUI         bool
}

var (
	styleTitle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	styleSubtitle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleError     = lipgloss.NewStyle().Foreground(lipgloss.Color("160")).Bold(true)
	stylePrompt    = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	styleSummary   = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	styleHighlight = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
)

// ErrStartWebUI is returned when the user selects the Web UI option
var ErrStartWebUI = fmt.Errorf("start web ui")

// RunWizard runs the interactive deployment wizard
func RunWizard(deployFunc func(DeployOptions) error) error {
	model := newWizardModel()
	prog := tea.NewProgram(model, tea.WithAltScreen())
	result, err := prog.Run()
	if err != nil {
		return err
	}

	finalModel, ok := result.(wizardModel)
	if !ok {
		return fmt.Errorf("wizard failed to return results")
	}
	if finalModel.err != nil {
		return finalModel.err
	}
	if finalModel.startWebUI {
		fmt.Println("Starting Web UI...")
		// Return special error to signal caller to start web server
		return ErrStartWebUI
	}
	if finalModel.cancelled {
		return nil
	}
	if finalModel.opts.ProviderName == "" {
		return nil
	}
	// Pass Cloudflare settings to deploy options
	finalModel.opts.CloudflareToken = finalModel.cloudflareToken
	finalModel.opts.CloudflareZoneName = finalModel.cloudflareZoneName
	finalModel.opts.CloudflareProxied = finalModel.cloudflareProxied
	return deployFunc(finalModel.opts)
}

func newWizardModel() wizardModel {
	model := wizardModel{
		step: stepMode,
	}
	model.list = newList("Select mode", modeItems())
	return model
}

func newList(title string, items []list.Item) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Foreground(lipgloss.Color("252"))
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("205")).Bold(true)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Foreground(lipgloss.Color("244")).Italic(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("212")).Italic(true)
	l := list.New(items, delegate, 0, 0)
	l.Title = styleTitle.Render(title)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowPagination(false)
	return l
}

func (m wizardModel) Init() tea.Cmd {
	return nil
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
		if m.step == stepDomain || m.step == stepEmail || m.step == stepSSHPrivate || m.step == stepSSHPublic || m.step == stepDeployName || m.step == stepCloudflareTokenInput {
			m.input.Width = msg.Width - 4
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if m.step != stepDomain && m.step != stepEmail && m.step != stepSSHPrivate && m.step != stepSSHPublic && m.step != stepDeployName && m.step != stepCloudflareTokenInput {
				return m.handleSelection()
			}
		}
	}

	switch m.step {
	case stepDomain, stepEmail, stepSSHPrivate, stepSSHPublic, stepDeployName, stepCloudflareTokenInput:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEnter {
			return m.handleInputSubmit()
		}
		return m, cmd
	default:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
}

func (m wizardModel) View() string {
	if m.step == stepDone {
		return ""
	}

	var header string
	if m.validationErr != "" {
		header = styleError.Render("Validation: "+m.validationErr) + "\n\n"
	}

	switch m.step {
	case stepDomain:
		return header + styleSubtitle.Render("Enter the domain for the app:") + "\n" + styleSummary.Render(m.domainHint()) + "\n\n" + m.input.View() + "\n\n" + stylePrompt.Render("Press Enter to continue.")
	case stepCloudflareTokenInput:
		instructions := styleSubtitle.Render("Create a Cloudflare API token with 'Zone.DNS' permissions:") + "\n" +
			styleSummary.Render(m.cloudflareTokenURL) + "\n\n" +
			styleSubtitle.Render("Paste your API token below:")
		return header + instructions + "\n\n" + m.input.View() + "\n\n" + stylePrompt.Render("Press Enter to continue.")
	case stepEmail:
		return header + styleSubtitle.Render("Enter email for SSL (required when SSL is enabled):") + "\n\n" + m.input.View() + "\n\n" + stylePrompt.Render("Press Enter to continue.")
	case stepSSHPrivate:
		return header + styleSubtitle.Render("Optional: path to SSH private key (leave blank to auto-detect):") + "\n\n" + m.input.View() + "\n\n" + stylePrompt.Render("Press Enter to continue.")
	case stepSSHPublic:
		return header + styleSubtitle.Render("Optional: path to SSH public key (leave blank to auto-detect):") + "\n\n" + m.input.View() + "\n\n" + stylePrompt.Render("Press Enter to continue.")
	case stepDeployName:
		return header + styleSubtitle.Render("Optional: server name (leave blank to use default):") + "\n\n" + m.input.View() + "\n\n" + stylePrompt.Render("Press Enter to continue.")
	case stepConfirm:
		return header + styleSummary.Render(m.confirmSummary()) + "\n\n" + m.list.View() + "\n\n" + stylePrompt.Render("Use Enter to confirm, q to quit.")
	default:
		return header + m.list.View() + "\n\n" + stylePrompt.Render("Use ↑/↓ to move, Enter to select, q to quit.")
	}
}

func (m wizardModel) handleSelection() (tea.Model, tea.Cmd) {
	item, ok := m.list.SelectedItem().(optionItem)
	if !ok {
		return m, nil
	}

	switch m.step {
	case stepMode:
		if item.value == "web" {
			m.startWebUI = true
			return m, tea.Quit
		}
		// Continue with CLI wizard
		m.list = newList("Select application", appItems())
		m.applyListSize()
		m.step = stepApp
	case stepApp:
		m.opts.AppName = item.value
		m.list = newList("Select provider", providerItems())
		m.applyListSize()
		m.step = stepProvider
	case stepProvider:
		m.opts.ProviderName = item.value
		regions, err := m.loadRegions()
		if err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.list = newList("Select region", regionItems(regions))
		m.applyListSize()
		m.step = stepRegion
	case stepRegion:
		m.opts.Region = item.value
		sizes, err := m.loadSizes()
		if err != nil {
			m.err = err
			return m, tea.Quit
		}
		m.list = newList("Select size", sizeItems(sizes, m.opts.AppName))
		m.applyListSize()
		m.step = stepSize
	case stepSize:
		if item.value == "auto" {
			m.opts.Size = ""
		} else {
			m.opts.Size = item.value
		}
		m.setInput(stepDomain, "example.com")
	case stepDNSProviderChoice:
		if item.value == "cloudflare" {
			// Try to create Cloudflare provider (checks for CLOUDFLARE_API_TOKEN)
			cfProvider, err := dns.NewCloudflareProvider()
			if err != nil {
				// No CLOUDFLARE_API_TOKEN env var - show token choice menu
				m.list = newList("Cloudflare API Token Required", cloudflareTokenChoiceItems())
				m.applyListSize()
				m.step = stepCloudflareTokenChoice
			} else {
				// Token found from env var, store it and proceed to Cloudflare setup
				m.cloudflareToken = cfProvider.GetToken()
				m.list = newList("Cloudflare DNS Setup", cloudflareSetupItems())
				m.applyListSize()
				m.step = stepCloudflareSetup
			}
		} else if item.value == "provider" {
			// User chose provider's native DNS (DigitalOcean, etc.)
			m.opts.DNSSetupMode = "force"
			if m.opts.AppName == "openreplay" {
				m.list = newList("DNS setup for OpenReplay", dnsSetupItems(m.opts.Domain))
				m.applyListSize()
				m.step = stepDNSSetup
			} else {
				m.list = newList("Enable SSL?", yesNoItems())
				m.applyListSize()
				m.step = stepSSL
			}
		} else if item.value == "skip" {
			// User chose to skip DNS setup - go straight to SSL
			m.opts.DNSSetupMode = "skip"
			m.list = newList("Enable SSL?", yesNoItems())
			m.applyListSize()
			m.step = stepSSL
		}
	case stepCloudflareTokenChoice:
		// Handle token choice - either enter token or skip
		if item.value == "enter-token" || item.value == "create-token" {
			// Cache the token URL once (to avoid calling wrangler on every keystroke)
			if m.cloudflareTokenURL == "" {
				m.cloudflareTokenURL = dns.GetTokenCreationURL()
			}
			// Show text input for token
			m.setInput(stepCloudflareTokenInput, "Paste your Cloudflare API token here...")
		} else if item.value == "use-env" {
			// User will set env var and restart
			m.cancelled = true
			fmt.Println("\nSet the CLOUDFLARE_API_TOKEN environment variable:")
			fmt.Println("  export CLOUDFLARE_API_TOKEN=your_token_here")
			fmt.Println("\nThen run the wizard again.")
			return m, tea.Quit
		} else if item.value == "skip" {
			// Skip Cloudflare setup
			m.opts.DNSSetupMode = "skip"
			m.list = newList("Enable SSL?", yesNoItems())
			m.applyListSize()
			m.step = stepSSL
		}
	case stepCloudflareSetup:
		if item.value == "setup" {
			// User wants to setup Cloudflare - fetch zones and let them confirm
			var cfProvider *dns.CloudflareProvider
			var err error

			// Use stored token or try to get from env
			if m.cloudflareToken != "" {
				cfProvider, err = dns.NewCloudflareProviderWithToken(m.cloudflareToken)
			} else {
				cfProvider, err = dns.NewCloudflareProvider()
			}

			if err != nil {
				m.validationErr = fmt.Sprintf("Failed to get Cloudflare API token: %v", err)
				return m, nil
			}
			zone, err := cfProvider.FindZoneForDomain(m.opts.Domain)
			if err != nil {
				m.validationErr = fmt.Sprintf("Failed to find Cloudflare zone: %v", err)
				return m, nil
			}
			m.cloudflareZoneName = zone.Name
			// Ask about proxy setting
			m.list = newList(fmt.Sprintf("Cloudflare zone found: %s", zone.Name), cloudflareProxyItems())
			m.applyListSize()
			// Stay in stepCloudflareSetup but with different state
		} else if item.value == "skip" {
			// User chose to skip Cloudflare setup
			m.opts.DNSSetupMode = "skip"
			if m.opts.AppName == "openreplay" {
				m.list = newList("DNS setup for OpenReplay", dnsSetupItems(m.opts.Domain))
				m.applyListSize()
				m.step = stepDNSSetup
			} else {
				m.list = newList("Enable SSL?", yesNoItems())
				m.applyListSize()
				m.step = stepSSL
			}
		} else if item.value == "install" {
			// User wants to install wrangler first
			m.cancelled = true
			fmt.Println("\nTo install wrangler CLI, run:")
			fmt.Println("  npm install -g wrangler")
			fmt.Println("or")
			fmt.Println("  yarn global add wrangler")
			fmt.Println("\nThen authenticate with: wrangler login")
			return m, tea.Quit
		} else if item.value == "proxied-yes" {
			// User confirmed zone and wants proxy enabled
			m.cloudflareProxied = true
			m.opts.DNSSetupMode = "cloudflare"
			// Skip DNS setup step since we already configured Cloudflare
			m.list = newList("Enable SSL?", yesNoItems())
			m.applyListSize()
			m.step = stepSSL
		} else if item.value == "proxied-no" {
			// User confirmed zone but wants DNS only mode
			m.cloudflareProxied = false
			m.opts.DNSSetupMode = "cloudflare"
			// Skip DNS setup step since we already configured Cloudflare
			m.list = newList("Enable SSL?", yesNoItems())
			m.applyListSize()
			m.step = stepSSL
		}
	case stepDNSSetup:
		// Only update DNSSetupMode if it's not already set (e.g., from Cloudflare setup)
		// This prevents overwriting "cloudflare" with "auto"/"force"/"skip"
		if m.opts.DNSSetupMode == "" || m.opts.DNSSetupMode == "force" || m.opts.DNSSetupMode == "skip" {
			m.opts.DNSSetupMode = item.value
		}
		m.list = newList("Enable SSL?", yesNoItems())
		m.applyListSize()
		m.step = stepSSL
	case stepSSL:
		m.opts.EnableSSL = item.value == "yes"
		if m.opts.EnableSSL {
			m.setInput(stepEmail, "you@example.com")
		} else {
			m.setInput(stepSSHPrivate, "~/.ssh/id_ed25519")
		}
	case stepConfirm:
		if item.value == "deploy" {
			m.step = stepDone
		} else {
			m.cancelled = true
		}
		return m, tea.Quit
	}

	return m, nil
}

func (m wizardModel) handleInputSubmit() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	m.validationErr = ""

	switch m.step {
	case stepDomain:
		if value == "" {
			m.validationErr = "domain is required"
			return m, nil
		}
		m.opts.Domain = value

		// Detect DNS provider from domain
		m.detectedDNS = dns.DetectDNSProvider(m.opts.Domain)

		// Show DNS provider options
		m.list = newList("Choose DNS setup method", dnsProviderChoiceItems(m.detectedDNS, m.opts.ProviderName))
		m.applyListSize()
		m.step = stepDNSProviderChoice
	case stepCloudflareTokenInput:
		if value == "" {
			m.validationErr = "token is required"
			return m, nil
		}
		// Validate token by trying to create a provider
		cfProvider, err := dns.NewCloudflareProviderWithToken(value)
		if err != nil {
			m.validationErr = fmt.Sprintf("invalid token: %v", err)
			return m, nil
		}
		// Test the token by trying to fetch zones
		_, err = cfProvider.FindZoneForDomain(m.opts.Domain)
		if err != nil {
			m.validationErr = fmt.Sprintf("token validation failed: %v", err)
			return m, nil
		}
		// Token is valid, store it and proceed
		m.cloudflareToken = value
		m.list = newList("Cloudflare DNS Setup", cloudflareSetupItems())
		m.applyListSize()
		m.step = stepCloudflareSetup
	case stepEmail:
		if value == "" {
			m.validationErr = "email is required when SSL is enabled"
			return m, nil
		}
		m.opts.Email = value
		m.setInput(stepSSHPrivate, "~/.ssh/id_ed25519")
	case stepSSHPrivate:
		m.opts.SSHKeyPath = value
		m.setInput(stepSSHPublic, "~/.ssh/id_ed25519.pub")
	case stepSSHPublic:
		m.opts.SSHPubKey = value
		m.setInput(stepDeployName, "openreplay-server")
	case stepDeployName:
		m.opts.DeployName = value
		if m.opts.DeployName == "" {
			m.opts.DeployName = fmt.Sprintf("%s-server", m.opts.AppName)
		}
		m.list = newList("Confirm deployment", confirmItems())
		m.applyListSizeWithOffset(m.confirmSummaryLineCount() + 4)
		m.step = stepConfirm
	}

	return m, nil
}

func (m *wizardModel) setInput(step wizardStep, placeholder string) {
	m.step = step
	m.validationErr = ""
	m.input = textinput.New()
	m.input.Prompt = stylePrompt.Render("> ")
	m.input.Placeholder = placeholder
	m.input.Focus()
	if m.width > 0 {
		m.input.Width = m.width - 4
	}
}

func (m *wizardModel) applyListSize() {
	if m.width > 0 && m.height > 0 {
		m.list.SetSize(m.width, m.height-4)
	}
}

func (m *wizardModel) applyListSizeWithOffset(offset int) {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	height := m.height - offset
	if height < 4 {
		height = 4
	}
	m.list.SetSize(m.width, height)
}

func (m wizardModel) confirmSummaryLineCount() int {
	return strings.Count(m.confirmSummary(), "\n") + 1
}

func (m wizardModel) loadRegions() ([]providers.Region, error) {
	provider, err := providers.Get(m.opts.ProviderName)
	if err != nil {
		return nil, err
	}
	return provider.ListRegions()
}

func (m wizardModel) loadSizes() ([]providers.Size, error) {
	provider, err := providers.Get(m.opts.ProviderName)
	if err != nil {
		return nil, err
	}
	type sizesByRegion interface {
		ListSizesForRegion(region string) ([]providers.Size, error)
	}
	if m.opts.Region != "" {
		if sp, ok := provider.(sizesByRegion); ok {
			return sp.ListSizesForRegion(m.opts.Region)
		}
	}
	return provider.ListSizes()
}

func modeItems() []list.Item {
	return []list.Item{
		optionItem{title: "CLI Wizard", desc: "Continue in the terminal", value: "cli"},
		optionItem{title: "Web UI", desc: "Open in browser (Rich UI)", value: "web"},
	}
}

func appItems() []list.Item {
	names := make([]string, 0, len(apps.Registry))
	for name := range apps.Registry {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]list.Item, 0, len(names))
	for _, name := range names {
		app := apps.Registry[name]
		items = append(items, optionItem{
			title: name,
			desc:  app.Description(),
			value: name,
		})
	}
	return items
}

func providerItems() []list.Item {
	names := make([]string, 0, len(providers.Registry))
	for name := range providers.Registry {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]list.Item, 0, len(names))
	for _, name := range names {
		provider := providers.Registry[name]
		items = append(items, optionItem{
			title: name,
			desc:  provider.Description(),
			value: name,
		})
	}
	return items
}

func regionItems(regions []providers.Region) []list.Item {
	sort.Slice(regions, func(i, j int) bool {
		return regions[i].Name < regions[j].Name
	})
	items := make([]list.Item, 0, len(regions))
	for _, region := range regions {
		items = append(items, optionItem{
			title: region.Name,
			desc:  region.Slug,
			value: region.Slug,
		})
	}
	return items
}

func sizeItems(sizes []providers.Size, appName string) []list.Item {
	sort.Slice(sizes, func(i, j int) bool {
		return sizes[i].PriceMonthly < sizes[j].PriceMonthly
	})
	items := make([]list.Item, 0, len(sizes)+1)
	autoDesc := "Let the provider choose a size"
	if best, ok := findSizeForApp(sizes, appName); ok {
		autoDesc = fmt.Sprintf("Recommended: %s", best)
	}
	items = append(items, optionItem{
		title: "auto (recommended)",
		desc:  autoDesc,
		value: "auto",
	})
	for _, size := range sizes {
		items = append(items, optionItem{
			title: size.Slug,
			desc:  fmt.Sprintf("%d vCPU, %dMB RAM, $%.2f/mo", size.VCPUs, size.MemoryMB, size.PriceMonthly),
			value: size.Slug,
		})
	}
	return items
}

func yesNoItems() []list.Item {
	return []list.Item{
		optionItem{title: "yes", desc: "Enable SSL via Let's Encrypt", value: "yes"},
		optionItem{title: "no", desc: "Skip SSL setup for now", value: "no"},
	}
}

func dnsProviderChoiceItems(detectedDNS dns.DNSProviderInfo, providerName string) []list.Item {
	items := []list.Item{}

	// Option 1: Detected DNS provider (if Cloudflare)
	if detectedDNS.Name == dns.DNSProviderCloudflare {
		items = append(items, optionItem{
			title: fmt.Sprintf("%s (recommended based on your domain)", detectedDNS.Name),
			desc:  "Automatically configure DNS via Cloudflare API",
			value: "cloudflare",
		})
	}

	// Option 2: Cloud provider's DNS (if it supports DNS)
	// DigitalOcean, for example, has DNS management
	providerSupportsDNS := providerName == "digitalocean" || providerName == "scaleway"
	if providerSupportsDNS {
		items = append(items, optionItem{
			title: fmt.Sprintf("Use %s DNS", providerName),
			desc:  fmt.Sprintf("Configure DNS using %s's DNS service", providerName),
			value: "provider",
		})
	}

	// Option 3: Skip DNS setup
	items = append(items, optionItem{
		title: "Skip DNS setup",
		desc:  "I'll configure DNS manually at my DNS provider",
		value: "skip",
	})

	return items
}

func dnsSetupItems(domain string) []list.Item {
	info := dns.DetectDNSProvider(domain)
	recommended := "unknown"
	if info.Name != "" {
		recommended = string(info.Name)
	}

	return []list.Item{
		optionItem{title: fmt.Sprintf("auto (recommended: %s)", recommended), desc: "Auto-skip DigitalOcean DNS if your NS points elsewhere", value: "auto"},
		optionItem{title: "force DNS setup", desc: "Attempt provider DNS setup anyway", value: "force"},
		optionItem{title: "skip DNS setup", desc: "Manage DNS manually (Cloudflare, Route 53, etc.)", value: "skip"},
	}
}

func cloudflareSetupItems() []list.Item {
	return []list.Item{
		optionItem{title: "Setup with Cloudflare (recommended)", desc: "Automatically configure DNS via Cloudflare API", value: "setup"},
		optionItem{title: "Skip - Manual setup", desc: "Configure DNS manually at Cloudflare dashboard", value: "skip"},
	}
}

func cloudflareProxyItems() []list.Item {
	return []list.Item{
		optionItem{title: "Enable proxy (recommended)", desc: "DDoS protection, CDN, hides your server IP", value: "proxied-yes"},
		optionItem{title: "DNS only mode", desc: "Direct connection to your server", value: "proxied-no"},
	}
}

func cloudflareTokenChoiceItems() []list.Item {
	tokenURL := dns.GetTokenCreationURL()
	return []list.Item{
		optionItem{
			title: "Enter API token",
			desc:  "I have a Cloudflare API token ready to paste",
			value: "enter-token",
		},
		optionItem{
			title: "Create token now",
			desc:  fmt.Sprintf("Open %s to create a token", tokenURL),
			value: "create-token",
		},
		optionItem{
			title: "Use CLOUDFLARE_API_TOKEN env",
			desc:  "I'll set the environment variable and restart",
			value: "use-env",
		},
		optionItem{
			title: "Skip Cloudflare setup",
			desc:  "Configure DNS manually",
			value: "skip",
		},
	}
}

func confirmItems() []list.Item {
	return []list.Item{
		optionItem{title: "Deploy now", desc: "Start provisioning and installation", value: "deploy"},
		optionItem{title: "Cancel", desc: "Exit without changes", value: "cancel"},
	}
}

func findSizeForApp(sizes []providers.Size, appName string) (string, bool) {
	app, err := apps.Get(appName)
	if err != nil {
		return "", false
	}
	specs := app.MinSpecs()
	var best *providers.Size
	for i, size := range sizes {
		if size.VCPUs >= specs.CPUs && size.MemoryMB >= specs.MemoryMB {
			if best == nil || size.PriceMonthly < best.PriceMonthly {
				best = &sizes[i]
			}
		}
	}
	if best == nil {
		return "", false
	}
	return best.Slug, true
}

func (m wizardModel) confirmSummary() string {
	sizeLabel := m.opts.Size
	if sizeLabel == "" {
		sizeLabel = "auto"
	}
	sslLabel := "no"
	if m.opts.EnableSSL {
		sslLabel = "yes"
	}
	nameLabel := m.opts.DeployName
	if nameLabel == "" {
		nameLabel = fmt.Sprintf("%s-server", m.opts.AppName)
	}

	lines := []string{
		styleHighlight.Render("Review your selections"),
		fmt.Sprintf("App:         %s", m.opts.AppName),
		fmt.Sprintf("Provider:    %s", m.opts.ProviderName),
		fmt.Sprintf("Region:      %s", m.opts.Region),
		fmt.Sprintf("Size:        %s", sizeLabel),
		fmt.Sprintf("Domain:      %s", m.opts.Domain),
		fmt.Sprintf("Server name: %s", nameLabel),
		fmt.Sprintf("SSL:         %s", sslLabel),
	}
	if m.opts.AppName == "openreplay" {
		mode := m.opts.DNSSetupMode
		if mode == "" {
			mode = "auto"
		}
		lines = append(lines, fmt.Sprintf("DNS setup:  %s", mode))
	}
	if m.opts.EnableSSL {
		lines = append(lines, fmt.Sprintf("Email:       %s", m.opts.Email))
	}
	if m.opts.SSHKeyPath != "" {
		lines = append(lines, fmt.Sprintf("SSH private: %s", m.opts.SSHKeyPath))
	}
	if m.opts.SSHPubKey != "" {
		lines = append(lines, fmt.Sprintf("SSH public:  %s", m.opts.SSHPubKey))
	}
	return strings.Join(lines, "\n")
}

func (m wizardModel) domainHint() string {
	appName := m.opts.AppName
	if appName == "" {
		return "Example: app.your-domain.com"
	}
	app, err := apps.Get(appName)
	if err != nil {
		return fmt.Sprintf("Example: %s.your-domain.com", appName)
	}
	if hint := strings.TrimSpace(app.DomainHint()); hint != "" {
		return hint
	}
	return fmt.Sprintf("Example: %s.your-domain.com", appName)
}

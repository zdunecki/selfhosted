package apps

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/zdunecki/selfhosted/pkg/dsl"
	"github.com/zdunecki/selfhosted/pkg/providers"
	"github.com/zdunecki/selfhosted/pkg/utils"
)

// DSLApp is a generic App implementation backed by a YAML DSL spec.
// This is intended to cover the common case where adding a new app only requires:
// 1) adding <app>.yaml
// 2) listing it in pkg/apps/apps.yaml
type DSLApp struct {
	spec dsl.Spec
}

func NewDSLApp(spec dsl.Spec) *DSLApp {
	return &DSLApp{spec: spec}
}

func (a *DSLApp) Name() string {
	if strings.TrimSpace(a.spec.App) != "" {
		return strings.TrimSpace(a.spec.App)
	}
	return "unknown"
}

func (a *DSLApp) Description() string {
	if strings.TrimSpace(a.spec.Description) != "" {
		return strings.TrimSpace(a.spec.Description)
	}
	return a.Name()
}

func (a *DSLApp) DomainHint() string {
	if strings.TrimSpace(a.spec.DomainHint) != "" {
		return strings.TrimSpace(a.spec.DomainHint)
	}
	return "Example: app.your-domain.com"
}

func (a *DSLApp) MinSpecs() providers.Specs {
	cpus := a.spec.MinSpec.CPU
	if cpus == 0 {
		cpus = 1
	}

	ramMB := int(a.spec.MinSpec.RAM)
	if ramMB == 0 {
		ramMB = 1024
	}

	diskGB := int(a.spec.MinSpec.Disk)
	if diskGB == 0 {
		diskGB = 20
	}

	return providers.Specs{
		CPUs:     cpus,
		MemoryMB: ramMB,
		DiskGB:   diskGB,
	}
}

func (a *DSLApp) Install(config *InstallConfig) error {
	return a.runSteps(config, false)
}

func (a *DSLApp) SetupSSL(config *InstallConfig) error {
	return a.runSteps(config, true)
}

func (a *DSLApp) runSteps(config *InstallConfig, conditional bool) error {
	runner := utils.NewSSHRunner(config.ServerIP, config.SSHUser, config.SSHKey)
	defer runner.Close()

	if config.Logger != nil {
		runner.SetLogger(config.Logger)
	}
	if err := runner.Connect(); err != nil {
		return err
	}

	logFunc := func(msg string) {
		if config.Logger != nil {
			config.Logger("%s\n", msg)
		} else {
			fmt.Println(msg)
		}
	}

	// We implement the step loop here (instead of dsl.RunStepsWithConfig) so we can support interactive PTY steps.
	vars := dsl.BuildVarsFromStruct(config)
	bools := dsl.BuildBoolsFromStruct(config)

	for _, step := range a.spec.Steps {
		hasCondition := strings.TrimSpace(step.If) != ""
		if conditional && !hasCondition {
			continue
		}
		if !conditional && hasCondition {
			continue
		}
		if hasCondition && !dsl.EvaluateCondition(step.If, bools) {
			continue
		}

		if step.Name != "" && logFunc != nil {
			logFunc("⏳ " + step.Name)
		}
		if strings.TrimSpace(step.Log) != "" && logFunc != nil {
			logFunc(dsl.RenderTemplate(step.Log, vars))
		}
		if step.Sleep != "" {
			dur, err := dsl.ParseDuration(step.Sleep)
			if err != nil {
				return err
			}
			time.Sleep(dur)
		}

		if strings.TrimSpace(step.Run) == "" {
			continue
		}
		cmd := dsl.BuildRunCommand(dsl.RenderTemplate(step.Run, vars))

		if step.TTY {
			// Interactive/TUI step: allocate a PTY and stream raw output to the installer UI.
			sessionID := randomID()
			if config.Logger != nil {
				config.Logger("[SELFHOSTED::PTY_SESSION] %s\n", sessionID)
			}

			stdin, wait, err := runner.RunPTY(cmd, func(chunk []byte) {
				if config.Logger == nil || len(chunk) == 0 {
					return
				}
				// Send raw bytes via SSE as base64 to preserve ANSI + cursor movements.
				b64 := base64.StdEncoding.EncodeToString(chunk)
				config.Logger("[SELFHOSTED::PTY] %s\n", b64)
			})
			if err != nil {
				if config.Logger != nil {
					config.Logger("[SELFHOSTED::PTY_END] %s\n", sessionID)
				}
				return err
			}

			utils.RegisterPTY(sessionID, stdin)
			err = wait()
			utils.ClosePTY(sessionID)
			if config.Logger != nil {
				config.Logger("[SELFHOSTED::PTY_END] %s\n", sessionID)
			}
			if err != nil {
				return err
			}
		} else {
			if err := runner.Run(cmd); err != nil {
				return err
			}
		}
	}

	return nil
}

func (a *DSLApp) PrintSummary(ip, domain string) {
	// This prints to stdout (not the SSE logger) by design since the interface
	// does not accept a writer/logger here.
	fmt.Println()
	fmt.Printf("🎉 %s installation complete\n", a.Name())
	fmt.Printf("🌐 Domain: %s\n", domain)
	fmt.Printf("🔗 URL: https://%s\n", domain)
	fmt.Printf("🔑 SSH: ssh root@%s\n", ip)
}

func (a *DSLApp) ShouldSetupDNS(dnsSetupMode, providerName, detectedDNSProvider string) bool {
	// Generic safety default:
	// In auto mode, only run provider-native DNS when the detected DNS provider matches the selected provider.
	// This avoids accidentally configuring DNS at a provider when the domain is managed elsewhere.
	p := strings.ToLower(strings.TrimSpace(providerName))
	d := strings.ToLower(strings.TrimSpace(detectedDNSProvider))
	if d != "" && p != "" && d != p {
		return false
	}
	return true
}

func (a *DSLApp) DNSRecords(domain string, serverIP string) []DNSRecord {
	if len(a.spec.DNS.Records) == 0 {
		return nil
	}

	vars := map[string]string{
		"{opts.Domain}":   domain,
		"{opts.ServerIP}": serverIP,
	}

	out := make([]DNSRecord, 0, len(a.spec.DNS.Records))
	for _, r := range a.spec.DNS.Records {
		recType := strings.ToUpper(strings.TrimSpace(r.Type))
		name := strings.TrimSpace(dsl.RenderTemplate(r.Name, vars))
		content := strings.TrimSpace(dsl.RenderTemplate(r.Content, vars))

		// Defaults / conveniences
		if name == "@" || name == "" {
			name = domain
		} else if !strings.Contains(name, ".") && !strings.Contains(name, "*") {
			// "worker" => "worker.<domain>"
			name = name + "." + domain
		}
		if recType == "" {
			recType = "A"
		}
		if content == "" && (recType == "A" || recType == "AAAA") {
			content = serverIP
		}

		out = append(out, DNSRecord{
			Type:    recType,
			Name:    name,
			Content: content,
			TTL:     r.TTL,
			Proxied: r.Proxied,
		})
	}

	return out
}

func randomID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	// URL-safe-ish without extra deps
	return base64.RawURLEncoding.EncodeToString(b)
}

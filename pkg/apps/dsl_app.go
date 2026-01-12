package apps

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/zdunecki/selfhosted/pkg/dsl"
	"github.com/zdunecki/selfhosted/pkg/providers"
	"github.com/zdunecki/selfhosted/pkg/utils"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)
var ansiCSI = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
var ansiOSC = regexp.MustCompile(`\x1b\][^\x07]*(\x07|\x1b\\)`)
var controlChars = regexp.MustCompile(`[\x00-\x08\x0b-\x1f\x7f]`)

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
	// Merge ExtraVars (used for wizard answers and any app-specific template keys)
	if config.ExtraVars != nil {
		for k, v := range config.ExtraVars {
			vars[k] = v
		}
	}
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

		if step.TTY.Enabled {
			// Interactive/TUI step: allocate a PTY and stream raw output to the installer UI.
			sessionID := randomID()
			if config.Logger != nil {
				config.Logger("[SELFHOSTED::PTY_SESSION] %s\n", sessionID)
			}

			// Keep a rolling text buffer of PTY output for wait_for matching (best-effort; ANSI junk may exist).
			var outMu sync.Mutex
			var outBuf strings.Builder
			outChanged := make(chan struct{}, 1)

			stdin, wait, err := runner.RunPTY(cmd, func(chunk []byte) {
				if config.Logger == nil || len(chunk) == 0 {
					return
				}
				// Send raw bytes via SSE as base64 to preserve ANSI + cursor movements.
				b64 := base64.StdEncoding.EncodeToString(chunk)
				config.Logger("[SELFHOSTED::PTY] %s\n", b64)

				// Append to rolling buffer for auto-answer prompt matching
				outMu.Lock()
				outBuf.Write(chunk)
				// cap memory (keep last ~64KB)
				s := outBuf.String()
				if len(s) > 65536 {
					outBuf.Reset()
					outBuf.WriteString(s[len(s)-65536:])
				}
				outMu.Unlock()
				select {
				case outChanged <- struct{}{}:
				default:
				}
			})
			if err != nil {
				if config.Logger != nil {
					config.Logger("[SELFHOSTED::PTY_END] %s\n", sessionID)
				}
				return err
			}

			utils.RegisterPTY(sessionID, stdin)

			// Optional: backend-driven auto-answer from YAML (best-effort).
			if len(step.TTY.AutoAnswer) > 0 {
				go func() {
					// Give the TUI a moment to render the first prompt.
					time.Sleep(800 * time.Millisecond)
					for _, a := range step.TTY.AutoAnswer {
						// Wait for prompt match if configured
						if strings.TrimSpace(a.WaitFor) != "" {
							timeout := a.TimeoutMS
							if timeout <= 0 {
								timeout = 10 * 60 * 1000
							}
							deadline := time.Now().Add(time.Duration(timeout) * time.Millisecond)

							var re *regexp.Regexp
							if a.WaitForRegex {
								if compiled, err := regexp.Compile(a.WaitFor); err == nil {
									re = compiled
								}
							}

							for {
								outMu.Lock()
								cur := stripANSI(outBuf.String())
								outMu.Unlock()

								matched := false
								if re != nil {
									matched = re.MatchString(cur)
								} else {
									matched = strings.Contains(cur, a.WaitFor)
								}
								if matched {
									break
								}
								if time.Now().After(deadline) {
									// Give up on this answer
									break
								}
								// Wait for more output or timeout tick
								select {
								case <-outChanged:
								case <-time.After(250 * time.Millisecond):
								}
							}
						}

						if a.DelayMS > 0 {
							time.Sleep(time.Duration(a.DelayMS) * time.Millisecond)
						} else {
							time.Sleep(350 * time.Millisecond)
						}
						// IMPORTANT: preserve raw control sequences like "\r" or "\t\r".
						// Some prompts (inquirer/whiptail) require Enter/Tab+Enter and trimming would drop them.
						renderedRaw := dsl.RenderTemplate(a.Value, vars)
						val := renderedRaw
						if !strings.Contains(a.Value, "\n") && !strings.Contains(a.Value, "\r") {
							// For normal string answers, trim trailing newlines and send + Enter.
							val = strings.TrimRight(val, "\r\n")
						}
						// Convenience: treat "true/false" as y/n
						if strings.EqualFold(strings.TrimSpace(val), "true") {
							val = "y"
						} else if strings.EqualFold(strings.TrimSpace(val), "false") {
							val = "n"
						}
						// Ensure enter unless caller provided explicit CR/LF in the YAML value.
						if !strings.Contains(a.Value, "\n") && !strings.Contains(a.Value, "\r") {
							val = val + "\r"
						}
						_, _ = stdin.Write([]byte(val))
					}
				}()
			}
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

func stripANSI(s string) string {
	if s == "" {
		return s
	}
	// Remove OSC first, then CSI, then remaining control chars.
	s = ansiOSC.ReplaceAllString(s, "")
	s = ansiCSI.ReplaceAllString(s, "")
	s = controlChars.ReplaceAllString(s, "")
	return s
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

func (a *DSLApp) WizardQuestions() []WizardQuestion {
	qs := a.spec.Wizard.Steps.Application.CustomQuestions
	if len(qs) == 0 {
		return nil
	}

	out := make([]WizardQuestion, 0, len(qs))
	for _, q := range qs {
		id := strings.TrimSpace(q.ID)
		if id == "" {
			id = slugify(q.Name)
		}
		wq := WizardQuestion{
			ID:       id,
			Name:     q.Name,
			Type:     strings.ToLower(strings.TrimSpace(q.Type)),
			Required: q.Required,
			Default:  q.Default,
		}
		if len(q.Choices) > 0 {
			wq.Choices = make([]WizardChoice, 0, len(q.Choices))
			for _, c := range q.Choices {
				wq.Choices = append(wq.Choices, WizardChoice{
					Name:    c.Name,
					Default: c.Default,
				})
			}
		}
		out = append(out, wq)
	}
	return out
}

func randomID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	// URL-safe-ish without extra deps
	return base64.RawURLEncoding.EncodeToString(b)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "q"
	}
	return s
}

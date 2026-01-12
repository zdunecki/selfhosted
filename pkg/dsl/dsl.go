package dsl

import (
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type SizeMB int
type SizeGB int

func (s *SizeMB) UnmarshalYAML(node *yaml.Node) error {
	if node == nil || node.Kind != yaml.ScalarNode {
		return fmt.Errorf("invalid size value")
	}
	value := strings.TrimSpace(node.Value)
	if value == "" {
		return fmt.Errorf("invalid size value")
	}
	*s = SizeMB(ParseSizeToMB(value))
	return nil
}

func (s *SizeGB) UnmarshalYAML(node *yaml.Node) error {
	if node == nil || node.Kind != yaml.ScalarNode {
		return fmt.Errorf("invalid size value")
	}
	value := strings.TrimSpace(node.Value)
	if value == "" {
		return fmt.Errorf("invalid size value")
	}
	*s = SizeGB(ParseSizeToGB(value))
	return nil
}

type Spec struct {
	App         string     `yaml:"app"`
	Description string     `yaml:"description"`
	OS          string     `yaml:"os"`
	DomainHint  string     `yaml:"domain_hint"`
	MinSpec     SpecHW     `yaml:"min_spec"`
	Providers   []string   `yaml:"providers"`
	DNS         DNSSpec    `yaml:"dns"`
	Wizard      WizardSpec `yaml:"wizard"`
	Steps       []Step     `yaml:"steps"`
}

type DNSSpec struct {
	Records []DNSRecordSpec `yaml:"records"`
}

// DNSRecordSpec is app-defined DNS desired state (provider-specific application happens elsewhere).
// Name can be a full hostname or a template using `{opts.Domain}` and `{opts.ServerIP}`.
type DNSRecordSpec struct {
	Type    string `yaml:"type"`    // A, AAAA, CNAME, etc.
	Name    string `yaml:"name"`    // record name (hostname)
	Content string `yaml:"content"` // optional; defaults to server IP for A/AAAA
	TTL     int    `yaml:"ttl"`     // 0 means provider default
	Proxied *bool  `yaml:"proxied"` // nil means "use global default"
}

type WizardSpec struct {
	DomainHint string      `yaml:"domain_hint"`
	Steps      WizardSteps `yaml:"steps"`
}

type WizardSteps struct {
	Application WizardApplicationStep `yaml:"application"`
}

type WizardApplicationStep struct {
	CustomQuestions []WizardQuestionSpec `yaml:"custom_questions"`
}

type WizardQuestionSpec struct {
	ID       string             `yaml:"id"`
	Name     string             `yaml:"name"`
	Type     string             `yaml:"type"` // boolean | text | choice
	Default  interface{}        `yaml:"default"`
	Required bool               `yaml:"required"`
	Choices  []WizardChoiceSpec `yaml:"choices"`
}

type WizardChoiceSpec struct {
	Name    string      `yaml:"name"`
	Default interface{} `yaml:"default"`
}

type SpecHW struct {
	CPU  int    `yaml:"cpu"`
	RAM  SizeMB `yaml:"ram"`
	Disk SizeGB `yaml:"disk"`
}

type Step struct {
	Name  string  `yaml:"name"`
	In    string  `yaml:"in"` // Where to run the step (e.g., "machine")
	If    string  `yaml:"if"`
	Run   string  `yaml:"run"`
	TTY   TTYSpec `yaml:"tty"` // Run step in a PTY (interactive/TUI)
	Sleep string  `yaml:"sleep"`
	Log   string  `yaml:"log"`
}

type TTYSpec struct {
	Enabled    bool        `yaml:"-"`
	AutoAnswer []TTYAnswer `yaml:"auto_answer"`
}

type TTYAnswer struct {
	// Value is a string that will be rendered with template vars and then sent to the PTY.
	// If it doesn't contain a newline/CR, we will append Enter automatically.
	Value string `yaml:"value"`
	// WaitFor waits until the PTY output contains this substring (or regex, see WaitForRegex) before sending.
	WaitFor string `yaml:"wait_for"`
	// WaitForRegex treats WaitFor as a regexp when true.
	WaitForRegex bool `yaml:"wait_for_regex"`
	// TimeoutMS is max time to wait for WaitFor before giving up on this answer (0 => default 10min).
	TimeoutMS int `yaml:"timeout_ms"`
	// DelayMS waits before sending this answer (optional).
	DelayMS int `yaml:"delay_ms"`
}

// UnmarshalYAML allows:
//   - tty: true
//   - tty: false
//   - tty:
//     auto_answer: ...
func (t *TTYSpec) UnmarshalYAML(node *yaml.Node) error {
	// Reset
	*t = TTYSpec{}
	if node == nil {
		return nil
	}

	// Scalar bool form
	if node.Kind == yaml.ScalarNode {
		var b bool
		if err := node.Decode(&b); err == nil {
			t.Enabled = b
			return nil
		}
	}

	// Mapping form
	type raw TTYSpec
	var tmp raw
	if err := node.Decode(&tmp); err != nil {
		return err
	}
	*t = TTYSpec(tmp)
	t.Enabled = true
	return nil
}

type Loader struct {
	once sync.Once
	spec Spec
	err  error
}

func (l *Loader) Load(data []byte) (Spec, error) {
	l.once.Do(func() {
		spec, err := LoadSpec(data)
		if err != nil {
			l.err = err
			return
		}
		l.spec = spec
	})
	return l.spec, l.err
}

func LoadSpec(data []byte) (Spec, error) {
	var spec Spec
	if len(data) == 0 {
		return spec, fmt.Errorf("empty DSL data")
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&spec); err != nil {
		return spec, err
	}
	return spec, nil
}

func RenderTemplate(input string, vars map[string]string) string {
	out := input
	for key, val := range vars {
		out = strings.ReplaceAll(out, key, val)
	}
	return out
}

func EvaluateCondition(expr string, bools map[string]bool) bool {
	parts := strings.Split(expr, "||")
	for _, part := range parts {
		if evalAnd(strings.TrimSpace(part), bools) {
			return true
		}
	}
	return false
}

func evalAnd(expr string, bools map[string]bool) bool {
	parts := strings.Split(expr, "&&")
	for _, part := range parts {
		if !evalToken(strings.TrimSpace(part), bools) {
			return false
		}
	}
	return true
}

func evalToken(token string, bools map[string]bool) bool {
	if token == "" {
		return false
	}
	negate := false
	if strings.HasPrefix(token, "!") {
		negate = true
		token = strings.TrimSpace(strings.TrimPrefix(token, "!"))
	}
	value := bools[token]
	if negate {
		return !value
	}
	return value
}

type Runner struct {
	Run         func(string) error
	RunPTY      func(cmd string, onData func([]byte)) (stdin io.WriteCloser, wait func() error, err error)
	Log         func(string)
	Sleep       func(time.Duration)
	Conditional bool
}

func RunSteps(r Runner, steps []Step, vars map[string]string, bools map[string]bool) error {
	for _, step := range steps {
		hasCondition := strings.TrimSpace(step.If) != ""
		if r.Conditional && !hasCondition {
			continue
		}
		if !r.Conditional && hasCondition {
			continue
		}
		if hasCondition && !EvaluateCondition(step.If, bools) {
			continue
		}

		if step.Name != "" && r.Log != nil {
			r.Log("⏳ " + step.Name)
		}
		if step.Log != "" && r.Log != nil {
			r.Log(RenderTemplate(step.Log, vars))
		}
		if step.Sleep != "" && r.Sleep != nil {
			dur, err := ParseDuration(step.Sleep)
			if err != nil {
				return err
			}
			r.Sleep(dur)
		}
		if strings.TrimSpace(step.Run) != "" && r.Run != nil {
			cmd := BuildRunCommand(RenderTemplate(step.Run, vars))
			if step.TTY.Enabled && r.RunPTY != nil {
				_, wait, err := r.RunPTY(cmd, nil)
				if err != nil {
					return err
				}
				if err := wait(); err != nil {
					return err
				}
			} else {
				if err := r.Run(cmd); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func RunStepsWithConfig(r Runner, steps []Step, config interface{}, conditional bool) error {
	r.Conditional = conditional
	vars := BuildVarsFromStruct(config)
	bools := BuildBoolsFromStruct(config)
	return RunSteps(r, steps, vars, bools)
}

func BuildRunCommand(script string) string {
	script = strings.TrimSpace(script)
	if script == "" {
		return ""
	}
	script = "set -e\n" + script
	return "bash -lc " + shellQuote(script)
}

func shellQuote(input string) string {
	return "'" + strings.ReplaceAll(input, "'", `'"'"'`) + "'"
}

func ParseDuration(input string) (time.Duration, error) {
	value := strings.TrimSpace(strings.ToLower(input))
	if value == "" {
		return 0, fmt.Errorf("invalid sleep duration")
	}
	if strings.HasSuffix(value, "s") || strings.HasSuffix(value, "m") || strings.HasSuffix(value, "h") {
		return time.ParseDuration(value)
	}
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid sleep duration: %s", input)
	}
	return time.Duration(seconds) * time.Second, nil
}

func ParseSizeToMB(input string) int {
	value := strings.TrimSpace(strings.ToLower(input))
	if value == "" {
		return 0
	}
	if strings.HasSuffix(value, "gib") {
		num, _ := strconv.Atoi(strings.TrimSuffix(value, "gib"))
		return num * 1024
	}
	if strings.HasSuffix(value, "gb") {
		num, _ := strconv.Atoi(strings.TrimSuffix(value, "gb"))
		return num * 1024
	}
	if strings.HasSuffix(value, "mb") {
		num, _ := strconv.Atoi(strings.TrimSuffix(value, "mb"))
		return num
	}
	num, _ := strconv.Atoi(value)
	return num
}

func ParseSizeToGB(input string) int {
	value := strings.TrimSpace(strings.ToLower(input))
	if value == "" {
		return 0
	}
	if strings.HasSuffix(value, "gib") {
		num, _ := strconv.Atoi(strings.TrimSuffix(value, "gib"))
		return num
	}
	if strings.HasSuffix(value, "gb") {
		num, _ := strconv.Atoi(strings.TrimSuffix(value, "gb"))
		return num
	}
	if strings.HasSuffix(value, "mb") {
		num, _ := strconv.Atoi(strings.TrimSuffix(value, "mb"))
		return num / 1024
	}
	num, _ := strconv.Atoi(value)
	return num
}

func BuildVarsFromStruct(config interface{}) map[string]string {
	vars := map[string]string{}
	v := reflect.ValueOf(config)
	if !v.IsValid() {
		return vars
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return vars
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return vars
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)
		if !fv.IsValid() {
			continue
		}
		switch fv.Kind() {
		case reflect.String:
			vars[fmt.Sprintf("{opts.%s}", field.Name)] = fv.String()
		case reflect.Bool:
			vars[fmt.Sprintf("{opts.%s}", field.Name)] = strconv.FormatBool(fv.Bool())
		}
	}
	return vars
}

func BuildBoolsFromStruct(config interface{}) map[string]bool {
	bools := map[string]bool{}
	v := reflect.ValueOf(config)
	if !v.IsValid() {
		return bools
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return bools
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return bools
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)
		if !fv.IsValid() {
			continue
		}
		switch fv.Kind() {
		case reflect.Bool:
			bools[fmt.Sprintf("opts.%s", field.Name)] = fv.Bool()
		case reflect.String:
			bools[fmt.Sprintf("opts.%s", field.Name)] = strings.TrimSpace(fv.String()) != ""
		}
	}
	return bools
}

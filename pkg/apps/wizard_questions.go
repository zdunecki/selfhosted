package apps

// WizardQuestion is a UI-only question definition to help users configure interactive installers.
// The wizard can render these questions before deployment, and (optionally) auto-answer a PTY step.
type WizardQuestion struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Type     string         `json:"type"` // boolean | text | choice
	Required bool           `json:"required"`
	Default  interface{}    `json:"default,omitempty"`
	Choices  []WizardChoice `json:"choices,omitempty"`
}

type WizardChoice struct {
	Name    string      `json:"name"`
	Default interface{} `json:"default,omitempty"`
}

// WizardProvider is an optional interface apps can implement to expose wizard question definitions.
type WizardProvider interface {
	WizardQuestions() []WizardQuestion
}

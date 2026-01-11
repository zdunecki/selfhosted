package apps

import (
	"embed"
	"fmt"
	"strings"

	"github.com/zdunecki/selfhosted/pkg/dsl"
	"gopkg.in/yaml.v3"
)

// apps.yaml + all referenced *.yaml live in this directory.
//
// Adding a new basic app should require only:
// - adding <app>.yaml next to apps.yaml
// - listing it in apps.yaml
//
//go:embed apps.yaml *.yaml
var appsFS embed.FS

type appsList struct {
	Apps []string `yaml:"apps"`
}

func init() {
	if err := registerAppsFromYAML(); err != nil {
		// Keep behavior consistent with other registries: fail fast on bad registry state.
		// This is a startup/config error.
		panic(err)
	}
}

func registerAppsFromYAML() error {
	data, err := appsFS.ReadFile("apps.yaml")
	if err != nil {
		return fmt.Errorf("apps registry: read apps.yaml: %w", err)
	}

	var list appsList
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&list); err != nil {
		return fmt.Errorf("apps registry: parse apps.yaml: %w", err)
	}

	if len(list.Apps) == 0 {
		return fmt.Errorf("apps registry: apps.yaml has no entries")
	}

	for _, filename := range list.Apps {
		filename = strings.TrimSpace(filename)
		if filename == "" {
			continue
		}

		appData, err := appsFS.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("apps registry: read %s: %w", filename, err)
		}

		spec, err := dsl.LoadSpec(appData)
		if err != nil {
			return fmt.Errorf("apps registry: parse %s: %w", filename, err)
		}

		app := NewDSLApp(spec)
		if strings.TrimSpace(app.Name()) == "" || app.Name() == "unknown" {
			return fmt.Errorf("apps registry: %s has no 'app' name", filename)
		}

		// If a custom Go implementation registered the same name earlier, keep it.
		// If it registers later, it will overwrite this entry via Register().
		if _, exists := Registry[app.Name()]; exists {
			continue
		}
		Register(app)
	}

	return nil
}

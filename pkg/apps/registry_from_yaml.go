package apps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zdunecki/selfhosted/pkg/dsl"
	"gopkg.in/yaml.v3"
)

// apps.yaml + all referenced *.yaml live in marketplace/ directory at the repo root.
//
// Adding a new basic app should require only:
// - adding <app>.yaml in marketplace/apps/
// - listing it in marketplace/apps.yaml
//
// The marketplace directory is resolved relative to the executable or working directory.

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

func findMarketplaceDir() (string, error) {
	// Try 1: Current working directory (for development)
	if cwd, err := os.Getwd(); err == nil {
		marketplaceDir := filepath.Join(cwd, "marketplace")
		if _, err := os.Stat(filepath.Join(marketplaceDir, "apps.yaml")); err == nil {
			return marketplaceDir, nil
		}
	}

	// Try 2: Relative to executable (for production binaries)
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		// Try same directory as executable
		marketplaceDir := filepath.Join(exeDir, "marketplace")
		if _, err := os.Stat(filepath.Join(marketplaceDir, "apps.yaml")); err == nil {
			return marketplaceDir, nil
		}
		// Try one level up (common for Go builds)
		marketplaceDir = filepath.Join(exeDir, "..", "marketplace")
		if _, err := os.Stat(filepath.Join(marketplaceDir, "apps.yaml")); err == nil {
			return marketplaceDir, nil
		}
	}

	return "", fmt.Errorf("marketplace directory not found (looked in cwd/marketplace and exe/marketplace)")
}

func registerAppsFromYAML() error {
	marketplaceDir, err := findMarketplaceDir()
	if err != nil {
		return fmt.Errorf("apps registry: %w", err)
	}

	appsYAMLPath := filepath.Join(marketplaceDir, "apps.yaml")
	data, err := os.ReadFile(appsYAMLPath)
	if err != nil {
		return fmt.Errorf("apps registry: read %s: %w", appsYAMLPath, err)
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

	appsDir := filepath.Join(marketplaceDir, "apps")
	for _, filename := range list.Apps {
		filename = strings.TrimSpace(filename)
		if filename == "" {
			continue
		}

		appPath := filepath.Join(appsDir, filename)
		appData, err := os.ReadFile(appPath)
		if err != nil {
			return fmt.Errorf("apps registry: read %s: %w", appPath, err)
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

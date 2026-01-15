package terraform

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tfexec "github.com/hashicorp/terraform-exec/tfexec"
)

const (
	defaultTerraformVersion = "1.6.6"
)

type OutputValue struct {
	Value     interface{} `json:"value"`
	Sensitive bool        `json:"sensitive"`
	Type      interface{} `json:"type"`
}

type ApplyResult struct {
	WorkDir string
	Outputs map[string]OutputValue
}

func FindModuleDir(provider, profile string) (string, error) {
	candidates := []string{}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "marketplace", "terraform"))
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(exeDir, "marketplace", "terraform"))
		candidates = append(candidates, filepath.Join(exeDir, "..", "marketplace", "terraform"))
	}

	for _, root := range candidates {
		moduleDir := filepath.Join(root, provider, profile)
		if _, err := os.Stat(filepath.Join(moduleDir, "main.tf")); err == nil {
			return moduleDir, nil
		}
	}

	return "", fmt.Errorf("terraform module not found for %s/%s", provider, profile)
}

func Apply(ctx context.Context, moduleDir, runID string, env map[string]string, vars map[string]interface{}) (*ApplyResult, error) {
	terraformPath, err := ensureTerraformBinary()
	if err != nil {
		return nil, err
	}

	workDir, err := prepareWorkDir(moduleDir, runID)
	if err != nil {
		return nil, err
	}

	tf, err := tfexec.NewTerraform(workDir, terraformPath)
	if err != nil {
		return nil, fmt.Errorf("terraform init: %w", err)
	}

	if err := tf.SetEnv(mergeEnvMap(env, nil)); err != nil {
		return nil, fmt.Errorf("terraform set env: %w", err)
	}

	if err := tf.Init(ctx, tfexec.Upgrade(true)); err != nil {
		// Try to get stderr output for better error messages
		return nil, fmt.Errorf("terraform init: %w (workDir: %s)", err, workDir)
	}

	var applyOpts []tfexec.ApplyOption
	for key, value := range vars {
		applyOpts = append(applyOpts, tfexec.Var(formatVar(key, value)))
	}

	if err := tf.Apply(ctx, applyOpts...); err != nil {
		return nil, fmt.Errorf("terraform apply: %w (workDir: %s, check terraform logs)", err, workDir)
	}

	outputs, err := readOutputs(terraformPath, workDir, env)
	if err != nil {
		return nil, err
	}

	return &ApplyResult{
		WorkDir: workDir,
		Outputs: outputs,
	}, nil
}

func Destroy(ctx context.Context, workDir string, env map[string]string) error {
	terraformPath, err := ensureTerraformBinary()
	if err != nil {
		return err
	}

	tf, err := tfexec.NewTerraform(workDir, terraformPath)
	if err != nil {
		return fmt.Errorf("terraform init: %w", err)
	}

	if err := tf.SetEnv(mergeEnvMap(env, nil)); err != nil {
		return fmt.Errorf("terraform set env: %w", err)
	}

	if err := tf.Init(ctx, tfexec.Upgrade(true)); err != nil {
		return fmt.Errorf("terraform init: %w", err)
	}

	if err := tf.Destroy(ctx); err != nil {
		return fmt.Errorf("terraform destroy: %w", err)
	}

	return nil
}

func OutputString(outputs map[string]OutputValue, key string) (string, bool) {
	out, ok := outputs[key]
	if !ok || out.Value == nil {
		return "", false
	}
	switch v := out.Value.(type) {
	case string:
		return v, true
	case fmt.Stringer:
		return v.String(), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func readOutputs(terraformPath, workDir string, env map[string]string) (map[string]OutputValue, error) {
	cmd := execCommand(terraformPath, "output", "-json")
	cmd.Dir = workDir
	cmd.Env = mergeEnv(env, nil)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("terraform output: %w", err)
	}

	var outputs map[string]OutputValue
	if err := json.Unmarshal(out, &outputs); err != nil {
		return nil, fmt.Errorf("terraform output parse: %w", err)
	}

	return outputs, nil
}

func formatVar(key string, value interface{}) string {
	switch v := value.(type) {
	case string:
		// Strings are passed directly without JSON encoding
		return fmt.Sprintf("%s=%s", key, v)
	case bool:
		return fmt.Sprintf("%s=%t", key, v)
	case int, int64, float64, float32:
		return fmt.Sprintf("%s=%v", key, v)
	default:
		// Complex types (slices, maps) need JSON encoding
		encoded, _ := json.Marshal(v)
		return fmt.Sprintf("%s=%s", key, string(encoded))
	}
}

func prepareWorkDir(moduleDir, runID string) (string, error) {
	workRoot, err := terraformWorkRoot()
	if err != nil {
		return "", err
	}

	safeRunID := sanitizeRunID(runID)
	if safeRunID == "" {
		safeRunID = fmt.Sprintf("run-%d", time.Now().Unix())
	}

	workDir := filepath.Join(workRoot, safeRunID)
	if err := os.RemoveAll(workDir); err != nil {
		return "", err
	}
	if err := copyDir(moduleDir, workDir); err != nil {
		return "", err
	}

	return workDir, nil
}

func terraformWorkRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".selfhosted", "terraform", "runs"), nil
}

func sanitizeRunID(input string) string {
	var b strings.Builder
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == ".terraform" {
			continue
		}
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	if info, err := os.Stat(src); err == nil {
		_ = os.Chmod(dst, info.Mode())
	}

	return nil
}

func ensureTerraformBinary() (string, error) {
	if override := os.Getenv("SELFHOSTED_TERRAFORM_BIN"); override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
		return "", fmt.Errorf("SELFHOSTED_TERRAFORM_BIN not found at %s", override)
	}

	binPath, err := terraformBinPath()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	if err := downloadTerraformBinary(defaultTerraformVersion, binPath); err != nil {
		return "", err
	}

	return binPath, nil
}

func terraformBinPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	name := "terraform"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	return filepath.Join(home, ".selfhosted", "bin", name), nil
}

func downloadTerraformBinary(version, target string) error {
	osArch, err := terraformReleaseArch()
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_%s.zip", version, version, osArch)

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download terraform: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download terraform: unexpected status %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp("", "terraform-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	return unzipTerraform(tmpFile.Name(), target)
}

func terraformReleaseArch() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			return "darwin_amd64", nil
		case "arm64":
			return "darwin_arm64", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "linux_amd64", nil
		case "arm64":
			return "linux_arm64", nil
		}
	case "windows":
		if runtime.GOARCH == "amd64" {
			return "windows_amd64", nil
		}
	}
	return "", errors.New("unsupported platform for terraform binary")
}

func unzipTerraform(zipPath, target string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, "terraform") || strings.HasSuffix(file.Name, "terraform.exe") {
			return unzipFile(file, target)
		}
	}
	return fmt.Errorf("terraform binary not found in zip")
}

func unzipFile(file *zip.File, target string) error {
	in, err := file.Open()
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return nil
}

func mergeEnv(overrides map[string]string, additions map[string]string) []string {
	base := os.Environ()
	out := make([]string, 0, len(base)+len(overrides)+len(additions))
	out = append(out, base...)
	for k, v := range overrides {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range additions {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}

// reservedTFVars are environment variables managed by terraform-exec internally.
// Setting these manually via SetEnv will cause an error.
var reservedTFVars = map[string]bool{
	"TF_INPUT":         true,
	"TF_IN_AUTOMATION": true,
}

func mergeEnvMap(overrides map[string]string, additions map[string]string) map[string]string {
	out := make(map[string]string)
	for _, kv := range os.Environ() {
		if idx := strings.Index(kv, "="); idx != -1 {
			key := kv[:idx]
			if reservedTFVars[key] {
				continue
			}
			out[key] = kv[idx+1:]
		}
	}
	for k, v := range overrides {
		if reservedTFVars[k] {
			continue
		}
		// If override value is empty string and key exists in base env, remove it instead of setting to empty
		// This helps with providers that check for variable existence rather than value
		if v == "" {
			delete(out, k)
		} else {
			out[k] = v
		}
	}
	for k, v := range additions {
		if reservedTFVars[k] {
			continue
		}
		if v == "" {
			delete(out, k)
		} else {
			out[k] = v
		}
	}
	return out
}

func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

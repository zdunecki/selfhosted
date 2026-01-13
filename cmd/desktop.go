package cmd

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var ErrDesktopUnavailable = errors.New("desktop wrapper unavailable")

func resolveDesktopDir() (string, error) {
	// 1) Prefer current working directory (repo root).
	if cwd, err := os.Getwd(); err == nil {
		desktopDir := filepath.Join(cwd, "desktop")
		if _, err := os.Stat(filepath.Join(desktopDir, "neutralino.config.json")); err == nil {
			return desktopDir, nil
		}
	}

	// 2) Try relative to executable (useful when running from a build output dir).
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		desktopDir := filepath.Join(exeDir, "desktop")
		if _, err := os.Stat(filepath.Join(desktopDir, "neutralino.config.json")); err == nil {
			return desktopDir, nil
		}
		// Common in Go builds: binary lives in repo root or ./bin; try one level up.
		desktopDir = filepath.Join(exeDir, "..", "desktop")
		if _, err := os.Stat(filepath.Join(desktopDir, "neutralino.config.json")); err == nil {
			return desktopDir, nil
		}
	}

	return "", fmt.Errorf("%w: could not find desktop/neutralino.config.json (run from repo root)", ErrDesktopUnavailable)
}

func launchDesktop() error {
	desktopDir, err := resolveDesktopDir()
	if err != nil {
		return err
	}

	neu, err := exec.LookPath("neu")
	if err != nil {
		return fmt.Errorf("%w: Neutralino CLI (neu) not found in PATH", ErrDesktopUnavailable)
	}

	// Pick an available local port for the backend to avoid conflicts.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to pick a free port: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	backendURL := fmt.Sprintf("http://127.0.0.1:%d/", port)

	// Start backend (serve mode) as a child process so the Neutralino webview can load it.
	// We intentionally keep the backend separate from the desktop window process.
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	backend := exec.Command(exe, "serve", "--port", strconv.Itoa(port), "--no-browser=true")
	backend.Stdout = os.Stdout
	backend.Stderr = os.Stderr
	backend.Stdin = os.Stdin
	if err := backend.Start(); err != nil {
		return fmt.Errorf("failed to start backend server: %w", err)
	}
	// Ensure backend is stopped when Neutralino exits.
	defer func() {
		_ = backend.Process.Kill()
		_, _ = backend.Process.Wait()
	}()

	// First run usually needs `neu update` to download Neutralino binaries (bin/neutralino-<platform>).
	// If this fails, surface the error since `neu run` will be confusing (ENOENT chmod bin/...).
	update := exec.Command(neu, "update")
	update.Dir = desktopDir
	update.Stdout = os.Stdout
	update.Stderr = os.Stderr
	update.Stdin = os.Stdin
	if err := update.Run(); err != nil {
		return fmt.Errorf("%w: neu update failed: %v", ErrDesktopUnavailable, err)
	}

	// Give backend a moment to bind before opening the window (best-effort).
	time.Sleep(400 * time.Millisecond)

	c := exec.Command(neu, "run")
	c.Dir = desktopDir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	c.Env = append(os.Environ(), "SELFHOSTED_BACKEND_URL="+backendURL)

	// Neutralino runtime may try to bind to port 3000; on macOS dev setups this is usually fine.
	// If users hit port conflicts, they can tweak desktop/neutralino.config.json.
	_ = runtime.GOOS
	return c.Run()
}

// desktopCmd launches the Neutralino desktop wrapper.
// This is intended for development workflows where `neu` is installed.
var desktopCmd = &cobra.Command{
	Use:   "desktop",
	Short: "Launch the Neutralino desktop installer (webview)",
	Long: `Launches the Neutralino desktop wrapper located in ./desktop.

This requires the Neutralino CLI (neu) to be installed.
If you just want the web UI in a browser, use:
  selfhosted serve --port 8080
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return launchDesktop()
	},
}

func init() {
	rootCmd.AddCommand(desktopCmd)
}

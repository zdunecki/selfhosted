package utils

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHRunner handles SSH connections and command execution
type SSHRunner struct {
	host       string
	user       string
	privateKey string
	client     *ssh.Client
}

// NewSSHRunner creates a new SSH runner
func NewSSHRunner(host, user, privateKey string) *SSHRunner {
	return &SSHRunner{
		host:       host,
		user:       user,
		privateKey: privateKey,
	}
}

// Connect establishes SSH connection
func (r *SSHRunner) Connect() error {
	signer, err := ssh.ParsePrivateKey([]byte(r.privateKey))
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: r.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", r.host), config)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	r.client = client
	return nil
}

// Close closes the SSH connection
func (r *SSHRunner) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// Run executes a single command
func (r *SSHRunner) Run(command string) error {
	session, err := r.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Set up output streaming
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	fmt.Printf("Running: %s\n", command)
	if err := session.Run(command); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// RunMultiple executes multiple commands sequentially
func (r *SSHRunner) RunMultiple(commands []string) error {
	for _, cmd := range commands {
		if err := r.Run(cmd); err != nil {
			return err
		}
	}
	return nil
}

// RunWithOutput executes a command and returns its output
func (r *SSHRunner) RunWithOutput(command string) (string, error) {
	session, err := r.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdout strings.Builder
	session.Stdout = io.MultiWriter(&stdout, os.Stdout)
	session.Stderr = os.Stderr

	fmt.Printf("Running: %s\n", command)
	if err := session.Run(command); err != nil {
		return "", fmt.Errorf("command failed: %w", err)
	}

	return stdout.String(), nil
}

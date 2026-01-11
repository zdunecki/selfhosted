package utils

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHRunner handles SSH connections and command execution
type SSHRunner struct {
	host       string
	user       string
	privateKey string
	client     *ssh.Client
	logger     func(string, ...interface{}) // Optional logger for streaming output
}

// NewSSHRunner creates a new SSH runner
func NewSSHRunner(host, user, privateKey string) *SSHRunner {
	return &SSHRunner{
		host:       host,
		user:       user,
		privateKey: privateKey,
	}
}

// SetLogger sets an optional logger function for capturing command output
func (r *SSHRunner) SetLogger(logger func(string, ...interface{})) {
	r.logger = logger
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

	if r.logger != nil {
		// Use logger to capture output
		r.logger("Running: %s\n", command)

		// Create a writer that streams to logger
		stdoutWriter := &streamWriter{logger: r.logger}
		stderrWriter := &streamWriter{logger: r.logger}

		session.Stdout = io.MultiWriter(stdoutWriter, os.Stdout)
		session.Stderr = io.MultiWriter(stderrWriter, os.Stderr)

		err := session.Run(command)

		// Flush any remaining buffer
		stdoutWriter.Flush()
		stderrWriter.Flush()

		if err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
	} else {
		// Fallback to original behavior
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr
		fmt.Printf("Running: %s\n", command)
		if err := session.Run(command); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
	}

	return nil
}

// streamWriter is a writer that streams output line by line to a logger
type streamWriter struct {
	logger func(string, ...interface{})
	buffer []byte
}

func (w *streamWriter) Write(p []byte) (n int, err error) {
	w.buffer = append(w.buffer, p...)

	// Process complete lines
	for {
		newlineIndex := -1
		for i, b := range w.buffer {
			if b == '\n' {
				newlineIndex = i
				break
			}
		}

		if newlineIndex == -1 {
			// No complete line yet
			break
		}

		// Extract and log the line
		line := string(w.buffer[:newlineIndex])
		w.buffer = w.buffer[newlineIndex+1:]
		if strings.TrimSpace(line) != "" {
			w.logger("%s\n", line)
		}
	}

	return len(p), nil
}

// Flush logs any remaining buffer content
func (w *streamWriter) Flush() {
	if len(w.buffer) > 0 {
		line := string(w.buffer)
		if strings.TrimSpace(line) != "" {
			w.logger("%s\n", line)
		}
		w.buffer = w.buffer[:0]
	}
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

// RunPTY executes a command in a PTY, suitable for interactive/TUI installers.
// It streams raw output (including ANSI escape codes) to onData if provided.
// Returns stdin writer to send user keystrokes, and a wait func to wait for completion.
func (r *SSHRunner) RunPTY(command string, onData func([]byte)) (io.WriteCloser, func() error, error) {
	session, err := r.client.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Request a PTY so interactive prompts render correctly.
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", 40, 120, modes); err != nil {
		_ = session.Close()
		return nil, nil, fmt.Errorf("failed to request pty: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		return nil, nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		_ = session.Close()
		return nil, nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start command
	if err := session.Start(command); err != nil {
		_ = session.Close()
		return nil, nil, fmt.Errorf("failed to start command: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	readPipe := func(rdr io.Reader) {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := rdr.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				if onData != nil {
					// copy because buf is reused
					tmp := make([]byte, len(chunk))
					copy(tmp, chunk)
					onData(tmp)
				}
				// also mirror to stdout for debugging if desired
				if r.logger != nil {
					// best-effort: log raw bytes as string (may include ansi)
					r.logger("%s", string(chunk))
				}
			}
			if err != nil {
				return
			}
		}
	}

	go readPipe(stdout)
	go readPipe(stderr)

	wait := func() error {
		err := session.Wait()
		wg.Wait()
		_ = stdin.Close()
		_ = session.Close()
		if err != nil {
			// Include command for context, but avoid huge strings
			cmdPreview := command
			if len(cmdPreview) > 200 {
				cmdPreview = cmdPreview[:200] + "..."
			}
			return fmt.Errorf("pty command failed (%s): %w", strings.TrimSpace(cmdPreview), err)
		}
		return nil
	}

	// Wrap stdin so callers can Write([]byte) easily
	return nopWriteCloser{Writer: stdin}, wait, nil
}

type nopWriteCloser struct{ io.Writer }

func (n nopWriteCloser) Close() error { return nil }

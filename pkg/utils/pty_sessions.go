package utils

import (
	"encoding/base64"
	"fmt"
	"io"
	"sync"
)

// Very small in-memory registry for interactive PTY sessions.
// This enables a "send keys from UI" flow without introducing websocket deps yet.

var (
	ptyMu       sync.RWMutex
	ptySessions = map[string]io.WriteCloser{}
)

func RegisterPTY(sessionID string, stdin io.WriteCloser) {
	ptyMu.Lock()
	defer ptyMu.Unlock()
	ptySessions[sessionID] = stdin
}

func ClosePTY(sessionID string) {
	ptyMu.Lock()
	defer ptyMu.Unlock()
	if w, ok := ptySessions[sessionID]; ok {
		_ = w.Close()
		delete(ptySessions, sessionID)
	}
}

func WritePTYBase64(sessionID string, b64 string) error {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("invalid base64: %w", err)
	}

	ptyMu.RLock()
	w, ok := ptySessions[sessionID]
	ptyMu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown PTY session: %s", sessionID)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	return nil
}

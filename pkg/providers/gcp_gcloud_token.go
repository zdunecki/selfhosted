package providers

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// gcloudTokenSource shells out to `gcloud auth print-access-token`.
// This allows using a developer's gcloud user login without ADC.
//
// Notes:
// - Tokens are short-lived; we assume ~1h and refresh proactively.
// - This is a convenience fallback; production should prefer ADC or service accounts.
type gcloudTokenSource struct {
	mu  sync.Mutex
	tok *oauth2.Token
}

func (s *gcloudTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Reuse cached token if it’s still valid (with a small safety window).
	if s.tok != nil && s.tok.Expiry.After(time.Now().Add(2*time.Minute)) {
		return s.tok, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gcloud", "auth", "print-access-token")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("gcloud auth print-access-token failed: %s", msg)
	}

	token := strings.TrimSpace(stdout.String())
	if token == "" {
		return nil, fmt.Errorf("gcloud returned empty access token")
	}

	// gcloud access tokens are typically valid for 1 hour.
	s.tok = &oauth2.Token{
		AccessToken: token,
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(55 * time.Minute),
	}
	return s.tok, nil
}

func canUseGcloudUserToken() bool {
	// Quick check: does `gcloud auth print-access-token` succeed?
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gcloud", "auth", "print-access-token")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

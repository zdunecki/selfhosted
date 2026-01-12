package providers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUpCloudCreateServer_ConfigFromLog(t *testing.T) {
	if os.Getenv("SELFHOSTED_UPCLOUD_ITEST") != "1" {
		t.Skip("set SELFHOSTED_UPCLOUD_ITEST=1 to run (creates real UpCloud resources)")
	}

	pub, err := loadDefaultSSHPublicKey()
	if err != nil {
		t.Skipf("no SSH public key found: %v", err)
	}

	p := NewUpCloud()

	// Preflight visibility: list templates and print a few to help debug accounts that can't see public templates.
	st, terr := p.getTemplateStoragesForZone("nl-ams1")
	if terr != nil {
		t.Fatalf("template list failed: %v", terr)
	}
	t.Logf("template storages fetched: %d", len(st.Storages))

	cfg := &DeployConfig{
		Name:         "selfhosted-itest-" + time.Now().UTC().Format("20060102-150405"),
		Region:       "nl-ams1",
		Size:         "2xCPU-2GB",
		SSHPublicKey: pub,
		Tags:         []string{"selfhosted", "itest"},
	}

	srv, err := p.CreateServer(cfg)
	if err != nil {
		t.Fatalf("CreateServer failed: %v", err)
	}
	t.Logf("created server id=%s name=%s status=%s", srv.ID, srv.Name, srv.Status)

	// Always attempt cleanup.
	defer func() {
		if derr := p.DestroyServer(srv.ID); derr != nil {
			t.Logf("DestroyServer failed (manual cleanup may be needed): %v", derr)
		} else {
			t.Log("server destroyed")
		}
	}()

	// Optional: wait for running + IP (slower, but verifies full flow).
	if os.Getenv("SELFHOSTED_UPCLOUD_ITEST_WAIT") == "1" {
		ready, err := p.WaitForServer(srv.ID)
		if err != nil {
			t.Fatalf("WaitForServer failed: %v", err)
		}
		t.Logf("server ready ip=%s status=%s", ready.IP, ready.Status)
	}
}

func loadDefaultSSHPublicKey() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cands := []string{
		filepath.Join(home, ".ssh", "id_ed25519.pub"),
		filepath.Join(home, ".ssh", "id_rsa.pub"),
	}
	for _, p := range cands {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(b))
		if s != "" {
			return s, nil
		}
	}
	return "", os.ErrNotExist
}

//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestPrivateKeyReadsRejectFIFOWithoutBlocking(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "AuthKey.p8")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Fatalf("Mkfifo() error: %v", err)
	}

	tests := []struct {
		name string
		call func() error
	}{
		{name: "validate", call: func() error { return ValidateKeyFile(fifo) }},
		{name: "load", call: func() error { _, err := LoadPrivateKey(fifo); return err }},
		{name: "storage comparison read", call: func() error { _, err := loadPrivateKeyPEMForStorage(fifo); return err }},
		{name: "migration existing path", call: func() error {
			_, _, err := migrationPrivateKeyPath(Credential{Name: "fifo", PrivateKeyPath: fifo}, filepath.Join(t.TempDir(), "exports"), "fifo")
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.call()
			if err == nil {
				t.Fatal("expected FIFO rejection, got nil")
			}
			if !strings.Contains(err.Error(), "not a regular file") {
				t.Fatalf("error = %v, want regular-file rejection", err)
			}
		})
	}
}

func TestConfigLoadRejectsFIFOWithoutBlocking(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "config.json")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Fatalf("Mkfifo() error: %v", err)
	}

	_, err := config.LoadAt(fifo)
	if err == nil {
		t.Fatal("expected FIFO rejection, got nil")
	}
	if !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("LoadAt() error = %v, want regular-file rejection", err)
	}
}

func TestConfigLoadUsesDarwinVarAliasWhenTMPDIRChanges(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("/var is a system symlink on Darwin")
	}
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"key_id":"KEY123"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(config) error: %v", err)
	}
	blockedTemp := filepath.Join(tempDir, "not-a-directory")
	if err := os.WriteFile(blockedTemp, []byte("blocked"), 0o600); err != nil {
		t.Fatalf("WriteFile(blocked TMPDIR) error: %v", err)
	}
	t.Setenv("TMPDIR", blockedTemp)

	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("LoadAt(%q) error after TMPDIR change: %v", configPath, err)
	}
	if cfg.KeyID != "KEY123" {
		t.Fatalf("LoadAt(%q) key ID = %q, want KEY123", configPath, cfg.KeyID)
	}
}

func TestMigrationScanIgnoresFIFO(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	if err := os.Mkdir(fastlaneDir, 0o755); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if err := unix.Mkfifo(filepath.Join(fastlaneDir, "Fastfile"), 0o600); err != nil {
		t.Fatalf("Mkfifo() error: %v", err)
	}

	signals := scanMigrationSignals(root)
	if len(signals.detectedFiles) != 0 {
		t.Fatalf("detectedFiles = %#v, want no FIFO files", signals.detectedFiles)
	}
}

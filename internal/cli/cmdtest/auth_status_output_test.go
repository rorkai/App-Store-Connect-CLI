package cmdtest

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestAuthStatusOutputJSON(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)

	cfg := &config.Config{
		DefaultKeyName: "default",
		Keys: []config.Credential{
			{
				Name:           "default",
				KeyID:          "KEY123",
				IssuerID:       "ISS456",
				PrivateKeyPath: keyPath,
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"auth", "status", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		StorageBackend string `json:"storageBackend"`
		Credentials    []struct {
			Name      string `json:"name"`
			KeyID     string `json:"keyId"`
			IsDefault bool   `json:"isDefault"`
			StoredIn  string `json:"storedIn"`
		} `json:"credentials"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to unmarshal auth status json: %v; stdout=%q", err, stdout)
	}
	if payload.StorageBackend != "Config File" {
		t.Fatalf("expected storage backend %q, got %q", "Config File", payload.StorageBackend)
	}
	if len(payload.Credentials) != 1 {
		t.Fatalf("expected one credential, got %d", len(payload.Credentials))
	}
	if payload.Credentials[0].Name != "default" || payload.Credentials[0].KeyID != "KEY123" || !payload.Credentials[0].IsDefault {
		t.Fatalf("unexpected credential payload: %+v", payload.Credentials[0])
	}
}

func TestAuthStatusOutputInvalidReturnsExitUsage(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"auth", "status", "--output", "yaml"}, "1.0.0")
		if code != cmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
		}
	})
	if !strings.Contains(stderr, "unsupported format: yaml") {
		t.Fatalf("expected stderr to contain unsupported format error, got %q", stderr)
	}
}

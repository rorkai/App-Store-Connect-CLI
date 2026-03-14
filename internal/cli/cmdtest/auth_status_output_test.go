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

func TestAuthStatusDefaultOutputRespectsASCDefaultOutputJSON(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "json")

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
		code = cmd.Run([]string{"auth", "status"}, "1.0.0")
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
			Name  string `json:"name"`
			KeyID string `json:"keyId"`
		} `json:"credentials"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to unmarshal auth status json: %v; stdout=%q", err, stdout)
	}
	if payload.StorageBackend != "Config File" {
		t.Fatalf("expected storage backend %q, got %q", "Config File", payload.StorageBackend)
	}
	if len(payload.Credentials) != 1 || payload.Credentials[0].Name != "default" {
		t.Fatalf("unexpected credentials payload: %+v", payload.Credentials)
	}
}

func TestAuthStatusTableNotesConfigPrecedenceOverEnv(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	envKeyPath := filepath.Join(tempDir, "AuthKey-Env.p8")
	writeECDSAPEM(t, keyPath)
	writeECDSAPEM(t, envKeyPath)

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
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", envKeyPath)
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"auth", "status", "--output", "table"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "stored config credentials are preferred") {
		t.Fatalf("expected config precedence note, got %q", stdout)
	}
	if strings.Contains(stdout, "will be used when no profile is selected") {
		t.Fatalf("expected auth status note to avoid claiming env credentials are preferred, got %q", stdout)
	}
	if strings.Contains(stdout, "ENVKEY") || strings.Contains(stdout, "ENVISS") {
		t.Fatalf("expected redacted env identifiers, got %q", stdout)
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

func TestAuthStatusInvalidBypassWarningPrintedOnce(t *testing.T) {
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

	t.Setenv("ASC_BYPASS_KEYCHAIN", "banana")
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
	if count := strings.Count(stderr, `Warning: invalid ASC_BYPASS_KEYCHAIN value "banana"`); count != 1 {
		t.Fatalf("expected one bypass warning, got %d in %q", count, stderr)
	}
	if !strings.Contains(stderr, "keychain bypass disabled") {
		t.Fatalf("expected conservative bypass warning, got %q", stderr)
	}

	var payload struct {
		StorageBackend string `json:"storageBackend"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to unmarshal auth status json: %v; stdout=%q", err, stdout)
	}
	if payload.StorageBackend == "" {
		t.Fatalf("expected storage backend in auth status output, got empty payload: %q", stdout)
	}
}

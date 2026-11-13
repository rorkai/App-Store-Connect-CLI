package cmdtest

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	authsvc "github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	authcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func writeDefaultAuthConfig(t *testing.T) {
	t.Helper()

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
}

func TestAuthStatusOutputJSON(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		defaultOutput string
	}{
		{name: "explicit output flag", args: []string{"auth", "status", "--output", "json"}},
		{name: "ASC_DEFAULT_OUTPUT", args: []string{"auth", "status"}, defaultOutput: "json"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetDefaultOutput(t)
			t.Setenv("ASC_DEFAULT_OUTPUT", test.defaultOutput)
			writeDefaultAuthConfig(t)

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run(test.args, "1.0.0")
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
		})
	}
}

func TestAuthStatusShowsEnvironmentPrecedenceWithoutValidatingPrivateKey(t *testing.T) {
	restoreSummaries := authcmd.SetListCredentialSummaries(func() ([]authsvc.Credential, error) {
		return []authsvc.Credential{}, nil
	})
	t.Cleanup(restoreSummaries)
	restoreKeychain := authcmd.SetKeychainAvailable(func() (bool, error) {
		return true, nil
	})
	t.Cleanup(restoreKeychain)

	keyPath := filepath.Join(t.TempDir(), "missing.p8")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_TYPE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	const wantNote = "Complete environment credential fields take precedence when no profile is selected; stored credential lookup is skipped."

	var jsonCode int
	jsonOutput, jsonStderr := captureOutput(t, func() {
		jsonCode = cmd.Run([]string{"auth", "status", "--output", "json"}, "1.0.0")
	})
	if jsonCode != cmd.ExitSuccess || jsonStderr != "" {
		t.Fatalf("json status: exit=%d stderr=%q", jsonCode, jsonStderr)
	}
	var payload struct {
		Credentials                    []json.RawMessage `json:"credentials"`
		EnvironmentCredentialsComplete bool              `json:"environmentCredentialsComplete"`
		EnvironmentNote                string            `json:"environmentNote"`
	}
	if err := json.Unmarshal([]byte(jsonOutput), &payload); err != nil {
		t.Fatalf("failed to unmarshal auth status json: %v; stdout=%q", err, jsonOutput)
	}
	if len(payload.Credentials) != 0 || !payload.EnvironmentCredentialsComplete || payload.EnvironmentNote != wantNote {
		t.Fatalf("unexpected environment status payload: %+v", payload)
	}

	var tableCode int
	tableOutput, tableStderr := captureOutput(t, func() {
		tableCode = cmd.Run([]string{"auth", "status", "--output", "table"}, "1.0.0")
	})
	if tableCode != cmd.ExitSuccess || tableStderr != "" {
		t.Fatalf("table status: exit=%d stderr=%q", tableCode, tableStderr)
	}
	if !strings.Contains(tableOutput, "No stored credentials found.") || !strings.Contains(tableOutput, wantNote) {
		t.Fatalf("expected active environment source in table output, got %q", tableOutput)
	}
	if strings.Contains(tableOutput, "Run 'asc auth login'") {
		t.Fatalf("active environment credentials should not prompt for login: %q", tableOutput)
	}
	if strings.Contains(tableOutput, "ENVKEY") {
		t.Fatalf("expected environment key ID to stay redacted: %q", tableOutput)
	}
}

func TestAuthStatusDoesNotMarkInvalidBase64EnvironmentActive(t *testing.T) {
	restoreSummaries := authcmd.SetListCredentialSummaries(func() ([]authsvc.Credential, error) {
		return []authsvc.Credential{}, nil
	})
	t.Cleanup(restoreSummaries)
	restoreKeychain := authcmd.SetKeychainAvailable(func() (bool, error) {
		return true, nil
	})
	t.Cleanup(restoreKeychain)

	t.Setenv("ASC_BYPASS_KEYCHAIN", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_TYPE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "not-base64")

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"auth", "status", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess || stderr != "" {
		t.Fatalf("status: exit=%d stderr=%q", code, stderr)
	}
	var payload struct {
		EnvironmentCredentialsComplete bool   `json:"environmentCredentialsComplete"`
		EnvironmentNote                string `json:"environmentNote"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to unmarshal auth status json: %v; stdout=%q", err, stdout)
	}
	if payload.EnvironmentCredentialsComplete || payload.EnvironmentNote != "" {
		t.Fatalf("invalid base64 must not be reported as an active environment source: %+v", payload)
	}
}

func TestAuthStatusMarksInlineEnvironmentActiveWithoutTempDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TMPDIR does not control os.TempDir on Windows")
	}
	restoreSummaries := authcmd.SetListCredentialSummaries(func() ([]authsvc.Credential, error) {
		return []authsvc.Credential{}, nil
	})
	t.Cleanup(restoreSummaries)
	restoreKeychain := authcmd.SetKeychainAvailable(func() (bool, error) {
		return true, nil
	})
	t.Cleanup(restoreKeychain)

	tempDir := t.TempDir()
	brokenTempDir := filepath.Join(tempDir, "missing-temp-dir")
	withBrokenTempDir := func(run func()) {
		previous, wasSet := os.LookupEnv("TMPDIR")
		if err := os.Setenv("TMPDIR", brokenTempDir); err != nil {
			t.Fatalf("Setenv(TMPDIR) error: %v", err)
		}
		defer func() {
			if wasSet {
				_ = os.Setenv("TMPDIR", previous)
			} else {
				_ = os.Unsetenv("TMPDIR")
			}
		}()
		run()
	}
	t.Setenv("ASC_BYPASS_KEYCHAIN", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_TYPE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "12345678-abcd-1234-abcd-123456789012")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	key, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASC_PRIVATE_KEY_B64", base64.StdEncoding.EncodeToString(key))

	var code int
	stdout, stderr := captureOutput(t, func() {
		withBrokenTempDir(func() {
			code = cmd.Run([]string{"auth", "status", "--output", "json"}, "1.0.0")
		})
	})
	if code != cmd.ExitSuccess || stderr != "" {
		t.Fatalf("status: exit=%d stderr=%q", code, stderr)
	}
	var payload struct {
		EnvironmentCredentialsComplete bool   `json:"environmentCredentialsComplete"`
		EnvironmentNote                string `json:"environmentNote"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to unmarshal auth status json: %v; stdout=%q", err, stdout)
	}
	if !payload.EnvironmentCredentialsComplete {
		t.Fatalf("expected syntactically complete environment credentials, got %+v", payload)
	}
	if !strings.Contains(payload.EnvironmentNote, "stored credential lookup is skipped") {
		t.Fatalf("inline environment key should be active without a temp directory: %+v", payload)
	}

	tableOutput, tableStderr := captureOutput(t, func() {
		withBrokenTempDir(func() {
			code = cmd.Run([]string{"auth", "status", "--output", "table"}, "1.0.0")
		})
	})
	if code != cmd.ExitSuccess || tableStderr != "" {
		t.Fatalf("table status: exit=%d stderr=%q", code, tableStderr)
	}
	if !strings.Contains(tableOutput, "No stored credentials found.") || strings.Contains(tableOutput, "Run 'asc auth login'") {
		t.Fatalf("expected active inline credentials without login guidance: %q", tableOutput)
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

func TestAuthOutputInvalidReturnsExitUsage(t *testing.T) {
	for _, command := range []string{"status", "issuer-id"} {
		t.Run(command, func(t *testing.T) {
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

			_, stderr := captureOutput(t, func() {
				code := cmd.Run([]string{"auth", command, "--output", "yaml"}, "1.0.0")
				if code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
				}
			})
			if !strings.Contains(stderr, `(got "yaml")`) {
				t.Fatalf("expected stderr to contain unsupported format error, got %q", stderr)
			}
		})
	}
}

func TestAuthStatusInvalidEnvKeyTypeMarksEnvironmentIncomplete(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_KEY_TYPE", "personal")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "/tmp/AuthKey.p8")
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
		EnvironmentCredentialsComplete bool   `json:"environmentCredentialsComplete"`
		EnvironmentNote                string `json:"environmentNote"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to unmarshal auth status json: %v; stdout=%q", err, stdout)
	}
	if payload.EnvironmentCredentialsComplete {
		t.Fatalf("expected environmentCredentialsComplete=false, got true; stdout=%q", stdout)
	}
	if !strings.Contains(payload.EnvironmentNote, "ASC_KEY_TYPE must be one of: team, individual, enterprise") {
		t.Fatalf("expected invalid key type environment note, got %q", payload.EnvironmentNote)
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

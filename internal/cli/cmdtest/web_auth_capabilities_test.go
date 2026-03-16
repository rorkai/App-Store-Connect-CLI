package cmdtest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func stubWebAuthCapabilitiesLookup(t *testing.T, fn func(context.Context, *webcore.Client, string) (*webcore.APIKeyRoleLookup, error)) {
	t.Helper()

	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{}, "cache", nil
	})
	restoreClient := webcmd.SetNewWebAuthClient(func(session *webcore.AuthSession) *webcore.Client {
		return &webcore.Client{}
	})
	restoreLookup := webcmd.SetLookupWebAuthKey(fn)
	t.Cleanup(restoreLookup)
	t.Cleanup(restoreClient)
	t.Cleanup(restoreSession)
}

func TestWebAuthCapabilitiesRunWithKeyIDOutputsJSON(t *testing.T) {
	restoreResolve := webcmd.SetResolveWebAuthCredentials(func(profile string) (shared.ResolvedAuthCredentials, error) {
		t.Fatal("did not expect local auth resolution when --key-id is provided")
		return shared.ResolvedAuthCredentials{}, nil
	})
	t.Cleanup(restoreResolve)

	stubWebAuthCapabilitiesLookup(t, func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKeyRoleLookup, error) {
		return &webcore.APIKeyRoleLookup{
			KeyID:      keyID,
			Name:       "asc_cli",
			Kind:       "team",
			Roles:      []string{"APP_MANAGER"},
			RoleSource: "key",
			Active:     true,
			Lookup:     "team_keys",
		}, nil
	})

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "capabilities", "--key-id", "39MX87M9Y4", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		KeyID        string   `json:"keyId"`
		ResolvedFrom string   `json:"resolvedFrom"`
		Roles        []string `json:"roles"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.KeyID != "39MX87M9Y4" || payload.ResolvedFrom != "flag" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if len(payload.Roles) != 1 || payload.Roles[0] != "APP_MANAGER" {
		t.Fatalf("unexpected roles: %#v", payload.Roles)
	}
}

func TestWebAuthCapabilitiesRunHonorsRootProfileFlag(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)

	cfg := &config.Config{
		DefaultKeyName: "first",
		Keys: []config.Credential{
			{
				Name:           "first",
				KeyID:          "KEY_A",
				IssuerID:       "ISS_A",
				PrivateKeyPath: keyPath,
			},
			{
				Name:           "second",
				KeyID:          "KEY_B",
				IssuerID:       "ISS_B",
				PrivateKeyPath: keyPath,
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	prevProfile := shared.SelectedProfile()
	shared.SetSelectedProfile("")
	t.Cleanup(func() {
		shared.SetSelectedProfile(prevProfile)
	})

	stubWebAuthCapabilitiesLookup(t, func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKeyRoleLookup, error) {
		return &webcore.APIKeyRoleLookup{
			KeyID:      keyID,
			Kind:       "team",
			Roles:      []string{"APP_MANAGER"},
			RoleSource: "key",
			Active:     true,
			Lookup:     "team_keys",
		}, nil
	})

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"--profile", "second", "web", "auth", "capabilities", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		KeyID        string `json:"keyId"`
		Profile      string `json:"profile"`
		ResolvedFrom string `json:"resolvedFrom"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.KeyID != "KEY_B" || payload.Profile != "second" || payload.ResolvedFrom != "auth" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestWebAuthCapabilitiesRunRejectsInvalidOutput(t *testing.T) {
	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"web", "auth", "capabilities", "--output", "yaml"}, "1.0.0")
		if code != cmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
		}
	})
	if !strings.Contains(stderr, "unsupported format: yaml") {
		t.Fatalf("expected unsupported format error, got %q", stderr)
	}
}

func TestWebAuthCapabilitiesRunRejectsPrettyForTable(t *testing.T) {
	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"web", "auth", "capabilities", "--output", "table", "--pretty"}, "1.0.0")
		if code != cmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
		}
	})
	if !strings.Contains(stderr, "--pretty is only valid with JSON output") {
		t.Fatalf("expected pretty validation error, got %q", stderr)
	}
}

func TestWebAuthCapabilitiesRunReturnsRuntimeErrorForMissingKey(t *testing.T) {
	stubWebAuthCapabilitiesLookup(t, func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKeyRoleLookup, error) {
		return nil, webcore.ErrAPIKeyNotFound
	})

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"web", "auth", "capabilities", "--key-id", "MISSING", "--output", "json"}, "1.0.0")
		if code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
	if !strings.Contains(stderr, `key "MISSING" not found in App Store Connect web key lists`) {
		t.Fatalf("expected runtime not-found error, got %q", stderr)
	}
}

func TestWebAuthCapabilitiesRunOutputsTable(t *testing.T) {
	stubWebAuthCapabilitiesLookup(t, func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKeyRoleLookup, error) {
		return &webcore.APIKeyRoleLookup{
			KeyID:      keyID,
			Name:       "asc_cli",
			Kind:       "team",
			Roles:      []string{"APP_MANAGER", "FINANCE"},
			RoleSource: "key",
			Active:     true,
			Lookup:     "team_keys",
		}, nil
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"web", "auth", "capabilities", "--key-id", "39MX87M9Y4", "--output", "table"}, "1.0.0")
		if code != cmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "KEY ID") || !strings.Contains(stdout, "CAPABILITIES") {
		t.Fatalf("expected table headers, got %q", stdout)
	}
	if !strings.Contains(stdout, "App Manager, Finance") {
		t.Fatalf("expected joined role labels, got %q", stdout)
	}
	if !strings.Contains(stdout, "Manage app pricing and App Store information") {
		t.Fatalf("expected capability labels, got %q", stdout)
	}
}

func TestWebAuthCapabilitiesRunUsesEnvAuthResolution(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)

	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	prevProfile := shared.SelectedProfile()
	shared.SetSelectedProfile("")
	t.Cleanup(func() {
		shared.SetSelectedProfile(prevProfile)
	})

	stubWebAuthCapabilitiesLookup(t, func(ctx context.Context, client *webcore.Client, keyID string) (*webcore.APIKeyRoleLookup, error) {
		return &webcore.APIKeyRoleLookup{
			KeyID:      keyID,
			Kind:       "team",
			Roles:      []string{"APP_MANAGER"},
			RoleSource: "key",
			Active:     true,
			Lookup:     "team_keys",
		}, nil
	})

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"web", "auth", "capabilities", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		KeyID        string `json:"keyId"`
		Profile      string `json:"profile"`
		ResolvedFrom string `json:"resolvedFrom"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.KeyID != "ENVKEY" || payload.Profile != "" || payload.ResolvedFrom != "auth" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

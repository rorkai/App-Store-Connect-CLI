package cmdtest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	authsvc "github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	authcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/auth"
)

func TestAuthStatusUsesCredentialSummariesWithoutValidate(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	restoreSummary := authcmd.SetListCredentialSummaries(func() ([]authsvc.Credential, error) {
		return []authsvc.Credential{{
			Name:      "default",
			KeyID:     "KEY123",
			IsDefault: true,
			Source:    "keychain",
		}}, nil
	})
	t.Cleanup(restoreSummary)

	restoreFull := authcmd.SetListStoredCredentials(func() ([]authsvc.Credential, error) {
		t.Fatal("expected auth status without --validate to avoid full credential loading")
		return nil, nil
	})
	t.Cleanup(restoreFull)

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
		Credentials []struct {
			Name  string `json:"name"`
			KeyID string `json:"keyId"`
		} `json:"credentials"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to unmarshal auth status json: %v; stdout=%q", err, stdout)
	}
	if len(payload.Credentials) != 1 {
		t.Fatalf("expected one credential, got %d", len(payload.Credentials))
	}
	if payload.Credentials[0].Name != "default" || payload.Credentials[0].KeyID != "KEY123" {
		t.Fatalf("unexpected auth status payload: %+v", payload.Credentials[0])
	}
}

func TestAuthStatusValidateUsesStoredCredentials(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	restoreSummary := authcmd.SetListCredentialSummaries(func() ([]authsvc.Credential, error) {
		t.Fatal("expected --validate to use full credential loading")
		return nil, nil
	})
	t.Cleanup(restoreSummary)

	restoreFull := authcmd.SetListStoredCredentials(func() ([]authsvc.Credential, error) {
		return []authsvc.Credential{{
			Name:      "default",
			KeyID:     "KEY123",
			IssuerID:  "ISS456",
			IsDefault: true,
			Source:    "keychain",
		}}, nil
	})
	t.Cleanup(restoreFull)

	restoreValidate := authcmd.SetStatusValidateCredential(func(context.Context, authsvc.Credential) error {
		return nil
	})
	t.Cleanup(restoreValidate)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"auth", "status", "--output", "json", "--validate"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"validation":"works"`) {
		t.Fatalf("expected validation result in output, got %q", stdout)
	}
}

func TestAuthCommandsUseCredentialSummaries(t *testing.T) {
	for _, test := range []struct {
		name       string
		args       []string
		wantStdout string
	}{
		{name: "status verbose", args: []string{"auth", "status", "--output", "json", "--verbose"}, wantStdout: `"storedIn":"keychain"`},
		{name: "switch", args: []string{"auth", "switch", "--name", "default"}, wantStdout: "Default profile set to 'default'"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

			restoreSummary := authcmd.SetListCredentialSummaries(func() ([]authsvc.Credential, error) {
				return []authsvc.Credential{{
					Name:      "default",
					KeyID:     "KEY123",
					IsDefault: true,
					Source:    "keychain",
				}}, nil
			})
			t.Cleanup(restoreSummary)

			restoreFull := authcmd.SetListStoredCredentials(func() ([]authsvc.Credential, error) {
				t.Fatal("expected credential summaries, not full credential loading")
				return nil, nil
			})
			t.Cleanup(restoreFull)

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
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("expected %q in stdout, got %q", test.wantStdout, stdout)
			}
		})
	}
}

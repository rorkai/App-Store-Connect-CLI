package auth

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func runCredentialShapeLogin(t *testing.T, args ...string) (string, error) {
	d := newAuthTestDeps()
	t.Helper()

	keyPath := writeTempECDSAKeyFile(t)
	cmd := newAuthLoginCommand(d)
	parsed := append([]string{
		"--bypass-keychain",
		"--local",
		"--name", "demo",
		"--private-key", keyPath,
	}, args...)
	if err := cmd.FlagSet.Parse(parsed); err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	var runErr error
	_, stderr := captureAuthOutput(t, d, func() {
		runErr = cmd.Exec(context.Background(), []string{})
	})
	return stderr, runErr
}

func TestAuthLoginRejectsSwappedCredentialIdentifiers(t *testing.T) {
	withTempRepo(t, func(repo string) {
		clearResolvedAuthEnv(t)
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))

		stderr, err := runCredentialShapeLogin(
			t,
			"--key-id", "69a6de00-aaaa-bbbb-cccc-123456789abc",
			"--issuer-id", "39MX87M9Y4",
		)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
		assertAuthDiagnostic(t, err, shared.DiagnosticInvalidInput, "--key-id")

		if !strings.Contains(stderr, "--key-id looks like an issuer ID — the values may be swapped") {
			t.Fatalf("expected swap error in stderr, got %q", stderr)
		}
		if strings.Contains(stderr, "69a6de00") || strings.Contains(stderr, "39MX87M9Y4") {
			t.Fatalf("expected credential identifiers to stay out of stderr, got %q", stderr)
		}
		if _, statErr := os.Stat(filepath.Join(repo, ".asc", "config.json")); statErr == nil || !os.IsNotExist(statErr) {
			t.Fatalf("expected credentials not to be stored, stat error = %v", statErr)
		}
	})
}

func TestAuthLoginWarnsWithoutFailingForUncertainCredentialShapes(t *testing.T) {
	for _, test := range []struct {
		name        string
		keyID       string
		issuerID    string
		wantWarning string
	}{
		{
			name:        "both identifiers are uuids",
			keyID:       "69a6de00-aaaa-bbbb-cccc-123456789abc",
			issuerID:    "09f4080c-6ee7-4e52-8103-e1241eaaa58a",
			wantWarning: "--key-id looks like an issuer ID",
		},
		{
			name:        "issuer id is not a uuid",
			keyID:       "39MX87M9Y4",
			issuerID:    "ISS456",
			wantWarning: "--issuer-id is not a UUID",
		},
		{
			name:        "uuid key id with generic malformed issuer",
			keyID:       "69a6de00-aaaa-bbbb-cccc-123456789abc",
			issuerID:    "issuer-uuid",
			wantWarning: "--key-id looks like an issuer ID",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			withTempRepo(t, func(repo string) {
				clearResolvedAuthEnv(t)
				t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))

				stderr, err := runCredentialShapeLogin(
					t,
					"--key-id", test.keyID,
					"--issuer-id", test.issuerID,
				)
				if err != nil {
					t.Fatalf("Exec() error: %v", err)
				}
				if !strings.Contains(stderr, test.wantWarning) {
					t.Fatalf("expected %q warning in stderr, got %q", test.wantWarning, stderr)
				}
				if !strings.HasPrefix(strings.TrimSpace(stderr), "Warning:") {
					t.Fatalf("expected a warning rather than an error, got %q", stderr)
				}
				if _, statErr := os.Stat(filepath.Join(repo, ".asc", "config.json")); statErr != nil {
					t.Fatalf("expected credentials to be stored, stat error = %v", statErr)
				}
			})
		})
	}
}

func TestAuthLoginAcceptsUnusualButValidCredentialShapesSilently(t *testing.T) {
	for _, test := range []struct {
		name     string
		keyID    string
		issuerID string
	}{
		{name: "typical pair", keyID: "39MX87M9Y4", issuerID: "69a6de00-aaaa-bbbb-cccc-123456789abc"},
		{name: "digits only key id", keyID: "1234567890", issuerID: "69a6de00-aaaa-bbbb-cccc-123456789abc"},
		{name: "mixed case key id", keyID: "39Mx87M9y4", issuerID: "09f4080c-6ee7-4e52-8103-e1241eaaa58a"},
		{name: "short key id", keyID: "KEY123", issuerID: "A7EFEF21-3432-404F-A488-083800B570FF"},
	} {
		t.Run(test.name, func(t *testing.T) {
			withTempRepo(t, func(repo string) {
				clearResolvedAuthEnv(t)
				t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))

				stderr, err := runCredentialShapeLogin(
					t,
					"--key-id", test.keyID,
					"--issuer-id", test.issuerID,
				)
				if err != nil {
					t.Fatalf("Exec() error: %v", err)
				}
				if strings.Contains(stderr, "swapped") || strings.Contains(stderr, "not a UUID") {
					t.Fatalf("expected no credential shape warning, got %q", stderr)
				}
			})
		})
	}
}

func TestAuthLoginIndividualKeyWithUUIDKeyIDSkipsTeamShapeWarnings(t *testing.T) {
	withTempRepo(t, func(repo string) {
		clearResolvedAuthEnv(t)
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))

		stderr, err := runCredentialShapeLogin(
			t,
			"--key-id", "69a6de00-aaaa-bbbb-cccc-123456789abc",
			"--key-type", "individual",
		)
		if err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
		if strings.Contains(stderr, "issuer") || strings.Contains(stderr, "swapped") {
			t.Fatalf("unexpected team credential warning in stderr: %q", stderr)
		}
		if _, statErr := os.Stat(filepath.Join(repo, ".asc", "config.json")); statErr != nil {
			t.Fatalf("expected credentials to be stored, stat error = %v", statErr)
		}
	})
}

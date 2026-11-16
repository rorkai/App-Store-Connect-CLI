package auth

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	authsvc "github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

const (
	defaultNameTestKeyID    = "39MX87M9Y4"
	defaultNameTestIssuerID = "69a6de00-aaaa-bbbb-cccc-123456789abc"
)

// runLocalLogin runs `auth login --bypass-keychain --local` against the temp
// repo's ./.asc/config.json, so no host configuration or keychain is touched.
func runLocalLogin(t *testing.T, args ...string) (string, error) {
	d := newAuthTestDeps()
	t.Helper()

	cmd := newAuthLoginCommand(d)
	parsed := append([]string{
		"--bypass-keychain",
		"--local",
		"--key-id", defaultNameTestKeyID,
		"--issuer-id", defaultNameTestIssuerID,
		"--private-key", writeTempECDSAKeyFile(t),
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

func localProfileNames(t *testing.T, repo string) []string {
	t.Helper()

	cfg, err := config.LoadAt(filepath.Join(repo, ".asc", "config.json"))
	if err != nil {
		t.Fatalf("LoadAt() error: %v", err)
	}
	names := make([]string, 0, len(cfg.Keys))
	for _, key := range cfg.Keys {
		names = append(names, key.Name)
	}
	return names
}

func isolateLoginEnv(t *testing.T, repo string) {
	t.Helper()
	clearResolvedAuthEnv(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))
}

func TestAuthLoginWithoutNameUsesDefaultWhenNoProfilesExist(t *testing.T) {
	withTempRepo(t, func(repo string) {
		isolateLoginEnv(t, repo)

		if _, err := runLocalLogin(t); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}

		if got := localProfileNames(t, repo); len(got) != 1 || got[0] != defaultLoginProfileName {
			t.Fatalf("expected a single %q profile, got %v", defaultLoginProfileName, got)
		}
	})
}

func TestAuthLoginWithoutNameRefusesToGuessWhenProfilesExist(t *testing.T) {
	withTempRepo(t, func(repo string) {
		isolateLoginEnv(t, repo)

		if _, err := runLocalLogin(t, "--name", "Work"); err != nil {
			t.Fatalf("seed login error: %v", err)
		}
		if _, err := runLocalLogin(t, "--name", "Client"); err != nil {
			t.Fatalf("seed login error: %v", err)
		}
		before, err := os.ReadFile(filepath.Join(repo, ".asc", "config.json"))
		if err != nil {
			t.Fatalf("ReadFile() error: %v", err)
		}

		stderr, err := runLocalLogin(t)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected a usage error (exit 2), got %v", err)
		}
		assertAuthDiagnostic(t, err, shared.DiagnosticRequiredInputMissing, "--name")
		for _, want := range []string{"Work", "Client", `--name "Work"`} {
			if !strings.Contains(stderr, want) {
				t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
			}
		}

		after, err := os.ReadFile(filepath.Join(repo, ".asc", "config.json"))
		if err != nil {
			t.Fatalf("ReadFile() error: %v", err)
		}
		if string(after) != string(before) {
			t.Fatalf("expected stored profiles to be unchanged")
		}
	})
}

func TestAuthLoginWithoutNameRefusesWhenOnlyALegacyCredentialExists(t *testing.T) {
	withTempRepo(t, func(repo string) {
		isolateLoginEnv(t, repo)

		path := filepath.Join(repo, ".asc", "config.json")
		if err := config.SaveAt(path, &config.Config{KeyID: "LEGACY1234", IssuerID: defaultNameTestIssuerID}); err != nil {
			t.Fatalf("SaveAt() error: %v", err)
		}

		stderr, err := runLocalLogin(t)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected a usage error, got %v", err)
		}
		if !strings.Contains(stderr, `--name "default"`) {
			t.Fatalf("expected the legacy credential to be listed as default, got %q", stderr)
		}
	})
}

func TestAuthLoginWithoutNameListsKeychainProfiles(t *testing.T) {
	d := newAuthTestDeps()
	withTempRepo(t, func(repo string) {
		clearResolvedAuthEnv(t)
		t.Setenv("ASC_BYPASS_KEYCHAIN", "0")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))
		restore := SetListCredentialSummaries(func() ([]authsvc.Credential, error) {
			return []authsvc.Credential{{Name: "Personal"}, {Name: "Client"}, {Name: "Personal"}}, nil
		})
		t.Cleanup(restore)

		cmd := newAuthLoginCommand(d)
		if err := cmd.FlagSet.Parse([]string{
			"--key-id", defaultNameTestKeyID,
			"--issuer-id", defaultNameTestIssuerID,
			"--private-key", writeTempECDSAKeyFile(t),
		}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}

		var runErr error
		_, stderr := captureAuthOutput(t, d, func() {
			runErr = cmd.Exec(context.Background(), []string{})
		})
		if !errors.Is(runErr, flag.ErrHelp) {
			t.Fatalf("expected a usage error, got %v", runErr)
		}
		if !strings.Contains(stderr, "(Personal, Client)") {
			t.Fatalf("expected each keychain profile listed once, got %q", stderr)
		}
	})
}

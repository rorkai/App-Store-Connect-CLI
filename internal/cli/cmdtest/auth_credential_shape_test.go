package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAuthDoctorWarnsWhenEnvironmentIdentifiersLookSwapped(t *testing.T) {
	withTempRepo(t, func(repo string) {
		keyPath := filepath.Join(repo, "AuthKey.p8")
		writeECDSAPEM(t, keyPath)
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))
		t.Setenv("ASC_KEY_ID", "69a6de00-aaaa-bbbb-cccc-123456789abc")
		t.Setenv("ASC_ISSUER_ID", "39MX87M9Y4")
		t.Setenv("ASC_KEY_TYPE", "team")
		t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)

		stdout, _, err := runRootWithDeps(t, shared.GetASCClient, "auth", "doctor")
		if err != nil {
			t.Fatalf("run error: %v", err)
		}

		if !strings.Contains(stdout, "[WARN] ASC_KEY_ID looks like an issuer ID — the values may be swapped") {
			t.Fatalf("expected swapped key ID warning, got %q", stdout)
		}
		if !strings.Contains(stdout, "[WARN] ASC_ISSUER_ID looks like a key ID — the values may be swapped") {
			t.Fatalf("expected swapped issuer ID warning, got %q", stdout)
		}
		if strings.Contains(stdout, "69a6de00") || strings.Contains(stdout, "39MX87M9Y4") {
			t.Fatalf("expected credential identifiers to be redacted, got %q", stdout)
		}
	})
}

func TestAuthDoctorStaysQuietForValidEnvironmentIdentifiers(t *testing.T) {
	withTempRepo(t, func(repo string) {
		keyPath := filepath.Join(repo, "AuthKey.p8")
		writeECDSAPEM(t, keyPath)
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))
		t.Setenv("ASC_KEY_ID", "1234567890")
		t.Setenv("ASC_ISSUER_ID", "A7EFEF21-3432-404F-A488-083800B570FF")
		t.Setenv("ASC_KEY_TYPE", "team")
		t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)

		stdout, _, err := runRootWithDeps(t, shared.GetASCClient, "auth", "doctor")
		if err != nil {
			t.Fatalf("run error: %v", err)
		}

		if strings.Contains(stdout, "swapped") || strings.Contains(stdout, "is not a UUID") {
			t.Fatalf("expected no credential shape warning, got %q", stdout)
		}
	})
}

func TestAuthLoginFailsWhenCredentialIdentifiersAreSwapped(t *testing.T) {
	withTempRepo(t, func(repo string) {
		keyPath := filepath.Join(repo, "AuthKey.p8")
		writeECDSAPEM(t, keyPath)

		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(repo, "config.json"))

		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)

		_, stderr := captureOutput(t, func() {
			if err := root.Parse([]string{
				"auth", "login",
				"--bypass-keychain",
				"--local",
				"--name", "CI",
				"--key-id", "69a6de00-aaaa-bbbb-cccc-123456789abc",
				"--issuer-id", "39MX87M9Y4",
				"--private-key", keyPath,
			}); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			err := root.Run(context.Background())
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected usage error, got %v", err)
			}
		})

		if !strings.Contains(stderr, "auth login: --key-id looks like an issuer ID — the values may be swapped") {
			t.Fatalf("expected swap usage error in stderr, got %q", stderr)
		}

		if _, err := os.Stat(filepath.Join(repo, ".asc", "config.json")); err == nil || !os.IsNotExist(err) {
			t.Fatalf("expected no credentials to be written, stat error = %v", err)
		}
	})
}

package cmdtest

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	authcli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	outC := make(chan string)
	errC := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		_ = rOut.Close()
		outC <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		_ = rErr.Close()
		errC <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = wOut.Close()
		_ = wErr.Close()
	}()

	fn()

	_ = wOut.Close()
	_ = wErr.Close()

	stdout := <-outC
	stderr := <-errC

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return stdout, stderr
}

func writeECDSAPEM(t *testing.T, path string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key error: %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if data == nil {
		t.Fatal("failed to encode PEM")
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write key file error: %v", err)
	}
}

func TestVersionSubcommandPrintsVersion(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"version"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stdout != "1.2.3\n" {
		t.Fatalf("expected stdout to be version, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestVersionFlagPrintsVersion(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"--version"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stdout != "1.2.3\n" {
		t.Fatalf("expected stdout to be version, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestCompletionZshPrintsScriptToStdout(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"completion", "--shell", "zsh"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stdout == "" || !strings.Contains(stdout, "#compdef asc") {
		t.Fatalf("expected zsh completion script, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestCompletionInvalidShellErrorsToStderr(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"completion", "--shell", "nope"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: unsupported shell") {
		t.Fatalf("expected shell error, got %q", stderr)
	}
}

func TestUnknownCommandPrintsHelpError(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"nope"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Unknown command: nope") {
		t.Fatalf("expected unknown command message, got %q", stderr)
	}
}

func TestUnknownCommandSuggestsSimilarCommand(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"finace"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Unknown command: finace") {
		t.Fatalf("expected unknown command message, got %q", stderr)
	}
	if !strings.Contains(stderr, "Did you mean: finance") {
		t.Fatalf("expected suggestion message, got %q", stderr)
	}
}

func TestBuildsInfoRequiresBuildID(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "info"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--build is required") {
		t.Fatalf("expected missing build error, got %q", stderr)
	}
}

func TestBuildsExpireRequiresBuildID(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"builds", "expire"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--build is required") {
		t.Fatalf("expected missing build error, got %q", stderr)
	}
}

func TestOfferCodesListRequiresOfferCode(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"offer-codes", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --offer-code is required") {
		t.Fatalf("expected missing offer code error, got %q", stderr)
	}
}

func TestOfferCodesGenerateValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing offer-code",
			args:    []string{"offer-codes", "generate", "--quantity", "1", "--expiration-date", "2026-02-01"},
			wantErr: "Error: --offer-code is required",
		},
		{
			name:    "missing expiration date",
			args:    []string{"offer-codes", "generate", "--offer-code", "OFFER_CODE_ID", "--quantity", "1"},
			wantErr: "Error: --expiration-date is required",
		},
		{
			name:    "missing quantity",
			args:    []string{"offer-codes", "generate", "--offer-code", "OFFER_CODE_ID", "--expiration-date", "2026-02-01"},
			wantErr: "Error: --quantity is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestBuildsExpireAllValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "builds expire-all missing app",
			args:    []string{"builds", "expire-all", "--older-than", "90d"},
			wantErr: "Error: --app is required",
		},
		{
			name:    "builds expire-all missing filter",
			args:    []string{"builds", "expire-all", "--app", "APP_ID", "--confirm"},
			wantErr: "--older-than or --keep-latest is required",
		},
		{
			name:    "builds expire-all missing confirm",
			args:    []string{"builds", "expire-all", "--app", "APP_ID", "--older-than", "90d"},
			wantErr: "--confirm is required to expire builds",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestBuildsGroupValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "builds add-groups missing build",
			args:    []string{"builds", "add-groups"},
			wantErr: "Error: --build is required",
		},
		{
			name:    "builds add-groups missing group",
			args:    []string{"builds", "add-groups", "--build", "BUILD_123"},
			wantErr: "Error: --group is required",
		},
		{
			name:    "builds remove-groups missing build",
			args:    []string{"builds", "remove-groups"},
			wantErr: "Error: --build is required",
		},
		{
			name:    "builds remove-groups missing group",
			args:    []string{"builds", "remove-groups", "--build", "BUILD_123"},
			wantErr: "Error: --group is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestBuildBundlesValidationErrors(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "build-bundles list missing build",
			args:    []string{"build-bundles", "list"},
			wantErr: "Error: --build is required",
		},
		{
			name:    "build-bundles file-sizes list missing id",
			args:    []string{"build-bundles", "file-sizes", "list"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "build-bundles app-clip cache-status get missing id",
			args:    []string{"build-bundles", "app-clip", "cache-status", "get"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "build-bundles app-clip debug-status get missing id",
			args:    []string{"build-bundles", "app-clip", "debug-status", "get"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "build-bundles app-clip invocations list missing id",
			args:    []string{"build-bundles", "app-clip", "invocations", "list"},
			wantErr: "Error: --id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestBetaManagementValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "beta-groups list missing app",
			args:    []string{"testflight", "beta-groups", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "beta-groups create missing app",
			args:    []string{"testflight", "beta-groups", "create", "--name", "Beta"},
			wantErr: "--app is required",
		},
		{
			name:    "beta-groups create missing name",
			args:    []string{"testflight", "beta-groups", "create", "--app", "APP_ID"},
			wantErr: "--name is required",
		},
		{
			name:    "beta-testers list missing app",
			args:    []string{"testflight", "beta-testers", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "beta-testers add missing app",
			args:    []string{"testflight", "beta-testers", "add", "--email", "tester@example.com", "--group", "Beta"},
			wantErr: "--app is required",
		},
		{
			name:    "beta-testers add missing email",
			args:    []string{"testflight", "beta-testers", "add", "--app", "APP_ID", "--group", "Beta"},
			wantErr: "--email is required",
		},
		{
			name:    "beta-testers add missing group",
			args:    []string{"testflight", "beta-testers", "add", "--app", "APP_ID", "--email", "tester@example.com"},
			wantErr: "--group is required",
		},
		{
			name:    "beta-testers remove missing app",
			args:    []string{"testflight", "beta-testers", "remove", "--email", "tester@example.com"},
			wantErr: "--app is required",
		},
		{
			name:    "beta-testers remove missing email",
			args:    []string{"testflight", "beta-testers", "remove", "--app", "APP_ID"},
			wantErr: "--email is required",
		},
		{
			name:    "beta-testers add-groups missing id",
			args:    []string{"testflight", "beta-testers", "add-groups", "--group", "GROUP_ID"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-testers add-groups missing group",
			args:    []string{"testflight", "beta-testers", "add-groups", "--id", "TESTER_ID"},
			wantErr: "--group is required",
		},
		{
			name:    "beta-testers remove-groups missing id",
			args:    []string{"testflight", "beta-testers", "remove-groups", "--group", "GROUP_ID"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-testers remove-groups missing group",
			args:    []string{"testflight", "beta-testers", "remove-groups", "--id", "TESTER_ID"},
			wantErr: "--group is required",
		},
		{
			name:    "beta-testers invite missing app",
			args:    []string{"testflight", "beta-testers", "invite", "--email", "tester@example.com"},
			wantErr: "--app is required",
		},
		{
			name:    "beta-testers invite missing email",
			args:    []string{"testflight", "beta-testers", "invite", "--app", "APP_ID"},
			wantErr: "--email is required",
		},
		{
			name:    "beta-testers get missing id",
			args:    []string{"testflight", "beta-testers", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-groups get missing id",
			args:    []string{"testflight", "beta-groups", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-groups update missing id",
			args:    []string{"testflight", "beta-groups", "update"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-groups update missing update flags",
			args:    []string{"testflight", "beta-groups", "update", "--id", "GROUP_ID"},
			wantErr: "at least one update flag is required",
		},
		{
			name:    "beta-groups update public-link-limit out of range",
			args:    []string{"testflight", "beta-groups", "update", "--id", "GROUP_ID", "--public-link-limit", "50000"},
			wantErr: "--public-link-limit must be between 1 and 10000",
		},
		{
			name:    "beta-groups add-testers missing group",
			args:    []string{"testflight", "beta-groups", "add-testers"},
			wantErr: "--group is required",
		},
		{
			name:    "beta-groups add-testers missing tester",
			args:    []string{"testflight", "beta-groups", "add-testers", "--group", "GROUP_ID"},
			wantErr: "--tester is required",
		},
		{
			name:    "beta-groups remove-testers missing group",
			args:    []string{"testflight", "beta-groups", "remove-testers"},
			wantErr: "--group is required",
		},
		{
			name:    "beta-groups remove-testers missing tester",
			args:    []string{"testflight", "beta-groups", "remove-testers", "--group", "GROUP_ID"},
			wantErr: "--tester is required",
		},
		{
			name:    "beta-groups delete missing id",
			args:    []string{"testflight", "beta-groups", "delete"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-groups delete missing confirm",
			args:    []string{"testflight", "beta-groups", "delete", "--id", "GROUP_ID"},
			wantErr: "--confirm is required to delete",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestIAPValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "iap list missing app",
			args:    []string{"iap", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "iap get missing id",
			args:    []string{"iap", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "iap create missing app",
			args:    []string{"iap", "create", "--type", "CONSUMABLE", "--ref-name", "Pro", "--product-id", "com.example.pro"},
			wantErr: "--app is required",
		},
		{
			name:    "iap create missing type",
			args:    []string{"iap", "create", "--app", "APP_ID", "--ref-name", "Pro", "--product-id", "com.example.pro"},
			wantErr: "--type is required",
		},
		{
			name:    "iap create invalid type",
			args:    []string{"iap", "create", "--app", "APP_ID", "--type", "UNKNOWN", "--ref-name", "Pro", "--product-id", "com.example.pro"},
			wantErr: "--type must be one of",
		},
		{
			name:    "iap create missing ref-name",
			args:    []string{"iap", "create", "--app", "APP_ID", "--type", "CONSUMABLE", "--product-id", "com.example.pro"},
			wantErr: "--ref-name is required",
		},
		{
			name:    "iap create missing product-id",
			args:    []string{"iap", "create", "--app", "APP_ID", "--type", "CONSUMABLE", "--ref-name", "Pro"},
			wantErr: "--product-id is required",
		},
		{
			name:    "iap update missing id",
			args:    []string{"iap", "update"},
			wantErr: "--id is required",
		},
		{
			name:    "iap update missing update flags",
			args:    []string{"iap", "update", "--id", "IAP_ID"},
			wantErr: "at least one update flag is required",
		},
		{
			name:    "iap delete missing id",
			args:    []string{"iap", "delete"},
			wantErr: "--id is required",
		},
		{
			name:    "iap delete missing confirm",
			args:    []string{"iap", "delete", "--id", "IAP_ID"},
			wantErr: "--confirm is required",
		},
		{
			name:    "iap localizations list missing id",
			args:    []string{"iap", "localizations", "list"},
			wantErr: "--id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestUsersValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "users get missing id",
			args:    []string{"users", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "users update missing id",
			args:    []string{"users", "update", "--roles", "ADMIN"},
			wantErr: "--id is required",
		},
		{
			name:    "users update missing roles",
			args:    []string{"users", "update", "--id", "USER_ID"},
			wantErr: "--roles is required",
		},
		{
			name:    "users delete missing confirm",
			args:    []string{"users", "delete", "--id", "USER_ID"},
			wantErr: "--confirm is required",
		},
		{
			name:    "users delete missing id",
			args:    []string{"users", "delete", "--confirm"},
			wantErr: "--id is required",
		},
		{
			name:    "users invite missing email",
			args:    []string{"users", "invite", "--first-name", "Jane", "--last-name", "Doe", "--roles", "ADMIN", "--all-apps"},
			wantErr: "--email is required",
		},
		{
			name:    "users invite missing first name",
			args:    []string{"users", "invite", "--email", "user@example.com", "--last-name", "Doe", "--roles", "ADMIN", "--all-apps"},
			wantErr: "--first-name is required",
		},
		{
			name:    "users invite missing last name",
			args:    []string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--roles", "ADMIN", "--all-apps"},
			wantErr: "--last-name is required",
		},
		{
			name:    "users invite missing roles",
			args:    []string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--all-apps"},
			wantErr: "--roles is required",
		},
		{
			name:    "users invite missing access",
			args:    []string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "ADMIN"},
			wantErr: "--all-apps or --visible-app is required",
		},
		{
			name:    "users invite conflicting access",
			args:    []string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "ADMIN", "--all-apps", "--visible-app", "APP_ID"},
			wantErr: "--all-apps and --visible-app cannot be used together",
		},
		{
			name:    "users invites get missing id",
			args:    []string{"users", "invites", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "users invites revoke missing confirm",
			args:    []string{"users", "invites", "revoke", "--id", "INVITE_ID"},
			wantErr: "--confirm is required",
		},
		{
			name:    "users invites revoke missing id",
			args:    []string{"users", "invites", "revoke", "--confirm"},
			wantErr: "--id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "subscriptions groups list missing app",
			args:    []string{"subscriptions", "groups", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "subscriptions groups create missing app",
			args:    []string{"subscriptions", "groups", "create", "--reference-name", "Premium"},
			wantErr: "--app is required",
		},
		{
			name:    "subscriptions groups create missing reference-name",
			args:    []string{"subscriptions", "groups", "create", "--app", "APP_ID"},
			wantErr: "--reference-name is required",
		},
		{
			name:    "subscriptions groups get missing id",
			args:    []string{"subscriptions", "groups", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "subscriptions groups update missing id",
			args:    []string{"subscriptions", "groups", "update"},
			wantErr: "--id is required",
		},
		{
			name:    "subscriptions groups update missing update flags",
			args:    []string{"subscriptions", "groups", "update", "--id", "GROUP_ID"},
			wantErr: "at least one update flag is required",
		},
		{
			name:    "subscriptions groups delete missing confirm",
			args:    []string{"subscriptions", "groups", "delete", "--id", "GROUP_ID"},
			wantErr: "--confirm is required",
		},
		{
			name:    "subscriptions list missing group",
			args:    []string{"subscriptions", "list"},
			wantErr: "--group is required",
		},
		{
			name:    "subscriptions create missing group",
			args:    []string{"subscriptions", "create", "--ref-name", "Monthly", "--product-id", "com.example.sub"},
			wantErr: "--group is required",
		},
		{
			name:    "subscriptions create missing ref-name",
			args:    []string{"subscriptions", "create", "--group", "GROUP_ID", "--product-id", "com.example.sub"},
			wantErr: "--ref-name is required",
		},
		{
			name:    "subscriptions create missing product-id",
			args:    []string{"subscriptions", "create", "--group", "GROUP_ID", "--ref-name", "Monthly"},
			wantErr: "--product-id is required",
		},
		{
			name:    "subscriptions get missing id",
			args:    []string{"subscriptions", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "subscriptions update missing id",
			args:    []string{"subscriptions", "update"},
			wantErr: "--id is required",
		},
		{
			name:    "subscriptions update missing update flags",
			args:    []string{"subscriptions", "update", "--id", "SUB_ID"},
			wantErr: "at least one update flag is required",
		},
		{
			name:    "subscriptions delete missing confirm",
			args:    []string{"subscriptions", "delete", "--id", "SUB_ID"},
			wantErr: "--confirm is required",
		},
		{
			name:    "subscriptions prices add missing id",
			args:    []string{"subscriptions", "prices", "add", "--price-point", "PRICE_POINT_ID"},
			wantErr: "--id is required",
		},
		{
			name:    "subscriptions prices add missing price-point",
			args:    []string{"subscriptions", "prices", "add", "--id", "SUB_ID"},
			wantErr: "--price-point is required",
		},
		{
			name:    "subscriptions availability set missing id",
			args:    []string{"subscriptions", "availability", "set", "--territory", "USA"},
			wantErr: "--id is required",
		},
		{
			name:    "subscriptions availability set missing territory",
			args:    []string{"subscriptions", "availability", "set", "--id", "SUB_ID"},
			wantErr: "--territory is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestDevicesValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "devices get missing id",
			args:    []string{"devices", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "devices update missing id",
			args:    []string{"devices", "update", "--status", "ENABLED"},
			wantErr: "--id is required",
		},
		{
			name:    "devices update missing updates",
			args:    []string{"devices", "update", "--id", "DEVICE_ID"},
			wantErr: "at least one update flag is required",
		},
		{
			name:    "devices register missing name",
			args:    []string{"devices", "register", "--udid", "UDID", "--platform", "IOS"},
			wantErr: "--name is required",
		},
		{
			name:    "devices register missing udid",
			args:    []string{"devices", "register", "--name", "My Device", "--platform", "IOS"},
			wantErr: "--udid is required",
		},
		{
			name:    "devices register missing platform",
			args:    []string{"devices", "register", "--name", "My Device", "--udid", "UDID"},
			wantErr: "--platform is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestDevicesListLimitValidation(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{"devices", "list", "--limit", "500"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTestFlightAppsValidationErrors(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	// Isolate from user's config file
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantHelp bool
	}{
		{
			name:     "testflight apps list missing auth",
			args:     []string{"testflight", "apps", "list"},
			wantErr:  "missing authentication",
			wantHelp: false,
		},
		{
			name:     "testflight apps get missing id",
			args:     []string{"testflight", "apps", "get"},
			wantErr:  "--app is required",
			wantHelp: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if test.wantHelp {
					if !errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected ErrHelp, got %v", err)
					}
				} else {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					if errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected non-help error, got %v", err)
					}
				}
			})

			if test.wantHelp {
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				if !strings.Contains(stderr, test.wantErr) {
					t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
				}
			}
		})
	}
}

func TestTestFlightReviewValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "review get missing app",
			args:    []string{"testflight", "review", "get"},
			wantErr: "--app is required",
		},
		{
			name:    "review update missing id",
			args:    []string{"testflight", "review", "update"},
			wantErr: "--id is required",
		},
		{
			name:    "review update missing updates",
			args:    []string{"testflight", "review", "update", "--id", "DETAIL_ID"},
			wantErr: "at least one update flag is required",
		},
		{
			name:    "review submit missing build",
			args:    []string{"testflight", "review", "submit", "--confirm"},
			wantErr: "--build is required",
		},
		{
			name:    "review submit missing confirm",
			args:    []string{"testflight", "review", "submit", "--build", "BUILD_ID"},
			wantErr: "--confirm is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAgeRatingValidationErrors(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantHelp bool
	}{
		{
			name:     "age-rating get missing app",
			args:     []string{"age-rating", "get"},
			wantErr:  "--app is required",
			wantHelp: true,
		},
		{
			name:     "age-rating get conflicting targets",
			args:     []string{"age-rating", "get", "--app-info-id", "INFO_ID", "--version-id", "VERSION_ID"},
			wantErr:  "only one of --app-info-id or --version-id is allowed",
			wantHelp: false,
		},
		{
			name:     "age-rating set missing target",
			args:     []string{"age-rating", "set", "--gambling", "true"},
			wantErr:  "--id or --app is required",
			wantHelp: true,
		},
		{
			name:     "age-rating set missing updates",
			args:     []string{"age-rating", "set", "--id", "AGE_ID"},
			wantErr:  "at least one update flag is required",
			wantHelp: false,
		},
		{
			name:     "age-rating set invalid enum",
			args:     []string{"age-rating", "set", "--id", "AGE_ID", "--gambling-simulated", "BAD"},
			wantErr:  "--gambling-simulated must be one of",
			wantHelp: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if test.wantHelp {
					if !errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected ErrHelp, got %v", err)
					}
				} else {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					if errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected non-help error, got %v", err)
					}
				}
			})

			if test.wantHelp {
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				if !strings.Contains(stderr, test.wantErr) {
					t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
				}
			}
		})
	}
}

func TestAccessibilityValidationErrors(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantHelp bool
	}{
		{
			name:     "accessibility list missing app",
			args:     []string{"accessibility", "list"},
			wantErr:  "--app is required",
			wantHelp: true,
		},
		{
			name:     "accessibility get missing id",
			args:     []string{"accessibility", "get"},
			wantErr:  "--id is required",
			wantHelp: true,
		},
		{
			name:     "accessibility create missing app",
			args:     []string{"accessibility", "create", "--device-family", "IPHONE"},
			wantErr:  "--app is required",
			wantHelp: true,
		},
		{
			name:     "accessibility create missing device family",
			args:     []string{"accessibility", "create", "--app", "APP_ID"},
			wantErr:  "--device-family is required",
			wantHelp: true,
		},
		{
			name:     "accessibility update missing id",
			args:     []string{"accessibility", "update", "--supports-voiceover", "true"},
			wantErr:  "--id is required",
			wantHelp: true,
		},
		{
			name:     "accessibility update missing updates",
			args:     []string{"accessibility", "update", "--id", "DECLARATION_ID"},
			wantErr:  "at least one update flag is required",
			wantHelp: false,
		},
		{
			name:     "accessibility delete missing confirm",
			args:     []string{"accessibility", "delete", "--id", "DECLARATION_ID"},
			wantErr:  "--confirm is required",
			wantHelp: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if test.wantHelp {
					if !errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected ErrHelp, got %v", err)
					}
				} else {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					if errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected non-help error, got %v", err)
					}
				}
			})

			if test.wantHelp {
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				if !strings.Contains(stderr, test.wantErr) {
					t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
				}
			}
		})
	}
}

func TestReviewCommandDetailsValidationErrors(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantHelp bool
	}{
		{
			name:     "review details-get missing id",
			args:     []string{"review", "details-get"},
			wantErr:  "--id is required",
			wantHelp: true,
		},
		{
			name:     "review details-for-version missing version id",
			args:     []string{"review", "details-for-version"},
			wantErr:  "--version-id is required",
			wantHelp: true,
		},
		{
			name:     "review details-create missing version id",
			args:     []string{"review", "details-create"},
			wantErr:  "--version-id is required",
			wantHelp: true,
		},
		{
			name:     "review details-update missing id",
			args:     []string{"review", "details-update", "--notes", "hi"},
			wantErr:  "--id is required",
			wantHelp: true,
		},
		{
			name:     "review details-update missing fields",
			args:     []string{"review", "details-update", "--id", "DETAIL_ID"},
			wantErr:  "at least one update flag is required",
			wantHelp: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if test.wantHelp {
					if !errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected ErrHelp, got %v", err)
					}
				} else {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					if errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected non-help error, got %v", err)
					}
				}
			})

			if test.wantHelp {
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				if !strings.Contains(stderr, test.wantErr) {
					t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
				}
			}
		})
	}
}

func TestReviewCommandAttachmentsValidationErrors(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantHelp bool
	}{
		{
			name:     "review attachments-list missing review detail",
			args:     []string{"review", "attachments-list"},
			wantErr:  "--review-detail is required",
			wantHelp: true,
		},
		{
			name:     "review attachments-get missing id",
			args:     []string{"review", "attachments-get"},
			wantErr:  "--id is required",
			wantHelp: true,
		},
		{
			name:     "review attachments-upload missing review detail",
			args:     []string{"review", "attachments-upload", "--file", "file.txt"},
			wantErr:  "--review-detail is required",
			wantHelp: true,
		},
		{
			name:     "review attachments-upload missing file",
			args:     []string{"review", "attachments-upload", "--review-detail", "DETAIL_ID"},
			wantErr:  "--file is required",
			wantHelp: true,
		},
		{
			name:     "review attachments-delete missing id",
			args:     []string{"review", "attachments-delete", "--confirm"},
			wantErr:  "--id is required",
			wantHelp: true,
		},
		{
			name:     "review attachments-delete missing confirm",
			args:     []string{"review", "attachments-delete", "--id", "ATTACHMENT_ID"},
			wantErr:  "--confirm is required",
			wantHelp: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if test.wantHelp {
					if !errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected ErrHelp, got %v", err)
					}
				} else {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					if errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected non-help error, got %v", err)
					}
				}
			})

			if test.wantHelp {
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				if !strings.Contains(stderr, test.wantErr) {
					t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
				}
			}
		})
	}
}

func TestRoutingCoverageValidationErrors(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantHelp bool
	}{
		{
			name:     "routing-coverage get missing version id",
			args:     []string{"routing-coverage", "get"},
			wantErr:  "--version-id is required",
			wantHelp: true,
		},
		{
			name:     "routing-coverage info missing id",
			args:     []string{"routing-coverage", "info"},
			wantErr:  "--id is required",
			wantHelp: true,
		},
		{
			name:     "routing-coverage create missing version id",
			args:     []string{"routing-coverage", "create", "--file", "coverage.geojson"},
			wantErr:  "--version-id is required",
			wantHelp: true,
		},
		{
			name:     "routing-coverage create missing file",
			args:     []string{"routing-coverage", "create", "--version-id", "VERSION_ID"},
			wantErr:  "--file is required",
			wantHelp: true,
		},
		{
			name:     "routing-coverage delete missing id",
			args:     []string{"routing-coverage", "delete", "--confirm"},
			wantErr:  "--id is required",
			wantHelp: true,
		},
		{
			name:     "routing-coverage delete missing confirm",
			args:     []string{"routing-coverage", "delete", "--id", "COVERAGE_ID"},
			wantErr:  "--confirm is required",
			wantHelp: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if test.wantHelp {
					if !errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected ErrHelp, got %v", err)
					}
				} else {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					if errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected non-help error, got %v", err)
					}
				}
			})

			if test.wantHelp {
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				if !strings.Contains(stderr, test.wantErr) {
					t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
				}
			}
		})
	}
}

func TestEncryptionValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantHelp bool
	}{
		{
			name:     "encryption declarations list missing app",
			args:     []string{"encryption", "declarations", "list"},
			wantErr:  "--app is required",
			wantHelp: true,
		},
		{
			name:     "encryption declarations get missing id",
			args:     []string{"encryption", "declarations", "get"},
			wantErr:  "--id is required",
			wantHelp: true,
		},
		{
			name:     "encryption declarations create missing app",
			args:     []string{"encryption", "declarations", "create", "--app-description", "Uses TLS", "--contains-proprietary-cryptography=false", "--contains-third-party-cryptography=true", "--available-on-french-store=true"},
			wantErr:  "--app is required",
			wantHelp: true,
		},
		{
			name:     "encryption declarations create missing description",
			args:     []string{"encryption", "declarations", "create", "--app", "APP_ID", "--contains-proprietary-cryptography=false", "--contains-third-party-cryptography=true", "--available-on-french-store=true"},
			wantErr:  "--app-description is required",
			wantHelp: true,
		},
		{
			name:     "encryption declarations create missing proprietary flag",
			args:     []string{"encryption", "declarations", "create", "--app", "APP_ID", "--app-description", "Uses TLS", "--contains-third-party-cryptography=true", "--available-on-french-store=true"},
			wantErr:  "--contains-proprietary-cryptography is required",
			wantHelp: true,
		},
		{
			name:     "encryption declarations create missing third-party flag",
			args:     []string{"encryption", "declarations", "create", "--app", "APP_ID", "--app-description", "Uses TLS", "--contains-proprietary-cryptography=false", "--available-on-french-store=true"},
			wantErr:  "--contains-third-party-cryptography is required",
			wantHelp: true,
		},
		{
			name:     "encryption declarations create missing french store flag",
			args:     []string{"encryption", "declarations", "create", "--app", "APP_ID", "--app-description", "Uses TLS", "--contains-proprietary-cryptography=false", "--contains-third-party-cryptography=true"},
			wantErr:  "--available-on-french-store is required",
			wantHelp: true,
		},
		{
			name:     "encryption declarations assign-builds missing id",
			args:     []string{"encryption", "declarations", "assign-builds", "--build", "BUILD_ID"},
			wantErr:  "--id is required",
			wantHelp: true,
		},
		{
			name:     "encryption declarations assign-builds missing build",
			args:     []string{"encryption", "declarations", "assign-builds", "--id", "DECL_ID"},
			wantErr:  "--build is required",
			wantHelp: true,
		},
		{
			name:     "encryption documents get missing id",
			args:     []string{"encryption", "documents", "get"},
			wantErr:  "--id is required",
			wantHelp: true,
		},
		{
			name:     "encryption documents upload missing declaration",
			args:     []string{"encryption", "documents", "upload", "--file", "export.pdf"},
			wantErr:  "--declaration is required",
			wantHelp: true,
		},
		{
			name:     "encryption documents upload missing file",
			args:     []string{"encryption", "documents", "upload", "--declaration", "DECL_ID"},
			wantErr:  "--file is required",
			wantHelp: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if test.wantHelp {
					if !errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected ErrHelp, got %v", err)
					}
				} else {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					if errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected non-help error, got %v", err)
					}
				}
			})

			if test.wantHelp {
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				if !strings.Contains(stderr, test.wantErr) {
					t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
				}
			}
		})
	}
}

func TestAndroidIosMappingValidationErrors(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantHelp bool
	}{
		{
			name:     "android-ios-mapping list missing app",
			args:     []string{"android-ios-mapping", "list"},
			wantErr:  "--app is required",
			wantHelp: true,
		},
		{
			name:     "android-ios-mapping get missing id",
			args:     []string{"android-ios-mapping", "get"},
			wantErr:  "--mapping-id is required",
			wantHelp: true,
		},
		{
			name:     "android-ios-mapping create missing app",
			args:     []string{"android-ios-mapping", "create", "--android-package-name", "com.example.android", "--fingerprints", "sha1"},
			wantErr:  "--app is required",
			wantHelp: true,
		},
		{
			name:     "android-ios-mapping create missing package",
			args:     []string{"android-ios-mapping", "create", "--app", "APP_ID", "--fingerprints", "sha1"},
			wantErr:  "--android-package-name is required",
			wantHelp: true,
		},
		{
			name:     "android-ios-mapping create missing fingerprints",
			args:     []string{"android-ios-mapping", "create", "--app", "APP_ID", "--android-package-name", "com.example.android"},
			wantErr:  "--fingerprints is required",
			wantHelp: true,
		},
		{
			name:     "android-ios-mapping update missing id",
			args:     []string{"android-ios-mapping", "update", "--android-package-name", "com.example.android"},
			wantErr:  "--mapping-id is required",
			wantHelp: true,
		},
		{
			name:    "android-ios-mapping update missing updates",
			args:    []string{"android-ios-mapping", "update", "--mapping-id", "MAP_ID"},
			wantErr: "at least one update flag is required",
		},
		{
			name:     "android-ios-mapping delete missing id",
			args:     []string{"android-ios-mapping", "delete", "--confirm"},
			wantErr:  "--mapping-id is required",
			wantHelp: true,
		},
		{
			name:     "android-ios-mapping delete missing confirm",
			args:     []string{"android-ios-mapping", "delete", "--mapping-id", "MAP_ID"},
			wantErr:  "--confirm is required",
			wantHelp: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if test.wantHelp {
					if !errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected ErrHelp, got %v", err)
					}
				} else {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					if errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected non-help error, got %v", err)
					}
				}
			})

			if test.wantHelp {
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				if !strings.Contains(stderr, test.wantErr) {
					t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
				}
			}
		})
	}
}

func TestPerformanceValidationErrors(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantHelp bool
	}{
		{
			name:     "performance metrics list missing app",
			args:     []string{"performance", "metrics", "list"},
			wantErr:  "--app is required",
			wantHelp: true,
		},
		{
			name:     "performance metrics get missing build",
			args:     []string{"performance", "metrics", "get"},
			wantErr:  "--build is required",
			wantHelp: true,
		},
		{
			name:     "performance diagnostics list missing build",
			args:     []string{"performance", "diagnostics", "list"},
			wantErr:  "--build is required",
			wantHelp: true,
		},
		{
			name:     "performance diagnostics get missing id",
			args:     []string{"performance", "diagnostics", "get"},
			wantErr:  "--id is required",
			wantHelp: true,
		},
		{
			name:     "performance download missing selection",
			args:     []string{"performance", "download"},
			wantErr:  "--app, --build, or --diagnostic-id is required",
			wantHelp: true,
		},
		{
			name:    "performance download mutually exclusive",
			args:    []string{"performance", "download", "--app", "APP_ID", "--build", "BUILD_ID"},
			wantErr: "mutually exclusive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if test.wantHelp {
					if !errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected ErrHelp, got %v", err)
					}
				} else {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					if errors.Is(err, flag.ErrHelp) {
						t.Fatalf("expected non-help error, got %v", err)
					}
				}
			})

			if test.wantHelp {
				if stdout != "" {
					t.Fatalf("expected empty stdout, got %q", stdout)
				}
				if !strings.Contains(stderr, test.wantErr) {
					t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
				}
			}
		})
	}
}

func TestTestFlightBetaDetailsValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "beta-details get missing build",
			args:    []string{"testflight", "beta-details", "get"},
			wantErr: "--build is required",
		},
		{
			name:    "beta-details update missing id",
			args:    []string{"testflight", "beta-details", "update"},
			wantErr: "--id is required",
		},
		{
			name:    "beta-details update missing updates",
			args:    []string{"testflight", "beta-details", "update", "--id", "DETAIL_ID"},
			wantErr: "at least one update flag is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestTestFlightRecruitmentValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "recruitment set missing group",
			args:    []string{"testflight", "recruitment", "set", "--criteria-id", "OPTION_ID"},
			wantErr: "--group is required",
		},
		{
			name:    "recruitment set missing criteria",
			args:    []string{"testflight", "recruitment", "set", "--group", "GROUP_ID"},
			wantErr: "--criteria-id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestTestFlightMetricsValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "metrics public-link missing group",
			args:    []string{"testflight", "metrics", "public-link"},
			wantErr: "--group is required",
		},
		{
			name:    "metrics testers missing group",
			args:    []string{"testflight", "metrics", "testers"},
			wantErr: "--group is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestTestFlightSyncValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "testflight sync pull missing app",
			args:    []string{"testflight", "sync", "pull", "--output", "./testflight.yaml"},
			wantErr: "--app is required",
		},
		{
			name:    "testflight sync pull missing output",
			args:    []string{"testflight", "sync", "pull", "--app", "APP_ID"},
			wantErr: "--output is required",
		},
		{
			name:    "testflight sync pull build filter without include",
			args:    []string{"testflight", "sync", "pull", "--app", "APP_ID", "--output", "./testflight.yaml", "--build", "BUILD_ID"},
			wantErr: "--build requires --include-builds",
		},
		{
			name:    "testflight sync pull tester filter without include",
			args:    []string{"testflight", "sync", "pull", "--app", "APP_ID", "--output", "./testflight.yaml", "--tester", "tester@example.com"},
			wantErr: "--tester requires --include-testers",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestParseCommaSeparatedIDs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty input",
			input: "",
			want:  []string{},
		},
		{
			name:  "single id",
			input: "tester-1",
			want:  []string{"tester-1"},
		},
		{
			name:  "comma separated",
			input: "tester-1, tester-2, tester-3",
			want:  []string{"tester-1", "tester-2", "tester-3"},
		},
		{
			name:  "drops empty entries",
			input: "tester-1,, ,tester-2",
			want:  []string{"tester-1", "tester-2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := parseCommaSeparatedIDs(test.input)
			if len(got) != len(test.want) {
				t.Fatalf("expected %d ids, got %d", len(test.want), len(got))
			}
			for i, want := range test.want {
				if got[i] != want {
					t.Fatalf("expected %q at index %d, got %q", want, i, got[i])
				}
			}
		})
	}
}

func TestBetaTestersListAcceptsBuildFilter(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{"testflight", "beta-testers", "list", "--app", "X", "--build", "Y"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestLocalizationsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "localizations list missing version",
			args:    []string{"localizations", "list"},
			wantErr: "--version is required",
		},
		{
			name:    "localizations list missing app for app-info",
			args:    []string{"localizations", "list", "--type", "app-info"},
			wantErr: "--app is required",
		},
		{
			name:    "localizations download missing version",
			args:    []string{"localizations", "download"},
			wantErr: "--version is required",
		},
		{
			name:    "localizations download missing app for app-info",
			args:    []string{"localizations", "download", "--type", "app-info"},
			wantErr: "--app is required",
		},
		{
			name:    "localizations upload missing path",
			args:    []string{"localizations", "upload", "--version", "VERSION_ID"},
			wantErr: "--path is required",
		},
		{
			name:    "localizations upload missing version",
			args:    []string{"localizations", "upload", "--path", "localizations"},
			wantErr: "--version is required",
		},
		{
			name:    "localizations upload missing app for app-info",
			args:    []string{"localizations", "upload", "--type", "app-info", "--path", "localizations"},
			wantErr: "--app is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAssetsValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "assets screenshots list missing localization",
			args:    []string{"assets", "screenshots", "list"},
			wantErr: "--version-localization is required",
		},
		{
			name:    "assets screenshots upload missing localization",
			args:    []string{"assets", "screenshots", "upload", "--path", "./screenshots", "--device-type", "IPHONE_65"},
			wantErr: "--version-localization is required",
		},
		{
			name:    "assets screenshots upload missing path",
			args:    []string{"assets", "screenshots", "upload", "--version-localization", "LOC_ID", "--device-type", "IPHONE_65"},
			wantErr: "--path is required",
		},
		{
			name:    "assets screenshots upload missing device type",
			args:    []string{"assets", "screenshots", "upload", "--version-localization", "LOC_ID", "--path", "./screenshots"},
			wantErr: "--device-type is required",
		},
		{
			name:    "assets screenshots delete missing id",
			args:    []string{"assets", "screenshots", "delete"},
			wantErr: "--id is required",
		},
		{
			name:    "assets screenshots delete missing confirm",
			args:    []string{"assets", "screenshots", "delete", "--id", "SCREENSHOT_ID"},
			wantErr: "--confirm is required to delete",
		},
		{
			name:    "assets previews list missing localization",
			args:    []string{"assets", "previews", "list"},
			wantErr: "--version-localization is required",
		},
		{
			name:    "assets previews upload missing localization",
			args:    []string{"assets", "previews", "upload", "--path", "./previews", "--device-type", "IPHONE_65"},
			wantErr: "--version-localization is required",
		},
		{
			name:    "assets previews upload missing path",
			args:    []string{"assets", "previews", "upload", "--version-localization", "LOC_ID", "--device-type", "IPHONE_65"},
			wantErr: "--path is required",
		},
		{
			name:    "assets previews upload missing device type",
			args:    []string{"assets", "previews", "upload", "--version-localization", "LOC_ID", "--path", "./previews"},
			wantErr: "--device-type is required",
		},
		{
			name:    "assets previews delete missing id",
			args:    []string{"assets", "previews", "delete"},
			wantErr: "--id is required",
		},
		{
			name:    "assets previews delete missing confirm",
			args:    []string{"assets", "previews", "delete", "--id", "PREVIEW_ID"},
			wantErr: "--confirm is required to delete",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestBuildLocalizationsValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "build-localizations list missing build",
			args:    []string{"build-localizations", "list"},
			wantErr: "--build is required",
		},
		{
			name:    "build-localizations create missing locale",
			args:    []string{"build-localizations", "create", "--build", "BUILD_ID"},
			wantErr: "--locale is required",
		},
		{
			name:    "build-localizations delete missing confirm",
			args:    []string{"build-localizations", "delete", "--id", "LOC_ID"},
			wantErr: "--confirm is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestBuildsTestNotesValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "builds test-notes list missing build",
			args:    []string{"builds", "test-notes", "list"},
			wantErr: "--build is required",
		},
		{
			name:    "builds test-notes create missing locale",
			args:    []string{"builds", "test-notes", "create", "--build", "BUILD_ID", "--whats-new", "Notes"},
			wantErr: "--locale is required",
		},
		{
			name:    "builds test-notes create missing whats-new",
			args:    []string{"builds", "test-notes", "create", "--build", "BUILD_ID", "--locale", "en-US"},
			wantErr: "--whats-new is required",
		},
		{
			name:    "builds test-notes update missing id",
			args:    []string{"builds", "test-notes", "update", "--whats-new", "Notes"},
			wantErr: "--id is required",
		},
		{
			name:    "builds test-notes delete missing confirm",
			args:    []string{"builds", "test-notes", "delete", "--id", "LOC_ID"},
			wantErr: "--confirm is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestBuildsUploadValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"builds", "upload", "--ipa", "app.ipa", "--version", "1.0.0", "--build-number", "123"},
			wantErr: "Error: --app is required",
		},
		{
			name:    "missing ipa",
			args:    []string{"builds", "upload", "--app", "APP_123", "--version", "1.0.0", "--build-number", "123"},
			wantErr: "Error: --ipa is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestPublishValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "publish testflight missing app",
			args:    []string{"publish", "testflight", "--ipa", "app.ipa", "--group", "GROUP_ID"},
			wantErr: "Error: --app is required",
		},
		{
			name:    "publish testflight missing ipa",
			args:    []string{"publish", "testflight", "--app", "APP_123", "--group", "GROUP_ID"},
			wantErr: "Error: --ipa is required",
		},
		{
			name:    "publish testflight missing group",
			args:    []string{"publish", "testflight", "--app", "APP_123", "--ipa", "app.ipa"},
			wantErr: "Error: --group is required",
		},
		{
			name:    "publish testflight test-notes missing locale",
			args:    []string{"publish", "testflight", "--app", "APP_123", "--ipa", "app.ipa", "--group", "GROUP_ID", "--test-notes", "Notes"},
			wantErr: "Error: --locale is required with --test-notes",
		},
		{
			name:    "publish testflight locale missing test-notes",
			args:    []string{"publish", "testflight", "--app", "APP_123", "--ipa", "app.ipa", "--group", "GROUP_ID", "--locale", "en-US"},
			wantErr: "Error: --test-notes is required with --locale",
		},
		{
			name:    "publish appstore missing app",
			args:    []string{"publish", "appstore", "--ipa", "app.ipa", "--version", "1.0.0"},
			wantErr: "Error: --app is required",
		},
		{
			name:    "publish appstore missing ipa",
			args:    []string{"publish", "appstore", "--app", "APP_123", "--version", "1.0.0"},
			wantErr: "Error: --ipa is required",
		},
		{
			name:    "publish appstore submit missing confirm",
			args:    []string{"publish", "appstore", "--app", "APP_123", "--ipa", "app.ipa", "--version", "1.0.0", "--submit"},
			wantErr: "Error: --confirm is required with --submit",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubmitValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "create missing confirm",
			args:    []string{"submit", "create", "--app", "APP_123", "--version", "1.0.0", "--build", "BUILD_123"},
			wantErr: "Error: --confirm is required",
		},
		{
			name:    "create missing build",
			args:    []string{"submit", "create", "--app", "APP_123", "--version", "1.0.0", "--confirm"},
			wantErr: "Error: --build is required",
		},
		{
			name:    "create missing version",
			args:    []string{"submit", "create", "--app", "APP_123", "--build", "BUILD_123", "--confirm"},
			wantErr: "Error: --version or --version-id is required",
		},
		{
			name:    "status missing id",
			args:    []string{"submit", "status"},
			wantErr: "Error: --id or --version-id is required",
		},
		{
			name:    "cancel missing confirm",
			args:    []string{"submit", "cancel", "--id", "SUBMIT_123"},
			wantErr: "Error: --confirm is required",
		},
		{
			name:    "cancel missing id",
			args:    []string{"submit", "cancel", "--confirm"},
			wantErr: "Error: --id or --version-id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubmitValidationConflicts(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	err := root.Parse([]string{"submit", "create", "--app", "APP_123", "--version", "1.0.0", "--version-id", "VERSION_123", "--build", "BUILD_123", "--confirm"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err == nil {
		t.Fatalf("expected error for conflicting flags")
	}
}

func TestVersionsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "list missing app",
			args:    []string{"versions", "list"},
			wantErr: "Error: --app is required",
		},
		{
			name:    "get missing version id",
			args:    []string{"versions", "get"},
			wantErr: "Error: --version-id is required",
		},
		{
			name:    "attach missing version id",
			args:    []string{"versions", "attach-build", "--build", "BUILD_123"},
			wantErr: "Error: --version-id is required",
		},
		{
			name:    "attach missing build",
			args:    []string{"versions", "attach-build", "--version-id", "VERSION_123"},
			wantErr: "Error: --build is required",
		},
		{
			name:    "release missing version id",
			args:    []string{"versions", "release", "--confirm"},
			wantErr: "Error: --version-id is required",
		},
		{
			name:    "release missing confirm",
			args:    []string{"versions", "release", "--version-id", "VERSION_123"},
			wantErr: "Error: --confirm is required to release a version",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAppInfoValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "app-info get missing app",
			args:    []string{"app-info", "get"},
			wantErr: "--app or --app-info is required",
		},
		{
			name:    "app-info get version missing platform",
			args:    []string{"app-info", "get", "--app", "APP_ID", "--version", "1.0.0"},
			wantErr: "--platform is required with --version",
		},
		{
			name:    "app-info set missing locale",
			args:    []string{"app-info", "set", "--app", "APP_ID", "--whats-new", "Fixes"},
			wantErr: "--locale is required",
		},
		{
			name:    "app-info set missing update fields",
			args:    []string{"app-info", "set", "--app", "APP_ID", "--locale", "en-US"},
			wantErr: "at least one update flag is required",
		},
		{
			name:    "app-info set missing app",
			args:    []string{"app-info", "set", "--locale", "en-US", "--whats-new", "Fixes"},
			wantErr: "--app is required",
		},
		{
			name:    "app-info set version missing platform",
			args:    []string{"app-info", "set", "--app", "APP_ID", "--version", "1.0.0", "--locale", "en-US", "--whats-new", "Fixes"},
			wantErr: "--platform is required with --version",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAppInfoMutualExclusiveFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "app-info get version and version-id are mutually exclusive",
			args:    []string{"app-info", "get", "--app", "APP_ID", "--version", "1.0.0", "--version-id", "VERSION_ID"},
			wantErr: "--version and --version-id are mutually exclusive",
		},
		{
			name:    "app-info set version and version-id are mutually exclusive",
			args:    []string{"app-info", "set", "--app", "APP_ID", "--version", "1.0.0", "--version-id", "VERSION_ID", "--locale", "en-US", "--whats-new", "Fixes"},
			wantErr: "--version and --version-id are mutually exclusive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			_, _ = captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected non-help error, got %v", err)
				}
			})
		})
	}
}

func TestAppTagsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "app-tags list missing app",
			args:    []string{"app-tags", "list"},
			wantErr: "Error: --app is required",
		},
		{
			name:    "app-tags get missing id",
			args:    []string{"app-tags", "get", "--app", "APP_ID"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "app-tags get missing app",
			args:    []string{"app-tags", "get", "--id", "TAG_ID"},
			wantErr: "Error: --app is required",
		},
		{
			name:    "app-tags update missing id",
			args:    []string{"app-tags", "update", "--visible-in-app-store", "--confirm"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "app-tags update missing visible",
			args:    []string{"app-tags", "update", "--id", "TAG_ID", "--confirm"},
			wantErr: "Error: --visible-in-app-store is required",
		},
		{
			name:    "app-tags update missing confirm",
			args:    []string{"app-tags", "update", "--id", "TAG_ID", "--visible-in-app-store"},
			wantErr: "Error: --confirm is required",
		},
		{
			name:    "app-tags territories missing id",
			args:    []string{"app-tags", "territories"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "app-tags territories-relationships missing id",
			args:    []string{"app-tags", "territories-relationships"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "app-tags relationships missing app",
			args:    []string{"app-tags", "relationships"},
			wantErr: "Error: --app is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAppClipsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "app-clips list missing app",
			args:    []string{"app-clips", "list"},
			wantErr: "Error: --app is required",
		},
		{
			name:    "app-clips get missing id",
			args:    []string{"app-clips", "get"},
			wantErr: "Error: --id is required",
		},
		{
			name:    "default experiences list missing app-clip-id",
			args:    []string{"app-clips", "default-experiences", "list"},
			wantErr: "Error: --app-clip-id is required",
		},
		{
			name:    "default experiences update missing experience-id",
			args:    []string{"app-clips", "default-experiences", "update"},
			wantErr: "Error: --experience-id is required",
		},
		{
			name:    "default experiences update missing updates",
			args:    []string{"app-clips", "default-experiences", "update", "--experience-id", "EXP_ID"},
			wantErr: "Error: at least one update flag is required",
		},
		{
			name:    "default experiences delete missing confirm",
			args:    []string{"app-clips", "default-experiences", "delete", "--experience-id", "EXP_ID"},
			wantErr: "Error: --confirm is required to delete",
		},
		{
			name:    "default experience localizations list missing experience",
			args:    []string{"app-clips", "default-experiences", "localizations", "list"},
			wantErr: "Error: --experience-id is required",
		},
		{
			name:    "default experience localizations create missing locale",
			args:    []string{"app-clips", "default-experiences", "localizations", "create", "--experience-id", "EXP_ID"},
			wantErr: "Error: --locale is required",
		},
		{
			name:    "default experience localizations header image relationship missing localization",
			args:    []string{"app-clips", "default-experiences", "localizations", "header-image-relationship"},
			wantErr: "Error: --localization-id is required",
		},
		{
			name:    "default experiences review detail missing experience-id",
			args:    []string{"app-clips", "default-experiences", "review-detail"},
			wantErr: "Error: --experience-id is required",
		},
		{
			name:    "default experiences release version missing experience-id",
			args:    []string{"app-clips", "default-experiences", "release-with-app-store-version"},
			wantErr: "Error: --experience-id is required",
		},
		{
			name:    "default experiences relationships missing app clip",
			args:    []string{"app-clips", "default-experiences-relationships"},
			wantErr: "Error: --app-clip-id is required",
		},
		{
			name:    "advanced experiences relationships missing app clip",
			args:    []string{"app-clips", "advanced-experiences-relationships"},
			wantErr: "Error: --app-clip-id is required",
		},
		{
			name:    "advanced experiences create missing link",
			args:    []string{"app-clips", "advanced-experiences", "create", "--app-clip-id", "CLIP_ID", "--default-language", "EN", "--is-powered-by"},
			wantErr: "Error: --link is required",
		},
		{
			name:    "advanced experiences create missing default language",
			args:    []string{"app-clips", "advanced-experiences", "create", "--app-clip-id", "CLIP_ID", "--link", "https://example.com", "--is-powered-by"},
			wantErr: "Error: --default-language is required",
		},
		{
			name:    "advanced experiences create missing powered-by",
			args:    []string{"app-clips", "advanced-experiences", "create", "--app-clip-id", "CLIP_ID", "--link", "https://example.com", "--default-language", "EN"},
			wantErr: "Error: --is-powered-by is required",
		},
		{
			name:    "advanced experience images create missing file",
			args:    []string{"app-clips", "advanced-experiences", "images", "create", "--experience-id", "EXP_ID"},
			wantErr: "Error: --file is required",
		},
		{
			name:    "header images create missing localization",
			args:    []string{"app-clips", "header-images", "create", "--file", "image.png"},
			wantErr: "Error: --localization-id is required",
		},
		{
			name:    "invocations list missing build bundle",
			args:    []string{"app-clips", "invocations", "list"},
			wantErr: "Error: --build-bundle-id is required",
		},
		{
			name:    "domain status cache missing build bundle",
			args:    []string{"app-clips", "domain-status", "cache"},
			wantErr: "Error: --build-bundle-id is required",
		},
		{
			name:    "domain status debug missing build bundle",
			args:    []string{"app-clips", "domain-status", "debug"},
			wantErr: "Error: --build-bundle-id is required",
		},
		{
			name:    "invocations delete missing confirm",
			args:    []string{"app-clips", "invocations", "delete", "--invocation-id", "INV_ID"},
			wantErr: "Error: --confirm is required to delete",
		},
		{
			name:    "invocation localizations list missing invocation",
			args:    []string{"app-clips", "invocations", "localizations", "list"},
			wantErr: "Error: --invocation-id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestPreReleaseVersionsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "pre-release-versions list missing app",
			args:    []string{"pre-release-versions", "list"},
			wantErr: "Error: --app is required",
		},
		{
			name:    "pre-release-versions get missing id",
			args:    []string{"pre-release-versions", "get"},
			wantErr: "Error: --id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAuthSwitchValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "switch"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --name is required") {
		t.Fatalf("expected missing --name error, got %q", stderr)
	}
}

func TestAuthLogoutBlankNameValidation(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "logout", "--name", "   "}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected non-help error, got %v", err)
		}
	})
}

func TestAuthSwitchUnknownProfile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	cfg := &config.Config{
		DefaultKeyName: "personal",
		Keys: []config.Credential{
			{
				Name:           "personal",
				KeyID:          "KEY123",
				IssuerID:       "ISS456",
				PrivateKeyPath: "/tmp/AuthKey.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "switch", "--name", "missing"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected non-help error, got %v", err)
		}
	})
}

func TestAuthStatusShowsEnvPreference(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "/tmp/AuthKey.p8")

	previousProfile := shared.SelectedProfile()
	shared.SetSelectedProfile("")
	t.Cleanup(func() {
		shared.SetSelectedProfile(previousProfile)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "status"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Environment credentials detected") {
		t.Fatalf("expected env credentials note, got %q", stdout)
	}
}

func TestAuthStatusEnvIncomplete(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")

	previousProfile := shared.SelectedProfile()
	shared.SetSelectedProfile("")
	t.Cleanup(func() {
		shared.SetSelectedProfile(previousProfile)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "status"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Environment credentials are incomplete") {
		t.Fatalf("expected env incomplete note, got %q", stdout)
	}
}

func TestAuthStatusProfileOverridesEnvNote(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "client")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "/tmp/AuthKey.p8")

	previousProfile := shared.SelectedProfile()
	shared.SetSelectedProfile("")
	t.Cleanup(func() {
		shared.SetSelectedProfile(previousProfile)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "status"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `Profile "client" selected; environment credentials will be ignored.`) {
		t.Fatalf("expected profile override note, got %q", stdout)
	}
}

func TestAuthStatusShowsStorageLocation(t *testing.T) {
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

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")

	previousProfile := shared.SelectedProfile()
	shared.SetSelectedProfile("")
	t.Cleanup(func() {
		shared.SetSelectedProfile(previousProfile)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "status"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Credential storage: Config File") {
		t.Fatalf("expected config storage output, got %q", stdout)
	}
	if !strings.Contains(stdout, fmt.Sprintf("Location: %s", configPath)) {
		t.Fatalf("expected config path in output, got %q", stdout)
	}
}

func TestAuthDoctorFixRequiresConfirm(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "doctor", "--fix"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestAuthStatusValidateSuccess(t *testing.T) {
	tempDir := t.TempDir()
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
	configPath := filepath.Join(tempDir, "config.json")
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")

	previousProfile := shared.SelectedProfile()
	shared.SetSelectedProfile("")
	t.Cleanup(func() {
		shared.SetSelectedProfile(previousProfile)
	})

	restoreValidator := authcli.SetStatusValidateCredential(func(ctx context.Context, cred auth.Credential) error {
		return nil
	})
	t.Cleanup(restoreValidator)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "status", "--validate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "default (Key ID: KEY123): works") {
		t.Fatalf("expected validation ok output, got %q", stdout)
	}
}

func TestAuthStatusValidateForbiddenReportsWorks(t *testing.T) {
	tempDir := t.TempDir()
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
	configPath := filepath.Join(tempDir, "config.json")
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")

	previousProfile := shared.SelectedProfile()
	shared.SetSelectedProfile("")
	t.Cleanup(func() {
		shared.SetSelectedProfile(previousProfile)
	})

	restoreValidator := authcli.SetStatusValidateCredential(func(ctx context.Context, cred auth.Credential) error {
		return authcli.NewPermissionWarning(errors.New("forbidden"))
	})
	t.Cleanup(restoreValidator)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "status", "--validate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "default (Key ID: KEY123): works (insufficient permissions for apps list)") {
		t.Fatalf("expected insufficient permissions output, got %q", stdout)
	}
}

func TestAuthStatusValidateFailureReturnsReportedError(t *testing.T) {
	tempDir := t.TempDir()
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
	configPath := filepath.Join(tempDir, "config.json")
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")

	previousProfile := shared.SelectedProfile()
	shared.SetSelectedProfile("")
	t.Cleanup(func() {
		shared.SetSelectedProfile(previousProfile)
	})

	restoreValidator := authcli.SetStatusValidateCredential(func(ctx context.Context, cred auth.Credential) error {
		return errors.New("validation failed")
	})
	t.Cleanup(restoreValidator)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"auth", "status", "--validate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err == nil {
			t.Fatal("expected error, got nil")
		} else {
			var reported ReportedError
			if !errors.As(err, &reported) {
				t.Fatalf("expected reported error, got %v", err)
			}
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "default (Key ID: KEY123): failed") {
		t.Fatalf("expected validation failed output, got %q", stdout)
	}
}

func TestAuthLoginValidationFailurePreventsStore(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)

	workDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	restoreGenerator := authcli.SetLoginJWTGenerator(func(_, _ string, _ *ecdsa.PrivateKey) (string, error) {
		return "", errors.New("jwt failure")
	})
	t.Cleanup(restoreGenerator)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{
			"auth", "login",
			"--bypass-keychain",
			"--local",
			"--name", "TestKey",
			"--key-id", "KEY123",
			"--issuer-id", "ISS456",
			"--private-key", keyPath,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	configPath := filepath.Join(workDir, ".asc", "config.json")
	if _, err := os.Stat(configPath); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected config not to be written, got %v", err)
	}
}

func TestAuthLoginSkipValidationBypassesJWT(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)

	workDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	restoreGenerator := authcli.SetLoginJWTGenerator(func(_, _ string, _ *ecdsa.PrivateKey) (string, error) {
		return "", errors.New("jwt failure")
	})
	t.Cleanup(restoreGenerator)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{
			"auth", "login",
			"--bypass-keychain",
			"--local",
			"--skip-validation",
			"--name", "TestKey",
			"--key-id", "KEY123",
			"--issuer-id", "ISS456",
			"--private-key", keyPath,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	configPath := filepath.Join(workDir, ".asc", "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config to be written, got %v", err)
	}
}

func TestAuthLoginUsesEnvBypass(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)

	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{
			"auth", "login",
			"--skip-validation",
			"--name", "EnvKey",
			"--key-id", "KEY123",
			"--issuer-id", "ISS456",
			"--private-key", keyPath,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	globalPath, err := config.GlobalPath()
	if err != nil {
		t.Fatalf("GlobalPath() error: %v", err)
	}
	if _, err := os.Stat(globalPath); err != nil {
		t.Fatalf("expected config to be written, got %v", err)
	}
}

func TestXcodeCloudValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "xcode-cloud run missing workflow",
			args:    []string{"xcode-cloud", "run", "--branch", "main"},
			wantErr: "--workflow or --workflow-id is required",
		},
		{
			name:    "xcode-cloud run missing branch",
			args:    []string{"xcode-cloud", "run", "--workflow-id", "WF_ID"},
			wantErr: "--branch or --git-reference-id is required",
		},
		{
			name:    "xcode-cloud run workflow by name without app",
			args:    []string{"xcode-cloud", "run", "--workflow", "CI", "--branch", "main"},
			wantErr: "--app is required when using --workflow",
		},
		{
			name:    "xcode-cloud status missing run-id",
			args:    []string{"xcode-cloud", "status"},
			wantErr: "--run-id is required",
		},
		{
			name:    "xcode-cloud workflows missing app",
			args:    []string{"xcode-cloud", "workflows"},
			wantErr: "--app is required",
		},
		{
			name:    "xcode-cloud workflows list missing app",
			args:    []string{"xcode-cloud", "workflows", "list"},
			wantErr: "--app is required",
		},
		{
			name:    "xcode-cloud workflows get missing id",
			args:    []string{"xcode-cloud", "workflows", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud workflows repository missing id",
			args:    []string{"xcode-cloud", "workflows", "repository"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud workflows create missing file",
			args:    []string{"xcode-cloud", "workflows", "create"},
			wantErr: "--file is required",
		},
		{
			name:    "xcode-cloud workflows update missing id",
			args:    []string{"xcode-cloud", "workflows", "update", "--file", "./workflow.json"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud workflows update missing file",
			args:    []string{"xcode-cloud", "workflows", "update", "--id", "WF_ID"},
			wantErr: "--file is required",
		},
		{
			name:    "xcode-cloud workflows delete missing id",
			args:    []string{"xcode-cloud", "workflows", "delete", "--confirm"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud workflows delete missing confirm",
			args:    []string{"xcode-cloud", "workflows", "delete", "--id", "WF_ID"},
			wantErr: "--confirm is required",
		},
		{
			name:    "xcode-cloud build-runs missing workflow-id",
			args:    []string{"xcode-cloud", "build-runs"},
			wantErr: "--workflow-id is required",
		},
		{
			name:    "xcode-cloud build-runs builds missing run-id",
			args:    []string{"xcode-cloud", "build-runs", "builds"},
			wantErr: "--run-id is required",
		},
		{
			name:    "xcode-cloud actions missing run-id",
			args:    []string{"xcode-cloud", "actions"},
			wantErr: "--run-id is required",
		},
		{
			name:    "xcode-cloud actions get missing id",
			args:    []string{"xcode-cloud", "actions", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud actions build-run missing id",
			args:    []string{"xcode-cloud", "actions", "build-run"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud artifacts list missing action-id",
			args:    []string{"xcode-cloud", "artifacts", "list"},
			wantErr: "--action-id is required",
		},
		{
			name:    "xcode-cloud artifacts get missing id",
			args:    []string{"xcode-cloud", "artifacts", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud artifacts download missing id",
			args:    []string{"xcode-cloud", "artifacts", "download", "--path", "./artifact.zip"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud artifacts download missing path",
			args:    []string{"xcode-cloud", "artifacts", "download", "--id", "ART_ID"},
			wantErr: "--path is required",
		},
		{
			name:    "xcode-cloud test-results list missing action-id",
			args:    []string{"xcode-cloud", "test-results", "list"},
			wantErr: "--action-id is required",
		},
		{
			name:    "xcode-cloud test-results get missing id",
			args:    []string{"xcode-cloud", "test-results", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud issues list missing action-id",
			args:    []string{"xcode-cloud", "issues", "list"},
			wantErr: "--action-id is required",
		},
		{
			name:    "xcode-cloud issues get missing id",
			args:    []string{"xcode-cloud", "issues", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud products get missing id",
			args:    []string{"xcode-cloud", "products", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud products app missing id",
			args:    []string{"xcode-cloud", "products", "app"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud products build-runs missing id",
			args:    []string{"xcode-cloud", "products", "build-runs"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud products workflows missing id",
			args:    []string{"xcode-cloud", "products", "workflows"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud products primary-repositories missing id",
			args:    []string{"xcode-cloud", "products", "primary-repositories"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud products additional-repositories missing id",
			args:    []string{"xcode-cloud", "products", "additional-repositories"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud products delete missing id",
			args:    []string{"xcode-cloud", "products", "delete", "--confirm"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud products delete missing confirm",
			args:    []string{"xcode-cloud", "products", "delete", "--id", "PROD_ID"},
			wantErr: "--confirm is required",
		},
		{
			name:    "xcode-cloud macos-versions get missing id",
			args:    []string{"xcode-cloud", "macos-versions", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud macos-versions xcode-versions missing id",
			args:    []string{"xcode-cloud", "macos-versions", "xcode-versions"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud xcode-versions get missing id",
			args:    []string{"xcode-cloud", "xcode-versions", "get"},
			wantErr: "--id is required",
		},
		{
			name:    "xcode-cloud xcode-versions macos-versions missing id",
			args:    []string{"xcode-cloud", "xcode-versions", "macos-versions"},
			wantErr: "--id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestXcodeCloudMutualExclusiveFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "xcode-cloud run workflow and workflow-id are mutually exclusive",
			args:    []string{"xcode-cloud", "run", "--workflow", "CI", "--workflow-id", "WF_ID", "--branch", "main"},
			wantErr: "--workflow and --workflow-id are mutually exclusive",
		},
		{
			name:    "xcode-cloud run branch and git-reference-id are mutually exclusive",
			args:    []string{"xcode-cloud", "run", "--workflow-id", "WF_ID", "--branch", "main", "--git-reference-id", "REF_ID"},
			wantErr: "--branch and --git-reference-id are mutually exclusive",
		},
		{
			name:    "xcode-cloud run invalid poll-interval",
			args:    []string{"xcode-cloud", "run", "--workflow-id", "WF_ID", "--branch", "main", "--wait", "--poll-interval", "0s"},
			wantErr: "--poll-interval must be greater than 0",
		},
		{
			name:    "xcode-cloud status invalid timeout",
			args:    []string{"xcode-cloud", "status", "--run-id", "RUN_ID", "--timeout", "-1s"},
			wantErr: "--timeout must be greater than or equal to 0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected non-help error, got %v", err)
				}
			})

			// These errors come from Exec, not from validation that returns ErrHelp
			_ = stdout
			_ = stderr
		})
	}
}

func TestVersionsPromotionsCreateRequiresVersionID(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"versions", "promotions", "create"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --version-id is required") {
		t.Fatalf("expected missing version error, got %q", stderr)
	}
}

func TestVersionsPromotionsCreateRequiresTreatmentID(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"versions", "promotions", "create", "--version-id", "VERSION_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --treatment-id is required") {
		t.Fatalf("expected missing treatment error, got %q", stderr)
	}
}

func TestAppEventsListRequiresAppID(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"app-events", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --app is required") {
		t.Fatalf("expected missing app error, got %q", stderr)
	}
}

func TestAppEventsCreateValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"app-events", "create", "--name", "Launch", "--event-type", "CHALLENGE", "--start", "2026-01-01T00:00:00Z", "--end", "2026-01-02T00:00:00Z"},
			wantErr: "Error: --app is required",
		},
		{
			name:    "missing name",
			args:    []string{"app-events", "create", "--app", "APP_ID", "--event-type", "CHALLENGE", "--start", "2026-01-01T00:00:00Z", "--end", "2026-01-02T00:00:00Z"},
			wantErr: "Error: --name is required",
		},
		{
			name:    "missing event type",
			args:    []string{"app-events", "create", "--app", "APP_ID", "--name", "Launch", "--start", "2026-01-01T00:00:00Z", "--end", "2026-01-02T00:00:00Z"},
			wantErr: "Error: --event-type is required",
		},
		{
			name:    "missing end time",
			args:    []string{"app-events", "create", "--app", "APP_ID", "--name", "Launch", "--event-type", "CHALLENGE", "--start", "2026-01-01T00:00:00Z"},
			wantErr: "Error: --end is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAppEventsUpdateValidationErrors(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing event-id",
			args:    []string{"app-events", "update", "--name", "Launch"},
			wantErr: "Error: --event-id is required",
		},
		{
			name:    "missing update fields",
			args:    []string{"app-events", "update", "--event-id", "EVENT_ID"},
			wantErr: "Error: at least one update flag is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAppEventsDeleteRequiresConfirm(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"app-events", "delete", "--event-id", "EVENT_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --confirm is required") {
		t.Fatalf("expected confirm error, got %q", stderr)
	}
}

func TestAppEventLocalizationsValidationErrors(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "list missing event-id",
			args:    []string{"app-events", "localizations", "list"},
			wantErr: "Error: --event-id is required",
		},
		{
			name:    "create missing locale",
			args:    []string{"app-events", "localizations", "create", "--event-id", "EVENT_ID"},
			wantErr: "Error: --locale is required",
		},
		{
			name:    "update missing localization-id",
			args:    []string{"app-events", "localizations", "update", "--name", "New Name"},
			wantErr: "Error: --localization-id is required",
		},
		{
			name:    "update missing fields",
			args:    []string{"app-events", "localizations", "update", "--localization-id", "LOC_ID"},
			wantErr: "Error: at least one update flag is required",
		},
		{
			name:    "delete missing confirm",
			args:    []string{"app-events", "localizations", "delete", "--localization-id", "LOC_ID"},
			wantErr: "Error: --confirm is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAppEventScreenshotsValidationErrors(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "list missing localization",
			args:    []string{"app-events", "screenshots", "list"},
			wantErr: "Error: --event-id or --localization-id is required",
		},
		{
			name:    "create missing path",
			args:    []string{"app-events", "screenshots", "create", "--event-id", "EVENT_ID", "--asset-type", "EVENT_CARD"},
			wantErr: "Error: --path is required",
		},
		{
			name:    "create missing localization",
			args:    []string{"app-events", "screenshots", "create", "--path", "./event.png", "--asset-type", "EVENT_CARD"},
			wantErr: "Error: --event-id or --localization-id is required",
		},
		{
			name:    "delete missing confirm",
			args:    []string{"app-events", "screenshots", "delete", "--screenshot-id", "SHOT_ID"},
			wantErr: "Error: --confirm is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAppEventVideoClipsValidationErrors(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "list missing localization",
			args:    []string{"app-events", "video-clips", "list"},
			wantErr: "Error: --event-id or --localization-id is required",
		},
		{
			name:    "create missing path",
			args:    []string{"app-events", "video-clips", "create", "--event-id", "EVENT_ID", "--asset-type", "EVENT_CARD"},
			wantErr: "Error: --path is required",
		},
		{
			name:    "create missing localization",
			args:    []string{"app-events", "video-clips", "create", "--path", "./clip.mov", "--asset-type", "EVENT_CARD"},
			wantErr: "Error: --event-id or --localization-id is required",
		},
		{
			name:    "delete missing confirm",
			args:    []string{"app-events", "video-clips", "delete", "--clip-id", "CLIP_ID"},
			wantErr: "Error: --confirm is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestAppEventsSubmitValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing confirm",
			args:    []string{"app-events", "submit", "--event-id", "EVENT_ID", "--app", "APP_ID"},
			wantErr: "Error: --confirm is required",
		},
		{
			name:    "missing event-id",
			args:    []string{"app-events", "submit", "--app", "APP_ID", "--confirm"},
			wantErr: "Error: --event-id is required",
		},
		{
			name:    "missing app",
			args:    []string{"app-events", "submit", "--event-id", "EVENT_ID", "--confirm"},
			wantErr: "Error: --app is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

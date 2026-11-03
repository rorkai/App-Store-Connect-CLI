package auth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	authsvc "github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/errfmt"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

type authRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn authRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestCommandWrapperReturnsAuthCommand(t *testing.T) {
	cmd := AuthCommand()
	if cmd == nil {
		t.Fatal("AuthCommand() returned nil")
		return
	}
	if cmd.Name != "auth" {
		t.Fatalf("AuthCommand().Name = %q, want %q", cmd.Name, "auth")
	}
}

func TestAuthHelpHighlightsStatusDiscoverability(t *testing.T) {
	cmd := AuthCommand()

	for _, expected := range []string{
		`Use "asc auth status" to see which credentials/profile are currently active.`,
		"asc auth status --verbose",
		"asc auth switch --name work",
	} {
		if !strings.Contains(cmd.LongHelp, expected) {
			t.Fatalf("expected AuthCommand().LongHelp to contain %q, got %q", expected, cmd.LongHelp)
		}
	}

	statusCmd := AuthStatusCommand()
	if !strings.Contains(statusCmd.ShortHelp, "active profile") {
		t.Fatalf("expected AuthStatusCommand().ShortHelp to mention active profile, got %q", statusCmd.ShortHelp)
	}
}

func TestAuthHelpExplainsCredentialResolutionBranches(t *testing.T) {
	longHelp := AuthCommand().LongHelp
	for _, expected := range []string{
		"--profile or ASC_PROFILE selects a stored profile and disables the env-only fast path.",
		"With no profile and keychain bypass disabled, a complete environment set skips stored lookup.",
		"ASC_BYPASS_KEYCHAIN skips keychain; env fallback follows only missing/default-selection config errors.",
		"Config selection: ASC_CONFIG_PATH; otherwise nearest ancestor .asc/config.json; otherwise ~/.asc/config.json.",
	} {
		if !strings.Contains(longHelp, expected) {
			t.Fatalf("expected AuthCommand().LongHelp to contain %q, got %q", expected, longHelp)
		}
	}
}

func TestAuthCommandUnknownSubcommand(t *testing.T) {
	cmd := AuthCommand()
	_, stderr := captureAuthOutput(t, func() {
		err := cmd.Exec(context.Background(), []string{"unknown"})
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("Exec() error = %v, want %v", err, flag.ErrHelp)
		}
	})
	if !strings.Contains(stderr, "Unknown subcommand: unknown") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestAuthInitCommandLocalLifecycle(t *testing.T) {
	withTempRepo(t, func(repo string) {
		cfgPath := filepath.Join(repo, ".asc", "config.json")

		cmd := AuthInitCommand()
		if err := cmd.FlagSet.Parse([]string{"--local"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		if err := cmd.Exec(context.Background(), []string{}); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
		if _, err := os.Stat(cfgPath); err != nil {
			t.Fatalf("expected config at %s: %v", cfgPath, err)
		}

		cmdAgain := AuthInitCommand()
		if err := cmdAgain.FlagSet.Parse([]string{"--local"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmdAgain.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "config already exists") {
			t.Fatalf("expected existing-config error, got %v", err)
		}

		cmdForce := AuthInitCommand()
		if err := cmdForce.FlagSet.Parse([]string{"--local", "--force"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		if err := cmdForce.Exec(context.Background(), []string{}); err != nil {
			t.Fatalf("Exec(force) error: %v", err)
		}
	})
}

func TestAuthDoctorCommandFlagValidation(t *testing.T) {
	t.Run("unsupported output", func(t *testing.T) {
		cmd := AuthDoctorCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "yaml"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "--output must be one of") {
			t.Fatalf("expected unsupported format error in stderr, got %q", stderr)
		}
	})

	t.Run("pretty requires json output", func(t *testing.T) {
		cmd := AuthDoctorCommand()
		if err := cmd.FlagSet.Parse([]string{"--pretty"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "--pretty is only valid with JSON output") {
			t.Fatalf("expected pretty/json error in stderr, got %q", stderr)
		}
	})

	t.Run("fix requires confirm", func(t *testing.T) {
		cmd := AuthDoctorCommand()
		if err := cmd.FlagSet.Parse([]string{"--fix"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "--fix requires --confirm") {
			t.Fatalf("expected fix/confirm error in stderr, got %q", stderr)
		}
	})
}

func TestDoctorHelpers(t *testing.T) {
	if got := doctorStatusLabel(authsvc.DoctorOK); got != "OK" {
		t.Fatalf("doctorStatusLabel(DoctorOK) = %q, want OK", got)
	}
	if got := doctorStatusLabel(authsvc.DoctorWarn); got != "WARN" {
		t.Fatalf("doctorStatusLabel(DoctorWarn) = %q, want WARN", got)
	}
	if got := doctorStatusLabel(authsvc.DoctorFail); got != "FAIL" {
		t.Fatalf("doctorStatusLabel(DoctorFail) = %q, want FAIL", got)
	}
	if got := doctorStatusLabel(authsvc.DoctorInfo); got != "INFO" {
		t.Fatalf("doctorStatusLabel(DoctorInfo) = %q, want INFO", got)
	}

	report := authsvc.DoctorReport{
		Sections: []authsvc.DoctorSection{
			{
				Title: "Storage\nforged section",
				Checks: []authsvc.DoctorCheck{
					{Status: authsvc.DoctorOK, Message: "all\x1b[31m good"},
				},
			},
		},
		Recommendations: []string{"fix\u2028forged row"},
		Summary:         authsvc.DoctorSummary{},
	}
	stdout, _ := captureAuthOutput(t, func() {
		printDoctorReport(report)
	})
	if !strings.Contains(stdout, "Auth Doctor") || !strings.Contains(stdout, "[OK] all[31m good") {
		t.Fatalf("unexpected doctor output: %q", stdout)
	}
	if strings.Contains(stdout, "\nforged section") || strings.Contains(stdout, "\x1b") || strings.ContainsRune(stdout, '\u2028') {
		t.Fatalf("doctor text output contains unsanitized terminal content: %q", stdout)
	}
}

func TestPermissionWarningAndHooks(t *testing.T) {
	baseErr := errors.New("permission")
	pw := NewPermissionWarning(baseErr)
	if _, ok := errors.AsType[*permissionWarning](pw); !ok {
		t.Fatal("expected NewPermissionWarning() to return permissionWarning")
	}
	if got := pw.Error(); !strings.Contains(got, "permission") {
		t.Fatalf("permission warning Error() = %q", got)
	}
	if !errors.Is(pw, baseErr) {
		t.Fatal("expected wrapped base error")
	}

	restoreStatus := SetStatusValidateCredential(func(context.Context, authsvc.Credential) error { return nil })
	restoreStatus()

	restoreJWT := SetLoginJWTGenerator(func(string, string, *ecdsa.PrivateKey) (string, error) {
		return "token", nil
	})
	restoreJWT()
}

func TestValidateLoginNetwork_InvalidKeyPath(t *testing.T) {
	err := validateLoginNetwork(context.Background(), "KEY", "ISS", "/definitely/missing/AuthKey.p8", "team")
	if err == nil {
		t.Fatal("expected validateLoginNetwork() to fail for invalid key path")
	}
}

func TestValidateStoredCredential_InvalidKeyPath(t *testing.T) {
	err := validateStoredCredential(context.Background(), authsvc.Credential{
		Name:           "bad",
		KeyID:          "KEY",
		IssuerID:       "ISS",
		PrivateKeyPath: "/definitely/missing/AuthKey.p8",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid private key") {
		t.Fatalf("expected invalid private key error, got %v", err)
	}
}

func TestValidateStoredCredential_UsesPEMWhenPathMissing(t *testing.T) {
	keyPath := writeTempECDSAKeyFile(t)
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = validateStoredCredential(ctx, authsvc.Credential{
		Name:           "pem",
		KeyID:          "KEY",
		IssuerID:       "ISS",
		PrivateKeyPath: keyPath,
		PrivateKeyPEM:  string(keyData),
	})
	if err == nil {
		t.Fatal("expected non-nil error due canceled context")
	}
	if strings.Contains(err.Error(), "failed to open key file") || strings.Contains(err.Error(), "invalid private key") {
		t.Fatalf("expected path-independent validation failure, got %v", err)
	}
}

func TestValidateStoredCredential_ClassifiesUnauthorizedNetworkFailure(t *testing.T) {
	keyPath := writeTempECDSAKeyFile(t)
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	previousTransport := http.DefaultTransport
	http.DefaultTransport = authRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Status:     "401 Unauthorized",
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"errors":[{"status":"401","code":"NOT_AUTHORIZED","title":"Unauthorized"}]
			}`)),
			Request: req,
		}, nil
	})
	t.Cleanup(func() {
		http.DefaultTransport = previousTransport
	})

	err = validateStoredCredential(context.Background(), authsvc.Credential{
		Name:          "unauthorized",
		KeyID:         "KEY",
		IssuerID:      "ISS",
		PrivateKeyPEM: string(keyData),
	})
	if err == nil || !errors.Is(err, asc.ErrUnauthorized) {
		t.Fatalf("expected unauthorized validation error, got %v", err)
	}
	assertAuthDiagnostic(t, err, shared.DiagnosticAuthenticationRejected, "")
}

func TestCredentialSigningIssuerIDClearsIndividualIssuer(t *testing.T) {
	team := authsvc.Credential{
		KeyID:    "KEY",
		IssuerID: "ISS",
	}
	if got := credentialSigningIssuerID(team); got != "ISS" {
		t.Fatalf("team signing issuer = %q, want ISS", got)
	}

	individual := authsvc.Credential{
		KeyID:    "KEY",
		IssuerID: "STRAYISS",
		KeyType:  config.CredentialKeyTypeIndividual,
	}
	if got := credentialSigningIssuerID(individual); got != "" {
		t.Fatalf("individual signing issuer = %q, want empty", got)
	}
}

func TestValidateLoginCredentials(t *testing.T) {
	keyPath := writeTempECDSAKeyFile(t)

	t.Run("jwt generation failure", func(t *testing.T) {
		restoreJWT := SetLoginJWTGenerator(func(string, string, *ecdsa.PrivateKey) (string, error) {
			return "", errors.New("jwt failed")
		})
		prevNetwork := loginNetworkValidate
		loginNetworkValidate = func(context.Context, string, string, string, string) error {
			t.Fatal("network validation should not run when jwt generation fails")
			return nil
		}
		t.Cleanup(func() {
			restoreJWT()
			loginNetworkValidate = prevNetwork
		})

		err := validateLoginCredentials(context.Background(), "KEY", "ISS", keyPath, "team", true)
		if err == nil || !strings.Contains(err.Error(), "failed to generate JWT") {
			t.Fatalf("expected jwt error, got %v", err)
		}
		assertAuthDiagnostic(t, err, shared.DiagnosticInternalError, "--private-key")
	})

	t.Run("network disabled succeeds", func(t *testing.T) {
		restoreJWT := SetLoginJWTGenerator(func(string, string, *ecdsa.PrivateKey) (string, error) {
			return "token", nil
		})
		prevNetwork := loginNetworkValidate
		loginNetworkValidate = func(context.Context, string, string, string, string) error {
			t.Fatal("network validation should not run when network=false")
			return nil
		}
		t.Cleanup(func() {
			restoreJWT()
			loginNetworkValidate = prevNetwork
		})

		if err := validateLoginCredentials(context.Background(), "KEY", "ISS", keyPath, "team", false); err != nil {
			t.Fatalf("validateLoginCredentials() error: %v", err)
		}
	})

	t.Run("network validation error", func(t *testing.T) {
		restoreJWT := SetLoginJWTGenerator(func(string, string, *ecdsa.PrivateKey) (string, error) {
			return "token", nil
		})
		prevNetwork := loginNetworkValidate
		loginNetworkValidate = func(context.Context, string, string, string, string) error {
			return errors.New("network down")
		}
		t.Cleanup(func() {
			restoreJWT()
			loginNetworkValidate = prevNetwork
		})

		err := validateLoginCredentials(context.Background(), "KEY", "ISS", keyPath, "team", true)
		if err == nil || !strings.Contains(err.Error(), "network validation failed") {
			t.Fatalf("expected network validation error, got %v", err)
		}
		assertAuthDiagnostic(t, err, shared.DiagnosticRequestFailed, "")
	})

	t.Run("network authentication rejected", func(t *testing.T) {
		restoreJWT := SetLoginJWTGenerator(func(string, string, *ecdsa.PrivateKey) (string, error) {
			return "token", nil
		})
		prevNetwork := loginNetworkValidate
		loginNetworkValidate = func(context.Context, string, string, string, string) error {
			return asc.ErrUnauthorized
		}
		t.Cleanup(func() {
			restoreJWT()
			loginNetworkValidate = prevNetwork
		})

		err := validateLoginCredentials(context.Background(), "KEY", "ISS", keyPath, "team", true)
		if err == nil || !strings.Contains(err.Error(), "network validation failed") {
			t.Fatalf("expected network validation error, got %v", err)
		}
		assertAuthDiagnostic(t, err, shared.DiagnosticAuthenticationRejected, "")
	})

	t.Run("network authorization denied", func(t *testing.T) {
		restoreJWT := SetLoginJWTGenerator(func(string, string, *ecdsa.PrivateKey) (string, error) {
			return "token", nil
		})
		prevNetwork := loginNetworkValidate
		loginNetworkValidate = func(context.Context, string, string, string, string) error {
			return asc.ErrForbidden
		}
		t.Cleanup(func() {
			restoreJWT()
			loginNetworkValidate = prevNetwork
		})

		err := validateLoginCredentials(context.Background(), "KEY", "ISS", keyPath, "team", true)
		if err == nil || !strings.Contains(err.Error(), "network validation failed") {
			t.Fatalf("expected network validation error, got %v", err)
		}
		assertAuthDiagnostic(t, err, shared.DiagnosticRequestFailed, "")
	})
}

func TestLoginStorageMessage_BypassModes(t *testing.T) {
	withTempRepo(t, func(repo string) {
		msg, err := loginStorageMessage(true, true)
		if err != nil {
			t.Fatalf("loginStorageMessage(local) error: %v", err)
		}
		expectedLocal := filepath.Join(repo, ".asc", "config.json")
		if !strings.Contains(msg, expectedLocal) {
			t.Fatalf("expected local config path in message, got %q", msg)
		}
	})

	msg, err := loginStorageMessage(true, false)
	if err != nil {
		t.Fatalf("loginStorageMessage(global) error: %v", err)
	}
	if !strings.Contains(msg, "Storing credentials in config file at ") {
		t.Fatalf("unexpected global message: %q", msg)
	}
}

func TestAuthLoginCommand(t *testing.T) {
	t.Run("local requires bypass", func(t *testing.T) {
		// Capture exact original state, including empty-but-present values.
		origValue, origPresent := os.LookupEnv("ASC_BYPASS_KEYCHAIN")
		os.Unsetenv("ASC_BYPASS_KEYCHAIN")
		t.Cleanup(func() {
			if origPresent {
				os.Setenv("ASC_BYPASS_KEYCHAIN", origValue)
			} else {
				os.Unsetenv("ASC_BYPASS_KEYCHAIN")
			}
		})
		cmd := AuthLoginCommand()
		if err := cmd.FlagSet.Parse([]string{"--local"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
			assertAuthDiagnostic(t, err, shared.DiagnosticInvalidInput, "--local")
		})
		if !strings.Contains(stderr, "--local requires --bypass-keychain") {
			t.Fatalf("expected local/bypass error in stderr, got %q", stderr)
		}
	})

	t.Run("missing name with stored profiles", func(t *testing.T) {
		t.Setenv("ASC_BYPASS_KEYCHAIN", "0")
		restore := SetListCredentialSummaries(func() ([]authsvc.Credential, error) {
			return []authsvc.Credential{{Name: "existing"}}, nil
		})
		t.Cleanup(restore)
		cmd := AuthLoginCommand()
		if err := cmd.FlagSet.Parse([]string{"--key-id", "KEY", "--issuer-id", "ISS", "--private-key", "/tmp/AuthKey.p8"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
		assertAuthDiagnostic(t, err, shared.DiagnosticRequiredInputMissing, "--name")
	})

	t.Run("whitespace key id", func(t *testing.T) {
		cmd := AuthLoginCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--name", "demo",
			"--key-id", "   ",
			"--issuer-id", "ISS",
			"--private-key", "/tmp/AuthKey.p8",
		}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
			assertAuthDiagnostic(t, err, shared.DiagnosticRequiredInputMissing, "--key-id")
		})
		if !strings.Contains(stderr, "--key-id is required") {
			t.Fatalf("expected key ID error in stderr, got %q", stderr)
		}
		if strings.Contains(stderr, "key file name suggests") {
			t.Fatalf("expected no key ID hint for a generic key file name, got %q", stderr)
		}
		if !strings.Contains(stderr, "Hint: create an API key at "+authsvc.APIKeysURL+", then run: "+authsvc.LoginCommandExample+"\n") {
			t.Fatalf("expected login form hint, got %q", stderr)
		}
	})

	t.Run("missing key id hints from key file name", func(t *testing.T) {
		cmd := AuthLoginCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--name", "demo",
			"--issuer-id", "ISS",
			"--private-key", "/tmp/keys/AuthKey_39MX87M9Y4.p8",
		}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
			assertAuthDiagnostic(t, err, shared.DiagnosticRequiredInputMissing, "--key-id")
		})
		if !strings.Contains(stderr, "Error: --key-id is required\nHint: the key file name suggests --key-id 39MX87M9Y4\n") {
			t.Fatalf("expected key ID hint in stderr, got %q", stderr)
		}
	})

	for _, test := range []struct {
		name      string
		args      []string
		parameter string
	}{
		{
			name:      "whitespace name",
			args:      []string{"--name", "   ", "--key-id", "KEY", "--issuer-id", "ISS", "--private-key", "/tmp/AuthKey.p8"},
			parameter: "--name",
		},
		{
			name:      "whitespace issuer id",
			args:      []string{"--name", "demo", "--key-id", "KEY", "--issuer-id", "   ", "--private-key", "/tmp/AuthKey.p8"},
			parameter: "--issuer-id",
		},
		{
			name:      "whitespace private key",
			args:      []string{"--name", "demo", "--key-id", "KEY", "--issuer-id", "ISS", "--private-key", "   "},
			parameter: "--private-key",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := AuthLoginCommand()
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			_, stderr := captureAuthOutput(t, func() {
				err := cmd.Exec(context.Background(), []string{})
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
				assertAuthDiagnostic(t, err, shared.DiagnosticRequiredInputMissing, test.parameter)
			})
			if !strings.Contains(stderr, test.parameter+" is required") {
				t.Fatalf("expected %s error in stderr, got %q", test.parameter, stderr)
			}
		})
	}

	t.Run("skip validation mutually exclusive with network", func(t *testing.T) {
		cmd := AuthLoginCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--name", "demo",
			"--key-id", "KEY",
			"--issuer-id", "ISS",
			"--private-key", "/tmp/AuthKey.p8",
			"--skip-validation",
			"--network",
		}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
			assertAuthDiagnostic(t, err, shared.DiagnosticConflictingInput, "--skip-validation")
		})
		if !strings.Contains(stderr, "mutually exclusive") {
			t.Fatalf("expected mutual exclusion error in stderr, got %q", stderr)
		}
	})

	t.Run("invalid private key path", func(t *testing.T) {
		withTempRepo(t, func(string) {
			cmd := AuthLoginCommand()
			if err := cmd.FlagSet.Parse([]string{
				"--name", "demo",
				"--key-id", "KEY",
				"--issuer-id", "ISS",
				"--private-key", "/definitely/missing/AuthKey.p8",
				"--bypass-keychain",
				"--local",
			}); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			err := cmd.Exec(context.Background(), []string{})
			if err == nil || !strings.Contains(err.Error(), "invalid private key") {
				t.Fatalf("expected invalid key error, got %v", err)
			}
			assertAuthDiagnostic(t, err, shared.DiagnosticFileNotFound, "--private-key")
		})
	})

	t.Run("insecure private key permissions", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not expose POSIX key permissions")
		}
		withTempRepo(t, func(string) {
			keyPath := writeTempECDSAKeyFile(t)
			if err := os.Chmod(keyPath, 0o644); err != nil {
				t.Fatalf("set key permissions: %v", err)
			}
			cmd := AuthLoginCommand()
			if err := cmd.FlagSet.Parse([]string{
				"--name", "demo",
				"--key-id", "KEY",
				"--issuer-id", "ISS",
				"--private-key", keyPath,
			}); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			err := cmd.Exec(context.Background(), []string{})
			if err == nil || !strings.Contains(err.Error(), "private key file is too permissive") {
				t.Fatalf("expected insecure permissions error, got %v", err)
			}
			assertAuthDiagnostic(t, err, shared.DiagnosticFilePermissionsInsecure, "--private-key")
		})
	})

	t.Run("insecure private key permissions print exact remediation", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not expose POSIX key permissions")
		}
		withTempRepo(t, func(string) {
			keyPath := writeTempECDSAKeyFile(t)
			if err := os.Chmod(keyPath, 0o644); err != nil {
				t.Fatalf("set key permissions: %v", err)
			}
			cmd := AuthLoginCommand()
			if err := cmd.FlagSet.Parse([]string{
				"--name", "demo",
				"--key-id", "KEY",
				"--issuer-id", "ISS",
				"--private-key", keyPath,
			}); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			var execErr error
			_, stderr := captureAuthOutput(t, func() {
				execErr = cmd.Exec(context.Background(), []string{})
			})
			if execErr == nil || !strings.Contains(execErr.Error(), "private key file is too permissive") {
				t.Fatalf("expected insecure permissions error, got %v", execErr)
			}
			assertAuthDiagnostic(t, execErr, shared.DiagnosticFilePermissionsInsecure, "--private-key")
			wantCommand, safe := authsvc.FilePermissionRemediationCommand(keyPath)
			if !safe || !strings.HasPrefix(wantCommand, "chmod 600 ") || !strings.Contains(wantCommand, keyPath) {
				t.Fatalf("remediation command = %q, safe=%t; want a chmod 600 command naming the key", wantCommand, safe)
			}
			if !strings.Contains(stderr, wantCommand) {
				t.Fatalf("stderr = %q, want remediation %q", stderr, wantCommand)
			}
			if !strings.Contains(stderr, "--fix-permissions") {
				t.Fatalf("stderr = %q, want --fix-permissions hint", stderr)
			}
			info, err := os.Stat(keyPath)
			if err != nil {
				t.Fatalf("Stat() error: %v", err)
			}
			if info.Mode().Perm() != 0o644 {
				t.Fatalf("permissions = %#o, want 0644 unchanged without --fix-permissions", info.Mode().Perm())
			}
		})
	})

	t.Run("unsafe private key path omits executable remediation", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not expose POSIX permission bits")
		}
		withTempRepo(t, func(string) {
			keyPath := writeTempECDSAKeyFile(t)
			unsafePath := filepath.Join(filepath.Dir(keyPath), "unsafe\nkey.p8")
			if err := os.Rename(keyPath, unsafePath); err != nil {
				t.Fatalf("rename key: %v", err)
			}
			if err := os.Chmod(unsafePath, 0o644); err != nil {
				t.Fatalf("set key permissions: %v", err)
			}
			cmd := AuthLoginCommand()
			if err := cmd.FlagSet.Parse([]string{
				"--name", "demo",
				"--key-id", "KEY",
				"--issuer-id", "ISS",
				"--private-key", unsafePath,
			}); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			var execErr error
			_, stderr := captureAuthOutput(t, func() {
				execErr = cmd.Exec(context.Background(), []string{})
			})
			if execErr == nil || !strings.Contains(execErr.Error(), "private key file is too permissive") {
				t.Fatalf("expected insecure permissions error, got %v", execErr)
			}
			if strings.Contains(stderr, "chmod 600") || !strings.Contains(stderr, "--fix-permissions") {
				t.Fatalf("stderr = %q, want only the safe --fix-permissions remediation", stderr)
			}
		})
	})

	t.Run("fix failure sanitizes unsafe private key path", func(t *testing.T) {
		withTempRepo(t, func(string) {
			unsafePath := filepath.Join(t.TempDir(), "missing\n\x1b[31mkey.p8")
			cmd := AuthLoginCommand()
			if err := cmd.FlagSet.Parse([]string{
				"--name", "demo",
				"--key-id", "KEY",
				"--issuer-id", "ISS",
				"--private-key", unsafePath,
				"--fix-permissions",
			}); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			execErr := cmd.Exec(context.Background(), []string{})
			if execErr == nil {
				t.Fatal("expected missing private key repair to fail")
			}
			formatted := errfmt.FormatStderr(execErr)
			if strings.ContainsAny(formatted, "\r\x1b") || strings.Count(formatted, "\n") != 1 {
				t.Fatalf("formatted error contains terminal control or forged lines: %q", formatted)
			}
		})
	})

	t.Run("fix-permissions repairs insecure key and completes login", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not expose POSIX key permissions")
		}
		withTempRepo(t, func(repo string) {
			keyPath := writeTempECDSAKeyFile(t)
			if err := os.Chmod(keyPath, 0o644); err != nil {
				t.Fatalf("set key permissions: %v", err)
			}
			cmd := AuthLoginCommand()
			if err := cmd.FlagSet.Parse([]string{
				"--name", "demo",
				"--key-id", "KEY",
				"--issuer-id", "ISS",
				"--private-key", keyPath,
				"--bypass-keychain",
				"--local",
				"--fix-permissions",
			}); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			var execErr error
			stdout, stderr := captureAuthOutput(t, func() {
				execErr = cmd.Exec(context.Background(), []string{})
			})
			if execErr != nil {
				t.Fatalf("Exec() error: %v (stderr=%q)", execErr, stderr)
			}
			info, err := os.Stat(keyPath)
			if err != nil {
				t.Fatalf("Stat() error: %v", err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatalf("permissions = %#o, want 0600 after repair", info.Mode().Perm())
			}
			if !strings.Contains(stderr, "0600") || !strings.Contains(stderr, keyPath) {
				t.Fatalf("stderr = %q, want repair notice naming the key file", stderr)
			}
			if !strings.Contains(stdout, "Successfully registered API key 'demo'") {
				t.Fatalf("stdout = %q, want successful login", stdout)
			}
			if _, err := os.Stat(filepath.Join(repo, ".asc", "config.json")); err != nil {
				t.Fatalf("expected local config written: %v", err)
			}
		})
	})

	t.Run("fix-permissions leaves an owner-only key silent", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not expose POSIX key permissions")
		}
		withTempRepo(t, func(string) {
			keyPath := writeTempECDSAKeyFile(t)
			cmd := AuthLoginCommand()
			if err := cmd.FlagSet.Parse([]string{
				"--name", "demo",
				"--key-id", "KEY",
				"--issuer-id", "ISS",
				"--private-key", keyPath,
				"--bypass-keychain",
				"--local",
				"--fix-permissions",
			}); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			var execErr error
			_, stderr := captureAuthOutput(t, func() {
				execErr = cmd.Exec(context.Background(), []string{})
			})
			if execErr != nil {
				t.Fatalf("Exec() error: %v (stderr=%q)", execErr, stderr)
			}
			if strings.Contains(stderr, "0600") {
				t.Fatalf("stderr = %q, want no repair notice for an already secure key", stderr)
			}
		})
	})

	t.Run("fix-permissions rejects an unsupported key identity", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not expose POSIX key permissions")
		}
		withTempRepo(t, func(string) {
			keyPath := writeTempECDSAKeyFile(t)
			if err := os.Chmod(keyPath, 0o644); err != nil {
				t.Fatalf("set key permissions: %v", err)
			}
			link := filepath.Join(filepath.Dir(keyPath), "link.p8")
			if err := os.Symlink(keyPath, link); err != nil {
				t.Fatalf("Symlink() error: %v", err)
			}
			cmd := AuthLoginCommand()
			if err := cmd.FlagSet.Parse([]string{
				"--name", "demo",
				"--key-id", "KEY",
				"--issuer-id", "ISS",
				"--private-key", link,
				"--fix-permissions",
			}); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			err := cmd.Exec(context.Background(), []string{})
			if err == nil || !strings.Contains(err.Error(), "failed to fix private key permissions") {
				t.Fatalf("expected repair failure, got %v", err)
			}
			if errors.Is(err, flag.ErrHelp) {
				t.Fatalf("repair failure must not be a usage error: %v", err)
			}
			info, statErr := os.Stat(keyPath)
			if statErr != nil {
				t.Fatalf("Stat() error: %v", statErr)
			}
			if info.Mode().Perm() != 0o644 {
				t.Fatalf("permissions = %#o, want 0644 untouched", info.Mode().Perm())
			}
		})
	})

	t.Run("successful local bypass login", func(t *testing.T) {
		withTempRepo(t, func(repo string) {
			keyPath := writeTempECDSAKeyFile(t)
			cmd := AuthLoginCommand()
			if err := cmd.FlagSet.Parse([]string{
				"--name", "demo",
				"--key-id", "KEY",
				"--issuer-id", "ISS",
				"--private-key", keyPath,
				"--bypass-keychain",
				"--local",
				"--skip-validation",
			}); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}

			cfgPath := filepath.Join(repo, ".asc", "config.json")
			cfg, err := config.LoadAt(cfgPath)
			if err != nil {
				t.Fatalf("LoadAt() error: %v", err)
			}
			if cfg.DefaultKeyName != "demo" {
				t.Fatalf("DefaultKeyName = %q, want demo", cfg.DefaultKeyName)
			}
		})
	})

	t.Run("success message echoes normalized name", func(t *testing.T) {
		withTempRepo(t, func(repo string) {
			keyPath := writeTempECDSAKeyFile(t)
			cmd := AuthLoginCommand()
			if err := cmd.FlagSet.Parse([]string{
				"--name", "  spaced  ",
				"--key-id", "KEY",
				"--issuer-id", "ISS",
				"--private-key", keyPath,
				"--bypass-keychain",
				"--local",
				"--skip-validation",
			}); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			stdout, _ := captureAuthOutput(t, func() {
				if err := cmd.Exec(context.Background(), []string{}); err != nil {
					t.Fatalf("Exec() error: %v", err)
				}
			})
			if !strings.Contains(stdout, "Successfully registered API key 'spaced'") {
				t.Fatalf("expected normalized profile name in success message, got %q", stdout)
			}
			if strings.Contains(stdout, "'  spaced  '") {
				t.Fatalf("success message echoed pre-normalized name: %q", stdout)
			}

			cfgPath := filepath.Join(repo, ".asc", "config.json")
			cfg, err := config.LoadAt(cfgPath)
			if err != nil {
				t.Fatalf("LoadAt() error: %v", err)
			}
			if cfg.DefaultKeyName != "spaced" {
				t.Fatalf("DefaultKeyName = %q, want spaced", cfg.DefaultKeyName)
			}
		})
	})
}

func assertAuthDiagnostic(t *testing.T, err error, code shared.DiagnosticCode, parameter string) {
	t.Helper()
	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok {
		t.Fatalf("DiagnosticFromError(%v) did not find metadata", err)
	}
	if diagnostic.Code != code || diagnostic.Parameter != parameter {
		t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, code, parameter)
	}
}

func TestAuthSwitchCommand(t *testing.T) {
	t.Run("missing name", func(t *testing.T) {
		cmd := AuthSwitchCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})

	t.Run("no credentials", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)

		cmd := AuthSwitchCommand()
		if err := cmd.FlagSet.Parse([]string{"--name", "demo"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "no credentials stored") {
			t.Fatalf("expected no credentials error, got %v", err)
		}
	})

	t.Run("profile not found", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		if err := authsvc.StoreCredentialsConfigAt("existing", "KEY", "ISS", "/tmp/AuthKey.p8", cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthSwitchCommand()
		if err := cmd.FlagSet.Parse([]string{"--name", "missing"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected profile not found error, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		if err := authsvc.StoreCredentialsConfigAt("demo", "KEY", "ISS", "/tmp/AuthKey.p8", cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthSwitchCommand()
		if err := cmd.FlagSet.Parse([]string{"--name", "demo"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		if err := cmd.Exec(context.Background(), []string{}); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}

		cfg, err := config.LoadAt(cfgPath)
		if err != nil {
			t.Fatalf("LoadAt() error: %v", err)
		}
		if cfg.DefaultKeyName != "demo" {
			t.Fatalf("DefaultKeyName = %q, want demo", cfg.DefaultKeyName)
		}
	})

	t.Run("preserves legacy fallback fields for summary-only profile", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)

		if err := config.SaveAt(cfgPath, &config.Config{
			DefaultKeyName: "personal",
			KeyID:          "KEY1",
			IssuerID:       "ISSUER1",
			PrivateKeyPath: "/tmp/personal.p8",
		}); err != nil {
			t.Fatalf("SaveAt() error: %v", err)
		}

		restoreSummary := SetListCredentialSummaries(func() ([]authsvc.Credential, error) {
			return []authsvc.Credential{{
				Name:      "other",
				KeyID:     "KEY2",
				IsDefault: false,
				Source:    "keychain",
			}}, nil
		})
		t.Cleanup(restoreSummary)

		cmd := AuthSwitchCommand()
		if err := cmd.FlagSet.Parse([]string{"--name", "other"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		if err := cmd.Exec(context.Background(), []string{}); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}

		cfg, err := config.LoadAt(cfgPath)
		if err != nil {
			t.Fatalf("LoadAt() error: %v", err)
		}
		if cfg.DefaultKeyName != "other" {
			t.Fatalf("DefaultKeyName = %q, want other", cfg.DefaultKeyName)
		}
		if cfg.KeyID != "" {
			t.Fatalf("KeyID = %q, want empty top-level field for keychain default", cfg.KeyID)
		}
		want := config.Credential{Name: "personal", KeyID: "KEY1", IssuerID: "ISSUER1", PrivateKeyPath: "/tmp/personal.p8"}
		if len(cfg.Keys) != 1 || cfg.Keys[0] != want {
			t.Fatalf("Keys = %+v, want legacy credential preserved as %+v", cfg.Keys, want)
		}
	})
}

type authLogoutTestCalls struct {
	names   []string
	all     int
	options []authsvc.RemoveOptions
}

func stubLogoutRemovers(t *testing.T) *authLogoutTestCalls {
	t.Helper()
	// The retained-global-config warning reads the global config, so keep it
	// away from the developer's home directory.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	calls := &authLogoutTestCalls{}
	restore := SetLogoutCredentialRemovers(
		func(name string, opts authsvc.RemoveOptions) error {
			calls.names = append(calls.names, name)
			calls.options = append(calls.options, opts)
			return nil
		},
		func(opts authsvc.RemoveOptions) error {
			calls.all++
			calls.options = append(calls.options, opts)
			return nil
		},
	)
	t.Cleanup(restore)
	return calls
}

func TestAuthLogoutCommand(t *testing.T) {
	t.Run("help requires confirmation", func(t *testing.T) {
		cmd := AuthLogoutCommand()
		confirmFlag := cmd.FlagSet.Lookup("confirm")
		if confirmFlag == nil {
			t.Fatal("expected --confirm flag")
		}
		if confirmFlag.Usage != "Confirm credential removal (required)" {
			t.Fatalf("--confirm usage = %q", confirmFlag.Usage)
		}
		if cmd.ShortUsage != "asc auth logout [--name NAME | --all] --confirm" {
			t.Fatalf("ShortUsage = %q", cmd.ShortUsage)
		}
		for _, expected := range []string{
			`asc auth logout --all --confirm`,
			`asc auth logout --name "MyKey" --confirm`,
			"--confirm is required",
		} {
			if !strings.Contains(cmd.LongHelp, expected) {
				t.Fatalf("expected logout help to contain %q, got %q", expected, cmd.LongHelp)
			}
		}
		for _, unexpected := range []string{"5.0.0", "deprecated", "compatibility"} {
			if strings.Contains(cmd.LongHelp, unexpected) {
				t.Fatalf("logout help still mentions %q: %q", unexpected, cmd.LongHelp)
			}
		}
	})

	t.Run("help documents config scope", func(t *testing.T) {
		cmd := AuthLogoutCommand()
		includeGlobal := cmd.FlagSet.Lookup("include-global")
		if includeGlobal == nil {
			t.Fatal("expected --include-global flag")
		}
		if includeGlobal.DefValue != "false" {
			t.Fatalf("--include-global default = %q, want false", includeGlobal.DefValue)
		}
		if !strings.Contains(includeGlobal.Usage, "~/.asc/config.json") || !strings.Contains(includeGlobal.Usage, "ASC_CONFIG_PATH") {
			t.Fatalf("--include-global usage = %q", includeGlobal.Usage)
		}
		for _, expected := range []string{
			"ASC_CONFIG_PATH",
			"--include-global",
			`asc auth logout --all --include-global --confirm`,
		} {
			if !strings.Contains(cmd.LongHelp, expected) {
				t.Fatalf("expected logout help to contain %q, got %q", expected, cmd.LongHelp)
			}
		}
	})

	t.Run("include-global reaches the removers", func(t *testing.T) {
		for _, args := range [][]string{
			{"--name", "demo", "--include-global", "--confirm"},
			{"--all", "--include-global", "--confirm"},
		} {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				calls := stubLogoutRemovers(t)

				cmd := AuthLogoutCommand()
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatalf("Parse() error: %v", err)
				}
				if err := cmd.Exec(context.Background(), []string{}); err != nil {
					t.Fatalf("Exec() error: %v", err)
				}
				if len(calls.options) != 1 || !calls.options[0].IncludeGlobalConfig {
					t.Fatalf("expected IncludeGlobalConfig to be passed, got %+v", calls.options)
				}
			})
		}
	})

	t.Run("removal defaults to the active config scope", func(t *testing.T) {
		calls := stubLogoutRemovers(t)

		cmd := AuthLogoutCommand()
		if err := cmd.FlagSet.Parse([]string{"--all", "--confirm"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		if err := cmd.Exec(context.Background(), []string{}); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
		if len(calls.options) != 1 || calls.options[0].IncludeGlobalConfig {
			t.Fatalf("expected default removal options, got %+v", calls.options)
		}
	})

	t.Run("blank name rejected", func(t *testing.T) {
		cmd := AuthLogoutCommand()
		if err := cmd.FlagSet.Parse([]string{"--name", "   "}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "--name cannot be blank") {
			t.Fatalf("expected blank-name error in stderr, got %q", stderr)
		}
	})

	t.Run("all and name mutually exclusive", func(t *testing.T) {
		cmd := AuthLogoutCommand()
		if err := cmd.FlagSet.Parse([]string{"--all", "--name", "demo"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "mutually exclusive") {
			t.Fatalf("expected mutually exclusive error in stderr, got %q", stderr)
		}
	})

	t.Run("remove named credential", func(t *testing.T) {
		calls := stubLogoutRemovers(t)

		cmd := AuthLogoutCommand()
		if err := cmd.FlagSet.Parse([]string{"--name", "demo", "--confirm"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		if err := cmd.Exec(context.Background(), []string{}); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}

		if len(calls.names) != 1 || calls.names[0] != "demo" || calls.all != 0 {
			t.Fatalf("unexpected removal calls: %+v", calls)
		}
	})

	t.Run("remove all credentials", func(t *testing.T) {
		calls := stubLogoutRemovers(t)

		cmd := AuthLogoutCommand()
		if err := cmd.FlagSet.Parse([]string{"--all", "--confirm"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		if err := cmd.Exec(context.Background(), []string{}); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
		if len(calls.names) != 0 || calls.all != 1 {
			t.Fatalf("unexpected removal calls: %+v", calls)
		}
	})

	t.Run("missing confirm is a usage error before removal", func(t *testing.T) {
		for _, args := range [][]string{
			{"--name", "legacy"},
			{"--all"},
			{},
		} {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				calls := stubLogoutRemovers(t)

				cmd := AuthLogoutCommand()
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatalf("Parse() error: %v", err)
				}
				stdout, stderr := captureAuthOutput(t, func() {
					err := cmd.Exec(context.Background(), []string{})
					if !errors.Is(err, flag.ErrHelp) {
						t.Fatalf("Exec() error = %v, want flag.ErrHelp", err)
					}
				})
				if stdout != "" {
					t.Fatalf("stdout = %q, want empty", stdout)
				}
				if !strings.Contains(stderr, "--confirm is required to remove stored credentials") {
					t.Fatalf("stderr = %q, want --confirm usage error", stderr)
				}
				if strings.Contains(stderr, "Warning") || strings.Contains(stderr, "5.0.0") {
					t.Fatalf("stderr still carries deprecation wording: %q", stderr)
				}
				if len(calls.names) != 0 || calls.all != 0 {
					t.Fatalf("credentials were removed without --confirm: %+v", calls)
				}
			})
		}
	})

	t.Run("explicit confirm false is rejected before removal", func(t *testing.T) {
		calls := stubLogoutRemovers(t)

		cmd := AuthLogoutCommand()
		if err := cmd.FlagSet.Parse([]string{"--all", "--confirm=false"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("Exec() error = %v, want flag.ErrHelp", err)
			}
		})
		if !strings.Contains(stderr, "--confirm must be true when specified") {
			t.Fatalf("stderr = %q, want explicit-false usage error", stderr)
		}
		if len(calls.names) != 0 || calls.all != 0 {
			t.Fatalf("credentials were removed with --confirm=false: %+v", calls)
		}
	})

	t.Run("unexpected arguments are rejected before removal", func(t *testing.T) {
		calls := stubLogoutRemovers(t)

		cmd := AuthLogoutCommand()
		if err := cmd.FlagSet.Parse([]string{"--all", "--confirm"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{"unexpected"})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("Exec() error = %v, want flag.ErrHelp", err)
			}
		})
		if !strings.Contains(stderr, "unexpected argument(s): unexpected") {
			t.Fatalf("expected unexpected-argument error, got %q", stderr)
		}

		if len(calls.names) != 0 || calls.all != 0 {
			t.Fatalf("expected no removal, got %+v", calls)
		}
	})
}

func TestAuthExportToConfigCommand(t *testing.T) {
	t.Run("requires confirm", func(t *testing.T) {
		cmd := AuthExportToConfigCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "--confirm is required") {
			t.Fatalf("expected confirm error in stderr, got %q", stderr)
		}
	})

	t.Run("invalid output format", func(t *testing.T) {
		cmd := AuthExportToConfigCommand()
		if err := cmd.FlagSet.Parse([]string{"--confirm", "--output", "yaml"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, `(got "yaml")`) {
			t.Fatalf("expected unsupported format error, got %q", stderr)
		}
	})

	t.Run("local and config are mutually exclusive", func(t *testing.T) {
		cmd := AuthExportToConfigCommand()
		if err := cmd.FlagSet.Parse([]string{"--confirm", "--local", "--config", filepath.Join(t.TempDir(), "config.json")}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "--local and --config are mutually exclusive") {
			t.Fatalf("expected local/config error, got %q", stderr)
		}
	})

	t.Run("local resolves repo config path", func(t *testing.T) {
		withTempRepo(t, func(repo string) {
			var captured authsvc.MigrateKeychainToConfigOptions
			restore := SetMigrateKeychainToConfig(func(opts authsvc.MigrateKeychainToConfigOptions) (authsvc.MigrateKeychainToConfigResult, error) {
				captured = opts
				return authsvc.MigrateKeychainToConfigResult{
					ConfigPath: opts.ConfigPath,
					Migrated: []authsvc.MigratedCredential{
						{Name: "demo", KeyID: "KEY123", PrivateKeyPath: "/tmp/AuthKey.p8"},
					},
				}, nil
			})
			t.Cleanup(restore)

			cmd := AuthExportToConfigCommand()
			if err := cmd.FlagSet.Parse([]string{"--confirm", "--local"}); err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
			expectedRepo := repo
			if resolvedRepo, err := filepath.EvalSymlinks(repo); err == nil {
				expectedRepo = resolvedRepo
			}
			expected := filepath.Join(expectedRepo, ".asc", "config.json")
			if captured.ConfigPath != expected {
				t.Fatalf("captured ConfigPath = %q, want %q", captured.ConfigPath, expected)
			}
		})
	})

	t.Run("default resolves active config path", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "active-config.json")
		t.Setenv("ASC_CONFIG_PATH", configPath)
		var captured authsvc.MigrateKeychainToConfigOptions
		restore := SetMigrateKeychainToConfig(func(opts authsvc.MigrateKeychainToConfigOptions) (authsvc.MigrateKeychainToConfigResult, error) {
			captured = opts
			return authsvc.MigrateKeychainToConfigResult{
				ConfigPath: opts.ConfigPath,
				Migrated: []authsvc.MigratedCredential{
					{Name: "demo", KeyID: "KEY123", PrivateKeyPath: "/tmp/AuthKey.p8"},
				},
			}, nil
		})
		t.Cleanup(restore)

		cmd := AuthExportToConfigCommand()
		if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		if err := cmd.Exec(context.Background(), []string{}); err != nil {
			t.Fatalf("Exec() error: %v", err)
		}
		if captured.ConfigPath != configPath {
			t.Fatalf("captured ConfigPath = %q, want active config path %q", captured.ConfigPath, configPath)
		}
	})

	t.Run("json success", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "config.json")
		privateKeyDir := filepath.Join(t.TempDir(), "keys")
		var captured authsvc.MigrateKeychainToConfigOptions
		restore := SetMigrateKeychainToConfig(func(opts authsvc.MigrateKeychainToConfigOptions) (authsvc.MigrateKeychainToConfigResult, error) {
			captured = opts
			return authsvc.MigrateKeychainToConfigResult{
				ConfigPath:          opts.ConfigPath,
				PrivateKeyDir:       opts.PrivateKeyDir,
				RemovedFromKeychain: opts.RemoveKeychain,
				Migrated: []authsvc.MigratedCredential{
					{
						Name:           "demo",
						KeyID:          "KEY123",
						PrivateKeyPath: filepath.Join(privateKeyDir, "AuthKey_demo.p8"),
					},
				},
			}, nil
		})
		t.Cleanup(restore)

		cmd := AuthExportToConfigCommand()
		if err := cmd.FlagSet.Parse([]string{
			"--confirm",
			"--output", "json",
			"--config", configPath,
			"--private-key-dir", privateKeyDir,
			"--remove-keychain",
		}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, stderr := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		if captured.ConfigPath != configPath {
			t.Fatalf("captured ConfigPath = %q, want %q", captured.ConfigPath, configPath)
		}
		if captured.PrivateKeyDir != privateKeyDir {
			t.Fatalf("captured PrivateKeyDir = %q, want %q", captured.PrivateKeyDir, privateKeyDir)
		}
		if !captured.RemoveKeychain {
			t.Fatal("expected RemoveKeychain option to be true")
		}

		var payload struct {
			ConfigPath          string `json:"configPath"`
			PrivateKeyDir       string `json:"privateKeyDir"`
			RemovedFromKeychain bool   `json:"removedFromKeychain"`
			Migrated            []struct {
				Name  string `json:"name"`
				KeyID string `json:"keyId"`
			} `json:"migrated"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("failed to unmarshal migration json: %v; stdout=%q", err, stdout)
		}
		if payload.ConfigPath != configPath || payload.PrivateKeyDir != privateKeyDir || !payload.RemovedFromKeychain {
			t.Fatalf("unexpected migration payload: %+v", payload)
		}
		if len(payload.Migrated) != 1 || payload.Migrated[0].Name != "demo" || payload.Migrated[0].KeyID != "KEY123" {
			t.Fatalf("unexpected migrated credentials payload: %+v", payload.Migrated)
		}
	})
}

func TestAuthStatusCommand(t *testing.T) {
	t.Run("no credentials", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)

		cmd := AuthStatusCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "table"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, _ := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})
		if !strings.Contains(stdout, "No credentials stored") {
			t.Fatalf("expected no-credentials message, got %q", stdout)
		}
	})

	t.Run("truthy bypass env value updates status warning text", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "yes")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)

		cmd := AuthStatusCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "table"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, _ := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})
		if !strings.Contains(stdout, "Keychain bypassed via ASC_BYPASS_KEYCHAIN") {
			t.Fatalf("expected bypass warning, got %q", stdout)
		}
		if strings.Contains(stdout, "ASC_BYPASS_KEYCHAIN=1") {
			t.Fatalf("expected warning to avoid hardcoded '=1', got %q", stdout)
		}
	})

	t.Run("stored credentials are rendered as table", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		if err := authsvc.StoreCredentialsConfigAt("demo", "KEY123", "ISS123", "/tmp/AuthKey.p8", cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthStatusCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "table"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, _ := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})
		if !strings.Contains(stdout, "Stored credentials:") {
			t.Fatalf("expected stored credentials heading, got %q", stdout)
		}
		if !strings.Contains(stdout, "┌") || !strings.Contains(stdout, "│ Name ") {
			t.Fatalf("expected table output for credentials, got %q", stdout)
		}
		if !strings.Contains(stdout, "demo") || !strings.Contains(stdout, "KEY123") {
			t.Fatalf("expected credential values in table output, got %q", stdout)
		}
	})

	t.Run("json output renders structured credentials", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		if err := authsvc.StoreCredentialsConfigAt("demo", "KEY123", "ISS123", "/tmp/AuthKey.p8", cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthStatusCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, stderr := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})
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
		if payload.StorageBackend == "" {
			t.Fatalf("expected storage backend in json output, got %q", stdout)
		}
		if len(payload.Credentials) != 1 {
			t.Fatalf("expected one credential in json output, got %d", len(payload.Credentials))
		}
		if payload.Credentials[0].Name != "demo" || payload.Credentials[0].KeyID != "KEY123" {
			t.Fatalf("unexpected credential json payload: %+v", payload.Credentials[0])
		}
	})

	t.Run("invalid output format returns usage error", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)

		cmd := AuthStatusCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "yaml"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if stdout != "" {
			t.Fatalf("expected empty stdout, got %q", stdout)
		}
		if !strings.Contains(stderr, `(got "yaml")`) {
			t.Fatalf("expected unsupported format error, got %q", stderr)
		}
	})

	t.Run("validate reports failures", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		if err := authsvc.StoreCredentialsConfigAt("demo", "KEY", "ISS", "/tmp/AuthKey.p8", cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		restore := SetStatusValidateCredential(func(context.Context, authsvc.Credential) error {
			return errors.New("validation failed")
		})
		t.Cleanup(restore)

		cmd := AuthStatusCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "table", "--validate"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "validation failed for 1 credential") {
			t.Fatalf("expected validation failure summary, got %v", err)
		}
	})

	t.Run("validate preserves private-key diagnostic", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		missingKeyPath := filepath.Join(t.TempDir(), "missing.p8")

		restore := SetListStoredCredentials(func() ([]authsvc.Credential, error) {
			return []authsvc.Credential{{
				Name:           "demo",
				KeyID:          "KEY",
				IssuerID:       "ISS",
				PrivateKeyPath: missingKeyPath,
			}}, nil
		})
		t.Cleanup(restore)

		cmd := AuthStatusCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "table", "--validate"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		var runErr error
		captureAuthOutput(t, func() {
			runErr = cmd.Exec(context.Background(), []string{})
		})
		if runErr == nil || !strings.Contains(runErr.Error(), "validation failed for 1 credential") {
			t.Fatalf("expected validation failure summary, got %v", runErr)
		}
		diagnostic, ok := shared.DiagnosticFromError(runErr)
		if !ok {
			t.Fatalf("DiagnosticFromError(%v) did not find metadata", runErr)
		}
		if diagnostic.Code != shared.DiagnosticFileNotFound || diagnostic.Parameter != "--private-key" {
			t.Fatalf("diagnostic = %+v, want file_not_found for --private-key", diagnostic)
		}
	})

	t.Run("validate omits diagnostic for mixed aggregate failures", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)

		restoreList := SetListStoredCredentials(func() ([]authsvc.Credential, error) {
			return []authsvc.Credential{
				{Name: "missing", KeyID: "KEY1", IssuerID: "ISS"},
				{Name: "rejected", KeyID: "KEY2", IssuerID: "ISS"},
			}, nil
		})
		t.Cleanup(restoreList)
		restoreValidate := SetStatusValidateCredential(func(_ context.Context, cred authsvc.Credential) error {
			if cred.Name == "missing" {
				return shared.WithDiagnostic(errors.New("missing key"), shared.DiagnosticFileNotFound, "--private-key")
			}
			return shared.WithDiagnostic(errors.New("rejected"), shared.DiagnosticAuthenticationRejected, "")
		})
		t.Cleanup(restoreValidate)

		cmd := AuthStatusCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "table", "--validate"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		var runErr error
		captureAuthOutput(t, func() {
			runErr = cmd.Exec(context.Background(), []string{})
		})
		if runErr == nil || !strings.Contains(runErr.Error(), "validation failed for 2 credential(s)") {
			t.Fatalf("expected validation failure summary, got %v", runErr)
		}
		if diagnostic, ok := shared.DiagnosticFromError(runErr); ok {
			t.Fatalf("diagnostic = %+v, want no diagnostic for mixed aggregate failures", diagnostic)
		}
	})

	t.Run("validate permission warning does not fail", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		if err := authsvc.StoreCredentialsConfigAt("demo", "KEY", "ISS", "/tmp/AuthKey.p8", cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		restore := SetStatusValidateCredential(func(context.Context, authsvc.Credential) error {
			return NewPermissionWarning(errors.New("forbidden"))
		})
		t.Cleanup(restore)

		cmd := AuthStatusCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "table", "--validate"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, _ := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})
		if !strings.Contains(stdout, "insufficient permissions") {
			t.Fatalf("expected permission warning message, got %q", stdout)
		}
	})
}

func TestAuthStatusEnvironmentNoteReportsCompleteEnvironmentPrecedence(t *testing.T) {
	note := authStatusEnvironmentNote("", false, true, true, true)
	want := "Complete environment credential fields take precedence when no profile is selected; stored credential lookup is skipped."
	if note != want {
		t.Fatalf("authStatusEnvironmentNote() = %q, want %q", note, want)
	}
}

func TestCredentialStorageLabel(t *testing.T) {
	if got := credentialStorageLabel(authsvc.Credential{}); got != "unknown" {
		t.Fatalf("credentialStorageLabel(empty) = %q, want unknown", got)
	}
	if got := credentialStorageLabel(authsvc.Credential{Source: "config"}); got != "config" {
		t.Fatalf("credentialStorageLabel(source) = %q, want config", got)
	}
	if got := credentialStorageLabel(authsvc.Credential{
		Source:     "config",
		SourcePath: "/tmp/config.json",
	}); got != "config: /tmp/config.json" {
		t.Fatalf("credentialStorageLabel(source+path) = %q", got)
	}
}

func TestAuthTokenCommand(t *testing.T) {
	t.Run("no credentials", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		clearResolvedAuthEnv(t)

		cmd := AuthTokenCommand()
		if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "missing authentication") {
			t.Fatalf("expected missing authentication error, got %v", err)
		}
	})

	t.Run("profile not found", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		clearResolvedAuthEnv(t)
		keyPath := writeTempECDSAKeyFile(t)
		if err := authsvc.StoreCredentialsConfigAt("demo", "KEY", "ISS", keyPath, cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthTokenCommand()
		if err := cmd.FlagSet.Parse([]string{"--name", "missing", "--confirm"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected profile not found error, got %v", err)
		}
	})

	t.Run("requires confirm", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		clearResolvedAuthEnv(t)
		keyPath := writeTempECDSAKeyFile(t)
		if err := authsvc.StoreCredentialsConfigAt("demo", "KEY123", "ISS456", keyPath, cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthTokenCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "--confirm is required") {
			t.Fatalf("expected confirm required error, got %q", stderr)
		}
	})

	t.Run("falls back to env credentials", func(t *testing.T) {
		keyPath := writeTempECDSAKeyFile(t)
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
		clearResolvedAuthEnv(t)
		t.Setenv("ASC_KEY_ID", "ENVKEY")
		t.Setenv("ASC_ISSUER_ID", "ENVISS")
		t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")

		cmd := AuthTokenCommand()
		if err := cmd.FlagSet.Parse([]string{"--confirm", "--output", "json"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, _ := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})

		var payload struct {
			Token   string `json:"token"`
			KeyID   string `json:"keyId"`
			Profile string `json:"profile"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("failed to unmarshal json: %v; stdout=%q", err, stdout)
		}
		if payload.KeyID != "ENVKEY" {
			t.Fatalf("expected keyId ENVKEY, got %q", payload.KeyID)
		}
		if payload.Profile != "" {
			t.Fatalf("expected empty profile for env credentials, got %q", payload.Profile)
		}
		if parts := strings.Split(payload.Token, "."); len(parts) != 3 {
			t.Fatalf("expected JWT in token field, got %q", payload.Token)
		}
	})

	t.Run("rejects permissive key files", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		clearResolvedAuthEnv(t)
		keyPath := writeTempECDSAKeyFile(t)
		if err := os.Chmod(keyPath, 0o644); err != nil {
			t.Fatalf("Chmod() error: %v", err)
		}
		if err := authsvc.StoreCredentialsConfigAt("demo", "KEY123", "ISS456", keyPath, cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthTokenCommand()
		if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "private key file is too permissive") {
			t.Fatalf("expected insecure key file error, got %v", err)
		}
		diagnostic, ok := shared.DiagnosticFromError(err)
		if !ok {
			t.Fatalf("DiagnosticFromError(%v) did not find metadata", err)
		}
		if diagnostic.Code != shared.DiagnosticFilePermissionsInsecure || diagnostic.Parameter != "--private-key" {
			t.Fatalf("diagnostic = %+v, want file_permissions_insecure for --private-key", diagnostic)
		}
	})

	t.Run("prints raw token to stdout", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		clearResolvedAuthEnv(t)
		keyPath := writeTempECDSAKeyFile(t)
		if err := authsvc.StoreCredentialsConfigAt("demo", "KEY123", "ISS456", keyPath, cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthTokenCommand()
		if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, _ := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})
		// JWT has three dot-separated parts
		parts := strings.Split(stdout, ".")
		if len(parts) != 3 {
			t.Fatalf("expected JWT with 3 parts, got %d: %q", len(parts), stdout)
		}
	})

	t.Run("json output includes token and keyId", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		clearResolvedAuthEnv(t)
		keyPath := writeTempECDSAKeyFile(t)
		if err := authsvc.StoreCredentialsConfigAt("demo", "KEY123", "ISS456", keyPath, cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthTokenCommand()
		if err := cmd.FlagSet.Parse([]string{"--confirm", "--output", "json"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, _ := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})

		var payload struct {
			Token   string `json:"token"`
			KeyID   string `json:"keyId"`
			Profile string `json:"profile"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("failed to unmarshal json: %v; stdout=%q", err, stdout)
		}
		if payload.KeyID != "KEY123" {
			t.Fatalf("expected keyId KEY123, got %q", payload.KeyID)
		}
		if payload.Profile != "demo" {
			t.Fatalf("expected profile demo, got %q", payload.Profile)
		}
		parts := strings.Split(payload.Token, ".")
		if len(parts) != 3 {
			t.Fatalf("expected JWT in token field, got %q", payload.Token)
		}
	})

	t.Run("selects named profile", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		clearResolvedAuthEnv(t)
		keyPath := writeTempECDSAKeyFile(t)
		if err := authsvc.StoreCredentialsConfigAt("first", "KEY_A", "ISS_A", keyPath, cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}
		if err := authsvc.StoreCredentialsConfigAt("second", "KEY_B", "ISS_B", keyPath, cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthTokenCommand()
		if err := cmd.FlagSet.Parse([]string{"--name", "second", "--confirm", "--output", "json"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, _ := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})

		var payload struct {
			KeyID   string `json:"keyId"`
			Profile string `json:"profile"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("failed to unmarshal json: %v", err)
		}
		if payload.KeyID != "KEY_B" {
			t.Fatalf("expected KEY_B, got %q", payload.KeyID)
		}
	})

	t.Run("multiple credentials without name errors", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		clearResolvedAuthEnv(t)
		keyPath := writeTempECDSAKeyFile(t)
		if err := authsvc.StoreCredentialsConfigAt("first", "KEY_A", "ISS_A", keyPath, cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}
		if err := authsvc.StoreCredentialsConfigAt("second", "KEY_B", "ISS_B", keyPath, cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}
		// Clear the default so neither is default
		cfg, err := config.LoadAt(cfgPath)
		if err != nil {
			t.Fatalf("LoadAt() error: %v", err)
		}
		cfg.DefaultKeyName = ""
		if err := config.SaveAt(cfgPath, cfg); err != nil {
			t.Fatalf("SaveAt() error: %v", err)
		}

		cmd := AuthTokenCommand()
		if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err = cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "missing authentication") {
			t.Fatalf("expected missing authentication error, got %v", err)
		}
	})

	t.Run("unexpected args rejected", func(t *testing.T) {
		cmd := AuthTokenCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{"extra"})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "unexpected argument") {
			t.Fatalf("expected unexpected argument error, got %q", stderr)
		}
	})
}

func TestAuthIssuerIDCommand(t *testing.T) {
	t.Run("no credentials", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		clearResolvedAuthEnv(t)

		cmd := AuthIssuerIDCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "missing authentication") {
			t.Fatalf("expected missing authentication error, got %v", err)
		}
	})

	t.Run("prints raw issuer id to stdout", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		clearResolvedAuthEnv(t)
		keyPath := writeTempECDSAKeyFile(t)
		if err := authsvc.StoreCredentialsConfigAt("demo", "KEY123", "ISS456", keyPath, cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthIssuerIDCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, _ := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})
		if stdout != "ISS456" {
			t.Fatalf("expected raw issuer id, got %q", stdout)
		}
	})

	t.Run("json output includes issuer id and profile", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		clearResolvedAuthEnv(t)
		keyPath := writeTempECDSAKeyFile(t)
		if err := authsvc.StoreCredentialsConfigAt("demo", "KEY123", "ISS456", keyPath, cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthIssuerIDCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, _ := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})

		var payload struct {
			IssuerID string `json:"issuerId"`
			Profile  string `json:"profile"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("failed to unmarshal json: %v; stdout=%q", err, stdout)
		}
		if payload.IssuerID != "ISS456" {
			t.Fatalf("expected issuerId ISS456, got %q", payload.IssuerID)
		}
		if payload.Profile != "demo" {
			t.Fatalf("expected profile demo, got %q", payload.Profile)
		}
	})

	t.Run("selects named profile", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		clearResolvedAuthEnv(t)
		keyPath := writeTempECDSAKeyFile(t)
		if err := authsvc.StoreCredentialsConfigAt("first", "KEY_A", "ISS_A", keyPath, cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}
		if err := authsvc.StoreCredentialsConfigAt("second", "KEY_B", "ISS_B", keyPath, cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthIssuerIDCommand()
		if err := cmd.FlagSet.Parse([]string{"--name", "second", "--output", "json"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, _ := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})

		var payload struct {
			IssuerID string `json:"issuerId"`
			Profile  string `json:"profile"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("failed to unmarshal json: %v", err)
		}
		if payload.IssuerID != "ISS_B" {
			t.Fatalf("expected ISS_B, got %q", payload.IssuerID)
		}
		if payload.Profile != "second" {
			t.Fatalf("expected profile second, got %q", payload.Profile)
		}
	})

	t.Run("falls back to env credentials", func(t *testing.T) {
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
		clearResolvedAuthEnv(t)
		t.Setenv("ASC_KEY_ID", "ENVKEY")
		t.Setenv("ASC_ISSUER_ID", "ENVISS")
		t.Setenv("ASC_PRIVATE_KEY_PATH", writeTempECDSAKeyFile(t))
		t.Setenv("ASC_PRIVATE_KEY", "")
		t.Setenv("ASC_PRIVATE_KEY_B64", "")

		cmd := AuthIssuerIDCommand()
		if err := cmd.FlagSet.Parse([]string{"--output", "json"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		stdout, _ := captureAuthOutput(t, func() {
			if err := cmd.Exec(context.Background(), []string{}); err != nil {
				t.Fatalf("Exec() error: %v", err)
			}
		})

		var payload struct {
			IssuerID string `json:"issuerId"`
			Profile  string `json:"profile"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("failed to unmarshal json: %v", err)
		}
		if payload.IssuerID != "ENVISS" {
			t.Fatalf("expected ENVISS, got %q", payload.IssuerID)
		}
		if payload.Profile != "" {
			t.Fatalf("expected empty profile for env credentials, got %q", payload.Profile)
		}
	})

	t.Run("unexpected args rejected", func(t *testing.T) {
		cmd := AuthIssuerIDCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		_, stderr := captureAuthOutput(t, func() {
			err := cmd.Exec(context.Background(), []string{"extra"})
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", err)
			}
		})
		if !strings.Contains(stderr, "unexpected argument") {
			t.Fatalf("expected unexpected argument error, got %q", stderr)
		}
	})
}

func writeTempECDSAKeyFile(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("x509.MarshalPKCS8PrivateKey() error: %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	path := filepath.Join(t.TempDir(), "AuthKey.p8")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	return path
}

func withTempRepo(t *testing.T, fn func(repo string)) {
	t.Helper()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}

	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir(.git) error: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir(%s) error: %v", repo, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	fn(repo)
}

func clearResolvedAuthEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
}

func captureAuthOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout) error: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stderr) error: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW

	outC := make(chan string)
	errC := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutR)
		_ = stdoutR.Close()
		outC <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrR)
		_ = stderrR.Close()
		errC <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = stdoutW.Close()
		_ = stderrW.Close()
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	return <-outC, <-errC
}

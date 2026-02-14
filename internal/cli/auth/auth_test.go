package auth

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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	authsvc "github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestCommandWrapperReturnsAuthCommand(t *testing.T) {
	cmd := Command()
	if cmd == nil {
		t.Fatal("Command() returned nil")
	}
	if cmd.Name != "auth" {
		t.Fatalf("Command().Name = %q, want %q", cmd.Name, "auth")
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
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "unsupported format") {
			t.Fatalf("expected unsupported format error, got %v", err)
		}
	})

	t.Run("pretty requires json output", func(t *testing.T) {
		cmd := AuthDoctorCommand()
		if err := cmd.FlagSet.Parse([]string{"--pretty"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "--pretty is only valid with JSON output") {
			t.Fatalf("expected pretty/json error, got %v", err)
		}
	})

	t.Run("fix requires confirm", func(t *testing.T) {
		cmd := AuthDoctorCommand()
		if err := cmd.FlagSet.Parse([]string{"--fix"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "--fix requires --confirm") {
			t.Fatalf("expected fix/confirm error, got %v", err)
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
				Title: "Storage",
				Checks: []authsvc.DoctorCheck{
					{Status: authsvc.DoctorOK, Message: "all good"},
				},
			},
		},
		Summary: authsvc.DoctorSummary{},
	}
	stdout, _ := captureAuthOutput(t, func() {
		printDoctorReport(report)
	})
	if !strings.Contains(stdout, "Auth Doctor") || !strings.Contains(stdout, "[OK] all good") {
		t.Fatalf("unexpected doctor output: %q", stdout)
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
	err := validateLoginNetwork(context.Background(), "KEY", "ISS", "/definitely/missing/AuthKey.p8")
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

func TestValidateLoginCredentials(t *testing.T) {
	keyPath := writeTempECDSAKeyFile(t)

	t.Run("jwt generation failure", func(t *testing.T) {
		restoreJWT := SetLoginJWTGenerator(func(string, string, *ecdsa.PrivateKey) (string, error) {
			return "", errors.New("jwt failed")
		})
		prevNetwork := loginNetworkValidate
		loginNetworkValidate = func(context.Context, string, string, string) error {
			t.Fatal("network validation should not run when jwt generation fails")
			return nil
		}
		t.Cleanup(func() {
			restoreJWT()
			loginNetworkValidate = prevNetwork
		})

		err := validateLoginCredentials(context.Background(), "KEY", "ISS", keyPath, true)
		if err == nil || !strings.Contains(err.Error(), "failed to generate JWT") {
			t.Fatalf("expected jwt error, got %v", err)
		}
	})

	t.Run("network disabled succeeds", func(t *testing.T) {
		restoreJWT := SetLoginJWTGenerator(func(string, string, *ecdsa.PrivateKey) (string, error) {
			return "token", nil
		})
		prevNetwork := loginNetworkValidate
		loginNetworkValidate = func(context.Context, string, string, string) error {
			t.Fatal("network validation should not run when network=false")
			return nil
		}
		t.Cleanup(func() {
			restoreJWT()
			loginNetworkValidate = prevNetwork
		})

		if err := validateLoginCredentials(context.Background(), "KEY", "ISS", keyPath, false); err != nil {
			t.Fatalf("validateLoginCredentials() error: %v", err)
		}
	})

	t.Run("network validation error", func(t *testing.T) {
		restoreJWT := SetLoginJWTGenerator(func(string, string, *ecdsa.PrivateKey) (string, error) {
			return "token", nil
		})
		prevNetwork := loginNetworkValidate
		loginNetworkValidate = func(context.Context, string, string, string) error {
			return errors.New("network down")
		}
		t.Cleanup(func() {
			restoreJWT()
			loginNetworkValidate = prevNetwork
		})

		err := validateLoginCredentials(context.Background(), "KEY", "ISS", keyPath, true)
		if err == nil || !strings.Contains(err.Error(), "network validation failed") {
			t.Fatalf("expected network validation error, got %v", err)
		}
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
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "--local requires --bypass-keychain") {
			t.Fatalf("expected local/bypass error, got %v", err)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		cmd := AuthLoginCommand()
		if err := cmd.FlagSet.Parse([]string{"--key-id", "KEY", "--issuer-id", "ISS", "--private-key", "/tmp/AuthKey.p8"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})

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
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Fatalf("expected mutual exclusion error, got %v", err)
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
}

func TestAuthLogoutCommand(t *testing.T) {
	t.Run("blank name rejected", func(t *testing.T) {
		cmd := AuthLogoutCommand()
		if err := cmd.FlagSet.Parse([]string{"--name", "   "}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "--name cannot be blank") {
			t.Fatalf("expected blank-name error, got %v", err)
		}
	})

	t.Run("all and name mutually exclusive", func(t *testing.T) {
		cmd := AuthLogoutCommand()
		if err := cmd.FlagSet.Parse([]string{"--all", "--name", "demo"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Fatalf("expected mutually exclusive error, got %v", err)
		}
	})

	t.Run("remove named credential", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		if err := authsvc.StoreCredentialsConfigAt("demo", "KEY", "ISS", "/tmp/AuthKey.p8", cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthLogoutCommand()
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
		if cfg.DefaultKeyName != "" {
			t.Fatalf("expected cleared default key, got %q", cfg.DefaultKeyName)
		}
	})

	t.Run("remove all credentials", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)
		if err := authsvc.StoreCredentialsConfigAt("one", "KEY1", "ISS1", "/tmp/AuthKey1.p8", cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}
		if err := authsvc.StoreCredentialsConfigAt("two", "KEY2", "ISS2", "/tmp/AuthKey2.p8", cfgPath); err != nil {
			t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
		}

		cmd := AuthLogoutCommand()
		if err := cmd.FlagSet.Parse([]string{"--all"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		execErr := cmd.Exec(context.Background(), []string{})

		// Only skip on specific keychain interaction errors (errSecInteractionNotAllowed = -25301)
		// This is expected in CI environments where keychain is locked
		if execErr != nil && (strings.Contains(execErr.Error(), "errSecInteractionNotAllowed") ||
			strings.Contains(execErr.Error(), "(-25301)")) {
			t.Skipf("skipping: keychain interaction not allowed - %v", execErr)
		}

		// Verify: either no error (success) or config was cleared
		cfg, err := config.LoadAt(cfgPath)
		if err != nil {
			t.Fatalf("LoadAt() error: %v", err)
		}
		if len(cfg.Keys) != 0 || cfg.DefaultKeyName != "" || cfg.KeyID != "" {
			t.Fatalf("expected cleared credentials, got %+v", cfg)
		}
	})
}

func TestAuthStatusCommand(t *testing.T) {
	t.Run("no credentials", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "config.json")
		t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
		t.Setenv("ASC_CONFIG_PATH", cfgPath)

		cmd := AuthStatusCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
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
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
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
		if err := cmd.FlagSet.Parse([]string{"--validate"}); err != nil {
			t.Fatalf("Parse() error: %v", err)
		}
		err := cmd.Exec(context.Background(), []string{})
		if err == nil || !strings.Contains(err.Error(), "validation failed for 1 credential") {
			t.Fatalf("expected validation failure summary, got %v", err)
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
		if err := cmd.FlagSet.Parse([]string{"--validate"}); err != nil {
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

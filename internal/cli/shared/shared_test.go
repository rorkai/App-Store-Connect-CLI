package shared

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func resetDefaultOutput(t *testing.T) {
	t.Helper()
	ResetDefaultOutputFormat()
	t.Cleanup(func() {
		ResetDefaultOutputFormat()
	})
}

func TestDefaultOutputFormat_ReturnsJSON(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "")
	if got := DefaultOutputFormat(); got != "json" {
		t.Fatalf("expected json, got %q", got)
	}
}

func TestDefaultOutputFormat_UnsetReturnsJSON(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "")
	os.Unsetenv("ASC_DEFAULT_OUTPUT")
	if got := DefaultOutputFormat(); got != "json" {
		t.Fatalf("expected json, got %q", got)
	}
}

func TestDefaultOutputFormat_Table(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "table")
	if got := DefaultOutputFormat(); got != "table" {
		t.Fatalf("expected table, got %q", got)
	}
}

func TestDefaultOutputFormat_Markdown(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "markdown")
	if got := DefaultOutputFormat(); got != "markdown" {
		t.Fatalf("expected markdown, got %q", got)
	}
}

func TestDefaultOutputFormat_MD(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "md")
	if got := DefaultOutputFormat(); got != "md" {
		t.Fatalf("expected md, got %q", got)
	}
}

func TestDefaultOutputFormat_JSON(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "json")
	if got := DefaultOutputFormat(); got != "json" {
		t.Fatalf("expected json, got %q", got)
	}
}

func TestDefaultOutputFormat_CaseInsensitive(t *testing.T) {
	for _, value := range []string{"TABLE", "Table", "tAbLe", "MARKDOWN", "JSON"} {
		t.Run(value, func(t *testing.T) {
			resetDefaultOutput(t)
			t.Setenv("ASC_DEFAULT_OUTPUT", value)
			got := DefaultOutputFormat()
			expected := strings.ToLower(value)
			if got != expected {
				t.Fatalf("expected %q, got %q", expected, got)
			}
		})
	}
}

func TestDefaultOutputFormat_WhitespaceHandled(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "  table  ")
	if got := DefaultOutputFormat(); got != "table" {
		t.Fatalf("expected table, got %q", got)
	}
}

func TestDefaultOutputFormat_InvalidFallsBackToJSON(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "xml")
	stdout, stderr := captureOutput(t, func() {
		got := DefaultOutputFormat()
		if got != "json" {
			t.Fatalf("expected json fallback, got %q", got)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "invalid ASC_DEFAULT_OUTPUT value") {
		t.Fatalf("expected warning on stderr, got %q", stderr)
	}
}

func TestValidateNextURL_ValidAppStoreConnectURL(t *testing.T) {
	err := validateNextURL("https://api.appstoreconnect.apple.com/v1/apps?cursor=abc")
	if err != nil {
		t.Fatalf("validateNextURL() error = %v", err)
	}
}

func TestValidateNextURL_RejectsMalformedHost(t *testing.T) {
	tests := []string{
		"http://localhost:80:80/v1/apps?cursor=abc",
		"http://::1/v1/apps?cursor=abc",
	}

	for _, next := range tests {
		t.Run(next, func(t *testing.T) {
			err := validateNextURL(next)
			if err == nil {
				t.Fatalf("expected error for malformed URL %q", next)
			}
			if !strings.Contains(err.Error(), "--next must be a valid URL") {
				t.Fatalf("expected parse validation error, got %v", err)
			}
		})
	}
}

func TestProgressEnabled_RespectsNoProgressFlag(t *testing.T) {
	prevNoProgress := noProgress
	prevIsTerminal := isTerminal
	t.Cleanup(func() {
		SetNoProgress(prevNoProgress)
		isTerminal = prevIsTerminal
	})

	isTerminal = func(int) bool { return true }
	SetNoProgress(true)

	if ProgressEnabled() {
		t.Fatal("expected progress to be disabled when --no-progress is set")
	}
}

func TestProgressEnabled_DisabledWhenStderrNotTTY(t *testing.T) {
	prevNoProgress := noProgress
	prevIsTerminal := isTerminal
	t.Cleanup(func() {
		SetNoProgress(prevNoProgress)
		isTerminal = prevIsTerminal
	})

	isTerminal = func(int) bool { return false }
	SetNoProgress(false)

	if ProgressEnabled() {
		t.Fatal("expected progress to be disabled when stderr is not a TTY")
	}
}

func TestProgressEnabled_EnabledWhenTTYAndNotDisabled(t *testing.T) {
	prevNoProgress := noProgress
	prevIsTerminal := isTerminal
	t.Cleanup(func() {
		SetNoProgress(prevNoProgress)
		isTerminal = prevIsTerminal
	})

	isTerminal = func(int) bool { return true }
	SetNoProgress(false)

	if !ProgressEnabled() {
		t.Fatal("expected progress to be enabled when stderr is a TTY and --no-progress is not set")
	}
}

func TestResolvePrivateKeyPathPrefersPath(t *testing.T) {
	resetPrivateKeyTemp(t)
	t.Setenv("ASC_PRIVATE_KEY_PATH", "/tmp/AuthKey.p8")
	t.Setenv("ASC_PRIVATE_KEY_B64", base64.StdEncoding.EncodeToString([]byte("ignored")))
	t.Setenv("ASC_PRIVATE_KEY", "ignored")

	path, err := resolvePrivateKeyPath()
	if err != nil {
		t.Fatalf("resolvePrivateKeyPath() error: %v", err)
	}
	if path != "/tmp/AuthKey.p8" {
		t.Fatalf("expected path /tmp/AuthKey.p8, got %q", path)
	}
}

func TestResolvePrivateKeyPathFromBase64(t *testing.T) {
	resetPrivateKeyTemp(t)
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")

	encoded := base64.StdEncoding.EncodeToString([]byte("key-data"))
	t.Setenv("ASC_PRIVATE_KEY_B64", encoded)

	path, err := resolvePrivateKeyPath()
	if err != nil {
		t.Fatalf("resolvePrivateKeyPath() error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "key-data" {
		t.Fatalf("expected key data %q, got %q", "key-data", string(data))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("expected 0600 permissions, got %v", info.Mode().Perm())
	}
}

func TestResolvePrivateKeyPathFromRawValue(t *testing.T) {
	resetPrivateKeyTemp(t)
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")

	t.Setenv("ASC_PRIVATE_KEY", "line1\\nline2")
	path, err := resolvePrivateKeyPath()
	if err != nil {
		t.Fatalf("resolvePrivateKeyPath() error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "line1\nline2" {
		t.Fatalf("expected newline expansion, got %q", string(data))
	}
}

func TestCleanupTempPrivateKeysRemovesFile(t *testing.T) {
	resetPrivateKeyTemp(t)
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")

	encoded := base64.StdEncoding.EncodeToString([]byte("key-data"))
	t.Setenv("ASC_PRIVATE_KEY_B64", encoded)

	path, err := resolvePrivateKeyPath()
	if err != nil {
		t.Fatalf("resolvePrivateKeyPath() error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected temp key file to exist, got %v", err)
	}

	CleanupTempPrivateKeys()

	if _, err := os.Stat(path); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected temp key file to be removed, got %v", err)
	}
	if privateKeyTempPath != "" {
		t.Fatalf("expected temp key path to be cleared, got %q", privateKeyTempPath)
	}
}

func TestResolvePrivateKeyPathInvalidBase64(t *testing.T) {
	resetPrivateKeyTemp(t)
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "not-base64")

	if _, err := resolvePrivateKeyPath(); err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestCheckMixedCredentialSourcesWarns(t *testing.T) {
	previousStrict := strictAuth
	strictAuth = false
	t.Cleanup(func() {
		strictAuth = previousStrict
	})
	t.Setenv(strictAuthEnvVar, "")

	stdout, stderr := captureOutput(t, func() {
		if err := checkMixedCredentialSources(credentialSource{
			keyID:    "keychain",
			issuerID: "env",
			keyPath:  "env",
		}); err != nil {
			t.Fatalf("expected warning only, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Warning: credentials loaded from multiple sources") {
		t.Fatalf("expected mixed-source warning, got %q", stderr)
	}
}

func TestCheckMixedCredentialSourcesStrictErrors(t *testing.T) {
	previousStrict := strictAuth
	strictAuth = true
	t.Cleanup(func() {
		strictAuth = previousStrict
	})
	t.Setenv(strictAuthEnvVar, "")

	stdout, stderr := captureOutput(t, func() {
		if err := checkMixedCredentialSources(credentialSource{
			keyID:    "keychain",
			issuerID: "env",
			keyPath:  "env",
		}); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestCheckMixedCredentialSourcesStrictAuthEnvErrors(t *testing.T) {
	previousStrict := strictAuth
	strictAuth = false
	t.Cleanup(func() {
		strictAuth = previousStrict
	})
	t.Setenv(strictAuthEnvVar, "yes")

	stdout, stderr := captureOutput(t, func() {
		if err := checkMixedCredentialSources(credentialSource{
			keyID:    "keychain",
			issuerID: "env",
			keyPath:  "env",
		}); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestStrictAuthEnabled_EnvTruthyValues(t *testing.T) {
	previousStrict := strictAuth
	strictAuth = false
	t.Cleanup(func() {
		strictAuth = previousStrict
	})

	values := []string{"1", "true", "TRUE", "yes", "y", "on", "On"}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			t.Setenv(strictAuthEnvVar, value)
			stdout, stderr := captureOutput(t, func() {
				if !strictAuthEnabled() {
					t.Fatalf("expected strict auth enabled for %q", value)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestStrictAuthEnabled_EnvFalseyValues(t *testing.T) {
	previousStrict := strictAuth
	strictAuth = false
	t.Cleanup(func() {
		strictAuth = previousStrict
	})

	values := []string{"0", "false", "FALSE", "no", "n", "off", "Off"}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			t.Setenv(strictAuthEnvVar, value)
			stdout, stderr := captureOutput(t, func() {
				if strictAuthEnabled() {
					t.Fatalf("expected strict auth disabled for %q", value)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestStrictAuthEnabled_InvalidValueWarnsAndDisables(t *testing.T) {
	previousStrict := strictAuth
	strictAuth = false
	t.Cleanup(func() {
		strictAuth = previousStrict
	})
	t.Setenv(strictAuthEnvVar, "maybe")

	stdout, stderr := captureOutput(t, func() {
		if strictAuthEnabled() {
			t.Fatal("expected strict auth to be disabled for invalid value")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "invalid ASC_STRICT_AUTH value \"maybe\"") {
		t.Fatalf("expected invalid value warning, got %q", stderr)
	}
}

func TestGetASCClient_ProfileMissingSkipsEnvFallback(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeECDSAPEM(t, keyPath)

	cfg := &config.Config{
		DefaultKeyName: "personal",
		Keys: []config.Credential{
			{
				Name:           "personal",
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
	t.Setenv("ASC_PROFILE", "missing")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)

	previousProfile := selectedProfile
	selectedProfile = ""
	t.Cleanup(func() {
		selectedProfile = previousProfile
	})

	_, err := getASCClient()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetASCClient_BypassKeychainPrefersEnvOverConfig(t *testing.T) {
	resetPrivateKeyTemp(t)

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	envKeyPath := filepath.Join(tempDir, "AuthKey-Env.p8")
	writeECDSAPEM(t, envKeyPath)

	cfg := &config.Config{
		DefaultKeyName: "config",
		Keys: []config.Credential{
			{
				Name:           "config",
				KeyID:          "CFGKEY",
				IssuerID:       "CFGISS",
				PrivateKeyPath: filepath.Join(tempDir, "missing.p8"),
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", envKeyPath)

	previousProfile := selectedProfile
	selectedProfile = ""
	t.Cleanup(func() {
		selectedProfile = previousProfile
	})

	if _, err := getASCClient(); err != nil {
		t.Fatalf("expected env credentials to override config, got %v", err)
	}
}

func resetPrivateKeyTemp(t *testing.T) {
	t.Helper()
	CleanupTempPrivateKeys()
	t.Cleanup(func() {
		CleanupTempPrivateKeys()
	})
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
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

func TestProgressEnabled_DisabledByFlag(t *testing.T) {
	previousNoProgress := noProgress
	t.Cleanup(func() {
		noProgress = previousNoProgress
	})

	SetNoProgress(true)
	if ProgressEnabled() {
		t.Fatal("expected ProgressEnabled() to return false when noProgress is true")
	}

	SetNoProgress(false)
	// Progress should still be disabled in tests because stderr is piped (not a TTY)
	if ProgressEnabled() {
		t.Fatal("expected ProgressEnabled() to return false in test environment (stderr not a TTY)")
	}
}

func TestProgressEnabled_DisabledInNonTTY(t *testing.T) {
	previousNoProgress := noProgress
	noProgress = false
	t.Cleanup(func() {
		noProgress = previousNoProgress
	})

	// In test environment, stderr is piped (not a TTY)
	// So ProgressEnabled should return false regardless of flag
	if ProgressEnabled() {
		t.Fatal("expected ProgressEnabled() to return false when stderr is not a TTY")
	}
}

func TestSetNoProgress(t *testing.T) {
	previousNoProgress := noProgress
	t.Cleanup(func() {
		noProgress = previousNoProgress
	})

	SetNoProgress(true)
	if !noProgress {
		t.Fatal("expected noProgress to be true after SetNoProgress(true)")
	}

	SetNoProgress(false)
	if noProgress {
		t.Fatal("expected noProgress to be false after SetNoProgress(false)")
	}
}

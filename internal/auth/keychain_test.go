package auth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/99designs/keyring"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
	}()

	outputC := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		outputC <- buf.String()
	}()

	fn()

	_ = w.Close()
	return <-outputC
}

func TestShouldBypassKeychainEnvSemantics(t *testing.T) {
	originalValue, originalPresent := os.LookupEnv("ASC_BYPASS_KEYCHAIN")
	t.Cleanup(func() {
		if originalPresent {
			_ = os.Setenv("ASC_BYPASS_KEYCHAIN", originalValue)
			return
		}
		_ = os.Unsetenv("ASC_BYPASS_KEYCHAIN")
	})

	tests := []struct {
		name   string
		value  *string
		expect bool
	}{
		{name: "unset", value: nil, expect: false},
		{name: "empty string", value: ptrTo(""), expect: false},
		{name: "whitespace only", value: ptrTo("   "), expect: false},
		{name: "truthy one", value: ptrTo("1"), expect: true},
		{name: "truthy true", value: ptrTo("true"), expect: true},
		{name: "truthy yes", value: ptrTo("yes"), expect: true},
		{name: "truthy on", value: ptrTo("on"), expect: true},
		{name: "truthy mixed case and spaces", value: ptrTo("  TrUe  "), expect: true},
		{name: "falsey zero", value: ptrTo("0"), expect: false},
		{name: "falsey false", value: ptrTo("false"), expect: false},
		{name: "falsey no", value: ptrTo("no"), expect: false},
		{name: "falsey off", value: ptrTo("off"), expect: false},
		{name: "invalid value", value: ptrTo("banana"), expect: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value == nil {
				_ = os.Unsetenv("ASC_BYPASS_KEYCHAIN")
			} else {
				_ = os.Setenv("ASC_BYPASS_KEYCHAIN", *tt.value)
			}
			if got := shouldBypassKeychain(); got != tt.expect {
				t.Fatalf("shouldBypassKeychain() = %v, want %v (value=%v)", got, tt.expect, tt.value)
			}
		})
	}
}

func TestShouldBypassKeychain_InvalidValueWarnsAndDisables(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "banana")
	resetInvalidBypassKeychainWarnings()
	t.Cleanup(resetInvalidBypassKeychainWarnings)

	stderr := captureStderr(t, func() {
		if shouldBypassKeychain() {
			t.Fatal("expected invalid value to keep keychain bypass disabled")
		}
		if shouldBypassKeychain() {
			t.Fatal("expected invalid value to continue keeping keychain bypass disabled")
		}
	})

	if count := strings.Count(stderr, `Warning: invalid ASC_BYPASS_KEYCHAIN value "banana"`); count != 1 {
		t.Fatalf("expected one invalid value warning, got %d in %q", count, stderr)
	}
	if !strings.Contains(stderr, "keychain bypass disabled") {
		t.Fatalf("expected warning to explain conservative behavior, got %q", stderr)
	}
}

func ptrTo(value string) *string {
	return &value
}

func TestConfigProfileSelection(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	cfg := &config.Config{
		DefaultKeyName: "personal",
		Keys: []config.Credential{
			{
				Name:           "personal",
				KeyID:          "KEY1",
				IssuerID:       "ISSUER1",
				PrivateKeyPath: "/tmp/AuthKey1.p8",
			},
			{
				Name:           "client",
				KeyID:          "KEY2",
				IssuerID:       "ISSUER2",
				PrivateKeyPath: "/tmp/AuthKey2.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	defaultCreds, err := GetCredentials("")
	if err != nil {
		t.Fatalf("GetCredentials(default) error: %v", err)
	}
	if defaultCreds.KeyID != "KEY1" {
		t.Fatalf("expected default KeyID KEY1, got %q", defaultCreds.KeyID)
	}

	clientCreds, err := GetCredentials("client")
	if err != nil {
		t.Fatalf("GetCredentials(client) error: %v", err)
	}
	if clientCreds.KeyID != "KEY2" {
		t.Fatalf("expected client KeyID KEY2, got %q", clientCreds.KeyID)
	}
}

func TestKeychainAvailableBypassSkipsKeyring(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	previous := keyringOpener
	keyringOpener = func() (keyring.Keyring, error) {
		t.Fatal("expected keyring opener to be skipped when bypassing keychain")
		return nil, nil
	}
	t.Cleanup(func() {
		keyringOpener = previous
	})

	available, err := KeychainAvailable()
	if err != nil {
		t.Fatalf("KeychainAvailable() error: %v", err)
	}
	if available {
		t.Fatal("expected keychain unavailable when bypassed")
	}
}

func TestKeychainAvailableInvalidBypassStillChecksKeyring(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "banana")
	resetInvalidBypassKeychainWarnings()
	t.Cleanup(resetInvalidBypassKeychainWarnings)

	previous := keyringOpener
	called := false
	keyringOpener = func() (keyring.Keyring, error) {
		called = true
		return nil, keyring.ErrNoAvailImpl
	}
	t.Cleanup(func() {
		keyringOpener = previous
	})

	stderr := captureStderr(t, func() {
		available, err := KeychainAvailable()
		if err != nil {
			t.Fatalf("KeychainAvailable() error: %v", err)
		}
		if available {
			t.Fatal("expected keychain to remain unavailable when opener reports no backend")
		}
	})

	if !called {
		t.Fatal("expected invalid bypass value to still consult the keyring opener")
	}
	if !strings.Contains(stderr, `Warning: invalid ASC_BYPASS_KEYCHAIN value "banana"`) {
		t.Fatalf("expected invalid bypass warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "keychain bypass disabled") {
		t.Fatalf("expected warning to explain disabled bypass behavior, got %q", stderr)
	}
}

func TestConfigProfileListAndSwitch(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	cfg := &config.Config{
		DefaultKeyName: "personal",
		Keys: []config.Credential{
			{
				Name:           "personal",
				KeyID:          "KEY1",
				IssuerID:       "ISSUER1",
				PrivateKeyPath: "/tmp/AuthKey1.p8",
			},
			{
				Name:           "client",
				KeyID:          "KEY2",
				IssuerID:       "ISSUER2",
				PrivateKeyPath: "/tmp/AuthKey2.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	credentials, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(credentials) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(credentials))
	}

	defaultFound := false
	for _, cred := range credentials {
		if cred.Name == "personal" && cred.IsDefault {
			defaultFound = true
		}
	}
	if !defaultFound {
		t.Fatal("expected personal credential to be default")
	}

	if err := SetDefaultCredentials("client"); err != nil {
		t.Fatalf("SetDefaultCredentials() error: %v", err)
	}
	updated, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("LoadAt() error: %v", err)
	}
	if updated.DefaultKeyName != "client" {
		t.Fatalf("expected DefaultKeyName to be client, got %q", updated.DefaultKeyName)
	}
}

func TestSaveDefaultNameAlignsLegacyFields(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	cfg := &config.Config{
		DefaultKeyName: "personal",
		KeyID:          "OLDKEY",
		IssuerID:       "OLDISSUER",
		PrivateKeyPath: "/tmp/old.p8",
		Keys: []config.Credential{
			{
				Name:           "personal",
				KeyID:          "KEY1",
				IssuerID:       "ISSUER1",
				PrivateKeyPath: "/tmp/personal.p8",
			},
			{
				Name:           "client",
				KeyID:          "KEY2",
				IssuerID:       "ISSUER2",
				PrivateKeyPath: "/tmp/client.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	if err := saveDefaultName("client"); err != nil {
		t.Fatalf("saveDefaultName() error: %v", err)
	}

	updated, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("LoadAt() error: %v", err)
	}
	if updated.DefaultKeyName != "client" {
		t.Fatalf("expected DefaultKeyName to be client, got %q", updated.DefaultKeyName)
	}
	if updated.KeyID != "KEY2" {
		t.Fatalf("expected legacy KeyID KEY2, got %q", updated.KeyID)
	}
	if updated.IssuerID != "ISSUER2" {
		t.Fatalf("expected legacy IssuerID ISSUER2, got %q", updated.IssuerID)
	}
	if updated.PrivateKeyPath != "/tmp/client.p8" {
		t.Fatalf("expected legacy PrivateKeyPath /tmp/client.p8, got %q", updated.PrivateKeyPath)
	}
}

func TestSaveDefaultNamePreservesLegacyFieldsOnMismatch(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	cfg := &config.Config{
		DefaultKeyName: "personal",
		KeyID:          "KEY1",
		IssuerID:       "ISSUER1",
		PrivateKeyPath: "/tmp/personal.p8",
		Keys: []config.Credential{
			{
				Name:           "personal",
				KeyID:          "KEY1",
				IssuerID:       "ISSUER1",
				PrivateKeyPath: "/tmp/personal.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	if err := saveDefaultName("other"); err != nil {
		t.Fatalf("saveDefaultName() error: %v", err)
	}

	updated, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("LoadAt() error: %v", err)
	}
	if updated.DefaultKeyName != "other" {
		t.Fatalf("expected DefaultKeyName to be other, got %q", updated.DefaultKeyName)
	}
	if updated.KeyID != "" || updated.IssuerID != "" || updated.PrivateKeyPath != "" {
		t.Fatalf("expected top-level fields cleared for keychain default, got %+v", updated)
	}
	if len(updated.Keys) != 1 || updated.Keys[0].Name != "personal" || updated.Keys[0].KeyID != "KEY1" {
		t.Fatalf("expected personal credential preserved in keys, got %+v", updated.Keys)
	}
}

func withRepoConfigWithoutCredentials(t *testing.T) (string, string) {
	t.Helper()
	withArrayKeyring(t)
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("HOME", t.TempDir())
	outside := t.TempDir()
	repo := t.TempDir()
	if err := config.SaveAt(filepath.Join(repo, ".asc", "config.json"), &config.Config{AppID: "123"}); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}
	t.Chdir(outside)
	return outside, repo
}

func TestStoreCredentials_InRepoWritesDefaultToGlobalConfig(t *testing.T) {
	outside, repo := withRepoConfigWithoutCredentials(t)
	if err := StoreCredentialsWithKeyType("old", "OLDKEY", "ISS", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials(old) error: %v", err)
	}

	t.Chdir(repo)
	if err := StoreCredentialsWithKeyType("new", "NEWKEY", "ISS", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials(new) error: %v", err)
	}

	t.Chdir(outside)
	cfg, err := GetCredentials("")
	if err != nil {
		t.Fatalf("GetCredentials() error: %v", err)
	}
	if cfg.KeyID != "NEWKEY" {
		t.Fatalf("expected new default NEWKEY outside the repo, got %q", cfg.KeyID)
	}
}

func TestStoreCredentials_KeychainUnavailableInRepoWritesGlobalConfig(t *testing.T) {
	_, repo := withRepoConfigWithoutCredentials(t)
	previous := keyringOpener
	keyringOpener = func() (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	}
	t.Cleanup(func() { keyringOpener = previous })

	t.Chdir(repo)
	if err := StoreCredentialsWithKeyType("fallback", "KEY123", "ISS", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}

	repoCfg, err := config.LoadAt(filepath.Join(repo, ".asc", "config.json"))
	if err != nil {
		t.Fatalf("LoadAt(repo) error: %v", err)
	}
	if hasAnyCredentials(repoCfg) || repoCfg.DefaultKeyName != "" {
		t.Fatalf("expected repo config to stay credential-free, got %+v", repoCfg)
	}
	globalPath, err := config.GlobalPath()
	if err != nil {
		t.Fatalf("GlobalPath() error: %v", err)
	}
	globalCfg, err := config.LoadAt(globalPath)
	if err != nil {
		t.Fatalf("LoadAt(global) error: %v", err)
	}
	if globalCfg.DefaultKeyName != "fallback" || globalCfg.KeyID != "KEY123" {
		t.Fatalf("expected credentials in global config, got %+v", globalCfg)
	}
}

func TestGetCredentials_RepoConfigWithoutCredentialsUsesGlobalDefault(t *testing.T) {
	_, repo := withRepoConfigWithoutCredentials(t)
	if err := StoreCredentialsWithKeyType("a", "AKEY", "ISS", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials(a) error: %v", err)
	}
	if err := StoreCredentialsWithKeyType("b", "BKEY", "ISS", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials(b) error: %v", err)
	}

	t.Chdir(repo)
	cfg, err := GetCredentials("")
	if err != nil {
		t.Fatalf("GetCredentials() error: %v", err)
	}
	if cfg.KeyID != "BKEY" {
		t.Fatalf("expected global default BKEY inside the repo, got %q", cfg.KeyID)
	}
}

func TestStoreCredentials_KeychainDefaultDoesNotInheritConfigProfileKey(t *testing.T) {
	withArrayKeyring(t)
	if err := StoreCredentialsConfigWithKeyType("old", "OLDKEY", "ISS", "/tmp/old.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentialsConfig(old) error: %v", err)
	}
	if err := StoreCredentialsWithKeyType("new", "NEWKEY", "ISS", "/tmp/new.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials(new) error: %v", err)
	}

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	if cfg, _, err := GetCredentialsWithSource(""); err == nil {
		t.Fatalf("expected keychain default to be unavailable from config, got KeyID %q for %q", cfg.KeyID, cfg.DefaultKeyName)
	}
	cfg, err := GetCredentials("old")
	if err != nil {
		t.Fatalf("GetCredentials(old) error: %v", err)
	}
	if cfg.KeyID != "OLDKEY" {
		t.Fatalf("expected profile old to keep OLDKEY, got %q", cfg.KeyID)
	}
}

func TestGetCredentials_PrefersKeychainOverConfig(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}

	storeCredentialInKeyring(t, newKr, "shared", "KEYCHAIN", "ISSUER-KEYCHAIN", "/tmp/keychain.p8")

	cfg := &config.Config{
		DefaultKeyName: "shared",
		Keys: []config.Credential{
			{
				Name:           "shared",
				KeyID:          "CONFIG",
				IssuerID:       "ISSUER-CONFIG",
				PrivateKeyPath: "/tmp/config.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	creds, err := GetCredentials("shared")
	if err != nil {
		t.Fatalf("GetCredentials(shared) error: %v", err)
	}
	if creds.KeyID != "KEYCHAIN" {
		t.Fatalf("expected keychain KeyID, got %q", creds.KeyID)
	}
	if creds.PrivateKeyPath != "/tmp/keychain.p8" {
		t.Fatalf("expected keychain path, got %q", creds.PrivateKeyPath)
	}
}

func TestGetCredentials_DefaultNameMissingReturnsError(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	cfg := &config.Config{
		DefaultKeyName: "missing",
		Keys: []config.Credential{
			{
				Name:           "personal",
				KeyID:          "KEY1",
				IssuerID:       "ISSUER1",
				PrivateKeyPath: "/tmp/personal.p8",
			},
			{
				Name:           "client",
				KeyID:          "KEY2",
				IssuerID:       "ISSUER2",
				PrivateKeyPath: "/tmp/client.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	if _, err := GetCredentials(""); err == nil {
		t.Fatal("expected error, got nil")
	}

	creds, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	for _, cred := range creds {
		if cred.IsDefault {
			t.Fatalf("expected no default credential, got %q", cred.Name)
		}
	}
}

func TestListCredentials_DedupesKeychainAndConfig(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}

	storeCredentialInKeyring(t, newKr, "shared", "KEYCHAIN", "ISSUER-KEYCHAIN", "/tmp/keychain.p8")

	cfg := &config.Config{
		DefaultKeyName: "shared",
		Keys: []config.Credential{
			{
				Name:           "shared",
				KeyID:          "CONFIG",
				IssuerID:       "ISSUER-CONFIG",
				PrivateKeyPath: "/tmp/config.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	creds, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	if creds[0].KeyID != "KEYCHAIN" {
		t.Fatalf("expected keychain KeyID, got %q", creds[0].KeyID)
	}
}

func TestListCredentials_MergesKeychainAndConfig(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}

	// Store one credential in keychain
	storeCredentialInKeyring(t, newKr, "keychain-only", "KC-KEY", "KC-ISSUER", "/tmp/kc.p8")

	// Store a different credential in config
	cfg := &config.Config{
		DefaultKeyName: "config-only",
		Keys: []config.Credential{
			{
				Name:           "config-only",
				KeyID:          "CFG-KEY",
				IssuerID:       "CFG-ISSUER",
				PrivateKeyPath: "/tmp/cfg.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	creds, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(creds))
	}

	// Verify both credentials are present
	foundKeychain := false
	foundConfig := false
	for _, cred := range creds {
		if cred.Name == "keychain-only" && cred.KeyID == "KC-KEY" && cred.Source == "keychain" {
			foundKeychain = true
		}
		if cred.Name == "config-only" && cred.KeyID == "CFG-KEY" && cred.Source == "config" {
			foundConfig = true
		}
	}
	if !foundKeychain {
		t.Fatal("expected keychain credential to be present")
	}
	if !foundConfig {
		t.Fatal("expected config credential to be present")
	}
}

func TestListCredentials_NoDefaultWhenMergedSourcesAndNoDefaultName(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}

	storeCredentialInKeyring(t, newKr, "keychain-only", "KC-KEY", "KC-ISSUER", "/tmp/kc.p8")

	cfg := &config.Config{
		Keys: []config.Credential{
			{
				Name:           "config-only",
				KeyID:          "CFG-KEY",
				IssuerID:       "CFG-ISSUER",
				PrivateKeyPath: "/tmp/cfg.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	creds, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(creds))
	}
	for _, cred := range creds {
		if cred.IsDefault {
			t.Fatalf("expected no default credential, got %q", cred.Name)
		}
	}
}

func TestListCredentials_ConfigErrorWhenKeychainAvailable(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}

	storeCredentialInKeyring(t, newKr, "keychain-only", "KC-KEY", "KC-ISSUER", "/tmp/kc.p8")

	if err := os.WriteFile(configPath, []byte("{invalid"), 0o600); err != nil {
		t.Fatalf("write invalid config error: %v", err)
	}

	creds, err := ListCredentials()
	if err == nil {
		t.Fatal("expected ListCredentials() error")
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	if creds[0].Name != "keychain-only" {
		t.Fatalf("expected keychain-only credential, got %q", creds[0].Name)
	}
}

func TestGetCredentials_DefaultFallsBackToConfigWhenKeychainHasCreds(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}

	storeCredentialInKeyring(t, newKr, "keychain-only", "KC-KEY", "KC-ISSUER", "/tmp/kc.p8")

	cfg := &config.Config{
		DefaultKeyName: "config-default",
		Keys: []config.Credential{
			{
				Name:           "config-default",
				KeyID:          "CFG-KEY",
				IssuerID:       "CFG-ISSUER",
				PrivateKeyPath: "/tmp/cfg.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	creds, source, err := GetCredentialsWithSource("")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource(default) error: %v", err)
	}
	if source != "config" {
		t.Fatalf("expected config source, got %q", source)
	}
	if creds.KeyID != "CFG-KEY" {
		t.Fatalf("expected KeyID CFG-KEY, got %q", creds.KeyID)
	}
}

func TestGetCredentials_PrefersKeysOverLegacy(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	cfg := &config.Config{
		DefaultKeyName: "personal",
		KeyID:          "LEGACY",
		IssuerID:       "LEGACYISS",
		PrivateKeyPath: "/tmp/legacy.p8",
		Keys: []config.Credential{
			{
				Name:           "personal",
				KeyID:          "KEY1",
				IssuerID:       "ISSUER1",
				PrivateKeyPath: "/tmp/personal.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	creds, err := GetCredentials("")
	if err != nil {
		t.Fatalf("GetCredentials(default) error: %v", err)
	}
	if creds.KeyID != "KEY1" {
		t.Fatalf("expected KeyID KEY1, got %q", creds.KeyID)
	}
}

func TestListCredentials_NoDefaultWhenMultipleAndNoDefaultName(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)

	storeCredentialInKeyring(t, newKr, "alpha", "KEYA", "ISSA", "/tmp/a.p8")
	storeCredentialInKeyring(t, newKr, "beta", "KEYB", "ISSB", "/tmp/b.p8")

	creds, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(creds))
	}
	for _, cred := range creds {
		if cred.IsDefault {
			t.Fatalf("expected no default credential, got %q", cred.Name)
		}
	}
}

func TestGetCredentials_TrimsAndIsCaseSensitive(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	cfg := &config.Config{
		DefaultKeyName: "personal",
		Keys: []config.Credential{
			{
				Name:           "personal",
				KeyID:          "KEY1",
				IssuerID:       "ISSUER1",
				PrivateKeyPath: "/tmp/personal.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	trimmed, err := GetCredentials("  personal  ")
	if err != nil {
		t.Fatalf("GetCredentials(trimmed) error: %v", err)
	}
	if trimmed.KeyID != "KEY1" {
		t.Fatalf("expected KeyID KEY1, got %q", trimmed.KeyID)
	}

	_, err = GetCredentials("Personal")
	if err == nil {
		t.Fatal("expected error for case mismatch, got nil")
	}
}

func TestGetCredentials_IncompleteConfigWhenKeychainUnavailable(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "0")

	cfg := &config.Config{
		KeyID: "ONLYKEY",
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	previous := keyringOpener
	previousLegacy := legacyKeyringOpener
	keyringOpener = func() (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	}
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	}
	t.Cleanup(func() {
		keyringOpener = previous
		legacyKeyringOpener = previousLegacy
	})

	if _, err := GetCredentials(""); err == nil {
		t.Fatal("expected error for incomplete config, got nil")
	}
}

func TestValidateKeyFilePermissions(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")

	writeECDSAPEM(t, keyPath, 0o644, true)

	if err := ValidateKeyFile(keyPath); err == nil {
		t.Fatalf("expected permission error for insecure key file")
	}
}

func TestValidateKeyFileSuccess(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")

	writeECDSAPEM(t, keyPath, 0o600, true)

	if err := ValidateKeyFile(keyPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateKeyFileRejectsNonP256Curve(t *testing.T) {
	for _, pkcs8 := range []bool{true, false} {
		name := "sec1"
		if pkcs8 {
			name = "pkcs8"
		}
		t.Run(name, func(t *testing.T) {
			keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
			writeECDSAPEMWithCurve(t, keyPath, 0o600, pkcs8, elliptic.P384())

			err := ValidateKeyFile(keyPath)
			if err == nil || !strings.Contains(err.Error(), "P-256") {
				t.Fatalf("ValidateKeyFile() error = %v, want P-256 requirement", err)
			}
		})
	}
}

func TestValidateKeyFileDirectory(t *testing.T) {
	tempDir := t.TempDir()

	if err := ValidateKeyFile(tempDir); err == nil {
		t.Fatalf("expected error for directory path")
	}
}

func TestLoadPrivateKeyPKCS8(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")

	writeECDSAPEM(t, keyPath, 0o600, true)

	key, err := LoadPrivateKey(keyPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey() error: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
}

func TestLoadPrivateKeySEC1(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey-EC.p8")

	writeECDSAPEM(t, keyPath, 0o600, false)

	key, err := LoadPrivateKey(keyPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey() error: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
}

func TestStoreAndListCredentials(t *testing.T) {
	withArrayKeyring(t)

	if err := StoreCredentialsWithKeyType("my-key", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}

	creds, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	if creds[0].Name != "my-key" {
		t.Fatalf("expected credential name %q, got %q", "my-key", creds[0].Name)
	}
	if !creds[0].IsDefault {
		t.Fatalf("expected credential to be default")
	}
}

func TestStoreCredentials_PersistsPrivateKeyPEMInKeychain(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)

	if err := StoreCredentialsWithKeyType("my-key", "KEY123", "ISS456", keyPath, config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}

	item, err := newKr.Get(keyringKey("my-key"))
	if err != nil {
		t.Fatalf("Get(keyring item) error: %v", err)
	}

	var payload credentialPayload
	if err := json.Unmarshal(item.Data, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if payload.PrivateKeyPath != keyPath {
		t.Fatalf("expected PrivateKeyPath %q, got %q", keyPath, payload.PrivateKeyPath)
	}
	if strings.TrimSpace(payload.PrivateKeyPEM) == "" {
		t.Fatal("expected private key PEM to be persisted in keychain payload")
	}
}

func TestGetCredentialsWithSource_KeychainEntrySurvivesOriginalKeyDeletion(t *testing.T) {
	withArrayKeyring(t)

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)

	if err := StoreCredentialsWithKeyType("my-key", "KEY123", "ISS456", keyPath, config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}
	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("os.Remove(%q) error: %v", keyPath, err)
	}

	creds, source, err := GetCredentialsWithSource("my-key")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource() error: %v", err)
	}
	if source != "keychain" {
		t.Fatalf("expected source keychain, got %q", source)
	}
	if creds.PrivateKeyPath != keyPath {
		t.Fatalf("expected original private key path %q, got %q", keyPath, creds.PrivateKeyPath)
	}
	if strings.TrimSpace(creds.PrivateKeyPEM) == "" {
		t.Fatal("expected private key PEM from keychain entry")
	}
}

func TestGetCredentialsWithSource_BackfillsLegacyKeychainPayload(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)
	storeCredentialInKeyring(t, newKr, "legacy", "KEY123", "ISS456", keyPath)

	first, source, err := GetCredentialsWithSource("legacy")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource(first) error: %v", err)
	}
	if source != "keychain" {
		t.Fatalf("expected source keychain, got %q", source)
	}
	if first.PrivateKeyPath != keyPath {
		t.Fatalf("expected original private key path %q, got %q", keyPath, first.PrivateKeyPath)
	}
	if strings.TrimSpace(first.PrivateKeyPEM) == "" {
		t.Fatal("expected first resolution to include private key PEM")
	}

	item, err := newKr.Get(keyringKey("legacy"))
	if err != nil {
		t.Fatalf("Get(keyring item) error: %v", err)
	}
	var payload credentialPayload
	if err := json.Unmarshal(item.Data, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if strings.TrimSpace(payload.PrivateKeyPEM) == "" {
		t.Fatal("expected legacy payload to be backfilled with private key PEM")
	}
	if item.Description != testCredentialMetadataDescription {
		t.Fatalf("expected metadata description %q, got %q", testCredentialMetadataDescription, item.Description)
	}

	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("os.Remove(%q) error: %v", keyPath, err)
	}
	second, source, err := GetCredentialsWithSource("legacy")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource(second) error: %v", err)
	}
	if source != "keychain" {
		t.Fatalf("expected source keychain, got %q", source)
	}
	if second.PrivateKeyPath != keyPath {
		t.Fatalf("expected original private key path %q, got %q", keyPath, second.PrivateKeyPath)
	}
	if strings.TrimSpace(second.PrivateKeyPEM) == "" {
		t.Fatal("expected private key PEM after deleting original file")
	}
}

func TestGetCredentialsWithSource_BackfillsMetadataForExistingPEM(t *testing.T) {
	modifiedAt := time.Date(2026, 3, 15, 4, 30, 0, 0, time.UTC)
	kr := &metadataKeyring{
		metadata: map[string]keyring.Metadata{
			keyringKey("legacy"): {
				Item: &keyring.Item{
					Key:   keyringKey("legacy"),
					Label: "ASC API Key (legacy)",
				},
				ModificationTime: modifiedAt,
			},
		},
		items: map[string]keyring.Item{},
	}
	withMetadataKeyring(t, kr)

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)

	privateKeyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error: %v", keyPath, err)
	}
	payload := credentialPayload{
		KeyID:          "KEY123",
		IssuerID:       "ISS456",
		PrivateKeyPath: keyPath,
		PrivateKeyPEM:  string(privateKeyPEM),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	kr.items[keyringKey("legacy")] = keyring.Item{
		Key:   keyringKey("legacy"),
		Data:  data,
		Label: "ASC API Key (legacy)",
	}

	creds, source, err := GetCredentialsWithSource("legacy")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource() error: %v", err)
	}
	if source != "keychain" {
		t.Fatalf("expected source keychain, got %q", source)
	}
	if strings.TrimSpace(creds.PrivateKeyPEM) == "" {
		t.Fatal("expected private key PEM from existing keychain payload")
	}

	summaries, err := ListCredentialSummaries()
	if err != nil {
		t.Fatalf("ListCredentialSummaries() error: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected one summary, got %d", len(summaries))
	}
	if summaries[0].KeyID != "KEY123" || summaries[0].IssuerID != "ISS456" {
		t.Fatalf("expected summary metadata from config fallback, got %#v", summaries[0])
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if len(cfg.KeychainMetadata) != 1 {
		t.Fatalf("expected one keychain metadata record, got %#v", cfg.KeychainMetadata)
	}
	if cfg.KeychainMetadata[0].Name != "legacy" || cfg.KeychainMetadata[0].KeyID != "KEY123" || cfg.KeychainMetadata[0].IssuerID != "ISS456" || cfg.KeychainMetadata[0].ModifiedAt != metadataModifiedAtString(modifiedAt) {
		t.Fatalf("unexpected keychain metadata record: %#v", cfg.KeychainMetadata[0])
	}

	item, err := kr.Get(keyringKey("legacy"))
	if err != nil {
		t.Fatalf("Get(keyring item) error: %v", err)
	}
	if item.Description != "" {
		t.Fatalf("expected keychain description to remain unchanged, got %q", item.Description)
	}
}

func TestGetCredentialsWithSource_BackfillsLegacyPEMOnlyOnce(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "0")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	previousKeyringOpener := keyringOpener
	previousLegacyKeyringOpener := legacyKeyringOpener
	kr := &countingKeyring{inner: keyring.NewArrayKeyring(nil)}
	keyringOpener = func() (keyring.Keyring, error) {
		return kr, nil
	}
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	}
	t.Cleanup(func() {
		keyringOpener = previousKeyringOpener
		legacyKeyringOpener = previousLegacyKeyringOpener
	})

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)
	storeCredentialInKeyring(t, kr.inner, "legacy", "KEY123", "ISS456", keyPath)

	kr.setCalls = 0
	_, source, err := GetCredentialsWithSource("legacy")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource() error: %v", err)
	}
	if source != "keychain" {
		t.Fatalf("expected source keychain, got %q", source)
	}
	if kr.setCalls != 1 {
		t.Fatalf("expected a single keychain rewrite for PEM backfill, got %d", kr.setCalls)
	}
}

func TestMigrateKeychainToConfigExportsMissingPrivateKeyPEM(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}

	sourceKeyPath := filepath.Join(t.TempDir(), "AuthKey-source.p8")
	writeECDSAPEM(t, sourceKeyPath, 0o600, true)
	privateKeyPEM, err := os.ReadFile(sourceKeyPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error: %v", sourceKeyPath, err)
	}

	missingKeyPath := filepath.Join(t.TempDir(), "missing", "AuthKey.p8")
	payload := credentialPayload{
		KeyID:          "KEY123",
		IssuerID:       "ISS456",
		PrivateKeyPath: missingKeyPath,
		PrivateKeyPEM:  string(privateKeyPEM),
	}
	item, err := keyringItemForCredential("demo profile", payload)
	if err != nil {
		t.Fatalf("keyringItemForCredential() error: %v", err)
	}
	if err := newKr.Set(item); err != nil {
		t.Fatalf("keyring Set() error: %v", err)
	}

	result, err := MigrateKeychainToConfig(MigrateKeychainToConfigOptions{
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("MigrateKeychainToConfig() error: %v", err)
	}
	if result.ConfigPath != configPath {
		t.Fatalf("ConfigPath = %q, want %q", result.ConfigPath, configPath)
	}
	if len(result.Migrated) != 1 {
		t.Fatalf("expected one migrated credential, got %#v", result.Migrated)
	}
	if !result.Migrated[0].ExportedPrivateKey {
		t.Fatalf("expected exported private key flag, got %#v", result.Migrated[0])
	}

	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("config.LoadAt() error: %v", err)
	}
	if cfg.DefaultKeyName != "demo profile" {
		t.Fatalf("DefaultKeyName = %q, want demo profile", cfg.DefaultKeyName)
	}
	if len(cfg.Keys) != 1 {
		t.Fatalf("expected one config credential, got %#v", cfg.Keys)
	}
	exportedPath := cfg.Keys[0].PrivateKeyPath
	if exportedPath == "" || exportedPath == missingKeyPath {
		t.Fatalf("expected migrated config to point at exported key, got %q", exportedPath)
	}
	exportedPEM, err := os.ReadFile(exportedPath)
	if err != nil {
		t.Fatalf("os.ReadFile(exported key) error: %v", err)
	}
	if string(exportedPEM) != string(privateKeyPEM) {
		t.Fatal("exported private key PEM did not match keychain PEM")
	}
	info, err := os.Stat(exportedPath)
	if err != nil {
		t.Fatalf("os.Stat(exported key) error: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected exported key permissions 0600, got %#o", got)
	}
}

func TestMigrateKeychainToConfigDefaultsToActiveConfigPath(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)
	storeCredentialInKeyring(t, newKr, "demo", "KEY123", "ISS456", keyPath)

	result, err := MigrateKeychainToConfig(MigrateKeychainToConfigOptions{})
	if err != nil {
		t.Fatalf("MigrateKeychainToConfig() error: %v", err)
	}
	if result.ConfigPath != configPath {
		t.Fatalf("ConfigPath = %q, want active config path %q", result.ConfigPath, configPath)
	}
	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("config.LoadAt(active) error: %v", err)
	}
	if len(cfg.Keys) != 1 || cfg.Keys[0].Name != "demo" {
		t.Fatalf("expected migrated credential in active config, got %#v", cfg.Keys)
	}
}

func TestMigrateKeychainToConfigExportsCollidingProfileNames(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}

	sourceKeyPath := filepath.Join(t.TempDir(), "AuthKey-source.p8")
	writeECDSAPEM(t, sourceKeyPath, 0o600, true)
	privateKeyPEM, err := os.ReadFile(sourceKeyPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error: %v", sourceKeyPath, err)
	}

	for _, profile := range []struct {
		name  string
		keyID string
	}{
		{name: "prod", keyID: "KEY123"},
		{name: "prod.", keyID: "KEY456"},
	} {
		item, err := keyringItemForCredential(profile.name, credentialPayload{
			KeyID:          profile.keyID,
			IssuerID:       "ISS456",
			PrivateKeyPath: filepath.Join(t.TempDir(), profile.name, "missing.p8"),
			PrivateKeyPEM:  string(privateKeyPEM),
		})
		if err != nil {
			t.Fatalf("keyringItemForCredential(%q) error: %v", profile.name, err)
		}
		if err := newKr.Set(item); err != nil {
			t.Fatalf("keyring Set(%q) error: %v", profile.name, err)
		}
	}

	result, err := MigrateKeychainToConfig(MigrateKeychainToConfigOptions{
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("MigrateKeychainToConfig() error: %v", err)
	}
	if len(result.Migrated) != 2 {
		t.Fatalf("expected two migrated credentials, got %#v", result.Migrated)
	}

	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("config.LoadAt() error: %v", err)
	}
	if len(cfg.Keys) != 2 {
		t.Fatalf("expected two config credentials, got %#v", cfg.Keys)
	}
	seen := map[string]struct{}{}
	for _, cred := range cfg.Keys {
		if cred.PrivateKeyPath == "" {
			t.Fatalf("expected exported key path for %#v", cred)
		}
		if _, ok := seen[cred.PrivateKeyPath]; ok {
			t.Fatalf("expected unique exported key path, got duplicate %q", cred.PrivateKeyPath)
		}
		seen[cred.PrivateKeyPath] = struct{}{}
		if _, err := os.Stat(cred.PrivateKeyPath); err != nil {
			t.Fatalf("expected exported key %q to exist: %v", cred.PrivateKeyPath, err)
		}
	}
}

func TestMigrateKeychainToConfigFailsWhenKeyMaterialIsMissing(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}

	missingKeyPath := filepath.Join(t.TempDir(), "missing", "AuthKey.p8")
	storeCredentialInKeyring(t, newKr, "demo", "KEY123", "ISS456", missingKeyPath)

	_, err := MigrateKeychainToConfig(MigrateKeychainToConfigOptions{
		ConfigPath: configPath,
	})
	if err == nil {
		t.Fatal("expected missing key material error")
	}
	if !strings.Contains(err.Error(), "private key file is missing") {
		t.Fatalf("expected missing private key error, got %v", err)
	}
}

func TestMigrateKeychainToConfigCanRemoveMigratedKeychainEntries(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)
	storeCredentialInKeyring(t, newKr, "demo", "KEY123", "ISS456", keyPath)

	result, err := MigrateKeychainToConfig(MigrateKeychainToConfigOptions{
		ConfigPath:     configPath,
		RemoveKeychain: true,
	})
	if err != nil {
		t.Fatalf("MigrateKeychainToConfig() error: %v", err)
	}
	if !result.RemovedFromKeychain {
		t.Fatalf("expected result to report keychain removal, got %#v", result)
	}
	if _, err := newKr.Get(keyringKey("demo")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("expected migrated keychain entry to be removed, got %v", err)
	}

	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("config.LoadAt() error: %v", err)
	}
	if len(cfg.Keys) != 1 || cfg.Keys[0].Name != "demo" {
		t.Fatalf("expected migrated config credential, got %#v", cfg.Keys)
	}
}

func TestMigrateKeychainToConfigRemovesOriginalSpacedKeychainName(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)
	storeCredentialInKeyring(t, newKr, " demo ", "KEY123", "ISS456", keyPath)

	result, err := MigrateKeychainToConfig(MigrateKeychainToConfigOptions{
		ConfigPath:     configPath,
		RemoveKeychain: true,
	})
	if err != nil {
		t.Fatalf("MigrateKeychainToConfig() error: %v", err)
	}
	if !result.RemovedFromKeychain {
		t.Fatalf("expected result to report keychain removal, got %#v", result)
	}
	if len(result.Migrated) != 1 || result.Migrated[0].Name != "demo" {
		t.Fatalf("expected trimmed config profile name, got %#v", result.Migrated)
	}
	if _, err := newKr.Get(keyringKey(" demo ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("expected original spaced keychain entry to be removed, got %v", err)
	}

	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("config.LoadAt() error: %v", err)
	}
	if len(cfg.Keys) != 1 || cfg.Keys[0].Name != "demo" {
		t.Fatalf("expected trimmed migrated config credential, got %#v", cfg.Keys)
	}
}

func TestMigrateKeychainToConfigRejectsTrimmedNameCollisions(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	configPath := os.Getenv("ASC_CONFIG_PATH")
	if configPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)
	storeCredentialInKeyring(t, newKr, "demo", "KEY123", "ISS456", keyPath)
	storeCredentialInKeyring(t, newKr, " demo ", "KEY456", "ISS789", keyPath)

	_, err := MigrateKeychainToConfig(MigrateKeychainToConfigOptions{
		ConfigPath:     configPath,
		RemoveKeychain: true,
	})
	if err == nil {
		t.Fatal("expected trimmed name collision error")
	}
	if !strings.Contains(err.Error(), `normalize to config profile "demo"`) {
		t.Fatalf("expected trimmed name collision error, got %v", err)
	}
	if _, err := config.LoadAt(configPath); !errors.Is(err, config.ErrNotFound) {
		t.Fatalf("expected config to remain unwritten, got %v", err)
	}
	if _, err := newKr.Get(keyringKey("demo")); err != nil {
		t.Fatalf("expected original demo keychain entry to remain, got %v", err)
	}
	if _, err := newKr.Get(keyringKey(" demo ")); err != nil {
		t.Fatalf("expected original spaced keychain entry to remain, got %v", err)
	}
}

func TestMigrateKeychainToConfigPreservesDestinationDefault(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	activeConfigPath := os.Getenv("ASC_CONFIG_PATH")
	if activeConfigPath == "" {
		t.Fatal("expected ASC_CONFIG_PATH to be set")
	}
	if err := config.SaveAt(activeConfigPath, &config.Config{DefaultKeyName: "alpha"}); err != nil {
		t.Fatalf("config.SaveAt(active) error: %v", err)
	}

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)
	storeCredentialInKeyring(t, newKr, "alpha", "KEY_ALPHA", "ISS_ALPHA", keyPath)
	storeCredentialInKeyring(t, newKr, "beta", "KEY_BETA", "ISS_BETA", keyPath)

	destinationPath := filepath.Join(t.TempDir(), "custom-config.json")
	if err := config.SaveAt(destinationPath, &config.Config{DefaultKeyName: "beta"}); err != nil {
		t.Fatalf("config.SaveAt(destination) error: %v", err)
	}

	result, err := MigrateKeychainToConfig(MigrateKeychainToConfigOptions{
		ConfigPath: destinationPath,
	})
	if err != nil {
		t.Fatalf("MigrateKeychainToConfig() error: %v", err)
	}
	if len(result.Migrated) != 2 {
		t.Fatalf("expected two migrated credentials, got %#v", result.Migrated)
	}

	cfg, err := config.LoadAt(destinationPath)
	if err != nil {
		t.Fatalf("config.LoadAt(destination) error: %v", err)
	}
	if cfg.DefaultKeyName != "beta" {
		t.Fatalf("DefaultKeyName = %q, want beta", cfg.DefaultKeyName)
	}
	if cfg.KeyID != "KEY_BETA" || cfg.IssuerID != "ISS_BETA" || cfg.PrivateKeyPath != keyPath {
		t.Fatalf("expected top-level fields to align with destination default beta, got %+v", cfg)
	}
}

func TestMigrateKeychainToConfigWarnsWhenKeychainRemovalFails(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "0")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	previousKeyringOpener := keyringOpener
	previousLegacyKeyringOpener := legacyKeyringOpener
	inner := keyring.NewArrayKeyring([]keyring.Item{})
	kr := &removeFailingKeyring{
		inner: inner,
		err:   errors.New("remove denied"),
	}
	keyringOpener = func() (keyring.Keyring, error) {
		return kr, nil
	}
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	}
	t.Cleanup(func() {
		keyringOpener = previousKeyringOpener
		legacyKeyringOpener = previousLegacyKeyringOpener
	})

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)
	storeCredentialInKeyring(t, inner, "demo", "KEY123", "ISS456", keyPath)

	configPath := filepath.Join(t.TempDir(), "config.json")
	result, err := MigrateKeychainToConfig(MigrateKeychainToConfigOptions{
		ConfigPath:     configPath,
		RemoveKeychain: true,
	})
	if err != nil {
		t.Fatalf("MigrateKeychainToConfig() error: %v", err)
	}
	if result.RemovedFromKeychain {
		t.Fatalf("expected RemovedFromKeychain=false when removal fails, got %#v", result)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "remove denied") {
		t.Fatalf("expected removal warning, got %#v", result.Warnings)
	}
	if _, err := inner.Get(keyringKey("demo")); err != nil {
		t.Fatalf("expected keychain entry to remain after removal failure, got %v", err)
	}

	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("config.LoadAt() error: %v", err)
	}
	if len(cfg.Keys) != 1 || cfg.Keys[0].Name != "demo" {
		t.Fatalf("expected config migration to remain successful, got %#v", cfg.Keys)
	}
}

func TestRemoveAllCredentials(t *testing.T) {
	withArrayKeyring(t)

	if err := StoreCredentialsWithKeyType("my-key", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}

	if err := RemoveAllCredentialsWithOptions(RemoveOptions{}); err != nil {
		t.Fatalf("RemoveAllCredentials() error: %v", err)
	}

	creds, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(creds) != 0 {
		t.Fatalf("expected no credentials after removal, got %d", len(creds))
	}
}

func TestRemoveAllCredentialsPreservesConfigSettings(t *testing.T) {
	withArrayKeyring(t)

	configPath, err := config.Path()
	if err != nil {
		t.Fatalf("config.Path() error: %v", err)
	}
	timeout, err := config.ParseDurationValue("60s")
	if err != nil {
		t.Fatalf("ParseDurationValue() error: %v", err)
	}
	if err := config.SaveAt(configPath, &config.Config{
		KeyID:          "KEY123",
		IssuerID:       "ISS456",
		PrivateKeyPath: "/tmp/AuthKey.p8",
		DefaultKeyName: "demo",
		AppID:          "12345",
		VendorNumber:   "67890",
		Timeout:        timeout,
	}); err != nil {
		t.Fatalf("config.SaveAt() error: %v", err)
	}

	if err := RemoveAllCredentialsWithOptions(RemoveOptions{}); err != nil {
		t.Fatalf("RemoveAllCredentials() error: %v", err)
	}

	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("config.LoadAt() error: %v", err)
	}
	if cfg.KeyID != "" || cfg.IssuerID != "" || cfg.PrivateKeyPath != "" || cfg.DefaultKeyName != "" {
		t.Fatalf("expected credentials cleared, got %+v", cfg)
	}
	if cfg.AppID != "12345" || cfg.VendorNumber != "67890" || cfg.Timeout.String() != "60s" {
		t.Fatalf("expected settings preserved, got %+v", cfg)
	}
}

func TestRemoveAllCredentials_ClearsStoredKeychainMetadata(t *testing.T) {
	withArrayKeyring(t)

	path, err := config.Path()
	if err != nil {
		t.Fatalf("config.Path() error: %v", err)
	}
	if err := config.SaveAt(path, &config.Config{
		KeychainMetadata: []config.KeychainMetadata{{
			Name:     "legacy",
			KeyID:    "KEY123",
			IssuerID: "ISS456",
		}},
	}); err != nil {
		t.Fatalf("config.SaveAt() error: %v", err)
	}

	if err := RemoveAllCredentialsWithOptions(RemoveOptions{}); err != nil {
		t.Fatalf("RemoveAllCredentials() error: %v", err)
	}

	cfg, err := config.LoadAt(path)
	if err != nil {
		t.Fatalf("config.LoadAt() error: %v", err)
	}
	if len(cfg.KeychainMetadata) != 0 {
		t.Fatalf("expected keychain metadata to be cleared, got %#v", cfg.KeychainMetadata)
	}
}

func TestStoreCredentialsFallbackToConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("HOME", tempDir)

	previous := keyringOpener
	keyringOpener = func() (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	}
	t.Cleanup(func() {
		keyringOpener = previous
	})

	if err := StoreCredentialsWithKeyType("test-fallback", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}

	creds, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	if creds[0].KeyID != "KEY123" {
		t.Fatalf("expected KeyID KEY123, got %q", creds[0].KeyID)
	}
}

func TestStoreCredentials_TrimsKeychainProfileNameBeforeSelectingDefault(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	storeCredentialInKeyring(t, newKr, "  spaced  ", "OLDKEY", "OLDISSUER", "/tmp/OldAuthKey.p8")

	if err := StoreCredentialsWithKeyType("  spaced  ", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}

	creds, err := GetCredentials("spaced")
	if err != nil {
		t.Fatalf("GetCredentials(trimmed profile) error: %v", err)
	}
	if creds.KeyID != "KEY123" || creds.IssuerID != "ISS456" {
		t.Fatalf("expected trimmed keychain profile credentials, got %+v", creds)
	}
	defaultCreds, err := GetCredentials("")
	if err != nil {
		t.Fatalf("GetCredentials(default) error: %v", err)
	}
	if defaultCreds.KeyID != "KEY123" || defaultCreds.IssuerID != "ISS456" {
		t.Fatalf("expected trimmed profile to remain default, got %+v", defaultCreds)
	}

	items, err := newKr.Keys()
	if err != nil {
		t.Fatalf("keychain Keys() error: %v", err)
	}
	for _, item := range items {
		if strings.Contains(item, "  spaced  ") {
			t.Fatalf("expected keychain profile key to use trimmed name, got %q", item)
		}
	}
}

func TestStoreCredentials_RemovesPreNormalizedProfileFromLegacyKeychain(t *testing.T) {
	newKr, legacyKr := withSeparateKeyrings(t)
	storeCredentialInKeyring(t, legacyKr, "  spaced  ", "OLDKEY", "OLDISSUER", "/tmp/OldAuthKey.p8")

	if err := StoreCredentialsWithKeyType("  spaced  ", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}
	if _, err := legacyKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("legacy raw credential error = %v, want ErrKeyNotFound", err)
	}
	item, err := newKr.Get(keyringKey("spaced"))
	if err != nil {
		t.Fatalf("normalized credential error: %v", err)
	}
	var payload credentialPayload
	if err := json.Unmarshal(item.Data, &payload); err != nil {
		t.Fatalf("decode normalized credential: %v", err)
	}
	if payload.KeyID != "KEY123" || payload.IssuerID != "ISS456" {
		t.Fatalf("normalized payload = %+v", payload)
	}
}

func TestStoreCredentials_RejectsDistinctNormalizedProfileCollisionBeforeMutation(t *testing.T) {
	newKr, legacyKr := withSeparateKeyrings(t)
	keyDir := t.TempDir()
	canonicalPath := filepath.Join(keyDir, "Canonical.p8")
	writeECDSAPEM(t, canonicalPath, 0o600, true)
	rawPath := filepath.Join(keyDir, "Raw.p8")
	writeECDSAPEM(t, rawPath, 0o600, true)
	storeCredentialInKeyring(t, newKr, "spaced", "CANONICAL", "ISSUER-CANONICAL", canonicalPath)
	storeCredentialInKeyring(t, legacyKr, "  spaced  ", "RAW", "ISSUER-RAW", rawPath)

	err := StoreCredentialsWithKeyType("  spaced  ", "NEW", "ISSUER-NEW", "/tmp/new.p8", config.CredentialKeyTypeTeam)
	wantErr := `credential profile name "  spaced  " conflicts with existing normalized profile "spaced"; ` +
		`remove the existing profile with 'asc auth logout --name "spaced" --confirm' and retry`
	if err == nil || err.Error() != wantErr {
		t.Fatalf("StoreCredentials() error = %v, want %q", err, wantErr)
	}

	canonical, err := newKr.Get(keyringKey("spaced"))
	if err != nil {
		t.Fatalf("canonical credential error: %v", err)
	}
	var canonicalPayload credentialPayload
	if err := json.Unmarshal(canonical.Data, &canonicalPayload); err != nil {
		t.Fatalf("decode canonical credential: %v", err)
	}
	if canonicalPayload.KeyID != "CANONICAL" {
		t.Fatalf("canonical credential was mutated: %+v", canonicalPayload)
	}
	if _, err := legacyKr.Get(keyringKey("  spaced  ")); err != nil {
		t.Fatalf("raw legacy credential was removed: %v", err)
	}
}

func TestStoreCredentials_RejectsDistinctThirdNormalizedSpellingBeforeMutation(t *testing.T) {
	newKr, legacyKr := withSeparateKeyrings(t)
	keyDir := t.TempDir()
	canonicalPath := filepath.Join(keyDir, "Canonical.p8")
	writeECDSAPEM(t, canonicalPath, 0o600, true)
	thirdPath := filepath.Join(keyDir, "Third.p8")
	writeECDSAPEM(t, thirdPath, 0o600, true)
	incomingPath := filepath.Join(keyDir, "Incoming.p8")
	writeECDSAPEM(t, incomingPath, 0o600, true)
	storeCredentialInKeyring(t, newKr, "spaced", "CANONICAL", "ISSUER-CANONICAL", canonicalPath)
	storeCredentialInKeyring(t, legacyKr, "\tspaced ", "THIRD", "ISSUER-THIRD", thirdPath)

	err := StoreCredentialsWithKeyType("  spaced  ", "INCOMING", "ISSUER-INCOMING", incomingPath, config.CredentialKeyTypeTeam)
	if err == nil || !strings.Contains(err.Error(), "conflicts with existing normalized profile") {
		t.Fatalf("StoreCredentials() error = %v, want normalized profile collision", err)
	}

	canonical, err := newKr.Get(keyringKey("spaced"))
	if err != nil {
		t.Fatalf("canonical credential error: %v", err)
	}
	var canonicalPayload credentialPayload
	if err := json.Unmarshal(canonical.Data, &canonicalPayload); err != nil {
		t.Fatalf("decode canonical credential: %v", err)
	}
	if canonicalPayload.KeyID != "CANONICAL" {
		t.Fatalf("canonical credential was mutated: %+v", canonicalPayload)
	}
	if _, err := legacyKr.Get(keyringKey("\tspaced ")); err != nil {
		t.Fatalf("third spelling was removed: %v", err)
	}
}

func TestStoreCredentials_CanonicalLoginRejectsDistinctNormalizedSpellingBeforeMutation(t *testing.T) {
	newKr, legacyKr := withSeparateKeyrings(t)
	keyDir := t.TempDir()
	canonicalPath := filepath.Join(keyDir, "Canonical.p8")
	writeECDSAPEM(t, canonicalPath, 0o600, true)
	rawPath := filepath.Join(keyDir, "Raw.p8")
	writeECDSAPEM(t, rawPath, 0o600, true)
	incomingPath := filepath.Join(keyDir, "Incoming.p8")
	writeECDSAPEM(t, incomingPath, 0o600, true)
	storeCredentialInKeyring(t, newKr, "spaced", "CANONICAL", "ISSUER-CANONICAL", canonicalPath)
	storeCredentialInKeyring(t, legacyKr, "  spaced  ", "RAW", "ISSUER-RAW", rawPath)

	err := StoreCredentialsWithKeyType("spaced", "INCOMING", "ISSUER-INCOMING", incomingPath, config.CredentialKeyTypeTeam)
	if err == nil || !strings.Contains(err.Error(), "conflicts with existing normalized profile") {
		t.Fatalf("StoreCredentials() error = %v, want normalized profile collision", err)
	}

	canonical, err := newKr.Get(keyringKey("spaced"))
	if err != nil {
		t.Fatalf("canonical credential error: %v", err)
	}
	var canonicalPayload credentialPayload
	if err := json.Unmarshal(canonical.Data, &canonicalPayload); err != nil {
		t.Fatalf("decode canonical credential: %v", err)
	}
	if canonicalPayload.KeyID != "CANONICAL" {
		t.Fatalf("canonical credential was mutated: %+v", canonicalPayload)
	}
	if _, err := legacyKr.Get(keyringKey("  spaced  ")); err != nil {
		t.Fatalf("raw credential was removed: %v", err)
	}
}

func TestStoreCredentials_UnusableCurrentSpellingDoesNotHideUsableLegacyCollision(t *testing.T) {
	newKr, legacyKr := withSeparateKeyrings(t)
	keyDir := t.TempDir()
	canonicalPath := filepath.Join(keyDir, "Canonical.p8")
	writeECDSAPEM(t, canonicalPath, 0o600, true)
	legacyPath := filepath.Join(keyDir, "Legacy.p8")
	writeECDSAPEM(t, legacyPath, 0o600, true)
	incomingPath := filepath.Join(keyDir, "Incoming.p8")
	writeECDSAPEM(t, incomingPath, 0o600, true)
	storeCredentialInKeyring(t, newKr, "spaced", "CANONICAL", "ISSUER-CANONICAL", canonicalPath)
	if err := newKr.Set(keyring.Item{Key: keyringKey("  spaced  "), Data: []byte("not-json")}); err != nil {
		t.Fatalf("seed unusable current credential: %v", err)
	}
	storeCredentialInKeyring(t, legacyKr, "  spaced  ", "LEGACY", "ISSUER-LEGACY", legacyPath)

	err := StoreCredentialsWithKeyType("spaced", "INCOMING", "ISSUER-INCOMING", incomingPath, config.CredentialKeyTypeTeam)
	if err == nil || !strings.Contains(err.Error(), "conflicts with existing normalized profile") {
		t.Fatalf("StoreCredentials() error = %v, want normalized profile collision", err)
	}
	if _, err := legacyKr.Get(keyringKey("  spaced  ")); err != nil {
		t.Fatalf("usable legacy credential was removed: %v", err)
	}
}

func TestStoreCredentials_IndividualCollisionIgnoresStaleIssuer(t *testing.T) {
	newKr, legacyKr := withSeparateKeyrings(t)
	keyPath := filepath.Join(t.TempDir(), "Individual.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)
	privateKeyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read private key: %v", err)
	}
	for kr, entry := range map[keyring.Keyring]struct {
		name     string
		issuerID string
	}{
		newKr:    {name: "spaced", issuerID: "STALE-ISSUER"},
		legacyKr: {name: "  spaced  ", issuerID: ""},
	} {
		item, err := keyringItemForCredential(entry.name, credentialPayload{
			KeyID:          "INDIVIDUAL-KEY",
			IssuerID:       entry.issuerID,
			PrivateKeyPath: keyPath,
			PrivateKeyPEM:  string(privateKeyPEM),
			KeyType:        config.CredentialKeyTypeIndividual,
		})
		if err != nil {
			t.Fatalf("credential item: %v", err)
		}
		if err := kr.Set(item); err != nil {
			t.Fatalf("seed credential %q: %v", entry.name, err)
		}
	}

	if err := StoreCredentialsWithKeyType(
		"  spaced  ",
		"INDIVIDUAL-KEY",
		"",
		keyPath,
		config.CredentialKeyTypeIndividual,
	); err != nil {
		t.Fatalf("StoreCredentialsWithKeyType() error: %v", err)
	}
	if _, err := legacyKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("equivalent individual credential error = %v, want ErrKeyNotFound", err)
	}
}

func TestStoreCredentials_RetryCompletesPartialLegacyCleanup(t *testing.T) {
	newKr, legacyKr := withSeparateKeyrings(t)
	keyDir := t.TempDir()
	oldPath := filepath.Join(keyDir, "OldAuthKey.p8")
	writeECDSAPEM(t, oldPath, 0o600, true)
	newPath := filepath.Join(keyDir, "AuthKey.p8")
	writeECDSAPEM(t, newPath, 0o600, true)
	storeCredentialInKeyring(t, legacyKr, "  spaced  ", "OLDKEY", "OLDISSUER", oldPath)

	previousLegacy := legacyKeyringOpener
	transientLegacy := &transientRemoveFailingKeyring{
		inner:             legacyKr,
		remainingFailures: 1,
		err:               errors.New("legacy keyring locked"),
	}
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		return transientLegacy, nil
	}
	t.Cleanup(func() { legacyKeyringOpener = previousLegacy })

	firstErr := StoreCredentialsWithKeyType("  spaced  ", "KEY123", "ISS456", newPath, config.CredentialKeyTypeTeam)
	if firstErr == nil || !strings.Contains(firstErr.Error(), "legacy keyring locked") {
		t.Fatalf("first StoreCredentials() error = %v, want legacy cleanup failure", firstErr)
	}
	if _, err := newKr.Get(keyringKey("spaced")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("canonical credential after rollback error = %v, want ErrKeyNotFound", err)
	}
	if _, err := legacyKr.Get(keyringKey("  spaced  ")); err != nil {
		t.Fatalf("legacy credential should remain after failed removal: %v", err)
	}

	if err := StoreCredentialsWithKeyType("  spaced  ", "KEY123", "ISS456", newPath, config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("retry StoreCredentials() error = %v", err)
	}
	if _, err := legacyKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("legacy credential after retry error = %v, want ErrKeyNotFound", err)
	}
	defaultCreds, err := GetCredentials("")
	if err != nil {
		t.Fatalf("GetCredentials(default) error = %v", err)
	}
	if defaultCreds.KeyID != "KEY123" || defaultCreds.IssuerID != "ISS456" {
		t.Fatalf("default credentials after retry = %+v", defaultCreds)
	}
}

func TestStoreCredentials_RestoresEarlierVariantsWhenLaterCleanupFails(t *testing.T) {
	newKr, legacyKr := withSeparateKeyrings(t)
	keyDir := t.TempDir()
	oldPath := filepath.Join(keyDir, "OldAuthKey.p8")
	writeECDSAPEM(t, oldPath, 0o600, true)
	newPath := filepath.Join(keyDir, "AuthKey.p8")
	writeECDSAPEM(t, newPath, 0o600, true)
	storeCredentialInKeyring(t, legacyKr, "  spaced", "OLDKEY", "OLDISSUER", oldPath)
	storeCredentialInKeyring(t, legacyKr, " spaced ", "OLDKEY", "OLDISSUER", oldPath)

	previousLegacy := legacyKeyringOpener
	failingLegacy := &nthRemoveFailingKeyring{
		inner:      legacyKr,
		failOnCall: 2,
		err:        errors.New("legacy keyring locked"),
	}
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		return failingLegacy, nil
	}
	t.Cleanup(func() { legacyKeyringOpener = previousLegacy })

	err := StoreCredentialsWithKeyType("spaced", "KEY123", "ISS456", newPath, config.CredentialKeyTypeTeam)
	if err == nil || !strings.Contains(err.Error(), "legacy keyring locked") {
		t.Fatalf("StoreCredentials() error = %v, want legacy cleanup failure", err)
	}
	if _, err := newKr.Get(keyringKey("spaced")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("canonical credential after rollback error = %v, want ErrKeyNotFound", err)
	}
	for _, name := range []string{"  spaced", " spaced "} {
		if _, err := legacyKr.Get(keyringKey(name)); err != nil {
			t.Fatalf("legacy credential %q should be restored after failed cleanup: %v", name, err)
		}
	}
}

func TestStoreCredentials_IgnoresPathOnlyCollisionWithInsecurePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce Unix private-key permission bits")
	}
	newKr, legacyKr := withSeparateKeyrings(t)
	keyDir := t.TempDir()
	canonicalPath := filepath.Join(keyDir, "Canonical.p8")
	writeECDSAPEM(t, canonicalPath, 0o600, true)
	insecurePath := filepath.Join(keyDir, "Insecure.p8")
	writeECDSAPEM(t, insecurePath, 0o644, true)
	incomingPath := filepath.Join(keyDir, "Incoming.p8")
	writeECDSAPEM(t, incomingPath, 0o600, true)
	storeCredentialInKeyring(t, newKr, "spaced", "CANONICAL", "ISSUER-CANONICAL", canonicalPath)
	storeCredentialInKeyring(t, legacyKr, "  spaced  ", "INSECURE", "ISSUER-INSECURE", insecurePath)

	if err := StoreCredentialsWithKeyType("  spaced  ", "INCOMING", "ISSUER-INCOMING", incomingPath, config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}
	if _, err := legacyKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("unusable legacy credential error = %v, want ErrKeyNotFound", err)
	}
}

func TestStoreCredentials_NormalizedCollisionUsesCurrentKeyringPrecedence(t *testing.T) {
	newKr, legacyKr := withSeparateKeyrings(t)
	storeCredentialInKeyring(t, newKr, "spaced", "KEY123", "ISS456", "/tmp/AuthKey.p8")
	storeCredentialInKeyring(t, newKr, "  spaced  ", "KEY123", "ISS456", "/tmp/AuthKey.p8")
	storeCredentialInKeyring(t, legacyKr, "spaced", "STALE", "STALE-ISSUER", "/tmp/Stale.p8")

	if err := StoreCredentialsWithKeyType("  spaced  ", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error = %v", err)
	}
	if _, err := newKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("raw current credential error = %v, want ErrKeyNotFound", err)
	}
}

func TestStoreCredentials_NormalizedPreflightPreservesUnavailableKeyringFallback(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "0")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	previousCurrent := keyringOpener
	previousLegacy := legacyKeyringOpener
	keyringOpener = func() (keyring.Keyring, error) {
		return failingKeyring{err: keyring.ErrNoAvailImpl}, nil
	}
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	}
	t.Cleanup(func() {
		keyringOpener = previousCurrent
		legacyKeyringOpener = previousLegacy
	})

	if err := StoreCredentialsWithKeyType("  spaced  ", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error = %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if len(cfg.Keys) != 1 || cfg.Keys[0].Name != "spaced" || cfg.Keys[0].KeyID != "KEY123" {
		t.Fatalf("fallback credentials = %+v", cfg.Keys)
	}
}

func TestStoreCredentials_UnavailablePreflightDoesNotResumeKeychainMutation(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "0")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	keyDir := t.TempDir()
	existingPath := filepath.Join(keyDir, "Existing.p8")
	writeECDSAPEM(t, existingPath, 0o600, true)
	incomingPath := filepath.Join(keyDir, "Incoming.p8")
	writeECDSAPEM(t, incomingPath, 0o600, true)

	inner := keyring.NewArrayKeyring([]keyring.Item{})
	storeCredentialInKeyring(t, inner, "spaced", "EXISTING", "EXISTING-ISSUER", existingPath)
	storeCredentialInKeyring(t, inner, "  spaced  ", "CONFLICT", "CONFLICT-ISSUER", existingPath)
	flaky := &transientKeysFailingKeyring{
		inner:             inner,
		remainingFailures: 1,
		err:               keyring.ErrNoAvailImpl,
	}
	previousCurrent := keyringOpener
	previousLegacy := legacyKeyringOpener
	keyringOpener = func() (keyring.Keyring, error) { return flaky, nil }
	legacyKeyringOpener = func() (keyring.Keyring, error) { return nil, keyring.ErrNoAvailImpl }
	t.Cleanup(func() {
		keyringOpener = previousCurrent
		legacyKeyringOpener = previousLegacy
	})

	if err := StoreCredentialsWithKeyType("  spaced  ", "INCOMING", "INCOMING-ISSUER", incomingPath, config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}
	for _, name := range []string{"spaced", "  spaced  "} {
		if _, err := inner.Get(keyringKey(name)); err != nil {
			t.Fatalf("keychain credential %q should remain after unavailable preflight: %v", name, err)
		}
	}
	item, err := inner.Get(keyringKey("spaced"))
	if err != nil {
		t.Fatalf("get canonical keychain credential: %v", err)
	}
	var stored credentialPayload
	if err := json.Unmarshal(item.Data, &stored); err != nil {
		t.Fatalf("decode canonical keychain credential: %v", err)
	}
	if stored.KeyID != "EXISTING" {
		t.Fatalf("canonical keychain KeyID = %q, want EXISTING", stored.KeyID)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if len(cfg.Keys) != 1 || cfg.Keys[0].Name != "spaced" || cfg.Keys[0].KeyID != "INCOMING" {
		t.Fatalf("fallback credentials = %+v", cfg.Keys)
	}
}

func TestStoreCredentials_LegacyUnavailableDoesNotBypassCurrentKeychain(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "0")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	keyDir := t.TempDir()
	existingPath := filepath.Join(keyDir, "Existing.p8")
	writeECDSAPEM(t, existingPath, 0o600, true)
	incomingPath := filepath.Join(keyDir, "Incoming.p8")
	writeECDSAPEM(t, incomingPath, 0o600, true)

	current := keyring.NewArrayKeyring([]keyring.Item{})
	storeCredentialInKeyring(t, current, "spaced", "EXISTING", "EXISTING-ISSUER", existingPath)
	previousCurrent := keyringOpener
	previousLegacy := legacyKeyringOpener
	keyringOpener = func() (keyring.Keyring, error) { return current, nil }
	legacyKeyringOpener = func() (keyring.Keyring, error) { return nil, keyring.ErrNoAvailImpl }
	t.Cleanup(func() {
		keyringOpener = previousCurrent
		legacyKeyringOpener = previousLegacy
	})

	if err := StoreCredentialsWithKeyType("spaced", "INCOMING", "INCOMING-ISSUER", incomingPath, config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}
	item, err := current.Get(keyringKey("spaced"))
	if err != nil {
		t.Fatalf("get current keychain credential: %v", err)
	}
	var stored credentialPayload
	if err := json.Unmarshal(item.Data, &stored); err != nil {
		t.Fatalf("decode current keychain credential: %v", err)
	}
	if stored.KeyID != "INCOMING" {
		t.Fatalf("current keychain KeyID = %q, want INCOMING", stored.KeyID)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}
	if len(cfg.Keys) != 0 {
		t.Fatalf("config credentials = %+v, want none", cfg.Keys)
	}
}

func TestStoreCredentials_ReplacesMalformedPreNormalizedEntry(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	if err := newKr.Set(keyring.Item{Key: keyringKey("  spaced  "), Data: []byte("not-json")}); err != nil {
		t.Fatalf("seed malformed keyring entry: %v", err)
	}

	if err := StoreCredentialsWithKeyType("  spaced  ", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error = %v", err)
	}
	if _, err := newKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("malformed raw credential error = %v, want ErrKeyNotFound", err)
	}
	item, err := newKr.Get(keyringKey("spaced"))
	if err != nil {
		t.Fatalf("normalized credential error = %v", err)
	}
	var payload credentialPayload
	if err := json.Unmarshal(item.Data, &payload); err != nil {
		t.Fatalf("decode normalized credential: %v", err)
	}
	if payload.KeyID != "KEY123" {
		t.Fatalf("normalized credential = %+v", payload)
	}
}

func TestStoreCredentials_ReplacesEmptyDecodedPreNormalizedEntry(t *testing.T) {
	for _, data := range []string{"null", "{}"} {
		t.Run(data, func(t *testing.T) {
			newKr, _ := withSeparateKeyrings(t)
			storeCredentialInKeyring(t, newKr, "spaced", "OLD", "OLD-ISSUER", "/tmp/Old.p8")
			if err := newKr.Set(keyring.Item{Key: keyringKey("  spaced  "), Data: []byte(data)}); err != nil {
				t.Fatalf("seed empty keyring entry: %v", err)
			}

			if err := StoreCredentialsWithKeyType("  spaced  ", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
				t.Fatalf("StoreCredentials() error = %v", err)
			}
			if _, err := newKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
				t.Fatalf("empty raw credential error = %v, want ErrKeyNotFound", err)
			}
			item, err := newKr.Get(keyringKey("spaced"))
			if err != nil {
				t.Fatalf("normalized credential error = %v", err)
			}
			var payload credentialPayload
			if err := json.Unmarshal(item.Data, &payload); err != nil {
				t.Fatalf("decode normalized credential: %v", err)
			}
			if payload.KeyID != "KEY123" {
				t.Fatalf("normalized credential = %+v", payload)
			}
		})
	}
}

func TestStoreCredentials_ReplacesIncompletePreNormalizedEntry(t *testing.T) {
	for _, data := range []string{
		`{"key_id":"BROKEN"}`,
		`{"key_id":"BROKEN","issuer_id":"ISSUER"}`,
		`{"key_id":"BROKEN","private_key_path":"/tmp/Broken.p8"}`,
		`{"key_id":"BROKEN","issuer_id":"ISSUER","private_key_path":"   "}`,
		`{"key_id":"BROKEN","issuer_id":"ISSUER","private_key_path":"/tmp/Broken.p8","key_type":"unsupported"}`,
	} {
		t.Run(data, func(t *testing.T) {
			newKr, _ := withSeparateKeyrings(t)
			storeCredentialInKeyring(t, newKr, "spaced", "OLD", "OLD-ISSUER", "/tmp/Old.p8")
			if err := newKr.Set(keyring.Item{Key: keyringKey("  spaced  "), Data: []byte(data)}); err != nil {
				t.Fatalf("seed incomplete keyring entry: %v", err)
			}

			if err := StoreCredentialsWithKeyType("  spaced  ", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
				t.Fatalf("StoreCredentials() error = %v", err)
			}
			if _, err := newKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
				t.Fatalf("incomplete raw credential error = %v, want ErrKeyNotFound", err)
			}
			item, err := newKr.Get(keyringKey("spaced"))
			if err != nil {
				t.Fatalf("normalized credential error = %v", err)
			}
			var payload credentialPayload
			if err := json.Unmarshal(item.Data, &payload); err != nil {
				t.Fatalf("decode normalized credential: %v", err)
			}
			if payload.KeyID != "KEY123" {
				t.Fatalf("normalized credential = %+v", payload)
			}
		})
	}
}

func TestStoreCredentials_ReplacesPreNormalizedEntryWithInvalidEmbeddedKey(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	storeCredentialInKeyring(t, newKr, "spaced", "OLD", "OLD-ISSUER", "/tmp/Old.p8")
	invalid := credentialPayload{
		KeyID:         "BROKEN",
		IssuerID:      "BROKEN-ISSUER",
		PrivateKeyPEM: "not-pem",
		KeyType:       config.CredentialKeyTypeTeam,
	}
	data, err := json.Marshal(invalid)
	if err != nil {
		t.Fatalf("encode invalid embedded-key credential: %v", err)
	}
	if err := newKr.Set(keyring.Item{Key: keyringKey("  spaced  "), Data: data}); err != nil {
		t.Fatalf("seed invalid embedded-key credential: %v", err)
	}

	if err := StoreCredentialsWithKeyType("  spaced  ", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}
	if _, err := newKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("invalid raw credential error = %v, want ErrKeyNotFound", err)
	}
	item, err := newKr.Get(keyringKey("spaced"))
	if err != nil {
		t.Fatalf("normalized credential error: %v", err)
	}
	var payload credentialPayload
	if err := json.Unmarshal(item.Data, &payload); err != nil {
		t.Fatalf("decode normalized credential: %v", err)
	}
	if payload.KeyID != "KEY123" {
		t.Fatalf("normalized credential = %+v", payload)
	}
}

func TestStoreCredentials_ReplacesCanonicalEntryWithValidPathAndInvalidEmbeddedKey(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	canonicalPath := filepath.Join(t.TempDir(), "Canonical.p8")
	writeECDSAPEM(t, canonicalPath, 0o600, true)
	// Credential resolution prefers the embedded PEM over the key path, so
	// this canonical entry is unusable at auth time despite its valid path
	// and must not be collision-authoritative.
	broken := credentialPayload{
		KeyID:          "BROKEN",
		IssuerID:       "BROKEN-ISSUER",
		PrivateKeyPath: canonicalPath,
		PrivateKeyPEM:  "not-pem",
		KeyType:        config.CredentialKeyTypeTeam,
	}
	data, err := json.Marshal(broken)
	if err != nil {
		t.Fatalf("encode invalid embedded-key credential: %v", err)
	}
	if err := newKr.Set(keyring.Item{Key: keyringKey("spaced"), Data: data}); err != nil {
		t.Fatalf("seed invalid embedded-key credential: %v", err)
	}
	storeCredentialInKeyring(t, newKr, "  spaced  ", "RAW", "ISSUER-RAW", "/tmp/raw.p8")

	if err := StoreCredentialsWithKeyType("  spaced  ", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}
	if _, err := newKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("raw credential error = %v, want ErrKeyNotFound", err)
	}
	item, err := newKr.Get(keyringKey("spaced"))
	if err != nil {
		t.Fatalf("normalized credential error: %v", err)
	}
	var payload credentialPayload
	if err := json.Unmarshal(item.Data, &payload); err != nil {
		t.Fatalf("decode normalized credential: %v", err)
	}
	if payload.KeyID != "KEY123" {
		t.Fatalf("normalized credential = %+v", payload)
	}
}

func TestStoreCredentials_ReplacesRawEntryWithUnloadableKeyPath(t *testing.T) {
	newKr, legacyKr := withSeparateKeyrings(t)
	canonicalPath := filepath.Join(t.TempDir(), "Canonical.p8")
	writeECDSAPEM(t, canonicalPath, 0o600, true)
	storeCredentialInKeyring(t, newKr, "spaced", "CANONICAL", "ISSUER-CANONICAL", canonicalPath)
	// The raw entry has complete IDs but its key file was deleted, so
	// credential resolution cannot authenticate with it; it must not stay
	// collision-authoritative and block a padded-name login.
	storeCredentialInKeyring(t, legacyKr, "  spaced  ", "RAW", "ISSUER-RAW", filepath.Join(t.TempDir(), "Deleted.p8"))

	if err := StoreCredentialsWithKeyType("  spaced  ", "NEW", "ISSUER-NEW", "/tmp/new.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}
	if _, err := legacyKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("legacy raw credential error = %v, want ErrKeyNotFound", err)
	}
	item, err := newKr.Get(keyringKey("spaced"))
	if err != nil {
		t.Fatalf("normalized credential error: %v", err)
	}
	var payload credentialPayload
	if err := json.Unmarshal(item.Data, &payload); err != nil {
		t.Fatalf("decode normalized credential: %v", err)
	}
	if payload.KeyID != "NEW" {
		t.Fatalf("normalized credential = %+v", payload)
	}
}

func TestStoreCredentials_CanonicalNameLoginCleansOrphanedPaddedEntries(t *testing.T) {
	newKr, legacyKr := withSeparateKeyrings(t)
	storeCredentialInKeyring(t, newKr, "spaced", "CANONICAL", "ISSUER-CANONICAL", "/tmp/canonical.p8")
	storeCredentialInKeyring(t, newKr, "  spaced  ", "RAW", "ISSUER-RAW", "/tmp/raw.p8")
	storeCredentialInKeyring(t, legacyKr, "  spaced  ", "RAW", "ISSUER-RAW", "/tmp/raw.p8")
	storeCredentialInKeyring(t, legacyKr, "\tspaced ", "RAW-LEGACY", "ISSUER-RAW-LEGACY", "/tmp/raw-legacy.p8")

	// Logging in with the canonical name is the advertised collision
	// remediation, so it must also clear orphaned pre-normalized entries
	// that RemoveCredentials cannot target.
	if err := StoreCredentialsWithKeyType("spaced", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}
	if _, err := newKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("raw credential error = %v, want ErrKeyNotFound", err)
	}
	if _, err := legacyKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("legacy raw credential error = %v, want ErrKeyNotFound", err)
	}
	if _, err := legacyKr.Get(keyringKey("\tspaced ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("legacy tabbed credential error = %v, want ErrKeyNotFound", err)
	}
	item, err := newKr.Get(keyringKey("spaced"))
	if err != nil {
		t.Fatalf("normalized credential error: %v", err)
	}
	var payload credentialPayload
	if err := json.Unmarshal(item.Data, &payload); err != nil {
		t.Fatalf("decode normalized credential: %v", err)
	}
	if payload.KeyID != "KEY123" {
		t.Fatalf("normalized credential = %+v", payload)
	}
}

func TestStoreCredentials_CleansRawEntryWhenCanonicalLacksPEMEnrichment(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	if err := os.WriteFile(keyPath, []byte("PRIVATE KEY"), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	storeCredentialInKeyring(t, newKr, "spaced", "KEY123", "ISS456", keyPath)
	storeCredentialInKeyring(t, newKr, "  spaced  ", "OTHER", "OTHER-ISSUER", "/tmp/Other.p8")

	if err := StoreCredentialsWithKeyType("  spaced  ", "KEY123", "ISS456", keyPath, config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error = %v", err)
	}
	if _, err := newKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("raw credential error = %v, want ErrKeyNotFound", err)
	}
	item, err := newKr.Get(keyringKey("spaced"))
	if err != nil {
		t.Fatalf("normalized credential error = %v", err)
	}
	var payload credentialPayload
	if err := json.Unmarshal(item.Data, &payload); err != nil {
		t.Fatalf("decode normalized credential: %v", err)
	}
	if payload.PrivateKeyPEM != "PRIVATE KEY" {
		t.Fatalf("normalized credential PEM = %q, want enrichment", payload.PrivateKeyPEM)
	}
}

func TestStoreCredentials_CleansRawEntryWhenCanonicalKeyWasRelocated(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	keyDir := t.TempDir()
	canonicalPath := filepath.Join(keyDir, "Original.p8")
	relocatedPath := filepath.Join(keyDir, "Relocated.p8")
	writeECDSAPEM(t, canonicalPath, 0o600, true)
	canonicalPEM, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatalf("read canonical private key: %v", err)
	}
	relocatedPEM := append(append([]byte(nil), canonicalPEM...), '\n')
	if err := os.WriteFile(relocatedPath, relocatedPEM, 0o600); err != nil {
		t.Fatalf("write relocated private key: %v", err)
	}
	storeCredentialInKeyring(t, newKr, "spaced", "KEY123", "ISS456", canonicalPath)
	storeCredentialInKeyring(t, newKr, "  spaced  ", "OTHER", "OTHER-ISSUER", "/tmp/Other.p8")

	if err := StoreCredentialsWithKeyType("  spaced  ", "KEY123", "ISS456", relocatedPath, config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error = %v", err)
	}
	if _, err := newKr.Get(keyringKey("  spaced  ")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("raw credential error = %v, want ErrKeyNotFound", err)
	}
	item, err := newKr.Get(keyringKey("spaced"))
	if err != nil {
		t.Fatalf("normalized credential error = %v", err)
	}
	var payload credentialPayload
	if err := json.Unmarshal(item.Data, &payload); err != nil {
		t.Fatalf("decode normalized credential: %v", err)
	}
	if payload.PrivateKeyPath != relocatedPath || payload.PrivateKeyPEM != string(relocatedPEM) {
		t.Fatalf("normalized credential = %+v, want relocated key enrichment", payload)
	}
}

func TestStoreCredentials_RejectsWhitespaceOnlyProfileName(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)

	err := StoreCredentialsWithKeyType("   ", "KEY123", "ISS456", "/tmp/AuthKey.p8", config.CredentialKeyTypeTeam)
	if err == nil || err.Error() != "credential name is required" {
		t.Fatalf("StoreCredentials() error = %v, want credential name is required", err)
	}
	items, keysErr := newKr.Keys()
	if keysErr != nil {
		t.Fatalf("keychain Keys() error: %v", keysErr)
	}
	if len(items) != 0 {
		t.Fatalf("keychain items = %q, want none", items)
	}
}

func TestStoreCredentials_RemovesStaleGlobalCredentialWhenLocalConfigActive(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "0")
	// TestMain sets ASC_CONFIG_PATH; clear it so the local config is active.
	t.Setenv("ASC_CONFIG_PATH", "")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	workDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(filepath.Join(workDir, ".asc"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	localPath := filepath.Join(workDir, ".asc", "config.json")
	globalPath := filepath.Join(homeDir, ".asc", "config.json")

	localCfg := &config.Config{
		DefaultKeyName: "local-only",
		Keys: []config.Credential{
			{
				Name:           "local-only",
				KeyID:          "LOCAL_KEY",
				IssuerID:       "LOCAL_ISSUER",
				PrivateKeyPath: "/tmp/local.p8",
			},
		},
	}
	if err := config.SaveAt(localPath, localCfg); err != nil {
		t.Fatalf("SaveAt(local) error: %v", err)
	}

	globalCfg := &config.Config{
		DefaultKeyName: "stale",
		Keys: []config.Credential{
			{
				Name:           "stale",
				KeyID:          "STALE_KEY",
				IssuerID:       "STALE_ISSUER",
				PrivateKeyPath: "/tmp/stale.p8",
			},
			{
				Name:           "keep-global",
				KeyID:          "KEEP_KEY",
				IssuerID:       "KEEP_ISSUER",
				PrivateKeyPath: "/tmp/keep.p8",
			},
		},
	}
	if err := config.SaveAt(globalPath, globalCfg); err != nil {
		t.Fatalf("SaveAt(global) error: %v", err)
	}

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

	previousKeyringOpener := keyringOpener
	kr := keyring.NewArrayKeyring([]keyring.Item{})
	keyringOpener = func() (keyring.Keyring, error) {
		return kr, nil
	}
	t.Cleanup(func() {
		keyringOpener = previousKeyringOpener
	})

	if err := StoreCredentialsWithKeyType("stale", "NEW_KEY", "NEW_ISSUER", "/tmp/new.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}

	loadedLocal, err := config.LoadAt(localPath)
	if err != nil {
		t.Fatalf("LoadAt(local) error: %v", err)
	}
	if len(loadedLocal.Keys) != 1 || loadedLocal.Keys[0].Name != "local-only" {
		t.Fatalf("expected local config credential to remain unchanged, got %+v", loadedLocal.Keys)
	}

	loadedGlobal, err := config.LoadAt(globalPath)
	if err != nil {
		t.Fatalf("LoadAt(global) error: %v", err)
	}
	if len(loadedGlobal.Keys) != 1 {
		t.Fatalf("expected only one global credential after cleanup, got %d", len(loadedGlobal.Keys))
	}
	if loadedGlobal.Keys[0].Name != "keep-global" {
		t.Fatalf("expected non-target global credential to be preserved, got %q", loadedGlobal.Keys[0].Name)
	}
	if loadedGlobal.Keys[0].KeyID != "KEEP_KEY" || loadedGlobal.Keys[0].IssuerID != "KEEP_ISSUER" {
		t.Fatalf("expected preserved global credential integrity, got %+v", loadedGlobal.Keys[0])
	}
}

// Before 5.9.1 a keychain login also removed the replaced profile from
// ~/.asc/config.json while ASC_CONFIG_PATH selected another file (#530). The
// cleanup is now scoped to the active config file; `asc auth logout
// --include-global` keeps the cross-file cleanup available.
func TestStoreCredentials_RemovesStaleCredentialFromOverrideConfigOnly(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "0")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	overridePath := filepath.Join(t.TempDir(), "custom", "config.json")
	t.Setenv("ASC_CONFIG_PATH", overridePath)

	globalPath := filepath.Join(homeDir, ".asc", "config.json")

	overrideCfg := &config.Config{
		DefaultKeyName: "stale",
		Keys: []config.Credential{
			{
				Name:           "stale",
				KeyID:          "OVERRIDE_STALE",
				IssuerID:       "OVERRIDE_STALE_ISSUER",
				PrivateKeyPath: "/tmp/override-stale.p8",
			},
			{
				Name:           "keep-override",
				KeyID:          "OVERRIDE_KEEP",
				IssuerID:       "OVERRIDE_KEEP_ISSUER",
				PrivateKeyPath: "/tmp/override-keep.p8",
			},
		},
	}
	if err := config.SaveAt(overridePath, overrideCfg); err != nil {
		t.Fatalf("SaveAt(override) error: %v", err)
	}

	globalCfg := &config.Config{
		DefaultKeyName: "stale",
		Keys: []config.Credential{
			{
				Name:           "stale",
				KeyID:          "GLOBAL_STALE",
				IssuerID:       "GLOBAL_STALE_ISSUER",
				PrivateKeyPath: "/tmp/global-stale.p8",
			},
			{
				Name:           "keep-global",
				KeyID:          "GLOBAL_KEEP",
				IssuerID:       "GLOBAL_KEEP_ISSUER",
				PrivateKeyPath: "/tmp/global-keep.p8",
			},
		},
	}
	if err := config.SaveAt(globalPath, globalCfg); err != nil {
		t.Fatalf("SaveAt(global) error: %v", err)
	}
	globalBefore, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("ReadFile(global) error: %v", err)
	}

	previousKeyringOpener := keyringOpener
	kr := keyring.NewArrayKeyring([]keyring.Item{})
	keyringOpener = func() (keyring.Keyring, error) {
		return kr, nil
	}
	t.Cleanup(func() {
		keyringOpener = previousKeyringOpener
	})

	if err := StoreCredentialsWithKeyType("stale", "NEW_KEY", "NEW_ISSUER", "/tmp/new.p8", config.CredentialKeyTypeTeam); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}

	loadedOverride, err := config.LoadAt(overridePath)
	if err != nil {
		t.Fatalf("LoadAt(override) error: %v", err)
	}
	if len(loadedOverride.Keys) != 1 || loadedOverride.Keys[0].Name != "keep-override" {
		t.Fatalf("expected override config to keep non-target credential, got %+v", loadedOverride.Keys)
	}

	globalAfter, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("ReadFile(global) error: %v", err)
	}
	if !bytes.Equal(globalAfter, globalBefore) {
		t.Fatalf("expected global config to stay untouched while ASC_CONFIG_PATH is set:\nbefore: %s\nafter: %s", globalBefore, globalAfter)
	}
}

func TestListCredentials_MigratesLegacyEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	newKr, legacyKr := withSeparateKeyrings(t)

	storeCredentialInKeyring(t, newKr, "new-key", "NEW123", "ISSNEW", "/tmp/new.p8")
	storeCredentialInKeyring(t, legacyKr, "legacy-key", "OLD123", "ISSOLD", "/tmp/old.p8")

	creds, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(creds))
	}

	if _, err := legacyKr.Get(keyringKey("legacy-key")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("expected legacy credential to be removed, got %v", err)
	}
	if _, err := newKr.Get(keyringKey("legacy-key")); err != nil {
		t.Fatalf("expected legacy credential to be migrated, got %v", err)
	}
}

func TestListCredentials_RemovesLegacyDuplicates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	newKr, legacyKr := withSeparateKeyrings(t)

	storeCredentialInKeyring(t, newKr, "shared-key", "NEW123", "ISSNEW", "/tmp/new.p8")
	storeCredentialInKeyring(t, legacyKr, "shared-key", "OLD123", "ISSOLD", "/tmp/old.p8")

	creds, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}

	if _, err := legacyKr.Get(keyringKey("shared-key")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("expected legacy duplicate to be removed, got %v", err)
	}
}

func TestRemoveCredentials_FallsBackToLegacy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, legacyKr := withSeparateKeyrings(t)

	storeCredentialInKeyring(t, legacyKr, "legacy-only", "OLD123", "ISSOLD", "/tmp/old.p8")

	if err := RemoveCredentialsWithOptions("legacy-only", RemoveOptions{}); err != nil {
		t.Fatalf("RemoveCredentials() error: %v", err)
	}
	if _, err := legacyKr.Get(keyringKey("legacy-only")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("expected legacy credential to be removed, got %v", err)
	}
}

func TestRemoveCredentials_TrimsName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	newKr, _ := withSeparateKeyrings(t)

	storeCredentialInKeyring(t, newKr, "trim-key", "KEY123", "ISS456", "/tmp/AuthKey.p8")

	if err := RemoveCredentialsWithOptions("  trim-key  ", RemoveOptions{}); err != nil {
		t.Fatalf("RemoveCredentials() error: %v", err)
	}
	if _, err := newKr.Get(keyringKey("trim-key")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("expected credential to be removed, got %v", err)
	}
}

func TestRemoveCredentials_RemovesAllNormalizedSpellings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	newKr, legacyKr := withSeparateKeyrings(t)

	storeCredentialInKeyring(t, newKr, "trim-key", "KEY123", "ISS456", "/tmp/AuthKey.p8")
	storeCredentialInKeyring(t, newKr, "  trim-key  ", "OTHER", "OTHER-ISSUER", "/tmp/Other.p8")
	storeCredentialInKeyring(t, legacyKr, "\ttrim-key ", "LEGACY", "LEGACY-ISSUER", "/tmp/Legacy.p8")

	if err := RemoveCredentialsWithOptions("trim-key", RemoveOptions{}); err != nil {
		t.Fatalf("RemoveCredentials() error: %v", err)
	}
	for _, check := range []struct {
		kr   keyring.Keyring
		name string
	}{
		{kr: newKr, name: "trim-key"},
		{kr: newKr, name: "  trim-key  "},
		{kr: legacyKr, name: "\ttrim-key "},
	} {
		if _, err := check.kr.Get(keyringKey(check.name)); !errors.Is(err, keyring.ErrKeyNotFound) {
			t.Fatalf("credential %q error = %v, want ErrKeyNotFound", check.name, err)
		}
	}
}

func TestRemoveCredentials_PreservesCurrentWhenLegacyListingFails(t *testing.T) {
	newKr, _ := withSeparateKeyrings(t)
	storeCredentialInKeyring(t, newKr, "trim-key", "KEY123", "ISS456", "/tmp/AuthKey.p8")

	previousLegacy := legacyKeyringOpener
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		return failingKeyring{err: errors.New("legacy keyring locked")}, nil
	}
	t.Cleanup(func() { legacyKeyringOpener = previousLegacy })

	err := RemoveCredentialsWithOptions("trim-key", RemoveOptions{})
	if err == nil || !strings.Contains(err.Error(), "legacy keyring locked") {
		t.Fatalf("RemoveCredentials() error = %v, want legacy listing failure", err)
	}
	if _, err := newKr.Get(keyringKey("trim-key")); err != nil {
		t.Fatalf("current credential should remain after legacy listing failure: %v", err)
	}
}

func TestRemoveCredentials_UnavailableCurrentDoesNotMutateOtherStores(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "0")
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	current := keyring.NewArrayKeyring([]keyring.Item{})
	legacy := keyring.NewArrayKeyring([]keyring.Item{})
	storeCredentialInKeyring(t, current, "trim-key", "CURRENT", "CURRENT-ISSUER", "/tmp/Current.p8")
	storeCredentialInKeyring(t, legacy, "trim-key", "LEGACY", "LEGACY-ISSUER", "/tmp/Legacy.p8")
	if err := config.SaveAt(configPath, &config.Config{
		DefaultKeyName: "trim-key",
		Keys: []config.Credential{{
			Name:           "trim-key",
			KeyID:          "CONFIG",
			IssuerID:       "CONFIG-ISSUER",
			PrivateKeyPath: "/tmp/Config.p8",
		}},
	}); err != nil {
		t.Fatalf("config.SaveAt() error: %v", err)
	}

	flakyCurrent := &transientKeysFailingKeyring{
		inner:             current,
		remainingFailures: 1,
		err:               keyring.ErrNoAvailImpl,
	}
	previousCurrent := keyringOpener
	previousLegacy := legacyKeyringOpener
	keyringOpener = func() (keyring.Keyring, error) { return flakyCurrent, nil }
	legacyKeyringOpener = func() (keyring.Keyring, error) { return legacy, nil }
	t.Cleanup(func() {
		keyringOpener = previousCurrent
		legacyKeyringOpener = previousLegacy
	})

	err := RemoveCredentialsWithOptions("trim-key", RemoveOptions{})
	if err == nil || !isKeyringUnavailable(err) {
		t.Fatalf("RemoveCredentials() error = %v, want keyring unavailable", err)
	}
	for source, kr := range map[string]keyring.Keyring{"current": current, "legacy": legacy} {
		if _, err := kr.Get(keyringKey("trim-key")); err != nil {
			t.Fatalf("%s credential should remain after unavailable preflight: %v", source, err)
		}
	}
	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("config.LoadAt() error: %v", err)
	}
	if len(cfg.Keys) != 1 || cfg.Keys[0].KeyID != "CONFIG" || cfg.DefaultKeyName != "trim-key" {
		t.Fatalf("config changed after unavailable preflight: %+v", cfg)
	}
}

func TestRemoveCredentials_RestoresEarlierConfigWhenLaterPathFails(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	overridePath := filepath.Join(t.TempDir(), "override", "config.json")
	t.Setenv("ASC_CONFIG_PATH", overridePath)
	if err := config.SaveAt(overridePath, &config.Config{
		DefaultKeyName: "trim-key",
		Keys: []config.Credential{{
			Name:           "trim-key",
			KeyID:          "CONFIG",
			IssuerID:       "CONFIG-ISSUER",
			PrivateKeyPath: "/tmp/Config.p8",
		}},
	}); err != nil {
		t.Fatalf("config.SaveAt(override) error: %v", err)
	}
	globalPath := filepath.Join(homeDir, ".asc", "config.json")
	if err := os.MkdirAll(globalPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(global config path) error: %v", err)
	}

	err := RemoveCredentialsWithOptions("trim-key", RemoveOptions{IncludeGlobalConfig: true})
	if err == nil {
		t.Fatal("RemoveCredentialsWithOptions() error = nil, want later config path failure")
	}
	cfg, err := config.LoadAt(overridePath)
	if err != nil {
		t.Fatalf("config.LoadAt(override) error: %v", err)
	}
	if len(cfg.Keys) != 1 || cfg.Keys[0].KeyID != "CONFIG" || cfg.DefaultKeyName != "trim-key" {
		t.Fatalf("override config changed after later path failure: %+v", cfg)
	}
}

func TestRemoveCredentials_ClearsStoredKeychainMetadata(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	newKr, _ := withSeparateKeyrings(t)

	storeCredentialInKeyring(t, newKr, "trim-key", "KEY123", "ISS456", "/tmp/AuthKey.p8")

	path, err := config.Path()
	if err != nil {
		t.Fatalf("config.Path() error: %v", err)
	}
	if err := config.SaveAt(path, &config.Config{
		KeychainMetadata: []config.KeychainMetadata{
			{Name: "trim-key", KeyID: "KEY123", IssuerID: "ISS456"},
			{Name: "keep", KeyID: "KEY999", IssuerID: "ISS999"},
		},
	}); err != nil {
		t.Fatalf("config.SaveAt() error: %v", err)
	}

	if err := RemoveCredentialsWithOptions("trim-key", RemoveOptions{}); err != nil {
		t.Fatalf("RemoveCredentials() error: %v", err)
	}

	cfg, err := config.LoadAt(path)
	if err != nil {
		t.Fatalf("config.LoadAt() error: %v", err)
	}
	if len(cfg.KeychainMetadata) != 1 {
		t.Fatalf("expected one remaining keychain metadata record, got %#v", cfg.KeychainMetadata)
	}
	if cfg.KeychainMetadata[0].Name != "keep" {
		t.Fatalf("expected keep metadata to remain, got %#v", cfg.KeychainMetadata)
	}
}

func TestRemoveCredentials_MissingReturnsErr(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	previous := keyringOpener
	previousLegacy := legacyKeyringOpener
	keyringOpener = func() (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	}
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	}
	t.Cleanup(func() {
		keyringOpener = previous
		legacyKeyringOpener = previousLegacy
	})

	cfg := &config.Config{
		DefaultKeyName: "existing",
		Keys: []config.Credential{
			{
				Name:           "existing",
				KeyID:          "KEY123",
				IssuerID:       "ISS456",
				PrivateKeyPath: "/tmp/AuthKey.p8",
			},
		},
	}
	if err := config.SaveAt(configPath, cfg); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	err := RemoveCredentialsWithOptions("missing", RemoveOptions{})
	if !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}

func writeECDSAPEM(t *testing.T, path string, mode os.FileMode, pkcs8 bool) {
	writeECDSAPEMWithCurve(t, path, mode, pkcs8, elliptic.P256())
}

func writeECDSAPEMWithCurve(t *testing.T, path string, mode os.FileMode, pkcs8 bool, curve elliptic.Curve) {
	t.Helper()

	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	var der []byte
	if pkcs8 {
		der, err = x509.MarshalPKCS8PrivateKey(key)
	} else {
		der, err = x509.MarshalECPrivateKey(key)
	}
	if err != nil {
		t.Fatalf("marshal key error: %v", err)
	}

	var buf bytes.Buffer
	blockType := "PRIVATE KEY"
	if !pkcs8 {
		blockType = "EC PRIVATE KEY"
	}
	if err := pem.Encode(&buf, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		t.Fatalf("pem encode error: %v", err)
	}

	if err := os.WriteFile(path, buf.Bytes(), mode); err != nil {
		t.Fatalf("write key file error: %v", err)
	}
}

func withArrayKeyring(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "0")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	previous := keyringOpener
	previousLegacy := legacyKeyringOpener
	kr := keyring.NewArrayKeyring([]keyring.Item{})
	keyringOpener = func() (keyring.Keyring, error) {
		return kr, nil
	}
	t.Cleanup(func() {
		keyringOpener = previous
		legacyKeyringOpener = previousLegacy
	})
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		return kr, nil
	}
}

func TestGetCredentials_SkipsLegacyProbeOnceLegacyKeychainIsEmpty(t *testing.T) {
	newKr, legacyKr := withSeparateKeyrings(t)
	marker := filepath.Join(t.TempDir(), "legacy-keychain-empty")
	previousMarker := legacyKeychainEmptyMarkerPath
	legacyKeychainEmptyMarkerPath = func() (string, error) { return marker, nil }
	t.Cleanup(func() { legacyKeychainEmptyMarkerPath = previousMarker })
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath, 0o600, true)
	storeCredentialInKeyring(t, legacyKr, "old", "OLDKEY", "OLDISSUER", keyPath)

	if creds, err := GetCredentials(""); err != nil || creds.KeyID != "OLDKEY" {
		t.Fatalf("GetCredentials() = %+v, %v; want migrated OLDKEY", creds, err)
	}
	if _, err := newKr.Get(keyringKey("old")); err != nil {
		t.Fatalf("legacy credential was not migrated: %v", err)
	}
	_, _ = GetCredentials("")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("marker not written after empty legacy probe: %v", err)
	}
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		t.Fatal("legacy keychain opened after it was marked empty")
		return nil, nil
	}
	if _, err := GetCredentials(""); err != nil {
		t.Fatalf("GetCredentials() with marker error = %v", err)
	}
}

func withSeparateKeyrings(t *testing.T) (keyring.Keyring, keyring.Keyring) {
	t.Helper()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "0")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	previous := keyringOpener
	previousLegacy := legacyKeyringOpener
	kr := keyring.NewArrayKeyring([]keyring.Item{})
	legacyKr := keyring.NewArrayKeyring([]keyring.Item{})
	keyringOpener = func() (keyring.Keyring, error) {
		return kr, nil
	}
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		return legacyKr, nil
	}
	t.Cleanup(func() {
		keyringOpener = previous
		legacyKeyringOpener = previousLegacy
	})
	return kr, legacyKr
}

func storeCredentialInKeyring(t *testing.T, kr keyring.Keyring, name, keyID, issuerID, keyPath string) {
	t.Helper()
	payload := credentialPayload{
		KeyID:          keyID,
		IssuerID:       issuerID,
		PrivateKeyPath: keyPath,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload error: %v", err)
	}
	if err := kr.Set(keyring.Item{Key: keyringKey(name), Data: data}); err != nil {
		t.Fatalf("store keyring item error: %v", err)
	}
}

type failingKeyring struct {
	err error
}

func (k failingKeyring) Get(string) (keyring.Item, error) { return keyring.Item{}, k.err }
func (k failingKeyring) GetMetadata(string) (keyring.Metadata, error) {
	return keyring.Metadata{}, k.err
}
func (k failingKeyring) Set(keyring.Item) error  { return k.err }
func (k failingKeyring) Remove(string) error     { return k.err }
func (k failingKeyring) Keys() ([]string, error) { return nil, k.err }

type countingKeyring struct {
	inner    *keyring.ArrayKeyring
	setCalls int
	getKeys  []string
}

func (k *countingKeyring) Get(key string) (keyring.Item, error) {
	k.getKeys = append(k.getKeys, key)
	return k.inner.Get(key)
}

func (k *countingKeyring) GetMetadata(key string) (keyring.Metadata, error) {
	return k.inner.GetMetadata(key)
}

func (k *countingKeyring) Set(item keyring.Item) error {
	k.setCalls++
	return k.inner.Set(item)
}
func (k *countingKeyring) Remove(key string) error { return k.inner.Remove(key) }
func (k *countingKeyring) Keys() ([]string, error) { return k.inner.Keys() }

type removeFailingKeyring struct {
	inner keyring.Keyring
	err   error
}

func (k *removeFailingKeyring) Get(key string) (keyring.Item, error) { return k.inner.Get(key) }
func (k *removeFailingKeyring) GetMetadata(key string) (keyring.Metadata, error) {
	return k.inner.GetMetadata(key)
}
func (k *removeFailingKeyring) Set(item keyring.Item) error { return k.inner.Set(item) }
func (k *removeFailingKeyring) Remove(string) error         { return k.err }
func (k *removeFailingKeyring) Keys() ([]string, error)     { return k.inner.Keys() }

type transientRemoveFailingKeyring struct {
	inner             keyring.Keyring
	remainingFailures int
	err               error
}

func (k *transientRemoveFailingKeyring) Get(key string) (keyring.Item, error) {
	return k.inner.Get(key)
}

func (k *transientRemoveFailingKeyring) GetMetadata(key string) (keyring.Metadata, error) {
	return k.inner.GetMetadata(key)
}
func (k *transientRemoveFailingKeyring) Set(item keyring.Item) error { return k.inner.Set(item) }
func (k *transientRemoveFailingKeyring) Remove(key string) error {
	if k.remainingFailures > 0 {
		k.remainingFailures--
		return k.err
	}
	return k.inner.Remove(key)
}
func (k *transientRemoveFailingKeyring) Keys() ([]string, error) { return k.inner.Keys() }

type nthRemoveFailingKeyring struct {
	inner       keyring.Keyring
	removeCalls int
	failOnCall  int
	err         error
}

func (k *nthRemoveFailingKeyring) Get(key string) (keyring.Item, error) {
	return k.inner.Get(key)
}

func (k *nthRemoveFailingKeyring) GetMetadata(key string) (keyring.Metadata, error) {
	return k.inner.GetMetadata(key)
}
func (k *nthRemoveFailingKeyring) Set(item keyring.Item) error { return k.inner.Set(item) }
func (k *nthRemoveFailingKeyring) Remove(key string) error {
	k.removeCalls++
	if k.removeCalls == k.failOnCall {
		return k.err
	}
	return k.inner.Remove(key)
}
func (k *nthRemoveFailingKeyring) Keys() ([]string, error) { return k.inner.Keys() }

type transientKeysFailingKeyring struct {
	inner             keyring.Keyring
	remainingFailures int
	err               error
}

func (k *transientKeysFailingKeyring) Get(key string) (keyring.Item, error) {
	return k.inner.Get(key)
}

func (k *transientKeysFailingKeyring) GetMetadata(key string) (keyring.Metadata, error) {
	return k.inner.GetMetadata(key)
}
func (k *transientKeysFailingKeyring) Set(item keyring.Item) error { return k.inner.Set(item) }
func (k *transientKeysFailingKeyring) Remove(key string) error     { return k.inner.Remove(key) }
func (k *transientKeysFailingKeyring) Keys() ([]string, error) {
	if k.remainingFailures > 0 {
		k.remainingFailures--
		return nil, k.err
	}
	return k.inner.Keys()
}

func TestGetCredentialsWithSource_KeychainAccessDeniedReturnsSentinel(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "")

	previousKeyringOpener := keyringOpener
	previousLegacyKeyringOpener := legacyKeyringOpener
	t.Cleanup(func() {
		keyringOpener = previousKeyringOpener
		legacyKeyringOpener = previousLegacyKeyringOpener
	})

	// Simulate the kind of stringified OSStatus errors produced by go-keychain.
	denyErr := errors.New("Failed to query keychain: The user name or passphrase you entered is not correct. (-25293)")
	keyringOpener = func() (keyring.Keyring, error) {
		return failingKeyring{err: denyErr}, nil
	}
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	}

	_, _, err := GetCredentialsWithSource("")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrKeychainAccessDenied) {
		t.Fatalf("expected ErrKeychainAccessDenied, got %v", err)
	}
}

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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/99designs/keyring"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

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

func TestSaveDefaultNameClearsLegacyFieldsOnMismatch(t *testing.T) {
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
		t.Fatal("expected legacy credentials to be cleared when no matching profile")
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

	if err := StoreCredentials("my-key", "KEY123", "ISS456", "/tmp/AuthKey.p8"); err != nil {
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

	if err := StoreCredentials("my-key", "KEY123", "ISS456", keyPath); err != nil {
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

	if err := StoreCredentials("my-key", "KEY123", "ISS456", keyPath); err != nil {
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

func TestRemoveAllCredentials(t *testing.T) {
	withArrayKeyring(t)

	if err := StoreCredentials("my-key", "KEY123", "ISS456", "/tmp/AuthKey.p8"); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}

	if err := RemoveAllCredentials(); err != nil {
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

	if err := StoreCredentials("test-fallback", "KEY123", "ISS456", "/tmp/AuthKey.p8"); err != nil {
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

func TestStoreCredentials_RemovesStaleGlobalCredentialWhenLocalConfigActive(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "0")

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

	if err := StoreCredentials("stale", "NEW_KEY", "NEW_ISSUER", "/tmp/new.p8"); err != nil {
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

func TestStoreCredentials_RemovesStaleCredentialFromOverrideAndGlobalConfigs(t *testing.T) {
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

	previousKeyringOpener := keyringOpener
	kr := keyring.NewArrayKeyring([]keyring.Item{})
	keyringOpener = func() (keyring.Keyring, error) {
		return kr, nil
	}
	t.Cleanup(func() {
		keyringOpener = previousKeyringOpener
	})

	if err := StoreCredentials("stale", "NEW_KEY", "NEW_ISSUER", "/tmp/new.p8"); err != nil {
		t.Fatalf("StoreCredentials() error: %v", err)
	}

	loadedOverride, err := config.LoadAt(overridePath)
	if err != nil {
		t.Fatalf("LoadAt(override) error: %v", err)
	}
	if len(loadedOverride.Keys) != 1 || loadedOverride.Keys[0].Name != "keep-override" {
		t.Fatalf("expected override config to keep non-target credential, got %+v", loadedOverride.Keys)
	}

	loadedGlobal, err := config.LoadAt(globalPath)
	if err != nil {
		t.Fatalf("LoadAt(global) error: %v", err)
	}
	if len(loadedGlobal.Keys) != 1 || loadedGlobal.Keys[0].Name != "keep-global" {
		t.Fatalf("expected global config to keep non-target credential, got %+v", loadedGlobal.Keys)
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

	if err := RemoveCredentials("legacy-only"); err != nil {
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

	if err := RemoveCredentials("  trim-key  "); err != nil {
		t.Fatalf("RemoveCredentials() error: %v", err)
	}
	if _, err := newKr.Get(keyringKey("trim-key")); !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("expected credential to be removed, got %v", err)
	}
}

func TestRemoveCredentials_MissingReturnsErr(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)

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

	err := RemoveCredentials("missing")
	if !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}
}

func writeECDSAPEM(t *testing.T, path string, mode os.FileMode, pkcs8 bool) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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

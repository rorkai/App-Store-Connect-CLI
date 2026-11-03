package storekit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/99designs/keyring"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestStoredCredentialNeverMarshalsPrivateKeyMaterial(t *testing.T) {
	stored := StoredCredential{
		Credentials: Credentials{PrivateKeyPEM: "TOP-SECRET-PEM", PrivateKeyPath: "/secret/key.p8"},
		Name:        "Primary",
		Source:      "keychain",
	}
	data, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(data) == "" || containsAny(string(data), "TOP-SECRET-PEM", "/secret/key.p8", "PrivateKeyPEM") {
		t.Fatalf("marshaled credential leaked private key material: %s", data)
	}
}

func TestConfigCredentialLifecycle(t *testing.T) {
	t.Setenv("ASC_STOREKIT_BYPASS_KEYCHAIN", "1")
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", path)
	credentials := testCredentials(t)
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	if err := os.WriteFile(keyPath, []byte(credentials.PrivateKeyPEM), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	credentials.PrivateKeyPath = keyPath
	credentials.PrivateKeyPEM = ""

	if err := StoreCredentialsConfig("Primary", credentials); err != nil {
		t.Fatalf("StoreCredentialsConfig() error = %v", err)
	}
	stored, source, err := GetCredentialsWithSource("")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource() error = %v", err)
	}
	if source != "config" || stored.KeyID != credentials.KeyID || stored.BundleID != credentials.BundleID {
		t.Fatalf("stored = %#v source=%q", stored, source)
	}
	if err := SetDefaultCredentials("Primary"); err != nil {
		t.Fatalf("SetDefaultCredentials() error = %v", err)
	}
	if err := RemoveCredentials("Primary"); err != nil {
		t.Fatalf("RemoveCredentials() error = %v", err)
	}
	if _, _, err := GetCredentialsWithSource(""); err == nil {
		t.Fatal("expected missing default credentials error")
	}
}

func TestGetCredentialsPrefersActiveConfigOverSameNamedKeychainProfile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	configCredentials := testCredentials(t)
	configCredentials.KeyID = "CONFIG_KEY"
	configCredentials.PrivateKeyPath = "/config/key.p8"
	configCredentials.PrivateKeyPEM = ""
	if err := StoreCredentialsConfigAt("shared", configCredentials, configPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt() error = %v", err)
	}

	payload, err := json.Marshal(credentialPayload{
		KeyID: "KEYCHAIN_KEY", IssuerID: "KEYCHAIN_ISSUER", PrivateKeyPEM: "keychain-pem", BundleID: "com.keychain.app",
	})
	if err != nil {
		t.Fatal(err)
	}
	original := openStoreKitKeyring
	openStoreKitKeyring = func() (keyring.Keyring, error) {
		return keyring.NewArrayKeyring([]keyring.Item{{Key: storeKitKeyringItemPrefix + "shared", Data: payload}}), nil
	}
	t.Cleanup(func() { openStoreKitKeyring = original })

	credentials, source, err := GetCredentialsWithSource("shared")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource() error = %v", err)
	}
	if source != "config" || credentials.KeyID != "CONFIG_KEY" {
		t.Fatalf("credentials = %#v source=%q", credentials, source)
	}
	listed, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error = %v", err)
	}
	if len(listed) != 1 || listed[0].Source != "config" || listed[0].KeyID != "CONFIG_KEY" {
		t.Fatalf("ListCredentials() = %#v", listed)
	}
}

func TestGetCredentialsFallsBackToGlobalWhenLocalConfigHasNoStoreKitKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("ASC_STOREKIT_BYPASS_KEYCHAIN", "1")
	globalPath, err := config.GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	credentials := testCredentials(t)
	credentials.PrivateKeyPath = "/global/SubscriptionKey.p8"
	credentials.PrivateKeyPEM = ""
	if err := StoreCredentialsConfigAt("global", credentials, globalPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt(global) error = %v", err)
	}

	projectDir := t.TempDir()
	localPath := filepath.Join(projectDir, ".asc", "config.json")
	if err := config.SaveAt(localPath, &config.Config{AppID: "local-app"}); err != nil {
		t.Fatalf("SaveAt(local) error = %v", err)
	}
	t.Chdir(projectDir)

	resolved, source, err := GetCredentialsWithSource("global")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource(global) error = %v", err)
	}
	if source != "config" || resolved.KeyID != credentials.KeyID || resolved.Profile != "global" {
		t.Fatalf("credentials = %#v source=%q", resolved, source)
	}
}

func TestStoreCredentialsConfigInRepoWithoutStoreKitConfigWritesGlobalConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("ASC_STOREKIT_BYPASS_KEYCHAIN", "1")
	projectDir := t.TempDir()
	localPath := filepath.Join(projectDir, ".asc", "config.json")
	if err := config.SaveAt(localPath, &config.Config{AppID: "local-app"}); err != nil {
		t.Fatalf("SaveAt(local) error = %v", err)
	}
	t.Chdir(projectDir)
	credentials := testCredentials(t)
	credentials.PrivateKeyPath = "/global/SubscriptionKey.p8"
	credentials.PrivateKeyPEM = ""

	if err := StoreCredentialsConfig("repo", credentials); err != nil {
		t.Fatalf("StoreCredentialsConfig() error = %v", err)
	}

	localCfg, err := config.LoadAt(localPath)
	if err != nil {
		t.Fatalf("LoadAt(local) error = %v", err)
	}
	if hasStoreKitConfig(localCfg) {
		t.Fatalf("expected repo config to stay free of StoreKit credentials, got %+v", localCfg.StoreKit)
	}
	globalPath, err := config.GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	globalCfg, err := config.LoadAt(globalPath)
	if err != nil {
		t.Fatalf("LoadAt(global) error = %v", err)
	}
	if len(globalCfg.StoreKit.Keys) != 1 || globalCfg.StoreKit.Keys[0].Name != "repo" {
		t.Fatalf("expected StoreKit credential in global config, got %+v", globalCfg.StoreKit)
	}
}

func TestStoreCredentialsKeychainInRepoWithoutStoreKitConfigKeepsDefaultGlobal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("ASC_STOREKIT_BYPASS_KEYCHAIN", "")
	original := openStoreKitKeyring
	kr := keyring.NewArrayKeyring(nil)
	openStoreKitKeyring = func() (keyring.Keyring, error) { return kr, nil }
	t.Cleanup(func() { openStoreKitKeyring = original })
	projectDir := t.TempDir()
	localPath := filepath.Join(projectDir, ".asc", "config.json")
	if err := config.SaveAt(localPath, &config.Config{AppID: "local-app"}); err != nil {
		t.Fatalf("SaveAt(local) error = %v", err)
	}
	t.Chdir(projectDir)

	if err := StoreCredentials("keychain", testCredentials(t)); err != nil {
		t.Fatalf("StoreCredentials() error = %v", err)
	}

	localCfg, err := config.LoadAt(localPath)
	if err != nil {
		t.Fatalf("LoadAt(local) error = %v", err)
	}
	if hasStoreKitConfig(localCfg) {
		t.Fatalf("expected repo config to stay free of StoreKit config, got %+v", localCfg.StoreKit)
	}
	globalPath, err := config.GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	globalCfg, err := config.LoadAt(globalPath)
	if err != nil {
		t.Fatalf("LoadAt(global) error = %v", err)
	}
	if globalCfg.StoreKit.DefaultKeyName != "keychain" {
		t.Fatalf("expected global StoreKit default keychain, got %q", globalCfg.StoreKit.DefaultKeyName)
	}
}

func TestGetCredentialsPrefersGlobalConfigOverSameNamedKeychainProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("ASC_STOREKIT_BYPASS_KEYCHAIN", "0")
	globalPath, err := config.GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	configCredentials := testCredentials(t)
	configCredentials.KeyID = "GLOBAL_CONFIG_KEY"
	configCredentials.PrivateKeyPath = "/global/SubscriptionKey.p8"
	configCredentials.PrivateKeyPEM = ""
	if err := StoreCredentialsConfigAt("shared", configCredentials, globalPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt(global) error = %v", err)
	}

	payload, err := json.Marshal(credentialPayload{
		KeyID: "KEYCHAIN_KEY", IssuerID: "KEYCHAIN_ISSUER", PrivateKeyPEM: "keychain-pem", BundleID: "com.keychain.app",
	})
	if err != nil {
		t.Fatal(err)
	}
	original := openStoreKitKeyring
	openStoreKitKeyring = func() (keyring.Keyring, error) {
		return keyring.NewArrayKeyring([]keyring.Item{{Key: storeKitKeyringItemPrefix + "shared", Data: payload}}), nil
	}
	t.Cleanup(func() { openStoreKitKeyring = original })

	projectDir := t.TempDir()
	localPath := filepath.Join(projectDir, ".asc", "config.json")
	if err := config.SaveAt(localPath, &config.Config{AppID: "local-app"}); err != nil {
		t.Fatalf("SaveAt(local) error = %v", err)
	}
	t.Chdir(projectDir)

	resolved, source, err := GetCredentialsWithSource("shared")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource(shared) error = %v", err)
	}
	if source != "config" || resolved.KeyID != "GLOBAL_CONFIG_KEY" {
		t.Fatalf("credentials = %#v source=%q", resolved, source)
	}
	defaultCredentials, defaultSource, err := GetCredentialsWithSource("")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource(default) error = %v", err)
	}
	if defaultSource != "config" || defaultCredentials.KeyID != "GLOBAL_CONFIG_KEY" {
		t.Fatalf("default credentials = %#v source=%q", defaultCredentials, defaultSource)
	}
	listed, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error = %v", err)
	}
	if len(listed) != 1 || listed[0].Source != source || listed[0].KeyID != resolved.KeyID {
		t.Fatalf("ListCredentials() = %#v, resolved = %#v source=%q", listed, resolved, source)
	}
}

func TestGetCredentialsPreservesInvalidFallbackConfigError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("ASC_STOREKIT_BYPASS_KEYCHAIN", "0")
	globalPath, err := config.GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(global) error = %v", err)
	}
	if err := os.WriteFile(globalPath, []byte("{invalid-json"), 0o600); err != nil {
		t.Fatalf("WriteFile(global) error = %v", err)
	}

	payload, err := json.Marshal(credentialPayload{
		KeyID: "OTHER_KEY", IssuerID: "OTHER_ISSUER", PrivateKeyPEM: "keychain-pem", BundleID: "com.keychain.app",
	})
	if err != nil {
		t.Fatal(err)
	}
	original := openStoreKitKeyring
	openStoreKitKeyring = func() (keyring.Keyring, error) {
		return keyring.NewArrayKeyring([]keyring.Item{{Key: storeKitKeyringItemPrefix + "other", Data: payload}}), nil
	}
	t.Cleanup(func() { openStoreKitKeyring = original })

	projectDir := t.TempDir()
	localPath := filepath.Join(projectDir, ".asc", "config.json")
	if err := config.SaveAt(localPath, &config.Config{AppID: "local-app"}); err != nil {
		t.Fatalf("SaveAt(local) error = %v", err)
	}
	t.Chdir(projectDir)

	_, _, err = GetCredentialsWithSource("wanted")
	if err == nil || !strings.Contains(err.Error(), "failed to parse config") {
		t.Fatalf("GetCredentialsWithSource(wanted) error = %v", err)
	}
	if strings.Contains(err.Error(), "credentials not found") {
		t.Fatalf("GetCredentialsWithSource(wanted) dropped config error: %v", err)
	}
}

func TestSetDefaultCredentialsUpdatesGlobalCredentialSource(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("ASC_STOREKIT_BYPASS_KEYCHAIN", "1")
	globalPath, err := config.GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	credentials := testCredentials(t)
	credentials.PrivateKeyPath = "/global/SubscriptionKey.p8"
	credentials.PrivateKeyPEM = ""
	if err := StoreCredentialsConfigAt("first", credentials, globalPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt(first) error = %v", err)
	}
	if err := StoreCredentialsConfigAt("second", credentials, globalPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt(second) error = %v", err)
	}

	projectDir := t.TempDir()
	localPath := filepath.Join(projectDir, ".asc", "config.json")
	if err := config.SaveAt(localPath, &config.Config{AppID: "local-app"}); err != nil {
		t.Fatalf("SaveAt(local) error = %v", err)
	}
	t.Chdir(projectDir)

	if err := SetDefaultCredentials("first"); err != nil {
		t.Fatalf("SetDefaultCredentials(first) error = %v", err)
	}
	localConfig, err := config.LoadAt(localPath)
	if err != nil {
		t.Fatalf("LoadAt(local) error = %v", err)
	}
	if localConfig.StoreKit.DefaultKeyName != "" {
		t.Fatalf("local default = %q, want empty", localConfig.StoreKit.DefaultKeyName)
	}
	globalConfig, err := config.LoadAt(globalPath)
	if err != nil {
		t.Fatalf("LoadAt(global) error = %v", err)
	}
	if globalConfig.StoreKit.DefaultKeyName != "first" {
		t.Fatalf("global default = %q, want first", globalConfig.StoreKit.DefaultKeyName)
	}
}

func TestRemoveCredentialsRemovesGlobalFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("ASC_STOREKIT_BYPASS_KEYCHAIN", "1")
	globalPath, err := config.GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	credentials := testCredentials(t)
	credentials.PrivateKeyPath = "/global/SubscriptionKey.p8"
	credentials.PrivateKeyPEM = ""
	if err := StoreCredentialsConfigAt("global", credentials, globalPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt(global) error = %v", err)
	}

	projectDir := t.TempDir()
	localPath := filepath.Join(projectDir, ".asc", "config.json")
	if err := config.SaveAt(localPath, &config.Config{AppID: "local-app"}); err != nil {
		t.Fatalf("SaveAt(local) error = %v", err)
	}
	t.Chdir(projectDir)

	if err := RemoveCredentials("global"); err != nil {
		t.Fatalf("RemoveCredentials(global) error = %v", err)
	}
	globalConfig, err := config.LoadAt(globalPath)
	if err != nil {
		t.Fatalf("LoadAt(global) error = %v", err)
	}
	if len(globalConfig.StoreKit.Keys) != 0 || globalConfig.StoreKit.DefaultKeyName != "" {
		t.Fatalf("global StoreKit config was not cleared: %#v", globalConfig.StoreKit)
	}
}

func TestRemoveAllCredentialsClearsLocalAndGlobalConfigs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("ASC_STOREKIT_BYPASS_KEYCHAIN", "1")
	globalPath, err := config.GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	credentials := testCredentials(t)
	credentials.PrivateKeyPath = "/global/SubscriptionKey.p8"
	credentials.PrivateKeyPEM = ""
	if err := StoreCredentialsConfigAt("global", credentials, globalPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt(global) error = %v", err)
	}

	projectDir := t.TempDir()
	localPath := filepath.Join(projectDir, ".asc", "config.json")
	if err := config.SaveAt(localPath, &config.Config{AppID: "local-app"}); err != nil {
		t.Fatalf("SaveAt(local) error = %v", err)
	}
	t.Chdir(projectDir)
	original := openStoreKitKeyring
	openStoreKitKeyring = func() (keyring.Keyring, error) {
		t.Fatal("RemoveAllCredentials opened the keychain despite ASC_STOREKIT_BYPASS_KEYCHAIN")
		return nil, nil
	}
	t.Cleanup(func() { openStoreKitKeyring = original })

	if err := RemoveAllCredentials(); err != nil {
		t.Fatalf("RemoveAllCredentials() error = %v", err)
	}
	globalConfig, err := config.LoadAt(globalPath)
	if err != nil {
		t.Fatalf("LoadAt(global) error = %v", err)
	}
	if len(globalConfig.StoreKit.Keys) != 0 || globalConfig.StoreKit.DefaultKeyName != "" {
		t.Fatalf("global StoreKit config was not cleared: %#v", globalConfig.StoreKit)
	}
	localConfig, err := config.LoadAt(localPath)
	if err != nil {
		t.Fatalf("LoadAt(local) error = %v", err)
	}
	if localConfig.AppID != "local-app" || len(localConfig.StoreKit.Keys) != 0 || localConfig.StoreKit.DefaultKeyName != "" {
		t.Fatalf("local non-StoreKit config changed or StoreKit config remains: %#v", localConfig)
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

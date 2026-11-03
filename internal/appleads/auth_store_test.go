package appleads

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/99designs/keyring"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

func TestStoreCredentialsConfigUsesActiveConfigPath(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "active-config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)

	if err := StoreCredentialsConfig("ads", testAdsCredentials()); err != nil {
		t.Fatalf("StoreCredentialsConfig() error: %v", err)
	}

	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("LoadAt(active) error: %v", err)
	}
	if len(cfg.Ads.Keys) != 1 || cfg.Ads.Keys[0].Name != "ads" {
		t.Fatalf("active config ads keys = %+v, want ads profile", cfg.Ads.Keys)
	}
}

func TestStoreCredentialsConfigInRepoWithoutAdsCredentialsWritesGlobalConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv(adsBypassKeychainEnvVar, "1")
	repo := t.TempDir()
	repoPath := filepath.Join(repo, ".asc", "config.json")
	if err := config.SaveAt(repoPath, &config.Config{AppID: "123"}); err != nil {
		t.Fatalf("SaveAt(repo) error: %v", err)
	}
	t.Chdir(repo)

	if err := StoreCredentialsConfig("ads", testAdsCredentials()); err != nil {
		t.Fatalf("StoreCredentialsConfig() error: %v", err)
	}

	repoCfg, err := config.LoadAt(repoPath)
	if err != nil {
		t.Fatalf("LoadAt(repo) error: %v", err)
	}
	if len(repoCfg.Ads.Keys) != 0 || repoCfg.Ads.DefaultKeyName != "" {
		t.Fatalf("expected repo config to stay free of Apple Ads credentials, got %+v", repoCfg.Ads)
	}
	creds, _, err := GetCredentialsWithSource("")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource() error: %v", err)
	}
	if creds.Profile != "ads" || creds.KeyID != "KEY" {
		t.Fatalf("expected global ads profile inside the repo, got %+v", creds)
	}
}

func TestStoreCredentialsConfigRejectsUnsafeAdAccountID(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "active-config.json")
	credentials := testAdsCredentials()
	credentials.AdAccountID = "123;orgId=999"

	err := StoreCredentialsConfigAt("ads", credentials, configPath)
	if err == nil || !strings.Contains(err.Error(), "invalid ad account ID") {
		t.Fatalf("StoreCredentialsConfigAt() error = %v, want invalid ad account ID", err)
	}
}

func TestStoreCredentialsConfigRejectsUnsafeOrgID(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "active-config.json")
	credentials := testAdsCredentials()
	credentials.OrgID = "123;adAccountId=999"

	err := StoreCredentialsConfigAt("ads", credentials, configPath)
	if err == nil || !strings.Contains(err.Error(), "invalid organization ID") {
		t.Fatalf("StoreCredentialsConfigAt() error = %v, want invalid organization ID", err)
	}
}

func TestStoreCredentialsConfigRoundTripsIndependentAdAccountID(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "active-config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv(adsBypassKeychainEnvVar, "1")
	credentials := testAdsCredentials()
	credentials.AdAccountID = "AD_ACCOUNT"

	if err := StoreCredentialsConfig("ads", credentials); err != nil {
		t.Fatalf("StoreCredentialsConfig() error: %v", err)
	}

	loaded, _, err := GetCredentialsWithSource("ads")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource() error: %v", err)
	}
	if loaded.OrgID != "123456" || loaded.AdAccountID != "AD_ACCOUNT" {
		t.Fatalf("contexts = org %q ad account %q", loaded.OrgID, loaded.AdAccountID)
	}
	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("LoadAt() error: %v", err)
	}
	if cfg.Ads.OrgID != "123456" || cfg.Ads.AdAccountID != "AD_ACCOUNT" || cfg.Ads.Keys[0].AdAccountID != "AD_ACCOUNT" {
		t.Fatalf("stored ads config = %+v", cfg.Ads)
	}
}

func TestStoreCredentialsConfigCanClearDefaultAdAccountID(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "active-config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv(adsBypassKeychainEnvVar, "1")
	credentials := testAdsCredentials()
	credentials.AdAccountID = "AD_ACCOUNT"
	if err := StoreCredentialsConfig("ads", credentials); err != nil {
		t.Fatalf("StoreCredentialsConfig(initial) error: %v", err)
	}

	credentials.AdAccountID = ""
	if err := StoreCredentialsConfig("ads", credentials); err != nil {
		t.Fatalf("StoreCredentialsConfig(clear) error: %v", err)
	}
	loaded, _, err := GetCredentialsWithSource("ads")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource() error: %v", err)
	}
	if loaded.AdAccountID != "" {
		t.Fatalf("AdAccountID = %q, want cleared", loaded.AdAccountID)
	}
	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("LoadAt() error: %v", err)
	}
	if cfg.Ads.AdAccountID != "" || cfg.Ads.Keys[0].AdAccountID != "" {
		t.Fatalf("stored ad account was not cleared: %+v", cfg.Ads)
	}
}

func TestNamedConfigProfileDoesNotInheritGlobalAdAccountID(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "active-config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv(adsBypassKeychainEnvVar, "1")
	credentialsA := testAdsCredentials()
	credentialsA.AdAccountID = "ACCOUNT_A"
	credentialsB := testAdsCredentials()
	credentialsB.ClientID = "CLIENT_B"
	if err := config.SaveAt(configPath, &config.Config{Ads: config.AdsConfig{
		DefaultKeyName: "profile-b",
		AdAccountID:    "ACCOUNT_A",
		Keys: []config.AdsCredential{
			{Name: "profile-a", ClientID: credentialsA.ClientID, TeamID: credentialsA.TeamID, KeyID: credentialsA.KeyID, PrivateKeyPath: credentialsA.PrivateKeyPath, OrgID: credentialsA.OrgID, AdAccountID: credentialsA.AdAccountID},
			{Name: "profile-b", ClientID: credentialsB.ClientID, TeamID: credentialsB.TeamID, KeyID: credentialsB.KeyID, PrivateKeyPath: credentialsB.PrivateKeyPath, OrgID: credentialsB.OrgID},
		},
	}}); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	for _, profile := range []string{"profile-b", ""} {
		loaded, _, err := GetCredentialsWithSource(profile)
		if err != nil {
			t.Fatalf("GetCredentialsWithSource(%q) error: %v", profile, err)
		}
		if loaded.AdAccountID != "" {
			t.Fatalf("GetCredentialsWithSource(%q).AdAccountID = %q, want empty", profile, loaded.AdAccountID)
		}
	}
}

func TestNamedConfigProfileDoesNotInheritRootContexts(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "active-config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv(adsBypassKeychainEnvVar, "1")
	if err := config.SaveAt(configPath, &config.Config{Ads: config.AdsConfig{
		DefaultKeyName: "profile-a",
		OrgID:          "ROOT_ORG",
		AdAccountID:    "ROOT_ACCOUNT",
		Keys: []config.AdsCredential{
			{Name: "profile-a", ClientID: "A", TeamID: "T", KeyID: "K", PrivateKeyPath: "a.pem"},
			{Name: "profile-b", ClientID: "B", TeamID: "T", KeyID: "K", PrivateKeyPath: "b.pem"},
		},
	}}); err != nil {
		t.Fatalf("SaveAt() error: %v", err)
	}

	loaded, _, err := GetCredentialsWithSource("profile-b")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource() error: %v", err)
	}
	if loaded.OrgID != "" {
		t.Fatalf("OrgID = %q, must not inherit root org", loaded.OrgID)
	}
	if loaded.AdAccountID != "" {
		t.Fatalf("AdAccountID = %q, must not inherit root ad account", loaded.AdAccountID)
	}
}

func TestNamedConfigProfileDoesNotInheritPreviousDefaultContexts(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "active-config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv(adsBypassKeychainEnvVar, "1")

	profileA := testAdsCredentials()
	profileA.OrgID = "ORG_A"
	profileA.AdAccountID = "ACCOUNT_A"
	if err := StoreCredentialsConfig("profile-a", profileA); err != nil {
		t.Fatalf("StoreCredentialsConfig(profile-a) error: %v", err)
	}

	profileB := testAdsCredentials()
	profileB.ClientID = "CLIENT_B"
	profileB.OrgID = ""
	profileB.AdAccountID = ""
	if err := StoreCredentialsConfig("profile-b", profileB); err != nil {
		t.Fatalf("StoreCredentialsConfig(profile-b) error: %v", err)
	}

	for _, profile := range []string{"profile-b", ""} {
		loaded, _, err := GetCredentialsWithSource(profile)
		if err != nil {
			t.Fatalf("GetCredentialsWithSource(%q) error: %v", profile, err)
		}
		if loaded.OrgID != "" || loaded.AdAccountID != "" {
			t.Fatalf("GetCredentialsWithSource(%q) contexts = org %q ad account %q, want empty", profile, loaded.OrgID, loaded.AdAccountID)
		}
	}

	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("LoadAt() error: %v", err)
	}
	if cfg.Ads.DefaultKeyName != "profile-b" || cfg.Ads.OrgID != "" || cfg.Ads.AdAccountID != "" {
		t.Fatalf("default profile contexts = %+v, want profile-b with empty contexts", cfg.Ads)
	}
}

func TestSwitchingDefaultToProfileWithoutOrgClearsRootContexts(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "active-config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv(adsBypassKeychainEnvVar, "1")

	profileA := testAdsCredentials()
	profileA.OrgID = "ORG_A"
	profileA.AdAccountID = "ACCOUNT_A"
	if err := StoreCredentialsConfig("profile-a", profileA); err != nil {
		t.Fatalf("StoreCredentialsConfig(profile-a) error: %v", err)
	}
	profileB := testAdsCredentials()
	profileB.ClientID = "CLIENT_B"
	profileB.OrgID = ""
	profileB.AdAccountID = ""
	if err := StoreCredentialsConfig("profile-b", profileB); err != nil {
		t.Fatalf("StoreCredentialsConfig(profile-b) error: %v", err)
	}
	if err := SetDefaultCredentials("profile-a"); err != nil {
		t.Fatalf("SetDefaultCredentials(profile-a) error: %v", err)
	}
	if err := SetDefaultCredentials("profile-b"); err != nil {
		t.Fatalf("SetDefaultCredentials(profile-b) error: %v", err)
	}

	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("LoadAt() error: %v", err)
	}
	if cfg.Ads.DefaultKeyName != "profile-b" || cfg.Ads.OrgID != "" || cfg.Ads.AdAccountID != "" {
		t.Fatalf("switched default contexts = %+v, want profile-b with empty contexts", cfg.Ads)
	}
}

func TestRemoveDefaultConfigProfileClearsAdAccountContext(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "active-config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv(adsBypassKeychainEnvVar, "1")
	credentials := testAdsCredentials()
	credentials.AdAccountID = "ACCOUNT_A"
	if err := StoreCredentialsConfig("profile-a", credentials); err != nil {
		t.Fatalf("StoreCredentialsConfig() error: %v", err)
	}
	if err := RemoveCredentials("profile-a"); err != nil {
		t.Fatalf("RemoveCredentials() error: %v", err)
	}

	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("LoadAt() error: %v", err)
	}
	if cfg.Ads.DefaultKeyName != "" || cfg.Ads.OrgID != "" || cfg.Ads.AdAccountID != "" {
		t.Fatalf("removed default left context behind: %+v", cfg.Ads)
	}
}

func TestLoadConfigWithPathDoesNotFallbackToGlobalWhenASCConfigPathSet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	globalPath, err := config.GlobalPath()
	if err != nil {
		t.Fatalf("GlobalPath() error: %v", err)
	}
	if err := StoreCredentialsConfigAt("global", testAdsCredentials(), globalPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt(global) error: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	_, _, err = loadConfigWithPath()
	if !errors.Is(err, config.ErrNotFound) {
		t.Fatalf("loadConfigWithPath() error = %v, want ErrNotFound", err)
	}
}

func TestGetCredentialsFallsBackToConfigDefaultWhenKeychainHasNoDefault(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	if err := StoreCredentialsConfigAt("config-default", testAdsCredentials(), configPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
	}

	keychainPayload, err := json.Marshal(credentialPayload{
		ClientID:       "KEYCHAIN_CLIENT",
		TeamID:         "KEYCHAIN_TEAM",
		KeyID:          "KEYCHAIN_KEY",
		PrivateKeyPath: "keychain-private-key.pem",
		OrgID:          "999999",
	})
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	original := openKeyring
	openKeyring = func() (keyring.Keyring, error) {
		return fakeAdsKeyring{
			items: map[string]keyring.Item{
				keyringKey("keychain-only"): {
					Key:  keyringKey("keychain-only"),
					Data: keychainPayload,
				},
				keyringKey("keychain-other"): {
					Key:  keyringKey("keychain-other"),
					Data: keychainPayload,
				},
			},
		}, nil
	}
	t.Cleanup(func() { openKeyring = original })

	credentials, source, err := GetCredentialsWithSource("")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource() error: %v", err)
	}
	if source != "config" || credentials.Profile != "config-default" || credentials.ClientID != "CLIENT" {
		t.Fatalf("credentials = %+v source = %q, want config default profile", credentials, source)
	}
}

func TestGetCredentialsPrefersConfigDefaultOverSingleKeychainFallback(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	configCreds := testAdsCredentials()
	configCreds.ClientID = "CONFIG_CLIENT"
	if err := StoreCredentialsConfigAt("config-default", configCreds, configPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
	}

	keychainPayload, err := json.Marshal(credentialPayload{
		ClientID:       "KEYCHAIN_CLIENT",
		TeamID:         "KEYCHAIN_TEAM",
		KeyID:          "KEYCHAIN_KEY",
		PrivateKeyPath: "keychain-private-key.pem",
		OrgID:          "999999",
	})
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	original := openKeyring
	openKeyring = func() (keyring.Keyring, error) {
		return fakeAdsKeyring{
			items: map[string]keyring.Item{
				keyringKey("keychain-only"): {
					Key:  keyringKey("keychain-only"),
					Data: keychainPayload,
				},
			},
		}, nil
	}
	t.Cleanup(func() { openKeyring = original })

	credentials, source, err := GetCredentialsWithSource("")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource() error: %v", err)
	}
	if source != "config" || credentials.Profile != "config-default" || credentials.ClientID != "CONFIG_CLIENT" {
		t.Fatalf("credentials = %+v source = %q, want config default over keychain fallback", credentials, source)
	}
}

func TestGetCredentialsPrefersActiveConfigProfileOverSameNamedKeychainProfile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	configCreds := testAdsCredentials()
	configCreds.ClientID = "CONFIG_CLIENT"
	configCreds.OrgID = "CONFIG_ORG"
	if err := StoreCredentialsConfigAt("shared", configCreds, configPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
	}

	keychainPayload, err := json.Marshal(credentialPayload{
		ClientID:       "KEYCHAIN_CLIENT",
		TeamID:         "KEYCHAIN_TEAM",
		KeyID:          "KEYCHAIN_KEY",
		PrivateKeyPath: "keychain-private-key.pem",
		OrgID:          "KEYCHAIN_ORG",
	})
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	original := openKeyring
	openKeyring = func() (keyring.Keyring, error) {
		return fakeAdsKeyring{
			items: map[string]keyring.Item{
				keyringKey("shared"): {
					Key:  keyringKey("shared"),
					Data: keychainPayload,
				},
			},
		}, nil
	}
	t.Cleanup(func() { openKeyring = original })

	credentials, source, err := GetCredentialsWithSource("shared")
	if err != nil {
		t.Fatalf("GetCredentialsWithSource() error: %v", err)
	}
	if source != "config" || credentials.ClientID != "CONFIG_CLIENT" || credentials.OrgID != "CONFIG_ORG" {
		t.Fatalf("credentials = %+v source = %q, want active config profile", credentials, source)
	}
}

func TestBypassKeychainRemovalSkipsKeychain(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	if err := StoreCredentialsConfigAt("ads", testAdsCredentials(), configPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
	}

	called := false
	original := openKeyring
	openKeyring = func() (keyring.Keyring, error) {
		called = true
		return nil, errors.New("keychain should be bypassed")
	}
	t.Cleanup(func() { openKeyring = original })

	if err := RemoveCredentials("ads"); err != nil {
		t.Fatalf("RemoveCredentials() error: %v", err)
	}
	if called {
		t.Fatal("RemoveCredentials opened keychain despite ASC_ADS_BYPASS_KEYCHAIN")
	}

	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("LoadAt() error: %v", err)
	}
	if len(cfg.Ads.Keys) != 0 {
		t.Fatalf("ads config after removal = %+v, want no keys", cfg.Ads.Keys)
	}
}

func TestShouldBypassKeychainAcceptsDocumentedTruthyValues(t *testing.T) {
	for _, value := range []string{"1", "true", "yes", "y", "on"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", value)
			if !ShouldBypassKeychain() {
				t.Fatalf("ShouldBypassKeychain() = false for %q, want true", value)
			}
		})
	}
}

func TestRemoveCredentialsReturnsNotFoundWhenProfileMissing(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	if err := StoreCredentialsConfigAt("ads", testAdsCredentials(), configPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
	}

	err := RemoveCredentials("missing")
	if !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("RemoveCredentials() error = %v, want key not found", err)
	}
}

func TestBypassKeychainRemoveAllSkipsKeychain(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	if err := StoreCredentialsConfigAt("ads", testAdsCredentials(), configPath); err != nil {
		t.Fatalf("StoreCredentialsConfigAt() error: %v", err)
	}

	called := false
	original := openKeyring
	openKeyring = func() (keyring.Keyring, error) {
		called = true
		return nil, errors.New("keychain should be bypassed")
	}
	t.Cleanup(func() { openKeyring = original })

	if err := RemoveAllCredentials(); err != nil {
		t.Fatalf("RemoveAllCredentials() error: %v", err)
	}
	if called {
		t.Fatal("RemoveAllCredentials opened keychain despite ASC_ADS_BYPASS_KEYCHAIN")
	}

	cfg, err := config.LoadAt(configPath)
	if err != nil {
		t.Fatalf("LoadAt() error: %v", err)
	}
	if len(cfg.Ads.Keys) != 0 || strings.TrimSpace(cfg.Ads.DefaultKeyName) != "" {
		t.Fatalf("ads config after clear = %+v, want empty credentials", cfg.Ads)
	}
}

type fakeAdsKeyring struct {
	items map[string]keyring.Item
}

func (f fakeAdsKeyring) Get(key string) (keyring.Item, error) {
	item, ok := f.items[key]
	if !ok {
		return keyring.Item{}, keyring.ErrKeyNotFound
	}
	return item, nil
}

func (f fakeAdsKeyring) GetMetadata(key string) (keyring.Metadata, error) {
	item, ok := f.items[key]
	if !ok {
		return keyring.Metadata{}, keyring.ErrKeyNotFound
	}
	return keyring.Metadata{
		Item: &keyring.Item{
			Key:         item.Key,
			Label:       item.Label,
			Description: item.Description,
		},
	}, nil
}

func (f fakeAdsKeyring) Set(item keyring.Item) error {
	f.items[item.Key] = item
	return nil
}

func (f fakeAdsKeyring) Remove(key string) error {
	if _, ok := f.items[key]; !ok {
		return keyring.ErrKeyNotFound
	}
	delete(f.items, key)
	return nil
}

func (f fakeAdsKeyring) Keys() ([]string, error) {
	keys := make([]string, 0, len(f.items))
	for key := range f.items {
		keys = append(keys, key)
	}
	return keys, nil
}

func testAdsCredentials() Credentials {
	return Credentials{
		ClientID:       "CLIENT",
		TeamID:         "TEAM",
		KeyID:          "KEY",
		PrivateKeyPath: "private-key.pem",
		OrgID:          "123456",
	}
}

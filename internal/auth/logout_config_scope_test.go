package auth

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/99designs/keyring"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

// scopedLogoutFixture isolates HOME and ASC_CONFIG_PATH in temp dirs, backs
// the keychain with an in-memory keyring holding one credential, and seeds the
// override and global config files with a shared "shared" profile plus one
// profile unique to each file.
type scopedLogoutFixture struct {
	overridePath string
	globalPath   string
	keyring      keyring.Keyring
}

func newScopedLogoutFixture(t *testing.T) scopedLogoutFixture {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	withArrayKeyring(t)
	overridePath := filepath.Join(t.TempDir(), "isolated", "config.json")
	t.Setenv("ASC_CONFIG_PATH", overridePath)

	kr, err := keyringOpener()
	if err != nil {
		t.Fatalf("keyringOpener() error: %v", err)
	}
	storeCredentialInKeyring(t, kr, "shared", "KEYCHAIN_KEY", "KEYCHAIN_ISSUER", "/tmp/keychain.p8")

	fixture := scopedLogoutFixture{
		overridePath: overridePath,
		globalPath:   filepath.Join(home, ".asc", "config.json"),
		keyring:      kr,
	}
	if err := config.SaveAt(fixture.overridePath, &config.Config{
		DefaultKeyName: "shared",
		Keys: []config.Credential{
			{Name: "shared", KeyID: "OVERRIDE_SHARED", IssuerID: "OVERRIDE_ISSUER", PrivateKeyPath: "/tmp/override-shared.p8"},
			{Name: "override-only", KeyID: "OVERRIDE_ONLY", IssuerID: "OVERRIDE_ISSUER", PrivateKeyPath: "/tmp/override-only.p8"},
		},
	}); err != nil {
		t.Fatalf("SaveAt(override) error: %v", err)
	}
	if err := config.SaveAt(fixture.globalPath, &config.Config{
		DefaultKeyName: "shared",
		Keys: []config.Credential{
			{Name: "shared", KeyID: "GLOBAL_SHARED", IssuerID: "GLOBAL_ISSUER", PrivateKeyPath: "/tmp/global-shared.p8"},
			{Name: "global-only", KeyID: "GLOBAL_ONLY", IssuerID: "GLOBAL_ISSUER", PrivateKeyPath: "/tmp/global-only.p8"},
		},
	}); err != nil {
		t.Fatalf("SaveAt(global) error: %v", err)
	}
	return fixture
}

func readConfigBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error: %v", path, err)
	}
	return data
}

func requireConfigKeyNames(t *testing.T, path string, want ...string) {
	t.Helper()
	cfg, err := config.LoadAt(path)
	if err != nil {
		t.Fatalf("LoadAt(%s) error: %v", path, err)
	}
	got := make([]string, 0, len(cfg.Keys))
	for _, cred := range cfg.Keys {
		got = append(got, cred.Name)
	}
	if len(got) != len(want) {
		t.Fatalf("%s profiles = %q, want %q", path, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("%s profiles = %q, want %q", path, got, want)
		}
	}
}

func requireKeychainEmpty(t *testing.T, kr keyring.Keyring) {
	t.Helper()
	keys, err := kr.Keys()
	if err != nil {
		t.Fatalf("Keys() error: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("keychain items = %q, want none", keys)
	}
}

func TestRemoveAllCredentialsWithConfigPathOverrideLeavesGlobalConfigUntouched(t *testing.T) {
	fixture := newScopedLogoutFixture(t)
	globalBefore := readConfigBytes(t, fixture.globalPath)

	if err := RemoveAllCredentialsWithOptions(RemoveOptions{}); err != nil {
		t.Fatalf("RemoveAllCredentials() error: %v", err)
	}

	requireConfigKeyNames(t, fixture.overridePath)
	if globalAfter := readConfigBytes(t, fixture.globalPath); !bytes.Equal(globalAfter, globalBefore) {
		t.Fatalf("global config changed:\nbefore: %s\nafter: %s", globalBefore, globalAfter)
	}
	requireKeychainEmpty(t, fixture.keyring)
}

func TestRemoveAllCredentialsIncludeGlobalClearsOverrideAndGlobalConfigs(t *testing.T) {
	fixture := newScopedLogoutFixture(t)

	if err := RemoveAllCredentialsWithOptions(RemoveOptions{IncludeGlobalConfig: true}); err != nil {
		t.Fatalf("RemoveAllCredentialsWithOptions() error: %v", err)
	}

	requireConfigKeyNames(t, fixture.overridePath)
	requireConfigKeyNames(t, fixture.globalPath)
	requireKeychainEmpty(t, fixture.keyring)
}

func TestRemoveAllCredentialsWithBypassKeychainLeavesKeychainUntouched(t *testing.T) {
	fixture := newScopedLogoutFixture(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	if err := RemoveAllCredentialsWithOptions(RemoveOptions{IncludeGlobalConfig: true}); err != nil {
		t.Fatalf("RemoveAllCredentialsWithOptions() error: %v", err)
	}

	requireConfigKeyNames(t, fixture.overridePath)
	requireConfigKeyNames(t, fixture.globalPath)
	if _, err := fixture.keyring.Get(keyringKey("shared")); err != nil {
		t.Fatalf("keychain credential removed despite ASC_BYPASS_KEYCHAIN: %v", err)
	}
}

func TestRemoveCredentialsWithConfigPathOverrideLeavesGlobalConfigUntouched(t *testing.T) {
	fixture := newScopedLogoutFixture(t)
	globalBefore := readConfigBytes(t, fixture.globalPath)

	if err := RemoveCredentialsWithOptions("shared", RemoveOptions{}); err != nil {
		t.Fatalf("RemoveCredentials() error: %v", err)
	}

	requireConfigKeyNames(t, fixture.overridePath, "override-only")
	if globalAfter := readConfigBytes(t, fixture.globalPath); !bytes.Equal(globalAfter, globalBefore) {
		t.Fatalf("global config changed:\nbefore: %s\nafter: %s", globalBefore, globalAfter)
	}
	requireKeychainEmpty(t, fixture.keyring)
}

func TestRemoveCredentialsIncludeGlobalRemovesFromOverrideAndGlobalConfigs(t *testing.T) {
	fixture := newScopedLogoutFixture(t)

	if err := RemoveCredentialsWithOptions("shared", RemoveOptions{IncludeGlobalConfig: true}); err != nil {
		t.Fatalf("RemoveCredentialsWithOptions() error: %v", err)
	}

	requireConfigKeyNames(t, fixture.overridePath, "override-only")
	requireConfigKeyNames(t, fixture.globalPath, "global-only")
	requireKeychainEmpty(t, fixture.keyring)
}

func TestRemoveCredentialsWithConfigPathOverrideDoesNotFindGlobalOnlyProfile(t *testing.T) {
	fixture := newScopedLogoutFixture(t)
	globalBefore := readConfigBytes(t, fixture.globalPath)

	err := RemoveCredentialsWithOptions("global-only", RemoveOptions{})
	if !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("RemoveCredentials() error = %v, want keyring.ErrKeyNotFound", err)
	}
	if globalAfter := readConfigBytes(t, fixture.globalPath); !bytes.Equal(globalAfter, globalBefore) {
		t.Fatalf("global config changed:\nbefore: %s\nafter: %s", globalBefore, globalAfter)
	}
}

func TestRemoveCredentialsWithMissingOverrideFileDoesNotFindGlobalOnlyProfile(t *testing.T) {
	fixture := newScopedLogoutFixture(t)
	if err := os.Remove(fixture.overridePath); err != nil {
		t.Fatalf("Remove(override) error: %v", err)
	}
	globalBefore := readConfigBytes(t, fixture.globalPath)

	err := RemoveCredentialsWithOptions("global-only", RemoveOptions{})
	if !errors.Is(err, keyring.ErrKeyNotFound) {
		t.Fatalf("RemoveCredentials() error = %v, want keyring.ErrKeyNotFound", err)
	}
	if globalAfter := readConfigBytes(t, fixture.globalPath); !bytes.Equal(globalAfter, globalBefore) {
		t.Fatalf("global config changed:\nbefore: %s\nafter: %s", globalBefore, globalAfter)
	}
}

func TestRemoveCredentialsWithoutConfigPathOverrideStillCleansActiveAndGlobalConfigs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	withArrayKeyring(t)
	t.Setenv("ASC_CONFIG_PATH", "")

	workDir := filepath.Join(t.TempDir(), "workspace")
	localPath := filepath.Join(workDir, ".asc", "config.json")
	globalPath := filepath.Join(home, ".asc", "config.json")
	for path, keyID := range map[string]string{localPath: "LOCAL", globalPath: "GLOBAL"} {
		if err := config.SaveAt(path, &config.Config{
			Keys: []config.Credential{
				{Name: "shared", KeyID: keyID, IssuerID: "ISSUER", PrivateKeyPath: "/tmp/shared.p8"},
				{Name: "keep", KeyID: keyID + "_KEEP", IssuerID: "ISSUER", PrivateKeyPath: "/tmp/keep.p8"},
			},
		}); err != nil {
			t.Fatalf("SaveAt(%s) error: %v", path, err)
		}
	}
	t.Chdir(workDir)

	if err := RemoveCredentialsWithOptions("shared", RemoveOptions{}); err != nil {
		t.Fatalf("RemoveCredentials() error: %v", err)
	}

	requireConfigKeyNames(t, localPath, "keep")
	requireConfigKeyNames(t, globalPath, "keep")
}

func TestRetainedGlobalConfigCredentials(t *testing.T) {
	t.Run("named profile left in global config", func(t *testing.T) {
		fixture := newScopedLogoutFixture(t)
		path, retained, err := RetainedGlobalConfigCredentials("shared")
		if err != nil {
			t.Fatalf("RetainedGlobalConfigCredentials() error: %v", err)
		}
		if !retained || path != fixture.globalPath {
			t.Fatalf("got (%q, %v), want (%q, true)", path, retained, fixture.globalPath)
		}
	})

	t.Run("named profile absent from global config", func(t *testing.T) {
		newScopedLogoutFixture(t)
		_, retained, err := RetainedGlobalConfigCredentials("override-only")
		if err != nil {
			t.Fatalf("RetainedGlobalConfigCredentials() error: %v", err)
		}
		if retained {
			t.Fatal("expected no retained credentials for a profile missing from the global config")
		}
	})

	t.Run("any credentials left in global config", func(t *testing.T) {
		fixture := newScopedLogoutFixture(t)
		path, retained, err := RetainedGlobalConfigCredentials("")
		if err != nil {
			t.Fatalf("RetainedGlobalConfigCredentials() error: %v", err)
		}
		if !retained || path != fixture.globalPath {
			t.Fatalf("got (%q, %v), want (%q, true)", path, retained, fixture.globalPath)
		}
	})

	t.Run("global config without credentials", func(t *testing.T) {
		fixture := newScopedLogoutFixture(t)
		if err := config.SaveAt(fixture.globalPath, &config.Config{AppID: "123"}); err != nil {
			t.Fatalf("SaveAt(global) error: %v", err)
		}
		_, retained, err := RetainedGlobalConfigCredentials("")
		if err != nil {
			t.Fatalf("RetainedGlobalConfigCredentials() error: %v", err)
		}
		if retained {
			t.Fatal("expected no retained credentials when the global config holds only settings")
		}
	})

	t.Run("global config with only keychain metadata", func(t *testing.T) {
		fixture := newScopedLogoutFixture(t)
		if err := config.SaveAt(fixture.globalPath, &config.Config{
			KeychainMetadata: []config.KeychainMetadata{{Name: "legacy", KeyID: "KEY", IssuerID: "ISS"}},
		}); err != nil {
			t.Fatalf("SaveAt(global) error: %v", err)
		}
		path, retained, err := RetainedGlobalConfigCredentials("")
		if err != nil {
			t.Fatalf("RetainedGlobalConfigCredentials() error: %v", err)
		}
		if !retained || path != fixture.globalPath {
			t.Fatalf("got (%q, %v), want (%q, true)", path, retained, fixture.globalPath)
		}
	})

	t.Run("missing global config", func(t *testing.T) {
		fixture := newScopedLogoutFixture(t)
		if err := os.Remove(fixture.globalPath); err != nil {
			t.Fatalf("Remove(global) error: %v", err)
		}
		_, retained, err := RetainedGlobalConfigCredentials("")
		if err != nil {
			t.Fatalf("RetainedGlobalConfigCredentials() error: %v", err)
		}
		if retained {
			t.Fatal("expected no retained credentials without a global config")
		}
	})

	t.Run("override unset", func(t *testing.T) {
		newScopedLogoutFixture(t)
		t.Setenv("ASC_CONFIG_PATH", "")
		t.Chdir(t.TempDir())
		_, retained, err := RetainedGlobalConfigCredentials("shared")
		if err != nil {
			t.Fatalf("RetainedGlobalConfigCredentials() error: %v", err)
		}
		if retained {
			t.Fatal("expected no retained credentials when cleanup already covers the global config")
		}
	})

	t.Run("override points at global config", func(t *testing.T) {
		fixture := newScopedLogoutFixture(t)
		t.Setenv("ASC_CONFIG_PATH", fixture.globalPath)
		_, retained, err := RetainedGlobalConfigCredentials("shared")
		if err != nil {
			t.Fatalf("RetainedGlobalConfigCredentials() error: %v", err)
		}
		if retained {
			t.Fatal("expected no retained credentials when the override is the global config")
		}
	})
}

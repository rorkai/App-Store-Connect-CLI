package auth

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/99designs/keyring"
)

func TestLegacyMigrationPreservesNonMacOSCredentials(t *testing.T) {
	for _, goos := range []string{"windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			t.Setenv("ASC_BYPASS_KEYCHAIN", "0")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			payload, err := json.Marshal(credentialPayload{KeyID: "TESTKEY", IssuerID: "TESTISSUER", PrivateKeyPath: "missing.p8"})
			if err != nil {
				t.Fatal(err)
			}
			kr := keyring.NewArrayKeyring([]keyring.Item{{Key: keyringKey("fixture"), Data: payload}})
			previous, previousLegacy := keyringOpener, legacyKeyringOpener
			t.Cleanup(func() { keyringOpener, legacyKeyringOpener = previous, previousLegacy })
			keyringOpener = func() (keyring.Keyring, error) { return kr, nil }
			legacyKeyringOpener = func() (keyring.Keyring, error) {
				// WinCred and Linux backends ignore KeychainName, so opening
				// the macOS legacy config would alias this current store.
				return openLegacyKeyringForOS(goos, func(keyring.Config) (keyring.Keyring, error) { return kr, nil })
			}
			for attempt := range 2 {
				cfg, source, err := GetCredentialsWithSource("fixture")
				if err != nil || cfg == nil || cfg.KeyID != "TESTKEY" || source != "keychain" {
					t.Fatalf("lookup %d lost secure credential: cfg=%+v source=%q err=%v", attempt, cfg, source, err)
				}
				if _, err := kr.Get(keyringKey("fixture")); err != nil {
					t.Fatalf("lookup %d deleted the current credential: %v", attempt, err)
				}
				credentials, err := listFromKeychain()
				if err != nil || len(credentials) != 1 {
					t.Fatalf("listing %d lost secure credential: %+v %v", attempt, credentials, err)
				}
			}
			if err := kr.Set(keyring.Item{Key: keyringKey("other"), Data: payload}); err != nil {
				t.Fatal(err)
			}
			if err := RemoveCredentialsWithOptions("fixture", RemoveOptions{}); err != nil {
				t.Fatalf("named logout with absent legacy store failed: %v", err)
			}
			if _, err := kr.Get(keyringKey("fixture")); !errors.Is(err, keyring.ErrKeyNotFound) {
				t.Fatal("named logout retained the selected credential")
			}
			if _, err := kr.Get(keyringKey("other")); err != nil {
				t.Fatal("named logout removed another profile")
			}
		})
	}
}

func TestLegacyKeyringStillOpensNamedMacOSKeychain(t *testing.T) {
	kr := keyring.NewArrayKeyring(nil)
	got, err := openLegacyKeyringForOS("darwin", func(cfg keyring.Config) (keyring.Keyring, error) {
		if cfg.KeychainName != legacyKeychain || cfg.ServiceName != keyringService {
			t.Fatalf("legacy keychain configuration changed: %+v", cfg)
		}
		return kr, nil
	})
	if err != nil || got != kr {
		t.Fatalf("legacy macOS opener returned %v, %v", got, err)
	}
}

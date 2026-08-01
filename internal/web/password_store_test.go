package web

import (
	"errors"
	"strings"
	"testing"

	"github.com/99designs/keyring"
)

func useTestPasswordKeyring(t *testing.T) keyring.Keyring {
	t.Helper()

	t.Setenv("ASC_BYPASS_KEYCHAIN", "")
	kr := keyring.NewArrayKeyring(nil)
	originalOpen := passwordKeyringOpen
	passwordKeyringOpen = func() (keyring.Keyring, error) {
		return kr, nil
	}
	t.Cleanup(func() {
		passwordKeyringOpen = originalOpen
	})
	return kr
}

func TestStoredWebPasswordRoundTripNormalizesAppleID(t *testing.T) {
	kr := useTestPasswordKeyring(t)

	if err := StorePassword(" User@Example.COM ", " secret "); err != nil {
		t.Fatalf("StorePassword() error = %v", err)
	}

	password, ok, err := LoadPassword("user@example.com")
	if err != nil {
		t.Fatalf("LoadPassword() error = %v", err)
	}
	if !ok {
		t.Fatal("LoadPassword() ok = false, want true")
	}
	if password != " secret " {
		t.Fatalf("LoadPassword() password = %q, want %q", password, " secret ")
	}

	keys, err := kr.Keys()
	if err != nil {
		t.Fatalf("Keys() error = %v", err)
	}
	if len(keys) != 1 || !strings.HasPrefix(keys[0], webPasswordKeyPrefix) {
		t.Fatalf("stored keys = %v, want one %q-prefixed key", keys, webPasswordKeyPrefix)
	}
	if strings.Contains(keys[0], "user@example.com") {
		t.Fatalf("stored key %q unexpectedly exposes the Apple ID", keys[0])
	}
}

func TestStoredWebPasswordsAreIsolatedAndDeletable(t *testing.T) {
	useTestPasswordKeyring(t)

	if err := StorePassword("one@example.com", "one-secret"); err != nil {
		t.Fatalf("StorePassword(one) error = %v", err)
	}
	if err := StorePassword("two@example.com", "two-secret"); err != nil {
		t.Fatalf("StorePassword(two) error = %v", err)
	}
	if err := DeletePassword("one@example.com"); err != nil {
		t.Fatalf("DeletePassword() error = %v", err)
	}

	if _, ok, err := LoadPassword("one@example.com"); err != nil || ok {
		t.Fatalf("LoadPassword(one) = (_, %v, %v), want (_, false, nil)", ok, err)
	}
	if password, ok, err := LoadPassword("two@example.com"); err != nil || !ok || password != "two-secret" {
		t.Fatalf("LoadPassword(two) = (%q, %v, %v), want (%q, true, nil)", password, ok, err, "two-secret")
	}

	if err := DeleteAllPasswords(); err != nil {
		t.Fatalf("DeleteAllPasswords() error = %v", err)
	}
	if _, ok, err := LoadPassword("two@example.com"); err != nil || ok {
		t.Fatalf("LoadPassword(two) after delete all = (_, %v, %v), want (_, false, nil)", ok, err)
	}
}

func TestStoredWebPasswordExistsDoesNotReturnSecret(t *testing.T) {
	useTestPasswordKeyring(t)

	if err := StorePassword("user@example.com", "secret"); err != nil {
		t.Fatalf("StorePassword() error = %v", err)
	}
	exists, err := PasswordStored("user@example.com")
	if err != nil {
		t.Fatalf("PasswordStored() error = %v", err)
	}
	if !exists {
		t.Fatal("PasswordStored() = false, want true")
	}
}

func TestStoredWebPasswordHonorsKeychainBypass(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	originalOpen := passwordKeyringOpen
	passwordKeyringOpen = func() (keyring.Keyring, error) {
		t.Fatal("password keyring should not be opened while keychain is bypassed")
		return nil, nil
	}
	t.Cleanup(func() {
		passwordKeyringOpen = originalOpen
	})

	if _, ok, err := LoadPassword("user@example.com"); err != nil || ok {
		t.Fatalf("LoadPassword() = (_, %v, %v), want (_, false, nil)", ok, err)
	}
	if err := StorePassword("user@example.com", "secret"); !errors.Is(err, ErrPasswordStoreUnavailable) {
		t.Fatalf("StorePassword() error = %v, want ErrPasswordStoreUnavailable", err)
	}
}

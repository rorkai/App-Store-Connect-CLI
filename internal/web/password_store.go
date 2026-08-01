package web

import (
	"errors"
	"fmt"
	"strings"

	"github.com/99designs/keyring"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
)

const (
	webPasswordKeyringService = "asc-web-password"
	webPasswordKeyPrefix      = "asc:web-password:"
)

// ErrPasswordStoreUnavailable indicates that native credential storage is
// unavailable or intentionally bypassed. Web login can continue without it.
var ErrPasswordStoreUnavailable = errors.New("native password store unavailable")

var passwordKeyringOpen = func() (keyring.Keyring, error) {
	return keyring.Open(keyring.Config{
		ServiceName:                    webPasswordKeyringService,
		KeychainTrustApplication:       true,
		KeychainSynchronizable:         false,
		KeychainAccessibleWhenUnlocked: true,
		AllowedBackends: []keyring.BackendType{
			keyring.KeychainBackend,
			keyring.WinCredBackend,
			keyring.SecretServiceBackend,
			keyring.KWalletBackend,
			keyring.KeyCtlBackend,
		},
	})
}

// PasswordStoreBypassed reports whether native credential-store access is
// intentionally disabled for this process.
func PasswordStoreBypassed() bool {
	return auth.ShouldBypassKeychain()
}

func normalizedPasswordAppleID(appleID string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(appleID))
	if normalized == "" {
		return "", fmt.Errorf("apple id is required")
	}
	return normalized, nil
}

func passwordItemKey(appleID string) (string, error) {
	normalized, err := normalizedPasswordAppleID(appleID)
	if err != nil {
		return "", err
	}
	return webPasswordKeyPrefix + normalized, nil
}

// LoadPassword loads a password for one Apple Account from the native
// credential store. It never falls back to a file-backed store.
func LoadPassword(appleID string) (string, bool, error) {
	if PasswordStoreBypassed() {
		return "", false, nil
	}
	key, err := passwordItemKey(appleID)
	if err != nil {
		return "", false, err
	}
	kr, err := passwordKeyringOpen()
	if err != nil {
		return "", false, fmt.Errorf("open native password store: %w", err)
	}
	item, err := kr.Get(key)
	if errors.Is(err, keyring.ErrKeyNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read native password store: %w", err)
	}
	if len(item.Data) == 0 {
		return "", false, nil
	}
	return string(item.Data), true, nil
}

// PasswordStored reports whether a password exists for one Apple Account.
func PasswordStored(appleID string) (bool, error) {
	if PasswordStoreBypassed() {
		return false, nil
	}
	key, err := passwordItemKey(appleID)
	if err != nil {
		return false, err
	}
	kr, err := passwordKeyringOpen()
	if err != nil {
		return false, fmt.Errorf("open native password store: %w", err)
	}
	if _, err := kr.GetMetadata(key); errors.Is(err, keyring.ErrKeyNotFound) {
		return false, nil
	} else if errors.Is(err, keyring.ErrMetadataNeedsCredentials) || errors.Is(err, keyring.ErrMetadataNotSupported) {
		// Some native backends cannot query existence without retrieving the
		// credential. Preserve status behavior there while keeping macOS
		// Keychain's metadata-only path secret-free.
		if _, err := kr.Get(key); errors.Is(err, keyring.ErrKeyNotFound) {
			return false, nil
		} else if err != nil {
			return false, fmt.Errorf("check native password store: %w", err)
		}
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("check native password store: %w", err)
	}
	return true, nil
}

// StorePassword saves a password for one Apple Account in the native
// credential store. The caller is responsible for only storing passwords that
// have already authenticated successfully.
func StorePassword(appleID, password string) error {
	if PasswordStoreBypassed() {
		return ErrPasswordStoreUnavailable
	}
	normalized, err := normalizedPasswordAppleID(appleID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("password is required")
	}
	key, err := passwordItemKey(normalized)
	if err != nil {
		return err
	}
	kr, err := passwordKeyringOpen()
	if err != nil {
		return fmt.Errorf("open native password store: %w", err)
	}
	if err := kr.Set(keyring.Item{
		Key:         key,
		Data:        []byte(password),
		Label:       "ASC Apple Account Password (" + normalized + ")",
		Description: "Password saved by asc web auth",
	}); err != nil {
		return fmt.Errorf("store native password: %w", err)
	}
	return nil
}

// DeletePassword removes the password for one Apple Account.
func DeletePassword(appleID string) error {
	if PasswordStoreBypassed() {
		return ErrPasswordStoreUnavailable
	}
	key, err := passwordItemKey(appleID)
	if err != nil {
		return err
	}
	kr, err := passwordKeyringOpen()
	if err != nil {
		return fmt.Errorf("open native password store: %w", err)
	}
	if err := kr.Remove(key); err != nil && !errors.Is(err, keyring.ErrKeyNotFound) {
		return fmt.Errorf("delete native password: %w", err)
	}
	return nil
}

// DeleteAllPasswords removes every password saved by asc web auth.
func DeleteAllPasswords() error {
	if PasswordStoreBypassed() {
		return ErrPasswordStoreUnavailable
	}
	kr, err := passwordKeyringOpen()
	if err != nil {
		return fmt.Errorf("open native password store: %w", err)
	}
	keys, err := kr.Keys()
	if err != nil {
		return fmt.Errorf("list native passwords: %w", err)
	}
	for _, key := range keys {
		if !strings.HasPrefix(key, webPasswordKeyPrefix) {
			continue
		}
		if err := kr.Remove(key); err != nil && !errors.Is(err, keyring.ErrKeyNotFound) {
			return fmt.Errorf("delete native password: %w", err)
		}
	}
	return nil
}

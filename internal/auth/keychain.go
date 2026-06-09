package auth

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/99designs/keyring"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

// ErrKeychainAccessDenied is returned when a keychain backend is available but
// access is explicitly denied (e.g. user clicked "Deny" on the prompt).
//
// This is distinct from keychain being unavailable (`keyring.ErrNoAvailImpl`).
var ErrKeychainAccessDenied = errors.New("keychain access denied")

// ErrDefaultCredentialsNotFound indicates that stored credentials exist, but no
// default selection resolves for the current no-profile lookup.
var ErrDefaultCredentialsNotFound = errors.New("default credentials not found")

var (
	invalidBypassKeychainWarningsMu sync.Mutex
	invalidBypassKeychainWarnings   = map[string]struct{}{}
)

const (
	keyringService    = "asc"
	keyringItemPrefix = "asc:credential:"
	keyringMetadataID = "asc:metadata:"
	legacyKeychain    = "asc"
	bypassKeychainEnv = "ASC_BYPASS_KEYCHAIN"
)

// Credential represents stored API credentials
type Credential struct {
	Name                  string    `json:"name"`
	KeyID                 string    `json:"key_id"`
	IssuerID              string    `json:"issuer_id"`
	PrivateKeyPath        string    `json:"private_key_path"`
	PrivateKeyPEM         string    `json:"-"`
	KeyType               string    `json:"key_type,omitempty"`
	IsDefault             bool      `json:"is_default"`
	Source                string    `json:"source,omitempty"`
	SourcePath            string    `json:"source_path,omitempty"`
	MetadataNeedsBackfill bool      `json:"-"`
	MetadataModifiedAt    time.Time `json:"-"`
}

// CredentialsWarning indicates that some credential sources could not be read.
// Credentials returned alongside the warning are still usable.
type CredentialsWarning struct {
	err error
}

func (w *CredentialsWarning) Error() string {
	return w.err.Error()
}

func (w *CredentialsWarning) Unwrap() error {
	return w.err
}

// Credentials stores multiple credentials
type Credentials struct {
	DefaultKey string       `json:"default_key"`
	Keys       []Credential `json:"keys"`
}

// MigrateKeychainToConfigOptions controls keychain-to-config migration.
type MigrateKeychainToConfigOptions struct {
	ConfigPath     string
	PrivateKeyDir  string
	RemoveKeychain bool
}

// MigratedCredential reports one profile copied into config.json.
type MigratedCredential struct {
	Name               string `json:"name"`
	KeyID              string `json:"keyId"`
	PrivateKeyPath     string `json:"privateKeyPath"`
	ExportedPrivateKey bool   `json:"exportedPrivateKey,omitempty"`
	keychainName       string
}

// MigrateKeychainToConfigResult summarizes a keychain-to-config migration.
type MigrateKeychainToConfigResult struct {
	ConfigPath          string               `json:"configPath"`
	PrivateKeyDir       string               `json:"privateKeyDir,omitempty"`
	Migrated            []MigratedCredential `json:"migrated"`
	RemovedFromKeychain bool                 `json:"removedFromKeychain"`
	Warnings            []string             `json:"warnings,omitempty"`
}

type credentialPayload struct {
	KeyID          string `json:"key_id"`
	IssuerID       string `json:"issuer_id"`
	PrivateKeyPath string `json:"private_key_path"`
	PrivateKeyPEM  string `json:"private_key_pem,omitempty"`
	KeyType        string `json:"key_type,omitempty"`
}

type credentialMetadata struct {
	KeyID    string `json:"key_id,omitempty"`
	IssuerID string `json:"issuer_id,omitempty"`
	KeyType  string `json:"key_type,omitempty"`
}

func keyringConfig(keychainName string) keyring.Config {
	cfg := keyring.Config{
		ServiceName:                    keyringService,
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
	}
	if keychainName != "" {
		cfg.KeychainName = keychainName
	}
	return cfg
}

func shouldBypassKeychain() bool {
	value, ok := os.LookupEnv(bypassKeychainEnv)
	if !ok {
		return false
	}
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return false
	}
	switch trimmed {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		warnInvalidBypassKeychainValueOnce(trimmed)
		return false
	}
}

func warnInvalidBypassKeychainValueOnce(value string) {
	if value == "" {
		return
	}

	invalidBypassKeychainWarningsMu.Lock()
	if _, ok := invalidBypassKeychainWarnings[value]; ok {
		invalidBypassKeychainWarningsMu.Unlock()
		return
	}
	invalidBypassKeychainWarnings[value] = struct{}{}
	invalidBypassKeychainWarningsMu.Unlock()

	fmt.Fprintf(
		os.Stderr,
		"Warning: invalid %s value %q (expected true/false, 1/0, yes/no, or on/off); %s\n",
		bypassKeychainEnv,
		value,
		"keychain bypass disabled",
	)
}

func resetInvalidBypassKeychainWarnings() {
	invalidBypassKeychainWarningsMu.Lock()
	defer invalidBypassKeychainWarningsMu.Unlock()
	invalidBypassKeychainWarnings = map[string]struct{}{}
}

// ShouldBypassKeychain reports whether keychain usage is disabled via env.
func ShouldBypassKeychain() bool {
	return shouldBypassKeychain()
}

// KeychainAvailable reports whether a system keychain backend is available.
func KeychainAvailable() (bool, error) {
	if shouldBypassKeychain() {
		return false, nil
	}
	_, err := keyringOpener()
	if err == nil {
		return true, nil
	}
	if isKeyringUnavailable(err) {
		return false, nil
	}
	return false, err
}

var keyringOpener = func() (keyring.Keyring, error) {
	return keyring.Open(keyringConfig(""))
}

var legacyKeyringOpener = func() (keyring.Keyring, error) {
	return keyring.Open(keyringConfig(legacyKeychain))
}

// ValidateKeyFile validates that the private key file exists and is valid
func ValidateKeyFile(path string) error {
	return validateKeyFileForOS(path, runtime.GOOS)
}

func validateKeyFileForOS(path, goos string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open key file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat key file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("private key path is a directory")
	}
	if filePermissionsTooPermissiveForOS(info.Mode(), goos) {
		return fmt.Errorf("private key file is too permissive; run: chmod 600 %q", path)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	// Parse the PEM block
	block, _ := pem.Decode(data)
	if block == nil {
		return fmt.Errorf("invalid PEM data")
	}

	// Try to parse as PKCS8 (App Store Connect keys are ECDSA)
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if _, ok := key.(*ecdsa.PrivateKey); ok {
			return nil
		}
		return fmt.Errorf("private key is not ECDSA")
	}

	// Try SEC1 EC private key as fallback
	if _, err := x509.ParseECPrivateKey(block.Bytes); err != nil {
		return fmt.Errorf("invalid private key format: %w", err)
	}

	return nil
}

// LoadPrivateKey loads the private key from the file
func LoadPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}
	return LoadPrivateKeyFromPEM(data)
}

// LoadPrivateKeyFromPEM loads an ECDSA private key from PEM bytes.
func LoadPrivateKeyFromPEM(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM data")
	}

	// Try PKCS8 first.
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		ecdsaKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not ECDSA")
		}
		return ecdsaKey, nil
	}

	// Try SEC1 EC private key as fallback.
	ecdsaKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	return ecdsaKey, nil
}

// StoreCredentials stores credentials in the keychain when available.
func StoreCredentials(name, keyID, issuerID, keyPath string) error {
	return StoreCredentialsWithKeyType(name, keyID, issuerID, keyPath, config.CredentialKeyTypeTeam)
}

// StoreCredentialsWithKeyType stores credentials with an explicit App Store Connect key type.
func StoreCredentialsWithKeyType(name, keyID, issuerID, keyPath, keyType string) error {
	payload := credentialPayload{
		KeyID:          keyID,
		IssuerID:       issuerID,
		PrivateKeyPath: keyPath,
		KeyType:        normalizedStoredKeyType(keyType),
	}
	if privateKeyPEM, err := loadPrivateKeyPEMForStorage(keyPath); err == nil && strings.TrimSpace(privateKeyPEM) != "" {
		payload.PrivateKeyPEM = privateKeyPEM
	}

	if err := storeInKeychain(name, payload); err == nil {
		// Successfully stored in keychain - remove matching config entry for security
		if err := removeFromConfigIfPresent(name); err != nil && !errors.Is(err, config.ErrNotFound) {
			// Log but don't fail - keychain is the authoritative storage
			_ = err
		}
		return saveDefaultName(name)
	} else if !isKeyringUnavailable(err) {
		return err
	}

	return storeInConfig(name, payload)
}

func loadPrivateKeyPEMForStorage(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// StoreCredentialsConfig stores credentials in the config file only.
func StoreCredentialsConfig(name, keyID, issuerID, keyPath string) error {
	return StoreCredentialsConfigWithKeyType(name, keyID, issuerID, keyPath, config.CredentialKeyTypeTeam)
}

// StoreCredentialsConfigWithKeyType stores credentials in the config file only with an explicit key type.
func StoreCredentialsConfigWithKeyType(name, keyID, issuerID, keyPath, keyType string) error {
	payload := credentialPayload{
		KeyID:          keyID,
		IssuerID:       issuerID,
		PrivateKeyPath: keyPath,
		KeyType:        normalizedStoredKeyType(keyType),
	}
	path, err := config.GlobalPath()
	if err != nil {
		return err
	}
	return storeInConfigAt(name, payload, path)
}

// StoreCredentialsConfigAt stores credentials in the specified config file.
func StoreCredentialsConfigAt(name, keyID, issuerID, keyPath, configPath string) error {
	return StoreCredentialsConfigAtWithKeyType(name, keyID, issuerID, keyPath, configPath, config.CredentialKeyTypeTeam)
}

// StoreCredentialsConfigAtWithKeyType stores credentials in the specified config file with an explicit key type.
func StoreCredentialsConfigAtWithKeyType(name, keyID, issuerID, keyPath, configPath, keyType string) error {
	payload := credentialPayload{
		KeyID:          keyID,
		IssuerID:       issuerID,
		PrivateKeyPath: keyPath,
		KeyType:        normalizedStoredKeyType(keyType),
	}
	return storeInConfigAt(name, payload, configPath)
}

func normalizedStoredKeyType(keyType string) string {
	normalized := config.NormalizeCredentialKeyType(keyType)
	if normalized == config.CredentialKeyTypeTeam {
		return ""
	}
	return normalized
}

// MigrateKeychainToConfig copies keychain-backed credentials into config.json.
//
// If a keychain entry contains embedded PEM data and its original private key
// path is missing, the PEM is exported to a secure .p8 file and config.json is
// updated to reference that file.
func MigrateKeychainToConfig(opts MigrateKeychainToConfigOptions) (MigrateKeychainToConfigResult, error) {
	configPath, err := resolveMigrationConfigPath(opts.ConfigPath)
	if err != nil {
		return MigrateKeychainToConfigResult{}, err
	}
	privateKeyDir, err := resolveMigrationPrivateKeyDir(opts.PrivateKeyDir, configPath)
	if err != nil {
		return MigrateKeychainToConfigResult{}, err
	}
	result := MigrateKeychainToConfigResult{
		ConfigPath:    configPath,
		PrivateKeyDir: privateKeyDir,
	}

	credentials, err := listFromKeychain()
	if err != nil {
		if isKeyringUnavailable(err) {
			return result, fmt.Errorf("keychain unavailable: %w", err)
		}
		return result, err
	}
	if len(credentials) == 0 {
		return result, fmt.Errorf("no keychain credentials found")
	}

	cfg, err := config.LoadAt(configPath)
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return result, err
	}
	if cfg == nil {
		cfg = &config.Config{}
	}

	migratedConfigCreds := make([]config.Credential, 0, len(credentials))
	seenConfigNames := make(map[string]string, len(credentials))
	for _, cred := range credentials {
		name := strings.TrimSpace(cred.Name)
		if name == "" {
			continue
		}
		if previousKeychainName, exists := seenConfigNames[name]; exists && previousKeychainName != cred.Name {
			return result, fmt.Errorf(
				"multiple keychain credentials normalize to config profile %q: %q and %q",
				name,
				previousKeychainName,
				cred.Name,
			)
		}
		seenConfigNames[name] = cred.Name
		privateKeyPath, exported, err := migrationPrivateKeyPath(cred, privateKeyDir, name)
		if err != nil {
			return result, err
		}
		configCred := config.Credential{
			Name:           name,
			KeyID:          strings.TrimSpace(cred.KeyID),
			IssuerID:       strings.TrimSpace(cred.IssuerID),
			PrivateKeyPath: privateKeyPath,
			KeyType:        normalizedStoredKeyType(cred.KeyType),
		}
		upsertConfigCredential(cfg, configCred)
		migratedConfigCreds = append(migratedConfigCreds, configCred)
		result.Migrated = append(result.Migrated, MigratedCredential{
			Name:               configCred.Name,
			KeyID:              configCred.KeyID,
			PrivateKeyPath:     configCred.PrivateKeyPath,
			ExportedPrivateKey: exported,
			keychainName:       cred.Name,
		})
	}
	if len(result.Migrated) == 0 {
		return result, fmt.Errorf("no named keychain credentials found")
	}

	applyMigratedDefault(cfg, credentials, migratedConfigCreds)
	if err := config.SaveAt(configPath, cfg); err != nil {
		return result, err
	}

	if opts.RemoveKeychain {
		removedAll := true
		for _, cred := range result.Migrated {
			if err := removeMigratedKeychainCredential(cred.keychainName); err != nil {
				removedAll = false
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to remove keychain credential %q: %v", cred.Name, err))
			}
		}
		result.RemovedFromKeychain = removedAll
	}

	return result, nil
}

func resolveMigrationConfigPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return config.Path()
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("invalid config path: %w", err)
	}
	return filepath.Clean(abs), nil
}

func resolveMigrationPrivateKeyDir(raw, configPath string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = filepath.Join(filepath.Dir(configPath), "keys")
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("invalid private key directory: %w", err)
	}
	return filepath.Clean(abs), nil
}

func migrationPrivateKeyPath(cred Credential, privateKeyDir string, configName string) (string, bool, error) {
	currentPath := strings.TrimSpace(cred.PrivateKeyPath)
	if currentPath != "" {
		info, err := os.Stat(currentPath)
		if err == nil {
			if info.IsDir() {
				return "", false, fmt.Errorf("profile %q private key path is a directory: %s", cred.Name, currentPath)
			}
			return currentPath, false, nil
		}
		if !os.IsNotExist(err) {
			return "", false, fmt.Errorf("profile %q private key file could not be read: %w", cred.Name, err)
		}
	}

	privateKeyPEM := cred.PrivateKeyPEM
	if strings.TrimSpace(privateKeyPEM) == "" {
		return "", false, fmt.Errorf("profile %q private key file is missing and keychain entry does not contain embedded private key PEM", cred.Name)
	}
	if _, err := LoadPrivateKeyFromPEM([]byte(privateKeyPEM)); err != nil {
		return "", false, fmt.Errorf("profile %q keychain private key PEM is invalid: %w", cred.Name, err)
	}

	exportPath := filepath.Join(privateKeyDir, migrationPrivateKeyFilename(configName, cred.KeyID))
	if err := writePrivateKeyPEMFile(exportPath, privateKeyPEM); err != nil {
		return "", false, fmt.Errorf("profile %q failed to export private key: %w", cred.Name, err)
	}
	return exportPath, true, nil
}

func migrationPrivateKeyFilename(name, keyID string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "default"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	safe := strings.Trim(b.String(), "._-")
	if safe == "" {
		safe = "default"
	}
	keyID = strings.TrimSpace(keyID)
	if keyID != "" {
		return "AuthKey_" + safe + "_" + keyID + ".p8"
	}
	return "AuthKey_" + safe + ".p8"
}

func writePrivateKeyPEMFile(path, privateKeyPEM string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("empty private key path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to overwrite symlink: %s", path)
		}
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.TrimSpace(string(existing)) != strings.TrimSpace(privateKeyPEM) {
			return fmt.Errorf("refusing to overwrite existing private key file: %s", path)
		}
		if filePermissionsTooPermissive(info.Mode()) {
			if err := os.Chmod(path, 0o600); err != nil {
				return err
			}
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(privateKeyPEM)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func upsertConfigCredential(cfg *config.Config, cred config.Credential) {
	for i := range cfg.Keys {
		if strings.TrimSpace(cfg.Keys[i].Name) == cred.Name {
			cfg.Keys[i] = cred
			return
		}
	}
	cfg.Keys = append(cfg.Keys, cred)
}

func applyMigratedDefault(cfg *config.Config, sourceCreds []Credential, migratedCreds []config.Credential) {
	destinationDefaultName := strings.TrimSpace(cfg.DefaultKeyName)
	if destinationDefaultName != "" {
		if alignDefaultCredentialFields(cfg, destinationDefaultName) {
			return
		}
	}

	defaultName := destinationDefaultName
	for _, cred := range sourceCreds {
		if cred.IsDefault {
			defaultName = strings.TrimSpace(cred.Name)
			break
		}
	}
	if defaultName == "" && len(migratedCreds) == 1 {
		defaultName = migratedCreds[0].Name
	}
	if defaultName == "" {
		return
	}
	cfg.DefaultKeyName = defaultName
	if alignDefaultCredentialFields(cfg, defaultName) {
		return
	}
	for _, cred := range migratedCreds {
		if cred.Name == defaultName {
			cfg.KeyID = cred.KeyID
			cfg.IssuerID = cred.IssuerID
			cfg.PrivateKeyPath = cred.PrivateKeyPath
			cfg.KeyType = normalizedStoredKeyType(cred.KeyType)
			return
		}
	}
}

func alignDefaultCredentialFields(cfg *config.Config, defaultName string) bool {
	cred, found, complete := findConfigCredential(cfg, defaultName)
	if !found || !complete {
		return false
	}
	cfg.DefaultKeyName = strings.TrimSpace(cred.Name)
	cfg.KeyID = cred.KeyID
	cfg.IssuerID = cred.IssuerID
	cfg.PrivateKeyPath = cred.PrivateKeyPath
	cfg.KeyType = normalizedStoredKeyType(cred.KeyType)
	return true
}

func removeMigratedKeychainCredential(name string) error {
	if err := removeFromKeychain(name); err != nil &&
		!errors.Is(err, keyring.ErrKeyNotFound) {
		return err
	}
	if err := removeFromLegacyKeychain(name); err != nil &&
		!errors.Is(err, keyring.ErrKeyNotFound) &&
		!isKeyringUnavailable(err) {
		return err
	}
	return nil
}

// clearConfigCredentials clears credentials from the config file.
// This is called after successfully migrating to keychain storage.
func clearConfigCredentials() error {
	paths, err := configCleanupPaths()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := clearConfigCredentialsAt(path); err != nil && !errors.Is(err, config.ErrNotFound) {
			return err
		}
	}
	return nil
}

func clearConfigCredentialsAt(path string) error {
	cfg, err := config.LoadAt(path)
	if err != nil {
		return err
	}
	cfg.KeyID = ""
	cfg.IssuerID = ""
	cfg.PrivateKeyPath = ""
	cfg.KeyType = ""
	cfg.DefaultKeyName = ""
	cfg.Keys = nil
	cfg.KeychainMetadata = nil
	return config.SaveAt(path, cfg)
}

// ListCredentials lists all stored credentials from all sources.
// Credentials are merged from keychain and config, with keychain taking
// precedence when the same name exists in both sources.
func ListCredentials() ([]Credential, error) {
	if shouldBypassKeychain() {
		credentials, err := listFromConfig()
		if err != nil {
			return nil, err
		}
		normalizeCredentialDefaults(credentials)
		return credentials, nil
	}

	return listCredentialsFromSources(listFromKeychain)
}

// ListCredentialSummaries lists stored credentials without reading key material
// when the active keyring backend exposes non-secret metadata.
func ListCredentialSummaries() ([]Credential, error) {
	if shouldBypassKeychain() {
		credentials, err := listFromConfig()
		if err != nil {
			return nil, err
		}
		normalizeCredentialDefaults(credentials)
		return credentials, nil
	}

	return listCredentialsFromSources(listCredentialSummariesFromKeychain)
}

func listCredentialsFromSources(keychainList func() ([]Credential, error)) ([]Credential, error) {
	keychainCreds, keychainErr := keychainList()
	if keychainErr != nil && !isKeyringUnavailable(keychainErr) {
		return nil, keychainErr
	}

	configCreds, configErr := listFromConfig()
	if configErr != nil {
		// If keychain worked, return those even if config failed
		if keychainErr == nil {
			normalizeCredentialDefaults(keychainCreds)
			return keychainCreds, &CredentialsWarning{
				err: fmt.Errorf("config credentials could not be read: %w", configErr),
			}
		}
		return nil, configErr
	}

	// If keychain is unavailable, return only config credentials
	if keychainErr != nil {
		normalizeCredentialDefaults(configCreds)
		return configCreds, nil
	}

	// Merge: keychain credentials take precedence for same names
	merged := mergeCredentials(keychainCreds, configCreds)
	normalizeCredentialDefaults(merged)
	return merged, nil
}

// mergeCredentials combines credentials from two sources, with the first
// source taking precedence when the same name exists in both.
func mergeCredentials(primary, secondary []Credential) []Credential {
	if len(primary) == 0 {
		return secondary
	}
	if len(secondary) == 0 {
		return primary
	}

	seen := make(map[string]struct{}, len(primary))
	for _, cred := range primary {
		seen[cred.Name] = struct{}{}
	}

	merged := make([]Credential, len(primary), len(primary)+len(secondary))
	copy(merged, primary)

	for _, cred := range secondary {
		if _, exists := seen[cred.Name]; !exists {
			merged = append(merged, cred)
		}
	}

	return merged
}

func normalizeCredentialDefaults(credentials []Credential) {
	if len(credentials) == 0 {
		return
	}
	defaultName, err := defaultName()
	if err != nil {
		defaultName = ""
	}
	defaultName = strings.TrimSpace(defaultName)
	for i := range credentials {
		credentials[i].IsDefault = false
	}
	if defaultName != "" {
		for i := range credentials {
			if credentials[i].Name == defaultName {
				credentials[i].IsDefault = true
				return
			}
		}
		return
	}
	if len(credentials) == 1 {
		credentials[0].IsDefault = true
	}
}

// RemoveCredentials removes a named credential.
func RemoveCredentials(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("credential name is required")
	}
	err := removeFromKeychain(name)
	if err == nil {
		if configErr := removeFromConfigIfPresent(name); configErr != nil &&
			!errors.Is(configErr, config.ErrNotFound) &&
			!errors.Is(configErr, keyring.ErrKeyNotFound) {
			return configErr
		}
		_ = removeFromLegacyKeychain(name)
		return clearDefaultNameIf(name)
	}
	if isKeyringUnavailable(err) {
		return removeFromConfigIfPresent(name)
	}
	if errors.Is(err, keyring.ErrKeyNotFound) {
		legacyErr := removeFromLegacyKeychain(name)
		if legacyErr == nil {
			if configErr := removeFromConfigIfPresent(name); configErr != nil &&
				!errors.Is(configErr, config.ErrNotFound) &&
				!errors.Is(configErr, keyring.ErrKeyNotFound) {
				return configErr
			}
			return clearDefaultNameIf(name)
		}
		if isKeyringUnavailable(legacyErr) {
			return removeFromConfigIfPresent(name)
		}
		if errors.Is(legacyErr, keyring.ErrKeyNotFound) {
			if err := removeFromConfigIfPresent(name); err != nil {
				return err
			}
			return nil
		}
		return legacyErr
	}
	return err
}

// RemoveAllCredentials removes all stored credentials
func RemoveAllCredentials() error {
	// Always attempt to clear config credentials first, regardless of keychain state
	// This ensures config is cleaned even if keychain has issues (e.g., locked, read-only)
	configErr := clearConfigCredentials()

	// Try to clear keychain as well, but don't fail if keychain has issues
	keychainErr := removeAllFromKeychain()
	if keychainErr == nil {
		_ = removeAllFromLegacyKeychain()
		return configErr // Return config error if any
	}

	// If keychain failed but config succeeded, that's acceptable
	if configErr == nil {
		return nil
	}

	// Both failed - return keychain error as primary
	return keychainErr
}

func sameConfigPath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

// GetDefaultCredentials returns the default credentials.
func GetDefaultCredentials() (*config.Config, error) {
	return GetCredentials("")
}

// GetCredentialsWithSource returns credentials for a named profile along with the source.
func GetCredentialsWithSource(profile string) (*config.Config, string, error) {
	profile = strings.TrimSpace(profile)
	if shouldBypassKeychain() {
		configCfg, err := getCredentialsFromConfig(profile)
		if err != nil {
			return nil, "", err
		}
		return configCfg, "config", nil
	}

	credentials, err := listFromKeychain()
	if err == nil {
		defaultKey := ""
		resolvedProfile := profile
		if profile == "" {
			defaultKey, err = defaultName()
			if err != nil {
				return nil, "", err
			}
			defaultKey = strings.TrimSpace(defaultKey)
			resolvedProfile = defaultKey
		}
		cfg, selectedCred, found := selectCredential(resolvedProfile, credentials)
		if found {
			maybeBackfillCredentialMetadata(selectedCred)
			return cfg, "keychain", nil
		}
		if profile != "" {
			if cfg, configErr := getCredentialsFromConfig(profile); configErr == nil {
				return cfg, "config", nil
			}
			return nil, "", fmt.Errorf("credentials not found for profile %q", profile)
		}
		if defaultKey != "" {
			configCfg, configErr := getCredentialsFromConfig(defaultKey)
			if configErr != nil {
				return nil, "", configErr
			}
			return configCfg, "config", nil
		}
		if len(credentials) > 0 {
			return nil, "", ErrDefaultCredentialsNotFound
		}
		configCfg, err := getCredentialsFromConfig(profile)
		if err != nil {
			return nil, "", err
		}
		return configCfg, "config", nil
	}
	if !isKeyringUnavailable(err) {
		if isKeychainAccessDeniedError(err) {
			return nil, "", fmt.Errorf("%w: %w", ErrKeychainAccessDenied, err)
		}
		return nil, "", err
	}
	cfg, err := getCredentialsFromConfig(profile)
	if err != nil {
		return nil, "", err
	}
	return cfg, "config", nil
}

func isKeychainAccessDeniedError(err error) bool {
	if err == nil {
		return false
	}

	// keyring's keychain backend doesn't wrap go-keychain errors with %w; it
	// typically stringifies them. Use the trailing OSStatus code as the stable signal.
	//
	// Common denial/cancel style codes:
	// - errSecAuthFailed (-25293): user denied / auth failed
	// - errSecInteractionNotAllowed (-25308): interaction not allowed
	// - errSecNoAccessForItem (-25291): no access for item
	if code, ok := parseTrailingOSStatus(err.Error()); ok {
		switch code {
		case -25293, -25308, -25291:
			return true
		}
	}

	// Fallback: match message fragments in case the OSStatus code is lost.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "interaction is not allowed") ||
		strings.Contains(msg, "passphrase you entered is not correct") ||
		strings.Contains(msg, "no access")
}

func parseTrailingOSStatus(message string) (int, bool) {
	// Expected format from go-keychain: "... (-25293)"
	message = strings.TrimSpace(message)
	end := strings.LastIndex(message, ")")
	start := strings.LastIndex(message, "(")
	if start < 0 || end < 0 || end <= start+1 || end != len(message)-1 {
		return 0, false
	}
	raw := strings.TrimSpace(message[start+1 : end])
	if raw == "" {
		return 0, false
	}
	code, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return code, true
}

// GetCredentials returns credentials for a named profile.
func GetCredentials(profile string) (*config.Config, error) {
	cfg, _, err := GetCredentialsWithSource(profile)
	return cfg, err
}

func selectCredential(profile string, credentials []Credential) (*config.Config, Credential, bool) {
	name := strings.TrimSpace(profile)
	if name != "" {
		for _, cred := range credentials {
			if cred.Name == name {
				return configFromCredential(cred), cred, true
			}
		}
		return nil, Credential{}, false
	}
	if len(credentials) == 1 {
		cred := credentials[0]
		return configFromCredential(cred), cred, true
	}
	return nil, Credential{}, false
}

func maybeBackfillCredentialMetadata(cred Credential) {
	if !cred.MetadataNeedsBackfill {
		return
	}
	if strings.TrimSpace(cred.PrivateKeyPEM) == "" {
		return
	}
	persistKeychainMetadata(cred)
}

func configFromCredential(cred Credential) *config.Config {
	return &config.Config{
		KeyID:          cred.KeyID,
		IssuerID:       cred.IssuerID,
		PrivateKeyPath: cred.PrivateKeyPath,
		PrivateKeyPEM:  cred.PrivateKeyPEM,
		KeyType:        normalizedStoredKeyType(cred.KeyType),
		DefaultKeyName: cred.Name,
	}
}

func getCredentialsFromConfig(profile string) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return nil, err
	}
	if cfg != nil {
		selected, selectErr := selectConfigCredential(cfg, profile)
		if selectErr == nil {
			return selected, nil
		}
		if hasAnyCredentials(cfg) {
			return nil, selectErr
		}
	}

	globalCfg, _, globalErr := loadGlobalConfigForCredentials()
	if globalErr != nil {
		if errors.Is(globalErr, config.ErrNotFound) {
			if errors.Is(err, config.ErrNotFound) {
				return nil, err
			}
			return nil, ErrDefaultCredentialsNotFound
		}
		return nil, globalErr
	}
	selected, selectErr := selectConfigCredential(globalCfg, profile)
	if selectErr != nil {
		return nil, selectErr
	}
	return selected, nil
}

func isKeyringUnavailable(err error) bool {
	return errors.Is(err, keyring.ErrNoAvailImpl)
}

func keyringKey(name string) string {
	return keyringItemPrefix + name
}

func keyringLabel(name string) string {
	return fmt.Sprintf("ASC API Key (%s)", name)
}

func credentialMetadataDescription(payload credentialPayload) string {
	data, err := json.Marshal(credentialMetadata{
		KeyID:    strings.TrimSpace(payload.KeyID),
		IssuerID: strings.TrimSpace(payload.IssuerID),
		KeyType:  normalizedStoredKeyType(payload.KeyType),
	})
	if err != nil {
		return ""
	}
	return keyringMetadataID + string(data)
}

func parseCredentialMetadataDescription(description string) credentialMetadata {
	description = strings.TrimSpace(description)
	if description == "" || !strings.HasPrefix(description, keyringMetadataID) {
		return credentialMetadata{}
	}
	raw := strings.TrimPrefix(description, keyringMetadataID)
	var metadata credentialMetadata
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return credentialMetadata{}
	}
	return metadata
}

func hasCredentialMetadata(metadata credentialMetadata) bool {
	return strings.TrimSpace(metadata.KeyID) != "" ||
		strings.TrimSpace(metadata.IssuerID) != "" ||
		strings.TrimSpace(metadata.KeyType) != ""
}

func metadataModifiedAtString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func credentialMetadataMatchesPayload(metadata credentialMetadata, payload credentialPayload) bool {
	return strings.TrimSpace(metadata.KeyID) == strings.TrimSpace(payload.KeyID) &&
		strings.TrimSpace(metadata.IssuerID) == strings.TrimSpace(payload.IssuerID) &&
		config.NormalizeCredentialKeyType(metadata.KeyType) == config.NormalizeCredentialKeyType(payload.KeyType)
}

func storedKeychainMetadataSummary(entry config.KeychainMetadata) credentialMetadata {
	return credentialMetadata{
		KeyID:    strings.TrimSpace(entry.KeyID),
		IssuerID: strings.TrimSpace(entry.IssuerID),
		KeyType:  normalizedStoredKeyType(entry.KeyType),
	}
}

func storedKeychainMetadataMatches(entry config.KeychainMetadata, modifiedAt time.Time) bool {
	if strings.TrimSpace(entry.ModifiedAt) == "" || modifiedAt.IsZero() {
		return false
	}
	return strings.TrimSpace(entry.ModifiedAt) == metadataModifiedAtString(modifiedAt)
}

func loadStoredKeychainMetadata() map[string]config.KeychainMetadata {
	path, err := config.Path()
	if err != nil {
		return map[string]config.KeychainMetadata{}
	}
	cfg, err := config.LoadAt(path)
	if err != nil {
		return map[string]config.KeychainMetadata{}
	}
	stored := make(map[string]config.KeychainMetadata, len(cfg.KeychainMetadata))
	for _, entry := range cfg.KeychainMetadata {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		entry.Name = name
		entry.KeyID = strings.TrimSpace(entry.KeyID)
		entry.IssuerID = strings.TrimSpace(entry.IssuerID)
		entry.KeyType = normalizedStoredKeyType(entry.KeyType)
		entry.ModifiedAt = strings.TrimSpace(entry.ModifiedAt)
		stored[name] = entry
	}
	return stored
}

func persistKeychainMetadata(cred Credential) {
	name := strings.TrimSpace(cred.Name)
	if name == "" {
		return
	}
	path, err := config.Path()
	if err != nil {
		return
	}
	cfg, err := config.LoadAt(path)
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return
	}
	if cfg == nil {
		cfg = &config.Config{}
	}
	metadata := config.KeychainMetadata{
		Name:       name,
		KeyID:      strings.TrimSpace(cred.KeyID),
		IssuerID:   strings.TrimSpace(cred.IssuerID),
		KeyType:    normalizedStoredKeyType(cred.KeyType),
		ModifiedAt: metadataModifiedAtString(cred.MetadataModifiedAt),
	}
	if (metadata.KeyID == "" && metadata.IssuerID == "" && metadata.KeyType == "") || metadata.ModifiedAt == "" {
		return
	}
	updated := false
	for i := range cfg.KeychainMetadata {
		if strings.TrimSpace(cfg.KeychainMetadata[i].Name) == name {
			cfg.KeychainMetadata[i] = metadata
			updated = true
			break
		}
	}
	if !updated {
		cfg.KeychainMetadata = append(cfg.KeychainMetadata, metadata)
	}
	_ = config.SaveAt(path, cfg)
}

func metadataRequiresSecretRead(err error) bool {
	return errors.Is(err, keyring.ErrMetadataNeedsCredentials) ||
		errors.Is(err, keyring.ErrMetadataNotSupported)
}

func keyringItemForCredential(name string, payload credentialPayload) (keyring.Item, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return keyring.Item{}, fmt.Errorf("failed to encode credentials: %w", err)
	}
	return keyring.Item{
		Key:         keyringKey(name),
		Data:        data,
		Label:       keyringLabel(name),
		Description: credentialMetadataDescription(payload),
	}, nil
}

func storeInKeychain(name string, payload credentialPayload) error {
	kr, err := keyringOpener()
	if err != nil {
		return err
	}
	item, err := keyringItemForCredential(name, payload)
	if err != nil {
		return err
	}
	return kr.Set(item)
}

func listFromKeychain() ([]Credential, error) {
	kr, err := keyringOpener()
	if err != nil {
		return nil, err
	}
	credentials, err := listFromKeyring(kr)
	if err != nil {
		return nil, err
	}

	legacy, err := listFromLegacyKeychain()
	if err == nil && len(legacy) > 0 {
		existing := make(map[string]struct{}, len(credentials))
		for _, cred := range credentials {
			existing[cred.Name] = struct{}{}
		}

		var toMigrate []Credential
		for _, cred := range legacy {
			if _, ok := existing[cred.Name]; ok {
				_ = removeFromLegacyKeychain(cred.Name)
				continue
			}
			credentials = append(credentials, cred)
			toMigrate = append(toMigrate, cred)
		}

		if len(toMigrate) > 0 {
			migrateLegacyCredentials(toMigrate)
		}
	}
	defaultName, _ := defaultName()
	if strings.TrimSpace(defaultName) == "" && len(credentials) == 1 {
		credentials[0].IsDefault = true
	}
	return credentials, nil
}

func listCredentialSummariesFromKeychain() ([]Credential, error) {
	kr, err := keyringOpener()
	if err != nil {
		return nil, err
	}
	credentials, err := listCredentialSummariesFromKeyring(kr)
	if err != nil {
		return nil, err
	}

	legacy, err := listCredentialSummariesFromLegacyKeychain()
	if err == nil && len(legacy) > 0 {
		existing := make(map[string]struct{}, len(credentials))
		for _, cred := range credentials {
			existing[cred.Name] = struct{}{}
		}
		for _, cred := range legacy {
			if _, ok := existing[cred.Name]; ok {
				continue
			}
			credentials = append(credentials, cred)
		}
	}

	defaultName, _ := defaultName()
	if strings.TrimSpace(defaultName) == "" && len(credentials) == 1 {
		credentials[0].IsDefault = true
	}
	return credentials, nil
}

func listFromLegacyKeychain() ([]Credential, error) {
	kr, err := legacyKeyringOpener()
	if err != nil {
		return nil, err
	}
	return listFromKeyring(kr)
}

func listCredentialSummariesFromLegacyKeychain() ([]Credential, error) {
	kr, err := legacyKeyringOpener()
	if err != nil {
		return nil, err
	}
	return listCredentialSummariesFromKeyring(kr)
}

func listCredentialSummariesFromKeyring(kr keyring.Keyring) ([]Credential, error) {
	keys, err := kr.Keys()
	if err != nil {
		return nil, err
	}

	defaultName, _ := defaultName()
	storedMetadata := loadStoredKeychainMetadata()
	credentials := []Credential{}
	for _, key := range keys {
		if !strings.HasPrefix(key, keyringItemPrefix) {
			continue
		}
		metadata, err := kr.GetMetadata(key)
		if err != nil {
			if errors.Is(err, keyring.ErrKeyNotFound) {
				continue
			}
			if metadataRequiresSecretRead(err) {
				return listFromKeyring(kr)
			}
			return nil, err
		}
		if metadata.Item == nil {
			return listFromKeyring(kr)
		}
		name := strings.TrimPrefix(key, keyringItemPrefix)
		summary := parseCredentialMetadataDescription(metadata.Description)
		if !hasCredentialMetadata(summary) {
			if stored, ok := storedMetadata[name]; ok && storedKeychainMetadataMatches(stored, metadata.ModificationTime) {
				summary = storedKeychainMetadataSummary(stored)
			}
		}
		credentials = append(credentials, Credential{
			Name:      name,
			KeyID:     summary.KeyID,
			IssuerID:  summary.IssuerID,
			KeyType:   normalizedStoredKeyType(summary.KeyType),
			IsDefault: name == defaultName,
			Source:    "keychain",
		})
	}

	return credentials, nil
}

func listFromKeyring(kr keyring.Keyring) ([]Credential, error) {
	keys, err := kr.Keys()
	if err != nil {
		return nil, err
	}

	defaultName, _ := defaultName()
	storedMetadata := loadStoredKeychainMetadata()
	credentials := []Credential{}
	for _, key := range keys {
		if !strings.HasPrefix(key, keyringItemPrefix) {
			continue
		}
		item, err := kr.Get(key)
		if err != nil {
			if errors.Is(err, keyring.ErrKeyNotFound) {
				continue
			}
			return nil, err
		}
		var payload credentialPayload
		if err := json.Unmarshal(item.Data, &payload); err != nil {
			return nil, fmt.Errorf("invalid keychain entry %q: %w", key, err)
		}
		name := strings.TrimPrefix(key, keyringItemPrefix)
		descriptionMetadata := parseCredentialMetadataDescription(item.Description)
		metadataNeedsBackfill := !hasCredentialMetadata(descriptionMetadata)
		metadataModifiedAt := time.Time{}
		if metadataInfo, metadataErr := kr.GetMetadata(key); metadataErr == nil {
			metadataModifiedAt = metadataInfo.ModificationTime
		}
		if metadataNeedsBackfill {
			if stored, ok := storedMetadata[name]; ok &&
				storedKeychainMetadataMatches(stored, metadataModifiedAt) &&
				credentialMetadataMatchesPayload(storedKeychainMetadataSummary(stored), payload) {
				metadataNeedsBackfill = false
			}
		}
		needsRewrite := false
		if strings.TrimSpace(payload.PrivateKeyPEM) == "" {
			if privateKeyPEM, err := loadPrivateKeyPEMForStorage(payload.PrivateKeyPath); err == nil && strings.TrimSpace(privateKeyPEM) != "" {
				payload.PrivateKeyPEM = privateKeyPEM
				needsRewrite = true
				metadataNeedsBackfill = false
			}
		}
		if needsRewrite {
			updatedItem, marshalErr := keyringItemForCredential(name, payload)
			if marshalErr == nil {
				_ = kr.Set(updatedItem)
			}
		}
		credentials = append(credentials, Credential{
			Name:                  name,
			KeyID:                 payload.KeyID,
			IssuerID:              payload.IssuerID,
			PrivateKeyPath:        payload.PrivateKeyPath,
			PrivateKeyPEM:         payload.PrivateKeyPEM,
			KeyType:               normalizedStoredKeyType(payload.KeyType),
			IsDefault:             name == defaultName,
			Source:                "keychain",
			MetadataNeedsBackfill: metadataNeedsBackfill,
			MetadataModifiedAt:    metadataModifiedAt,
		})
	}

	return credentials, nil
}

func migrateLegacyCredentials(credentials []Credential) {
	for _, cred := range credentials {
		payload := credentialPayload{
			KeyID:          cred.KeyID,
			IssuerID:       cred.IssuerID,
			PrivateKeyPath: cred.PrivateKeyPath,
			PrivateKeyPEM:  cred.PrivateKeyPEM,
			KeyType:        normalizedStoredKeyType(cred.KeyType),
		}
		if err := storeInKeychain(cred.Name, payload); err != nil {
			continue
		}
		_ = removeFromLegacyKeychain(cred.Name)
	}
}

func removeFromConfigIfPresent(name string) error {
	paths, err := configCleanupPaths()
	if err != nil {
		return err
	}

	removed := false
	missingCredential := false
	for _, path := range paths {
		err := removeFromConfigAt(name, path)
		switch {
		case err == nil:
			removed = true
		case errors.Is(err, config.ErrNotFound):
			continue
		case errors.Is(err, keyring.ErrKeyNotFound):
			missingCredential = true
		default:
			return err
		}
	}

	if removed {
		return nil
	}
	if missingCredential {
		return keyring.ErrKeyNotFound
	}
	return nil
}

func removeFromKeychain(name string) error {
	kr, err := keyringOpener()
	if err != nil {
		return err
	}
	return kr.Remove(keyringKey(name))
}

func removeFromLegacyKeychain(name string) error {
	kr, err := legacyKeyringOpener()
	if err != nil {
		return err
	}
	return kr.Remove(keyringKey(name))
}

func removeAllFromKeychain() error {
	kr, err := keyringOpener()
	if err != nil {
		return err
	}
	keys, err := kr.Keys()
	if err != nil {
		return err
	}
	for _, key := range keys {
		if strings.HasPrefix(key, keyringItemPrefix) {
			if err := kr.Remove(key); err != nil {
				return err
			}
		}
	}
	return nil
}

func removeAllFromLegacyKeychain() error {
	kr, err := legacyKeyringOpener()
	if err != nil {
		return err
	}
	keys, err := kr.Keys()
	if err != nil {
		return err
	}
	for _, key := range keys {
		if strings.HasPrefix(key, keyringItemPrefix) {
			if err := kr.Remove(key); err != nil {
				return err
			}
		}
	}
	return nil
}

func storeInConfig(name string, payload credentialPayload) error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	return storeInConfigAt(name, payload, path)
}

func storeInConfigAt(name string, payload credentialPayload, configPath string) error {
	cfg, err := config.LoadAt(configPath)
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return err
	}
	if cfg == nil {
		cfg = &config.Config{}
	}

	name = strings.TrimSpace(name)
	updated := false
	for i, cred := range cfg.Keys {
		if strings.TrimSpace(cred.Name) == name {
			cfg.Keys[i].Name = name
			cfg.Keys[i].KeyID = payload.KeyID
			cfg.Keys[i].IssuerID = payload.IssuerID
			cfg.Keys[i].PrivateKeyPath = payload.PrivateKeyPath
			cfg.Keys[i].KeyType = normalizedStoredKeyType(payload.KeyType)
			updated = true
			break
		}
	}
	if !updated {
		cfg.Keys = append(cfg.Keys, config.Credential{
			Name:           name,
			KeyID:          payload.KeyID,
			IssuerID:       payload.IssuerID,
			PrivateKeyPath: payload.PrivateKeyPath,
			KeyType:        normalizedStoredKeyType(payload.KeyType),
		})
	}

	cfg.KeyID = payload.KeyID
	cfg.IssuerID = payload.IssuerID
	cfg.PrivateKeyPath = payload.PrivateKeyPath
	cfg.KeyType = normalizedStoredKeyType(payload.KeyType)
	cfg.DefaultKeyName = name
	return config.SaveAt(configPath, cfg)
}

func hasCompleteCredentials(cfg *config.Config) bool {
	return len(configCredentialList(cfg)) > 0
}

func hasAnyCredentials(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if strings.TrimSpace(cfg.KeyID) != "" ||
		strings.TrimSpace(cfg.IssuerID) != "" ||
		strings.TrimSpace(cfg.PrivateKeyPath) != "" ||
		strings.TrimSpace(cfg.KeyType) != "" {
		return true
	}
	for _, cred := range cfg.Keys {
		if strings.TrimSpace(cred.Name) != "" ||
			strings.TrimSpace(cred.KeyID) != "" ||
			strings.TrimSpace(cred.IssuerID) != "" ||
			strings.TrimSpace(cred.PrivateKeyPath) != "" ||
			strings.TrimSpace(cred.KeyType) != "" {
			return true
		}
	}
	return false
}

func isCompleteConfigCredential(cred config.Credential) bool {
	hasIssuer := strings.TrimSpace(cred.IssuerID) != "" ||
		config.IsIndividualCredentialKeyType(cred.KeyType)
	return strings.TrimSpace(cred.KeyID) != "" &&
		hasIssuer &&
		strings.TrimSpace(cred.PrivateKeyPath) != ""
}

func hasLegacyCredentials(cfg *config.Config) bool {
	return cfg != nil &&
		strings.TrimSpace(cfg.KeyID) != "" &&
		(strings.TrimSpace(cfg.IssuerID) != "" || config.IsIndividualCredentialKeyType(cfg.KeyType)) &&
		strings.TrimSpace(cfg.PrivateKeyPath) != ""
}

func configCredentialList(cfg *config.Config) []config.Credential {
	if cfg == nil {
		return nil
	}
	credentials := make([]config.Credential, 0, len(cfg.Keys)+1)
	seen := make(map[string]struct{})
	for _, cred := range cfg.Keys {
		name := strings.TrimSpace(cred.Name)
		if name == "" || !isCompleteConfigCredential(cred) {
			continue
		}
		cred.Name = name
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		credentials = append(credentials, cred)
	}

	if hasLegacyCredentials(cfg) {
		name := strings.TrimSpace(cfg.DefaultKeyName)
		if name == "" {
			name = "default"
		}
		if _, ok := seen[name]; !ok {
			credentials = append(credentials, config.Credential{
				Name:           name,
				KeyID:          cfg.KeyID,
				IssuerID:       cfg.IssuerID,
				PrivateKeyPath: cfg.PrivateKeyPath,
				KeyType:        normalizedStoredKeyType(cfg.KeyType),
			})
		}
	}

	return credentials
}

func findConfigCredential(cfg *config.Config, name string) (config.Credential, bool, bool) {
	if cfg == nil {
		return config.Credential{}, false, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return config.Credential{}, false, false
	}
	for _, cred := range cfg.Keys {
		if strings.TrimSpace(cred.Name) != name {
			continue
		}
		cred.Name = name
		return cred, true, isCompleteConfigCredential(cred)
	}
	legacyName := strings.TrimSpace(cfg.DefaultKeyName)
	if legacyName == "" {
		legacyName = "default"
	}
	if name == legacyName && (strings.TrimSpace(cfg.KeyID) != "" ||
		strings.TrimSpace(cfg.IssuerID) != "" ||
		strings.TrimSpace(cfg.PrivateKeyPath) != "" ||
		strings.TrimSpace(cfg.KeyType) != "") {
		cred := config.Credential{
			Name:           legacyName,
			KeyID:          cfg.KeyID,
			IssuerID:       cfg.IssuerID,
			PrivateKeyPath: cfg.PrivateKeyPath,
			KeyType:        normalizedStoredKeyType(cfg.KeyType),
		}
		return cred, true, isCompleteConfigCredential(cred)
	}
	return config.Credential{}, false, false
}

func applyConfigCredential(cfg *config.Config, cred config.Credential) *config.Config {
	if cfg == nil {
		return &config.Config{
			KeyID:          cred.KeyID,
			IssuerID:       cred.IssuerID,
			PrivateKeyPath: cred.PrivateKeyPath,
			KeyType:        normalizedStoredKeyType(cred.KeyType),
			DefaultKeyName: strings.TrimSpace(cred.Name),
		}
	}
	copied := *cfg
	copied.KeyID = cred.KeyID
	copied.IssuerID = cred.IssuerID
	copied.PrivateKeyPath = cred.PrivateKeyPath
	copied.KeyType = normalizedStoredKeyType(cred.KeyType)
	if strings.TrimSpace(cred.Name) != "" {
		copied.DefaultKeyName = strings.TrimSpace(cred.Name)
	}
	return &copied
}

func selectConfigCredential(cfg *config.Config, profile string) (*config.Config, error) {
	if cfg == nil {
		return nil, config.ErrNotFound
	}

	profile = strings.TrimSpace(profile)
	if profile != "" {
		cred, found, complete := findConfigCredential(cfg, profile)
		if !found {
			return nil, fmt.Errorf("credentials not found for profile %q", profile)
		}
		if !complete {
			return nil, fmt.Errorf("incomplete credentials for profile %q", profile)
		}
		return applyConfigCredential(cfg, cred), nil
	}

	defaultName := strings.TrimSpace(cfg.DefaultKeyName)
	if defaultName != "" {
		cred, found, complete := findConfigCredential(cfg, defaultName)
		if !found {
			return nil, ErrDefaultCredentialsNotFound
		}
		if !complete {
			return nil, fmt.Errorf("incomplete credentials for profile %q", defaultName)
		}
		return applyConfigCredential(cfg, cred), nil
	}

	credentials := configCredentialList(cfg)
	if len(credentials) == 1 {
		return applyConfigCredential(cfg, credentials[0]), nil
	}
	if hasAnyCredentials(cfg) {
		return nil, ErrDefaultCredentialsNotFound
	}
	return nil, config.ErrNotFound
}

func loadGlobalConfigForCredentials() (*config.Config, string, error) {
	if strings.TrimSpace(os.Getenv("ASC_CONFIG_PATH")) != "" {
		return nil, "", config.ErrNotFound
	}
	path, err := config.GlobalPath()
	if err != nil {
		return nil, "", err
	}
	cfg, err := config.LoadAt(path)
	if err != nil {
		return nil, "", err
	}
	return cfg, path, nil
}

func listFromConfig() ([]Credential, error) {
	path, err := config.Path()
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadAt(path)
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return []Credential{}, nil
		}
		return nil, err
	}
	if !hasCompleteCredentials(cfg) {
		if hasAnyCredentials(cfg) {
			return []Credential{}, nil
		}
		globalCfg, globalPath, err := loadGlobalConfigForCredentials()
		if err != nil {
			if errors.Is(err, config.ErrNotFound) {
				return []Credential{}, nil
			}
			return nil, err
		}
		if !hasCompleteCredentials(globalCfg) {
			return []Credential{}, nil
		}
		cfg = globalCfg
		path = globalPath
	}
	configCreds := configCredentialList(cfg)
	if len(configCreds) == 0 {
		return []Credential{}, nil
	}
	defaultName := strings.TrimSpace(cfg.DefaultKeyName)
	if defaultName == "" && len(configCreds) == 1 {
		defaultName = configCreds[0].Name
	}
	credentials := make([]Credential, 0, len(configCreds))
	for _, cred := range configCreds {
		credentials = append(credentials, Credential{
			Name:           cred.Name,
			KeyID:          cred.KeyID,
			IssuerID:       cred.IssuerID,
			PrivateKeyPath: cred.PrivateKeyPath,
			KeyType:        normalizedStoredKeyType(cred.KeyType),
			IsDefault:      cred.Name == defaultName,
			Source:         "config",
			SourcePath:     path,
		})
	}
	return credentials, nil
}

// SetDefaultCredentials sets the default profile name for credential resolution.
func SetDefaultCredentials(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("default profile name is required")
	}
	return saveDefaultName(name)
}

func saveDefaultName(name string) error {
	cfg, err := config.Load()
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return err
	}
	if cfg == nil {
		cfg = &config.Config{}
	}
	trimmedName := strings.TrimSpace(name)
	cfg.DefaultKeyName = trimmedName
	if trimmedName != "" {
		for _, cred := range cfg.Keys {
			if strings.TrimSpace(cred.Name) == trimmedName {
				cfg.KeyID = cred.KeyID
				cfg.IssuerID = cred.IssuerID
				cfg.PrivateKeyPath = cred.PrivateKeyPath
				cfg.KeyType = normalizedStoredKeyType(cred.KeyType)
				return config.Save(cfg)
			}
		}
	}
	return config.Save(cfg)
}

func defaultName() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(cfg.DefaultKeyName), nil
}

func clearDefaultNameIf(name string) error {
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(cfg.DefaultKeyName) == strings.TrimSpace(name) {
		cfg.DefaultKeyName = ""
		return config.Save(cfg)
	}
	return nil
}

func removeFromConfigAt(name, path string) error {
	cfg, err := config.LoadAt(path)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		cfg.KeyID = ""
		cfg.IssuerID = ""
		cfg.PrivateKeyPath = ""
		cfg.KeyType = ""
		cfg.DefaultKeyName = ""
		cfg.Keys = nil
		cfg.KeychainMetadata = nil
		return config.SaveAt(path, cfg)
	}

	removed := false
	if len(cfg.Keys) > 0 {
		filtered := cfg.Keys[:0]
		for _, cred := range cfg.Keys {
			if strings.TrimSpace(cred.Name) == name {
				removed = true
				continue
			}
			filtered = append(filtered, cred)
		}
		cfg.Keys = filtered
	}
	if len(cfg.KeychainMetadata) > 0 {
		filteredMetadata := cfg.KeychainMetadata[:0]
		for _, entry := range cfg.KeychainMetadata {
			if strings.TrimSpace(entry.Name) == name {
				removed = true
				continue
			}
			filteredMetadata = append(filteredMetadata, entry)
		}
		cfg.KeychainMetadata = filteredMetadata
	}

	if strings.TrimSpace(cfg.DefaultKeyName) == name {
		cfg.KeyID = ""
		cfg.IssuerID = ""
		cfg.PrivateKeyPath = ""
		cfg.KeyType = ""
		cfg.DefaultKeyName = ""
		removed = true
	}
	if !removed {
		return keyring.ErrKeyNotFound
	}
	return config.SaveAt(path, cfg)
}

func configCleanupPaths() ([]string, error) {
	activePath, err := config.Path()
	if err != nil {
		return nil, err
	}
	globalPath, err := config.GlobalPath()
	if err != nil {
		return nil, err
	}
	paths := []string{activePath}
	if !sameConfigPath(activePath, globalPath) {
		paths = append(paths, globalPath)
	}
	return paths, nil
}

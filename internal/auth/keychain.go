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
	"strings"

	"github.com/99designs/keyring"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

const (
	keyringService    = "asc"
	keyringItemPrefix = "asc:credential:"
	legacyKeychain    = "asc"
	bypassKeychainEnv = "ASC_BYPASS_KEYCHAIN"
)

// Credential represents stored API credentials
type Credential struct {
	Name           string `json:"name"`
	KeyID          string `json:"key_id"`
	IssuerID       string `json:"issuer_id"`
	PrivateKeyPath string `json:"private_key_path"`
	IsDefault      bool   `json:"is_default"`
	Source         string `json:"source,omitempty"`
	SourcePath     string `json:"source_path,omitempty"`
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

type credentialPayload struct {
	KeyID          string `json:"key_id"`
	IssuerID       string `json:"issuer_id"`
	PrivateKeyPath string `json:"private_key_path"`
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
	switch trimmed {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return false
	}
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
	if info.Mode().Perm()&0o077 != 0 {
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

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM data")
	}

	// Try PKCS8 first
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		ecdsaKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not ECDSA")
		}
		return ecdsaKey, nil
	}

	// Try SEC1 EC private key as fallback
	ecdsaKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	return ecdsaKey, nil
}

// StoreCredentials stores credentials in the keychain when available.
func StoreCredentials(name, keyID, issuerID, keyPath string) error {
	payload := credentialPayload{
		KeyID:          keyID,
		IssuerID:       issuerID,
		PrivateKeyPath: keyPath,
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

// StoreCredentialsConfig stores credentials in the config file only.
func StoreCredentialsConfig(name, keyID, issuerID, keyPath string) error {
	payload := credentialPayload{
		KeyID:          keyID,
		IssuerID:       issuerID,
		PrivateKeyPath: keyPath,
	}
	path, err := config.GlobalPath()
	if err != nil {
		return err
	}
	return storeInConfigAt(name, payload, path)
}

// StoreCredentialsConfigAt stores credentials in the specified config file.
func StoreCredentialsConfigAt(name, keyID, issuerID, keyPath, configPath string) error {
	payload := credentialPayload{
		KeyID:          keyID,
		IssuerID:       issuerID,
		PrivateKeyPath: keyPath,
	}
	return storeInConfigAt(name, payload, configPath)
}

// clearConfigCredentials clears credentials from the config file.
// This is called after successfully migrating to keychain storage.
func clearConfigCredentials() error {
	activePath, err := config.Path()
	if err != nil {
		return err
	}
	globalPath, err := config.GlobalPath()
	if err != nil {
		return err
	}
	if err := clearConfigCredentialsAt(activePath); err != nil && !errors.Is(err, config.ErrNotFound) {
		return err
	}
	if !sameConfigPath(activePath, globalPath) {
		if err := clearConfigCredentialsAt(globalPath); err != nil && !errors.Is(err, config.ErrNotFound) {
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
	cfg.DefaultKeyName = ""
	cfg.Keys = nil
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

	keychainCreds, keychainErr := listFromKeychain()
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
		_ = removeFromLegacyKeychain(name)
		return clearDefaultNameIf(name)
	}
	if isKeyringUnavailable(err) {
		return removeFromConfigIfPresent(name)
	}
	if errors.Is(err, keyring.ErrKeyNotFound) {
		legacyErr := removeFromLegacyKeychain(name)
		if legacyErr == nil {
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
		cfg, found, err := selectCredential(resolvedProfile, credentials)
		if err != nil {
			return nil, "", err
		}
		if found {
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
			return nil, "", fmt.Errorf("default credentials not found")
		}
		configCfg, err := getCredentialsFromConfig(profile)
		if err != nil {
			return nil, "", err
		}
		return configCfg, "config", nil
	}
	if !isKeyringUnavailable(err) {
		return nil, "", err
	}
	cfg, err := getCredentialsFromConfig(profile)
	if err != nil {
		return nil, "", err
	}
	return cfg, "config", nil
}

// GetCredentials returns credentials for a named profile.
func GetCredentials(profile string) (*config.Config, error) {
	cfg, _, err := GetCredentialsWithSource(profile)
	return cfg, err
}

func selectCredential(profile string, credentials []Credential) (*config.Config, bool, error) {
	name := strings.TrimSpace(profile)
	if name != "" {
		for _, cred := range credentials {
			if cred.Name == name {
				return &config.Config{
					KeyID:          cred.KeyID,
					IssuerID:       cred.IssuerID,
					PrivateKeyPath: cred.PrivateKeyPath,
					DefaultKeyName: cred.Name,
				}, true, nil
			}
		}
		return nil, false, nil
	}
	if len(credentials) == 1 {
		cred := credentials[0]
		return &config.Config{
			KeyID:          cred.KeyID,
			IssuerID:       cred.IssuerID,
			PrivateKeyPath: cred.PrivateKeyPath,
			DefaultKeyName: cred.Name,
		}, true, nil
	}
	return nil, false, nil
}

func getCredentialsFromConfig(profile string) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil && err != config.ErrNotFound {
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
		if globalErr == config.ErrNotFound {
			if err == config.ErrNotFound {
				return nil, err
			}
			return nil, fmt.Errorf("default credentials not found")
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

func storeInKeychain(name string, payload credentialPayload) error {
	kr, err := keyringOpener()
	if err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode credentials: %w", err)
	}
	return kr.Set(keyring.Item{
		Key:   keyringKey(name),
		Data:  data,
		Label: fmt.Sprintf("ASC API Key (%s)", name),
	})
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

func listFromLegacyKeychain() ([]Credential, error) {
	kr, err := legacyKeyringOpener()
	if err != nil {
		return nil, err
	}
	return listFromKeyring(kr)
}

func listFromKeyring(kr keyring.Keyring) ([]Credential, error) {
	keys, err := kr.Keys()
	if err != nil {
		return nil, err
	}

	defaultName, _ := defaultName()
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
		credentials = append(credentials, Credential{
			Name:           name,
			KeyID:          payload.KeyID,
			IssuerID:       payload.IssuerID,
			PrivateKeyPath: payload.PrivateKeyPath,
			IsDefault:      name == defaultName,
			Source:         "keychain",
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
		}
		if err := storeInKeychain(cred.Name, payload); err != nil {
			continue
		}
		_ = removeFromLegacyKeychain(cred.Name)
	}
}

func removeFromConfigIfPresent(name string) error {
	if err := removeFromConfig(name); err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return nil
		}
		return err
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
	if err != nil && err != config.ErrNotFound {
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
		})
	}

	cfg.KeyID = payload.KeyID
	cfg.IssuerID = payload.IssuerID
	cfg.PrivateKeyPath = payload.PrivateKeyPath
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
		strings.TrimSpace(cfg.PrivateKeyPath) != "" {
		return true
	}
	for _, cred := range cfg.Keys {
		if strings.TrimSpace(cred.Name) != "" ||
			strings.TrimSpace(cred.KeyID) != "" ||
			strings.TrimSpace(cred.IssuerID) != "" ||
			strings.TrimSpace(cred.PrivateKeyPath) != "" {
			return true
		}
	}
	return false
}

func isCompleteConfigCredential(cred config.Credential) bool {
	return strings.TrimSpace(cred.KeyID) != "" &&
		strings.TrimSpace(cred.IssuerID) != "" &&
		strings.TrimSpace(cred.PrivateKeyPath) != ""
}

func hasLegacyCredentials(cfg *config.Config) bool {
	return cfg != nil &&
		strings.TrimSpace(cfg.KeyID) != "" &&
		strings.TrimSpace(cfg.IssuerID) != "" &&
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
		strings.TrimSpace(cfg.PrivateKeyPath) != "") {
		cred := config.Credential{
			Name:           legacyName,
			KeyID:          cfg.KeyID,
			IssuerID:       cfg.IssuerID,
			PrivateKeyPath: cfg.PrivateKeyPath,
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
			DefaultKeyName: strings.TrimSpace(cred.Name),
		}
	}
	copied := *cfg
	copied.KeyID = cred.KeyID
	copied.IssuerID = cred.IssuerID
	copied.PrivateKeyPath = cred.PrivateKeyPath
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
			return nil, fmt.Errorf("default credentials not found")
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
		return nil, fmt.Errorf("default credentials not found")
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
		if err == config.ErrNotFound {
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
			if err == config.ErrNotFound {
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
	if err != nil && err != config.ErrNotFound {
		return err
	}
	if cfg == nil {
		cfg = &config.Config{}
	}
	trimmedName := strings.TrimSpace(name)
	previousDefault := strings.TrimSpace(cfg.DefaultKeyName)
	if previousDefault == "" {
		previousDefault = "default"
	}
	cfg.DefaultKeyName = trimmedName
	if trimmedName != "" {
		for _, cred := range cfg.Keys {
			if strings.TrimSpace(cred.Name) == trimmedName {
				cfg.KeyID = cred.KeyID
				cfg.IssuerID = cred.IssuerID
				cfg.PrivateKeyPath = cred.PrivateKeyPath
				return config.Save(cfg)
			}
		}
	}
	if trimmedName != previousDefault {
		cfg.KeyID = ""
		cfg.IssuerID = ""
		cfg.PrivateKeyPath = ""
	}
	return config.Save(cfg)
}

func defaultName() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		if err == config.ErrNotFound {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(cfg.DefaultKeyName), nil
}

func clearDefaultNameIf(name string) error {
	cfg, err := config.Load()
	if err != nil {
		if err == config.ErrNotFound {
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

func removeFromConfig(name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		cfg.KeyID = ""
		cfg.IssuerID = ""
		cfg.PrivateKeyPath = ""
		cfg.DefaultKeyName = ""
		cfg.Keys = nil
		return config.Save(cfg)
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

	if strings.TrimSpace(cfg.DefaultKeyName) == name {
		cfg.KeyID = ""
		cfg.IssuerID = ""
		cfg.PrivateKeyPath = ""
		cfg.DefaultKeyName = ""
		removed = true
	}
	if !removed {
		return keyring.ErrKeyNotFound
	}
	return config.Save(cfg)
}

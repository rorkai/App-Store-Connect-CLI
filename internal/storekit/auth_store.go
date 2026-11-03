package storekit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/99designs/keyring"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

const (
	storeKitKeyringService       = "asc"
	storeKitKeyringItemPrefix    = "asc:storekit-credential:"
	storeKitBypassKeychainEnvVar = "ASC_STOREKIT_BYPASS_KEYCHAIN"
)

type StoredCredential struct {
	Credentials `json:"-"`
	Name        string `json:"name"`
	IsDefault   bool   `json:"default"`
	Source      string `json:"source"`
	SourcePath  string `json:"source_path,omitempty"`
}

type credentialPayload struct {
	KeyID          string `json:"key_id"`
	IssuerID       string `json:"issuer_id"`
	PrivateKeyPath string `json:"private_key_path,omitempty"`
	PrivateKeyPEM  string `json:"private_key_pem,omitempty"`
	BundleID       string `json:"bundle_id"`
}

// ShouldBypassKeychain reports whether StoreKit keychain access is disabled.
func ShouldBypassKeychain() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(storeKitBypassKeychainEnvVar))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// StoreCredentials stores a credential in the system keychain when available.
func StoreCredentials(name string, credentials Credentials) error {
	payload, err := payloadFromCredentials(credentials, true)
	if err != nil {
		return err
	}
	if !ShouldBypassKeychain() {
		if err := storeInKeychain(name, payload); err == nil {
			_ = removeFromConfigIfPresent(name)
			return saveDefaultName(name)
		} else if !isKeyringUnavailable(err) {
			return err
		}
	}
	return StoreCredentialsConfig(name, credentials)
}

func StoreCredentialsConfig(name string, credentials Credentials) error {
	path, err := storeKitConfigPath()
	if err != nil {
		return err
	}
	return StoreCredentialsConfigAt(name, credentials, path)
}

// storeKitConfigPath returns the config that StoreKit credentials and the
// default profile are written to: the active config when it has StoreKit
// config, otherwise the config DefaultWritePath chooses.
func storeKitConfigPath() (string, error) {
	path, err := config.Path()
	if err != nil {
		return "", err
	}
	cfg, err := config.LoadAt(path)
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return "", err
	}
	if hasStoreKitConfig(cfg) {
		return path, nil
	}
	return config.DefaultWritePath()
}

func StoreCredentialsConfigAt(name string, credentials Credentials, path string) error {
	payload, err := payloadFromCredentials(credentials, false)
	if err != nil {
		return err
	}
	return storeInConfigAt(name, payload, path)
}

func payloadFromCredentials(credentials Credentials, includePEM bool) (credentialPayload, error) {
	credentials = normalizeCredentials(credentials)
	payload := credentialPayload{
		KeyID:          credentials.KeyID,
		IssuerID:       credentials.IssuerID,
		PrivateKeyPath: credentials.PrivateKeyPath,
		BundleID:       credentials.BundleID,
	}
	if includePEM && credentials.PrivateKeyPEM != "" {
		payload.PrivateKeyPEM = credentials.PrivateKeyPEM
	} else if includePEM && credentials.PrivateKeyPath != "" {
		data, err := os.ReadFile(credentials.PrivateKeyPath)
		if err != nil {
			return payload, fmt.Errorf("read private key for keychain storage: %w", err)
		}
		payload.PrivateKeyPEM = string(data)
	}
	if err := validateCredentials(Credentials{
		KeyID:          payload.KeyID,
		IssuerID:       payload.IssuerID,
		PrivateKeyPath: payload.PrivateKeyPath,
		PrivateKeyPEM:  payload.PrivateKeyPEM,
		BundleID:       payload.BundleID,
	}); err != nil {
		return payload, err
	}
	return payload, nil
}

func GetCredentialsWithSource(profile string) (Credentials, string, error) {
	if strings.TrimSpace(profile) != "" {
		if selected, ok, err := findCredentialInActiveConfig(profile); err != nil {
			return Credentials{}, "", err
		} else if ok {
			return selected.Credentials, "config", nil
		}
		if selected, ok, err := findCredentialInResolvedConfig(profile); err == nil && ok {
			return selected.Credentials, "config", nil
		}
	} else if selected, ok, err := findDefaultCredentialInActiveConfig(); err != nil {
		return Credentials{}, "", err
	} else if ok {
		return selected.Credentials, "config", nil
	} else if selected, ok, err := findDefaultCredentialInResolvedConfig(); err == nil && ok {
		return selected.Credentials, "config", nil
	}
	if !ShouldBypassKeychain() {
		credentials, err := listFromKeychain()
		if err == nil {
			if selected, ok := selectCredential(profile, credentials); ok {
				return selected.Credentials, "keychain", nil
			}
			if strings.TrimSpace(profile) != "" {
				configCredential, configErr := getCredentialFromConfig(profile)
				if configErr == nil {
					return configCredential.Credentials, "config", nil
				}
				if !errors.Is(configErr, config.ErrNotFound) {
					return Credentials{}, "", configErr
				}
				return Credentials{}, "", fmt.Errorf("StoreKit credentials not found for profile %q", profile)
			}
			if configCredential, configErr := getCredentialFromConfig(""); configErr == nil {
				return configCredential.Credentials, "config", nil
			}
			if len(credentials) > 0 {
				return Credentials{}, "", fmt.Errorf("default StoreKit credentials not found")
			}
		} else if !isKeyringUnavailable(err) {
			return Credentials{}, "", err
		}
	}
	selected, err := getCredentialFromConfig(profile)
	if err != nil {
		return Credentials{}, "", err
	}
	return selected.Credentials, "config", nil
}

func ListCredentials() ([]StoredCredential, error) {
	var keychainCredentials []StoredCredential
	var keychainErr error
	if !ShouldBypassKeychain() {
		keychainCredentials, keychainErr = listFromKeychain()
		if keychainErr != nil && !isKeyringUnavailable(keychainErr) {
			return nil, keychainErr
		}
	}
	configCredentials, configErr := listFromConfig()
	if configErr != nil && !errors.Is(configErr, config.ErrNotFound) {
		if len(keychainCredentials) == 0 {
			return nil, configErr
		}
		configCredentials = nil
	}
	if keychainErr != nil || ShouldBypassKeychain() {
		return normalizeDefaults(configCredentials), nil
	}
	return normalizeDefaults(mergeCredentials(configCredentials, keychainCredentials)), nil
}

func SetDefaultCredentials(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("credential name is required")
	}
	credentials, err := ListCredentials()
	if err != nil {
		return err
	}
	for _, credential := range credentials {
		if credential.Name == name {
			if credential.Source == "config" && strings.TrimSpace(credential.SourcePath) != "" {
				return saveDefaultNameAt(name, credential.SourcePath)
			}
			return saveDefaultName(name)
		}
	}
	return fmt.Errorf("StoreKit credentials not found for profile %q", name)
}

func RemoveCredentials(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("credential name is required")
	}
	removed := false
	var keychainErr error
	if !ShouldBypassKeychain() {
		keychainErr = removeFromKeychain(name)
		if keychainErr == nil {
			removed = true
		}
	}
	configErr := removeFromConfigIfPresent(name)
	if configErr == nil {
		removed = true
	}
	if keychainErr != nil && !isKeyringUnavailable(keychainErr) && !errors.Is(keychainErr, keyring.ErrKeyNotFound) {
		return keychainErr
	}
	if configErr != nil && !errors.Is(configErr, config.ErrNotFound) && !errors.Is(configErr, keyring.ErrKeyNotFound) {
		return configErr
	}
	if !removed {
		return keyring.ErrKeyNotFound
	}
	return clearDefaultNameIf(name)
}

func RemoveAllCredentials() error {
	configErr := clearConfigCredentials()
	if ShouldBypassKeychain() {
		return configErr
	}
	keychainErr := removeAllFromKeychain()
	if keychainErr != nil && !isKeyringUnavailable(keychainErr) {
		return keychainErr
	}
	return configErr
}

func keyringConfig() keyring.Config {
	return keyring.Config{
		ServiceName:                    storeKitKeyringService,
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
}

var openStoreKitKeyring = func() (keyring.Keyring, error) {
	return keyring.Open(keyringConfig())
}

func storeInKeychain(name string, payload credentialPayload) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("credential name is required")
	}
	kr, err := openStoreKitKeyring()
	if err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return kr.Set(keyring.Item{
		Key:         storeKitKeyringItemPrefix + name,
		Data:        data,
		Label:       "ASC StoreKit In-App Purchase API Key (" + name + ")",
		Description: "StoreKit Retention Messaging credentials for " + payload.BundleID,
	})
}

func listFromKeychain() ([]StoredCredential, error) {
	kr, err := openStoreKitKeyring()
	if err != nil {
		return nil, err
	}
	keys, err := kr.Keys()
	if err != nil {
		return nil, err
	}
	credentials := []StoredCredential{}
	for _, key := range keys {
		if !strings.HasPrefix(key, storeKitKeyringItemPrefix) {
			continue
		}
		item, err := kr.Get(key)
		if err != nil {
			return nil, err
		}
		var payload credentialPayload
		if err := json.Unmarshal(item.Data, &payload); err != nil {
			return nil, fmt.Errorf("invalid StoreKit keychain entry %q: %w", key, err)
		}
		name := strings.TrimPrefix(key, storeKitKeyringItemPrefix)
		credentials = append(credentials, storedFromPayload(name, payload, "keychain", "system keychain"))
	}
	return credentials, nil
}

func removeFromKeychain(name string) error {
	kr, err := openStoreKitKeyring()
	if err != nil {
		return err
	}
	return kr.Remove(storeKitKeyringItemPrefix + strings.TrimSpace(name))
}

func removeAllFromKeychain() error {
	kr, err := openStoreKitKeyring()
	if err != nil {
		return err
	}
	keys, err := kr.Keys()
	if err != nil {
		return err
	}
	for _, key := range keys {
		if strings.HasPrefix(key, storeKitKeyringItemPrefix) {
			if err := kr.Remove(key); err != nil && !errors.Is(err, keyring.ErrKeyNotFound) {
				return err
			}
		}
	}
	return nil
}

func isKeyringUnavailable(err error) bool {
	return errors.Is(err, keyring.ErrNoAvailImpl)
}

func storeInConfigAt(name string, payload credentialPayload, path string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("credential name is required")
	}
	cfg, err := config.LoadAt(path)
	if err != nil {
		if !errors.Is(err, config.ErrNotFound) {
			return err
		}
		cfg = &config.Config{}
	}
	replacement := config.StoreKitCredential{
		Name:           name,
		KeyID:          payload.KeyID,
		IssuerID:       payload.IssuerID,
		PrivateKeyPath: payload.PrivateKeyPath,
		BundleID:       payload.BundleID,
	}
	replaced := false
	for i := range cfg.StoreKit.Keys {
		if cfg.StoreKit.Keys[i].Name == name {
			cfg.StoreKit.Keys[i] = replacement
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.StoreKit.Keys = append(cfg.StoreKit.Keys, replacement)
	}
	cfg.StoreKit.DefaultKeyName = name
	return config.SaveAt(path, cfg)
}

func removeFromConfigIfPresent(name string) error {
	paths, err := configStoragePaths()
	if err != nil {
		return err
	}
	removed := false
	var mutationErrors []error
	for _, path := range paths {
		err := removeFromConfigAtIfPresent(name, path)
		if err == nil {
			removed = true
			continue
		}
		if errors.Is(err, config.ErrNotFound) || errors.Is(err, keyring.ErrKeyNotFound) {
			continue
		}
		mutationErrors = append(mutationErrors, fmt.Errorf("update StoreKit config %q: %w", path, err))
	}
	if len(mutationErrors) > 0 {
		return errors.Join(mutationErrors...)
	}
	if !removed {
		return keyring.ErrKeyNotFound
	}
	return nil
}

func removeFromConfigAtIfPresent(name, path string) error {
	cfg, err := config.LoadAt(path)
	if err != nil {
		return err
	}
	filtered := cfg.StoreKit.Keys[:0]
	removed := false
	for _, credential := range cfg.StoreKit.Keys {
		if credential.Name == name {
			removed = true
			continue
		}
		filtered = append(filtered, credential)
	}
	if !removed {
		return keyring.ErrKeyNotFound
	}
	cfg.StoreKit.Keys = filtered
	if cfg.StoreKit.DefaultKeyName == name {
		cfg.StoreKit.DefaultKeyName = ""
	}
	return config.SaveAt(path, cfg)
}

func clearConfigCredentials() error {
	paths, err := configStoragePaths()
	if err != nil {
		return err
	}
	var mutationErrors []error
	for _, path := range paths {
		cfg, err := config.LoadAt(path)
		if err != nil {
			if errors.Is(err, config.ErrNotFound) {
				continue
			}
			mutationErrors = append(mutationErrors, fmt.Errorf("load StoreKit config %q: %w", path, err))
			continue
		}
		if !hasStoreKitConfig(cfg) {
			continue
		}
		cfg.StoreKit = config.StoreKitConfig{}
		if err := config.SaveAt(path, cfg); err != nil {
			mutationErrors = append(mutationErrors, fmt.Errorf("clear StoreKit config %q: %w", path, err))
		}
	}
	return errors.Join(mutationErrors...)
}

func saveDefaultName(name string) error {
	path, err := storeKitConfigPath()
	if err != nil {
		return err
	}
	return saveDefaultNameAt(name, path)
}

func saveDefaultNameAt(name, path string) error {
	cfg, err := config.LoadAt(path)
	if err != nil {
		if !errors.Is(err, config.ErrNotFound) {
			return err
		}
		cfg = &config.Config{}
	}
	cfg.StoreKit.DefaultKeyName = strings.TrimSpace(name)
	return config.SaveAt(path, cfg)
}

func clearDefaultNameIf(name string) error {
	paths, err := configStoragePaths()
	if err != nil {
		return err
	}
	var mutationErrors []error
	for _, path := range paths {
		cfg, err := config.LoadAt(path)
		if err != nil {
			if errors.Is(err, config.ErrNotFound) {
				continue
			}
			mutationErrors = append(mutationErrors, fmt.Errorf("load StoreKit config %q: %w", path, err))
			continue
		}
		if cfg.StoreKit.DefaultKeyName == name {
			cfg.StoreKit.DefaultKeyName = ""
			if err := config.SaveAt(path, cfg); err != nil {
				mutationErrors = append(mutationErrors, fmt.Errorf("update StoreKit config %q: %w", path, err))
			}
		}
	}
	return errors.Join(mutationErrors...)
}

func configStoragePaths() ([]string, error) {
	activePath, err := config.Path()
	if err != nil {
		return nil, err
	}
	paths := []string{activePath}
	if strings.TrimSpace(os.Getenv("ASC_CONFIG_PATH")) != "" {
		return paths, nil
	}
	globalPath, err := config.GlobalPath()
	if err != nil {
		return nil, err
	}
	if globalPath != activePath {
		paths = append(paths, globalPath)
	}
	return paths, nil
}

func listFromConfig() ([]StoredCredential, error) {
	cfg, path, err := loadConfigWithPath()
	if err != nil {
		return nil, err
	}
	return storedCredentialsFromConfig(cfg, path), nil
}

func findCredentialInActiveConfig(profile string) (StoredCredential, bool, error) {
	path, err := config.Path()
	if err != nil {
		return StoredCredential{}, false, err
	}
	cfg, err := config.LoadAt(path)
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return StoredCredential{}, false, nil
		}
		return StoredCredential{}, false, err
	}
	selected, ok := selectCredential(profile, storedCredentialsFromConfig(cfg, path))
	return selected, ok, nil
}

func findDefaultCredentialInActiveConfig() (StoredCredential, bool, error) {
	path, err := config.Path()
	if err != nil {
		return StoredCredential{}, false, err
	}
	cfg, err := config.LoadAt(path)
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return StoredCredential{}, false, nil
		}
		return StoredCredential{}, false, err
	}
	defaultName := strings.TrimSpace(cfg.StoreKit.DefaultKeyName)
	if defaultName == "" {
		return StoredCredential{}, false, nil
	}
	selected, ok := selectCredential(defaultName, storedCredentialsFromConfig(cfg, path))
	return selected, ok, nil
}

func findCredentialInResolvedConfig(profile string) (StoredCredential, bool, error) {
	cfg, path, err := loadConfigWithPath()
	if err != nil {
		return StoredCredential{}, false, err
	}
	selected, ok := selectCredential(profile, storedCredentialsFromConfig(cfg, path))
	return selected, ok, nil
}

func findDefaultCredentialInResolvedConfig() (StoredCredential, bool, error) {
	cfg, path, err := loadConfigWithPath()
	if err != nil {
		return StoredCredential{}, false, err
	}
	defaultName := strings.TrimSpace(cfg.StoreKit.DefaultKeyName)
	if defaultName == "" {
		return StoredCredential{}, false, nil
	}
	selected, ok := selectCredential(defaultName, storedCredentialsFromConfig(cfg, path))
	return selected, ok, nil
}

func storedCredentialsFromConfig(cfg *config.Config, path string) []StoredCredential {
	credentials := make([]StoredCredential, 0, len(cfg.StoreKit.Keys))
	for _, credential := range cfg.StoreKit.Keys {
		payload := credentialPayload{
			KeyID:          credential.KeyID,
			IssuerID:       credential.IssuerID,
			PrivateKeyPath: credential.PrivateKeyPath,
			BundleID:       credential.BundleID,
		}
		credentials = append(credentials, storedFromPayload(credential.Name, payload, "config", path))
	}
	return credentials
}

func getCredentialFromConfig(profile string) (StoredCredential, error) {
	credentials, err := listFromConfig()
	if err != nil {
		return StoredCredential{}, err
	}
	if selected, ok := selectCredential(profile, credentials); ok {
		return selected, nil
	}
	if strings.TrimSpace(profile) != "" {
		return StoredCredential{}, fmt.Errorf("StoreKit credentials not found for profile %q", profile)
	}
	return StoredCredential{}, fmt.Errorf("default StoreKit credentials not found")
}

func loadConfigWithPath() (*config.Config, string, error) {
	path, err := config.Path()
	if err != nil {
		return nil, "", err
	}
	cfg, err := config.LoadAt(path)
	if err == nil {
		if strings.TrimSpace(os.Getenv("ASC_CONFIG_PATH")) != "" || hasStoreKitConfig(cfg) {
			return cfg, path, nil
		}
		globalPath, globalErr := config.GlobalPath()
		if globalErr != nil || globalPath == path {
			return cfg, path, nil
		}
		globalConfig, globalErr := config.LoadAt(globalPath)
		if globalErr == nil {
			return globalConfig, globalPath, nil
		}
		if !errors.Is(globalErr, config.ErrNotFound) {
			return nil, "", globalErr
		}
		return cfg, path, nil
	}
	if !errors.Is(err, config.ErrNotFound) || strings.TrimSpace(os.Getenv("ASC_CONFIG_PATH")) != "" {
		return nil, "", err
	}
	globalPath, globalErr := config.GlobalPath()
	if globalErr != nil || globalPath == path {
		return nil, "", err
	}
	cfg, err = config.LoadAt(globalPath)
	if err != nil {
		return nil, "", err
	}
	return cfg, globalPath, nil
}

func hasStoreKitConfig(cfg *config.Config) bool {
	return cfg != nil && (strings.TrimSpace(cfg.StoreKit.DefaultKeyName) != "" || len(cfg.StoreKit.Keys) > 0)
}

func storedFromPayload(name string, payload credentialPayload, source, sourcePath string) StoredCredential {
	return StoredCredential{
		Credentials: Credentials{
			KeyID:          payload.KeyID,
			IssuerID:       payload.IssuerID,
			PrivateKeyPath: payload.PrivateKeyPath,
			PrivateKeyPEM:  payload.PrivateKeyPEM,
			BundleID:       payload.BundleID,
			Profile:        name,
		},
		Name:       name,
		Source:     source,
		SourcePath: sourcePath,
	}
}

func selectCredential(profile string, credentials []StoredCredential) (StoredCredential, bool) {
	profile = strings.TrimSpace(profile)
	if profile != "" {
		for _, credential := range credentials {
			if credential.Name == profile {
				return credential, true
			}
		}
		return StoredCredential{}, false
	}
	credentials = normalizeDefaults(credentials)
	for _, credential := range credentials {
		if credential.IsDefault {
			return credential, true
		}
	}
	return StoredCredential{}, false
}

func normalizeDefaults(credentials []StoredCredential) []StoredCredential {
	if len(credentials) == 0 {
		return credentials
	}
	defaultName := ""
	if cfg, _, err := loadConfigWithPath(); err == nil {
		defaultName = strings.TrimSpace(cfg.StoreKit.DefaultKeyName)
	}
	for i := range credentials {
		credentials[i].IsDefault = credentials[i].Name == defaultName
	}
	if defaultName == "" && len(credentials) == 1 {
		credentials[0].IsDefault = true
	}
	sort.Slice(credentials, func(i, j int) bool { return credentials[i].Name < credentials[j].Name })
	return credentials
}

func mergeCredentials(primary, secondary []StoredCredential) []StoredCredential {
	seen := map[string]struct{}{}
	merged := make([]StoredCredential, 0, len(primary)+len(secondary))
	for _, credential := range primary {
		seen[credential.Name] = struct{}{}
		merged = append(merged, credential)
	}
	for _, credential := range secondary {
		if _, ok := seen[credential.Name]; !ok {
			merged = append(merged, credential)
		}
	}
	return merged
}

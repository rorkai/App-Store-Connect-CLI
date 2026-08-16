package appleads

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/99designs/keyring"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

const (
	adsKeyringService       = "asc"
	adsKeyringItemPrefix    = "asc:ads-credential:"
	adsKeyringMetadataID    = "asc:ads-metadata:"
	adsBypassKeychainEnvVar = "ASC_ADS_BYPASS_KEYCHAIN"
)

// ErrDefaultCredentialsNotFound reports that no default Apple Ads credential
// is stored in the keychain or configuration.
var ErrDefaultCredentialsNotFound = errors.New("default credentials not found")

// StoredCredential is an Apple Ads credential with storage metadata.
type StoredCredential struct {
	Credentials
	Name       string
	IsDefault  bool
	Source     string
	SourcePath string
}

type credentialPayload struct {
	ClientID       string `json:"client_id"`
	TeamID         string `json:"team_id"`
	KeyID          string `json:"key_id"`
	PrivateKeyPath string `json:"private_key_path"`
	PrivateKeyPEM  string `json:"private_key_pem,omitempty"`
	OrgID          string `json:"org_id,omitempty"`
	AdAccountID    string `json:"ad_account_id,omitempty"`
}

// ShouldBypassKeychain reports whether Apple Ads keychain usage is disabled.
func ShouldBypassKeychain() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(adsBypassKeychainEnvVar)))
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// StoreCredentials stores Apple Ads credentials in keychain when available.
func StoreCredentials(name string, credentials Credentials) error {
	payload, err := payloadFromCredentials(credentials, true)
	if err != nil {
		return err
	}
	if !ShouldBypassKeychain() {
		if err := storeInKeychain(name, payload); err == nil {
			_ = removeFromConfigIfPresent(name)
			return saveDefaultName(name, payload.OrgID, payload.AdAccountID)
		} else if !isKeyringUnavailable(err) {
			return err
		}
	}
	return StoreCredentialsConfig(name, credentials)
}

// StoreCredentialsConfig stores Apple Ads credentials in the active config file.
func StoreCredentialsConfig(name string, credentials Credentials) error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	return StoreCredentialsConfigAt(name, credentials, path)
}

// StoreCredentialsConfigAt stores Apple Ads credentials in a specific config file.
func StoreCredentialsConfigAt(name string, credentials Credentials, path string) error {
	payload, err := payloadFromCredentials(credentials, false)
	if err != nil {
		return err
	}
	return storeInConfigAt(name, payload, path)
}

func payloadFromCredentials(credentials Credentials, includePEM bool) (credentialPayload, error) {
	if err := ValidateOrgID(credentials.OrgID); err != nil {
		return credentialPayload{}, err
	}
	if err := ValidateAdAccountID(credentials.AdAccountID); err != nil {
		return credentialPayload{}, err
	}
	payload := credentialPayload{
		ClientID:       strings.TrimSpace(credentials.ClientID),
		TeamID:         strings.TrimSpace(credentials.TeamID),
		KeyID:          strings.TrimSpace(credentials.KeyID),
		PrivateKeyPath: strings.TrimSpace(credentials.PrivateKeyPath),
		OrgID:          strings.TrimSpace(credentials.OrgID),
		AdAccountID:    strings.TrimSpace(credentials.AdAccountID),
	}
	if includePEM && strings.TrimSpace(credentials.PrivateKeyPEM) != "" {
		payload.PrivateKeyPEM = strings.TrimSpace(credentials.PrivateKeyPEM)
	} else if includePEM && payload.PrivateKeyPath != "" {
		if data, err := os.ReadFile(payload.PrivateKeyPath); err == nil {
			payload.PrivateKeyPEM = string(data)
		}
	}
	if payload.ClientID == "" {
		return payload, fmt.Errorf("client ID is required")
	}
	if payload.TeamID == "" {
		return payload, fmt.Errorf("team ID is required")
	}
	if payload.KeyID == "" {
		return payload, fmt.Errorf("key ID is required")
	}
	if payload.PrivateKeyPath == "" && payload.PrivateKeyPEM == "" {
		return payload, fmt.Errorf("private key is required")
	}
	return payload, nil
}

// GetCredentialsWithSource resolves Apple Ads credentials by profile.
func GetCredentialsWithSource(profile string) (Credentials, string, error) {
	if strings.TrimSpace(profile) != "" {
		if selected, ok, err := findCredentialInActiveConfig(profile); err != nil {
			return Credentials{}, "", err
		} else if ok {
			return selected.Credentials, "config", nil
		}
	} else if selected, ok, err := findDefaultCredentialInActiveConfig(); err != nil {
		return Credentials{}, "", err
	} else if ok {
		return selected.Credentials, "config", nil
	}
	if !ShouldBypassKeychain() {
		credentials, err := listFromKeychain()
		if err == nil {
			if selected, ok := selectCredential(profile, credentials); ok {
				return selected.Credentials, "keychain", nil
			}
			if strings.TrimSpace(profile) != "" {
				if cfgCred, err := getCredentialFromConfig(profile); err == nil {
					return cfgCred.Credentials, "config", nil
				}
				return Credentials{}, "", fmt.Errorf("credentials not found for profile %q", profile)
			}
			if cfgCred, err := getCredentialFromConfig(""); err == nil {
				return cfgCred.Credentials, "config", nil
			}
			if len(credentials) > 0 {
				return Credentials{}, "", ErrDefaultCredentialsNotFound
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

// ListCredentials lists Apple Ads credentials.
func ListCredentials() ([]StoredCredential, error) {
	var keychainCreds []StoredCredential
	var keychainErr error
	if !ShouldBypassKeychain() {
		keychainCreds, keychainErr = listFromKeychain()
		if keychainErr != nil && !isKeyringUnavailable(keychainErr) {
			return nil, keychainErr
		}
	}
	configCreds, configErr := listFromConfig()
	if configErr != nil && !errors.Is(configErr, config.ErrNotFound) {
		if len(keychainCreds) > 0 {
			return normalizeDefaults(keychainCreds), nil
		}
		return nil, configErr
	}
	if keychainErr != nil || ShouldBypassKeychain() {
		return normalizeDefaults(configCreds), nil
	}
	return normalizeDefaults(mergeCredentials(keychainCreds, configCreds)), nil
}

// RemoveCredentials removes one Apple Ads credential.
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

// SetDefaultCredentials switches the default Apple Ads profile.
func SetDefaultCredentials(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("credential name is required")
	}
	credentials, err := ListCredentials()
	if err != nil {
		return err
	}
	for _, cred := range credentials {
		if cred.Name == name {
			return saveDefaultName(name, cred.OrgID, cred.AdAccountID)
		}
	}
	return fmt.Errorf("credentials not found for profile %q", name)
}

// RemoveAllCredentials removes all Apple Ads credentials.
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
		ServiceName:                    adsKeyringService,
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

var openKeyring = func() (keyring.Keyring, error) {
	return keyring.Open(keyringConfig())
}

func storeInKeychain(name string, payload credentialPayload) error {
	kr, err := openKeyring()
	if err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return kr.Set(keyring.Item{
		Key:         keyringKey(name),
		Data:        data,
		Label:       "ASC Apple Ads API Key (" + name + ")",
		Description: metadataDescription(payload),
	})
}

func listFromKeychain() ([]StoredCredential, error) {
	kr, err := openKeyring()
	if err != nil {
		return nil, err
	}
	keys, err := kr.Keys()
	if err != nil {
		return nil, err
	}
	credentials := []StoredCredential{}
	for _, key := range keys {
		if !strings.HasPrefix(key, adsKeyringItemPrefix) {
			continue
		}
		item, err := kr.Get(key)
		if err != nil {
			return nil, err
		}
		var payload credentialPayload
		if err := json.Unmarshal(item.Data, &payload); err != nil {
			return nil, fmt.Errorf("invalid keychain entry %q: %w", key, err)
		}
		name := strings.TrimPrefix(key, adsKeyringItemPrefix)
		credentials = append(credentials, storedFromPayload(name, payload, "keychain", "system keychain"))
	}
	return credentials, nil
}

func removeFromKeychain(name string) error {
	kr, err := openKeyring()
	if err != nil {
		return err
	}
	return kr.Remove(keyringKey(name))
}

func removeAllFromKeychain() error {
	kr, err := openKeyring()
	if err != nil {
		return err
	}
	keys, err := kr.Keys()
	if err != nil {
		return err
	}
	for _, key := range keys {
		if strings.HasPrefix(key, adsKeyringItemPrefix) {
			if err := kr.Remove(key); err != nil && !errors.Is(err, keyring.ErrKeyNotFound) {
				return err
			}
		}
	}
	return nil
}

func keyringKey(name string) string {
	return adsKeyringItemPrefix + strings.TrimSpace(name)
}

func metadataDescription(payload credentialPayload) string {
	data, err := json.Marshal(config.AdsKeychainMetadata{
		ClientID:    strings.TrimSpace(payload.ClientID),
		TeamID:      strings.TrimSpace(payload.TeamID),
		KeyID:       strings.TrimSpace(payload.KeyID),
		OrgID:       strings.TrimSpace(payload.OrgID),
		AdAccountID: strings.TrimSpace(payload.AdAccountID),
		ModifiedAt:  time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return ""
	}
	return adsKeyringMetadataID + string(data)
}

func isKeyringUnavailable(err error) bool {
	return errors.Is(err, keyring.ErrNoAvailImpl)
}

func storeInConfigAt(name string, payload credentialPayload, path string) error {
	cfg, err := config.LoadAt(path)
	if err != nil {
		if !errors.Is(err, config.ErrNotFound) {
			return err
		}
		cfg = &config.Config{}
	}
	name = strings.TrimSpace(name)
	replacement := config.AdsCredential{
		Name:           name,
		ClientID:       payload.ClientID,
		TeamID:         payload.TeamID,
		KeyID:          payload.KeyID,
		PrivateKeyPath: payload.PrivateKeyPath,
		OrgID:          payload.OrgID,
		AdAccountID:    payload.AdAccountID,
	}
	replaced := false
	for i := range cfg.Ads.Keys {
		if cfg.Ads.Keys[i].Name == name {
			cfg.Ads.Keys[i] = replacement
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Ads.Keys = append(cfg.Ads.Keys, replacement)
	}
	cfg.Ads.DefaultKeyName = name
	cfg.Ads.OrgID = payload.OrgID
	cfg.Ads.AdAccountID = payload.AdAccountID
	return config.SaveAt(path, cfg)
}

func removeFromConfigIfPresent(name string) error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	cfg, err := config.LoadAt(path)
	if err != nil {
		return err
	}
	filtered := cfg.Ads.Keys[:0]
	removed := false
	for _, cred := range cfg.Ads.Keys {
		if cred.Name == name {
			removed = true
			continue
		}
		filtered = append(filtered, cred)
	}
	if !removed {
		return keyring.ErrKeyNotFound
	}
	cfg.Ads.Keys = filtered
	if cfg.Ads.DefaultKeyName == name {
		cfg.Ads.DefaultKeyName = ""
		cfg.Ads.OrgID = ""
		cfg.Ads.AdAccountID = ""
	}
	return config.SaveAt(path, cfg)
}

func clearConfigCredentials() error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	cfg, err := config.LoadAt(path)
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return nil
		}
		return err
	}
	cfg.Ads.DefaultKeyName = ""
	cfg.Ads.Keys = nil
	cfg.Ads.KeychainMetadata = nil
	cfg.Ads.OrgID = ""
	cfg.Ads.AdAccountID = ""
	return config.SaveAt(path, cfg)
}

func saveDefaultName(name, orgID, adAccountID string) error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	cfg, err := config.LoadAt(path)
	if err != nil {
		if !errors.Is(err, config.ErrNotFound) {
			return err
		}
		cfg = &config.Config{}
	}
	cfg.Ads.DefaultKeyName = strings.TrimSpace(name)
	cfg.Ads.OrgID = strings.TrimSpace(orgID)
	cfg.Ads.AdAccountID = strings.TrimSpace(adAccountID)
	return config.SaveAt(path, cfg)
}

func clearDefaultNameIf(name string) error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	cfg, err := config.LoadAt(path)
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return nil
		}
		return err
	}
	if cfg.Ads.DefaultKeyName == name {
		cfg.Ads.DefaultKeyName = ""
		cfg.Ads.OrgID = ""
		cfg.Ads.AdAccountID = ""
		return config.SaveAt(path, cfg)
	}
	return nil
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
	defaultName := strings.TrimSpace(cfg.Ads.DefaultKeyName)
	if defaultName == "" {
		return StoredCredential{}, false, nil
	}
	selected, ok := selectCredential(defaultName, storedCredentialsFromConfig(cfg, path))
	return selected, ok, nil
}

func storedCredentialsFromConfig(cfg *config.Config, path string) []StoredCredential {
	credentials := make([]StoredCredential, 0, len(cfg.Ads.Keys))
	for _, cred := range cfg.Ads.Keys {
		payload := credentialPayload{
			ClientID:       cred.ClientID,
			TeamID:         cred.TeamID,
			KeyID:          cred.KeyID,
			PrivateKeyPath: cred.PrivateKeyPath,
			// A named profile owns its context. Root Ads context is only a
			// standalone fallback for profile-less authentication.
			OrgID:       cred.OrgID,
			AdAccountID: cred.AdAccountID,
		}
		credentials = append(credentials, storedFromPayload(cred.Name, payload, "config", path))
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
		return StoredCredential{}, fmt.Errorf("credentials not found for profile %q", profile)
	}
	return StoredCredential{}, ErrDefaultCredentialsNotFound
}

func loadConfigWithPath() (*config.Config, string, error) {
	path, err := config.Path()
	if err != nil {
		return nil, "", err
	}
	cfg, err := config.LoadAt(path)
	if err == nil {
		return cfg, path, nil
	}
	if !errors.Is(err, config.ErrNotFound) {
		return nil, "", err
	}
	if strings.TrimSpace(os.Getenv("ASC_CONFIG_PATH")) != "" {
		return nil, "", err
	}
	global, globalErr := config.GlobalPath()
	if globalErr != nil {
		return nil, "", globalErr
	}
	if global == path {
		return nil, "", err
	}
	cfg, err = config.LoadAt(global)
	if err != nil {
		return nil, "", err
	}
	return cfg, global, nil
}

func selectCredential(profile string, credentials []StoredCredential) (StoredCredential, bool) {
	profile = strings.TrimSpace(profile)
	if profile != "" {
		for _, cred := range credentials {
			if cred.Name == profile {
				return cred, true
			}
		}
		return StoredCredential{}, false
	}
	for _, cred := range normalizeDefaults(credentials) {
		if cred.IsDefault {
			return cred, true
		}
	}
	return StoredCredential{}, false
}

func storedFromPayload(name string, payload credentialPayload, source, sourcePath string) StoredCredential {
	credentials := Credentials{
		ClientID:       payload.ClientID,
		TeamID:         payload.TeamID,
		KeyID:          payload.KeyID,
		PrivateKeyPath: payload.PrivateKeyPath,
		PrivateKeyPEM:  payload.PrivateKeyPEM,
		OrgID:          payload.OrgID,
		AdAccountID:    payload.AdAccountID,
		Profile:        name,
	}
	return StoredCredential{
		Credentials: credentials,
		Name:        name,
		Source:      source,
		SourcePath:  sourcePath,
	}
}

func normalizeDefaults(credentials []StoredCredential) []StoredCredential {
	if len(credentials) == 0 {
		return credentials
	}
	cfg, _, err := loadConfigWithPath()
	defaultName := ""
	if err == nil {
		defaultName = strings.TrimSpace(cfg.Ads.DefaultKeyName)
	}
	for i := range credentials {
		credentials[i].IsDefault = false
	}
	if defaultName != "" {
		for i := range credentials {
			if credentials[i].Name == defaultName {
				credentials[i].IsDefault = true
				return credentials
			}
		}
	}
	if len(credentials) == 1 {
		credentials[0].IsDefault = true
	}
	return credentials
}

func mergeCredentials(primary, secondary []StoredCredential) []StoredCredential {
	seen := map[string]struct{}{}
	merged := make([]StoredCredential, 0, len(primary)+len(secondary))
	for _, cred := range primary {
		seen[cred.Name] = struct{}{}
		merged = append(merged, cred)
	}
	for _, cred := range secondary {
		if _, ok := seen[cred.Name]; !ok {
			merged = append(merged, cred)
		}
	}
	return merged
}

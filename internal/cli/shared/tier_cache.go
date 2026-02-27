package shared

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const tierCacheTTL = 24 * time.Hour

type tierCacheFile struct {
	AsOf       time.Time   `json:"asOf"`
	Scope      string      `json:"scope,omitempty"`
	ResourceID string      `json:"resourceId,omitempty"`
	AppID      string      `json:"appId,omitempty"`
	Territory  string      `json:"territory"`
	Tiers      []TierEntry `json:"tiers"`
}

func tierCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	dir := filepath.Join(home, ".asc", "cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	return dir, nil
}

func tierCachePath(appID, territory string) (string, error) {
	dir, err := tierCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("tiers-%s-%s.json", appID, territory)), nil
}

func tierScopedCachePath(scope, resourceID, territory string) (string, error) {
	dir, err := tierCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(
		dir,
		fmt.Sprintf(
			"tiers-%s-%s-%s.json",
			sanitizeTierCacheToken(scope),
			sanitizeTierCacheToken(resourceID),
			sanitizeTierCacheToken(strings.ToUpper(strings.TrimSpace(territory))),
		),
	), nil
}

func sanitizeTierCacheToken(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(trimmed))
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

func loadTierCacheAtPath(path string) ([]TierEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cache tierCacheFile
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("parse cache: %w", err)
	}

	if time.Since(cache.AsOf) > tierCacheTTL {
		return nil, fmt.Errorf("cache expired")
	}

	return cache.Tiers, nil
}

func saveTierCacheAtPath(path string, cache tierCacheFile) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadTierCache loads cached tier data. Returns an error if the cache is missing or expired.
func LoadTierCache(appID, territory string) ([]TierEntry, error) {
	path, err := tierCachePath(appID, territory)
	if err != nil {
		return nil, err
	}
	return loadTierCacheAtPath(path)
}

// SaveTierCache writes tier data to the cache file.
func SaveTierCache(appID, territory string, tiers []TierEntry) error {
	path, err := tierCachePath(appID, territory)
	if err != nil {
		return err
	}

	cache := tierCacheFile{
		AsOf:      time.Now(),
		Scope:     "app",
		AppID:     appID,
		Territory: territory,
		Tiers:     tiers,
	}
	return saveTierCacheAtPath(path, cache)
}

func loadScopedTierCache(scope, resourceID, territory string) ([]TierEntry, error) {
	path, err := tierScopedCachePath(scope, resourceID, territory)
	if err != nil {
		return nil, err
	}
	return loadTierCacheAtPath(path)
}

func saveScopedTierCache(scope, resourceID, territory string, tiers []TierEntry) error {
	path, err := tierScopedCachePath(scope, resourceID, territory)
	if err != nil {
		return err
	}
	cache := tierCacheFile{
		AsOf:       time.Now(),
		Scope:      scope,
		ResourceID: resourceID,
		Territory:  territory,
		Tiers:      tiers,
	}
	return saveTierCacheAtPath(path, cache)
}

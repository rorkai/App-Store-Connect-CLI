package shared

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// The app lookup cache stores successful bundle ID -> app ID resolutions on
// disk so repeated invocations with --app <bundleID> skip the extra apps-list
// API round trip. App IDs are effectively immutable, so entries stay valid for
// a short TTL and every cache failure silently falls back to a live lookup.
const (
	appLookupCacheTTL = 24 * time.Hour
	noCacheEnvVar     = "ASC_NO_CACHE"
)

// appLookupCacheEntry is the on-disk format of one cached resolution.
// It never contains key material; scope is derived from non-secret
// credential identifiers (issuer ID or key ID).
type appLookupCacheEntry struct {
	AsOf     time.Time `json:"asOf"`
	Scope    string    `json:"scope"`
	BundleID string    `json:"bundleId"`
	AppID    string    `json:"appId"`
}

var (
	// appLookupCacheEnabled is off by default so library callers and tests
	// stay hermetic; the CLI entrypoint opts in via EnableAppLookupCache.
	appLookupCacheEnabled atomic.Bool
	noCache               bool

	noCacheWarnMu sync.Mutex
	noCacheWarned = map[string]struct{}{}

	appLookupCacheDirOverride   string
	appLookupCacheDirOverrideMu sync.RWMutex
)

// EnableAppLookupCache activates the cross-invocation app lookup disk cache.
// It is called once from the CLI entrypoint.
func EnableAppLookupCache() {
	appLookupCacheEnabled.Store(true)
}

// appLookupCredentialsProvider exposes the non-secret credential identifiers
// used to scope cache entries to the credentials that resolved them.
type appLookupCredentialsProvider interface {
	IssuerID() string
	KeyID() string
}

var _ appLookupCredentialsProvider = (*asc.Client)(nil)

// appLookupCacheScope derives the cache scope for a lookup client. It returns
// an empty scope (cache disabled) when the client does not expose credential
// identifiers, e.g. test stubs.
func appLookupCacheScope(client appLookupClient) string {
	provider, ok := client.(appLookupCredentialsProvider)
	if !ok {
		return ""
	}
	if issuerID := strings.TrimSpace(provider.IssuerID()); issuerID != "" {
		return "issuer:" + issuerID
	}
	// Individual API keys have no issuer ID; scope by key ID instead.
	if keyID := strings.TrimSpace(provider.KeyID()); keyID != "" {
		return "key:" + keyID
	}
	return ""
}

func appLookupCacheActive() bool {
	return appLookupCacheEnabled.Load() && !noCacheRequested()
}

func noCacheRequested() bool {
	if noCache {
		return true
	}
	value := strings.TrimSpace(os.Getenv(noCacheEnvVar))
	if value == "" {
		return false
	}
	switch strings.ToLower(value) {
	case "1", "t", "true", "yes", "y", "on":
		return true
	case "0", "f", "false", "no", "n", "off":
		return false
	default:
		warnInvalidNoCacheValueOnce(value)
		return false
	}
}

func warnInvalidNoCacheValueOnce(value string) {
	noCacheWarnMu.Lock()
	if _, ok := noCacheWarned[value]; ok {
		noCacheWarnMu.Unlock()
		return
	}
	noCacheWarned[value] = struct{}{}
	noCacheWarnMu.Unlock()

	fmt.Fprintf(
		os.Stderr,
		"Warning: invalid %s value %q (expected true/false, 1/0, yes/no, y/n, or on/off); cache enabled\n",
		noCacheEnvVar,
		SanitizeTerminal(value),
	)
}

func appLookupCacheDir() (string, error) {
	appLookupCacheDirOverrideMu.RLock()
	override := appLookupCacheDirOverride
	appLookupCacheDirOverrideMu.RUnlock()
	if override != "" {
		return override, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".asc", "cache"), nil
}

func appLookupCachePath(scope, bundleID string) (string, error) {
	dir, err := appLookupCacheDir()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(scope + "\x00" + bundleID))
	return filepath.Join(dir, fmt.Sprintf("app-id-%s.json", hex.EncodeToString(digest[:16]))), nil
}

// cachedAppIDForBundleID returns a cached resolution when the cache is active
// and holds a fresh, matching entry. Unreadable, corrupt, mismatched, or
// expired entries are dropped so the caller re-resolves live.
func cachedAppIDForBundleID(scope, bundleID string) (string, bool) {
	if scope == "" || !appLookupCacheActive() {
		return "", false
	}
	path, err := appLookupCachePath(scope, bundleID)
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	var entry appLookupCacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		_ = os.Remove(path)
		return "", false
	}
	appID := strings.TrimSpace(entry.AppID)
	if entry.Scope != scope || entry.BundleID != bundleID || appID == "" || time.Since(entry.AsOf) > appLookupCacheTTL {
		_ = os.Remove(path)
		return "", false
	}
	return appID, true
}

// storeCachedAppIDForBundleID persists a successful resolution. Write failures
// are silently ignored; the cache is a best-effort optimization.
func storeCachedAppIDForBundleID(scope, bundleID, appID string) {
	if scope == "" || appID == "" || !appLookupCacheActive() {
		return
	}
	path, err := appLookupCachePath(scope, bundleID)
	if err != nil {
		return
	}
	entry := appLookupCacheEntry{
		AsOf:     time.Now().UTC(),
		Scope:    scope,
		BundleID: bundleID,
		AppID:    appID,
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return
	}
	_, _ = writeFileNoSymlinkOverwrite(path, 0o600, ".asc-app-id-*", ".asc-app-id-backup-*", func(file *os.File) (int64, error) {
		written, err := file.Write(data)
		return int64(written), err
	})
}

// invalidateCachedAppIDForBundleID drops a cached resolution, e.g. when a live
// lookup shows the app is no longer visible to the credentials. It runs even
// under --no-cache so stale entries do not outlive a bypassed re-resolution.
func invalidateCachedAppIDForBundleID(scope, bundleID string) {
	if scope == "" || !appLookupCacheEnabled.Load() {
		return
	}
	path, err := appLookupCachePath(scope, bundleID)
	if err != nil {
		return
	}
	_ = os.Remove(path)
}

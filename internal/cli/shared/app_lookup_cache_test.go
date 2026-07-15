package shared

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type scopedAppLookupStub struct {
	sequenceAppLookupStub
	issuerID string
	keyID    string
}

func (s *scopedAppLookupStub) IssuerID() string { return s.issuerID }

func (s *scopedAppLookupStub) KeyID() string { return s.keyID }

func enableAppLookupCacheForTest(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	appLookupCacheDirOverrideMu.Lock()
	previousDir := appLookupCacheDirOverride
	appLookupCacheDirOverride = dir
	appLookupCacheDirOverrideMu.Unlock()

	previousEnabled := appLookupCacheEnabled.Swap(true)
	previousNoCache := noCache
	noCache = false
	t.Setenv(noCacheEnvVar, "")

	t.Cleanup(func() {
		appLookupCacheDirOverrideMu.Lock()
		appLookupCacheDirOverride = previousDir
		appLookupCacheDirOverrideMu.Unlock()
		appLookupCacheEnabled.Store(previousEnabled)
		noCache = previousNoCache
	})
	return dir
}

func cacheFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func writeCacheEntry(t *testing.T, scope, bundleID string, entry appLookupCacheEntry) string {
	t.Helper()
	path, err := appLookupCachePath(scope, bundleID)
	if err != nil {
		t.Fatalf("cache path: %v", err)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal cache entry: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write cache entry: %v", err)
	}
	return path
}

func TestResolveAppIDWithLookup_CacheHitSkipsLiveLookup(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	dir := enableAppLookupCacheForTest(t)

	first := &scopedAppLookupStub{
		sequenceAppLookupStub: sequenceAppLookupStub{
			responses: []*asc.AppsResponse{
				appsResponseFromApps([]appFixture{{id: "app-bundle", name: "Bundle App"}}),
			},
		},
		issuerID: "issuer-a",
		keyID:    "KEY1",
	}
	got, err := ResolveAppIDWithLookup(context.Background(), first, "com.example.app")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "app-bundle" {
		t.Fatalf("expected app-bundle, got %q", got)
	}
	if first.calls != 1 {
		t.Fatalf("expected one live lookup call, got %d", first.calls)
	}
	if files := cacheFiles(t, dir); len(files) != 1 {
		t.Fatalf("expected one cache file, got %v", files)
	}

	// A second invocation with the same credentials must resolve from disk.
	second := &scopedAppLookupStub{issuerID: "issuer-a", keyID: "KEY1"}
	got, err = ResolveAppIDWithLookup(context.Background(), second, "com.example.app")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() cached error: %v", err)
	}
	if got != "app-bundle" {
		t.Fatalf("expected cached app-bundle, got %q", got)
	}
	if second.calls != 0 {
		t.Fatalf("expected cache hit without live calls, got %d", second.calls)
	}
}

func TestResolveAppIDWithExactLookup_CacheHitSkipsLiveLookup(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	enableAppLookupCacheForTest(t)

	first := &scopedAppLookupStub{
		sequenceAppLookupStub: sequenceAppLookupStub{
			responses: []*asc.AppsResponse{
				appsResponseFromApps([]appFixture{{id: "app-exact", name: "Exact App"}}),
			},
		},
		issuerID: "issuer-a",
	}
	if _, err := ResolveAppIDWithExactLookup(context.Background(), first, "com.example.exact"); err != nil {
		t.Fatalf("ResolveAppIDWithExactLookup() error: %v", err)
	}

	second := &scopedAppLookupStub{issuerID: "issuer-a"}
	got, err := ResolveAppIDWithExactLookup(context.Background(), second, "com.example.exact")
	if err != nil {
		t.Fatalf("ResolveAppIDWithExactLookup() cached error: %v", err)
	}
	if got != "app-exact" {
		t.Fatalf("expected cached app-exact, got %q", got)
	}
	if second.calls != 0 {
		t.Fatalf("expected cache hit without live calls, got %d", second.calls)
	}
}

func TestResolveAppIDWithLookup_CacheMissForDifferentCredentials(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	enableAppLookupCacheForTest(t)

	first := &scopedAppLookupStub{
		sequenceAppLookupStub: sequenceAppLookupStub{
			responses: []*asc.AppsResponse{
				appsResponseFromApps([]appFixture{{id: "app-team-a", name: "Team A App"}}),
			},
		},
		issuerID: "issuer-a",
	}
	if _, err := ResolveAppIDWithLookup(context.Background(), first, "com.example.app"); err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}

	other := &scopedAppLookupStub{
		sequenceAppLookupStub: sequenceAppLookupStub{
			responses: []*asc.AppsResponse{
				appsResponseFromApps([]appFixture{{id: "app-team-b", name: "Team B App"}}),
			},
		},
		issuerID: "issuer-b",
	}
	got, err := ResolveAppIDWithLookup(context.Background(), other, "com.example.app")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "app-team-b" {
		t.Fatalf("expected live app-team-b for other credentials, got %q", got)
	}
	if other.calls != 1 {
		t.Fatalf("expected live lookup for other credentials, got %d calls", other.calls)
	}
}

func TestResolveAppIDWithLookup_ExpiredCacheEntryReResolvesLive(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	enableAppLookupCacheForTest(t)

	scope := "issuer:issuer-a"
	writeCacheEntry(t, scope, "com.example.app", appLookupCacheEntry{
		AsOf:     time.Now().Add(-appLookupCacheTTL - time.Minute).UTC(),
		Scope:    scope,
		BundleID: "com.example.app",
		AppID:    "app-stale",
	})

	stub := &scopedAppLookupStub{
		sequenceAppLookupStub: sequenceAppLookupStub{
			responses: []*asc.AppsResponse{
				appsResponseFromApps([]appFixture{{id: "app-fresh", name: "Fresh App"}}),
			},
		},
		issuerID: "issuer-a",
	}
	got, err := ResolveAppIDWithLookup(context.Background(), stub, "com.example.app")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "app-fresh" {
		t.Fatalf("expected live app-fresh after expiry, got %q", got)
	}
	if stub.calls != 1 {
		t.Fatalf("expected live lookup after expiry, got %d calls", stub.calls)
	}

	// The refreshed entry must serve later invocations again.
	if cached, ok := cachedAppIDForBundleID(scope, "com.example.app"); !ok || cached != "app-fresh" {
		t.Fatalf("expected refreshed cache entry app-fresh, got %q (ok=%t)", cached, ok)
	}
}

func TestResolveAppIDWithLookup_CorruptCacheEntryFallsBackToLiveLookup(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	enableAppLookupCacheForTest(t)

	scope := "issuer:issuer-a"
	path, err := appLookupCachePath(scope, "com.example.app")
	if err != nil {
		t.Fatalf("cache path: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write corrupt cache entry: %v", err)
	}

	stub := &scopedAppLookupStub{
		sequenceAppLookupStub: sequenceAppLookupStub{
			responses: []*asc.AppsResponse{
				appsResponseFromApps([]appFixture{{id: "app-live", name: "Live App"}}),
			},
		},
		issuerID: "issuer-a",
	}
	got, err := ResolveAppIDWithLookup(context.Background(), stub, "com.example.app")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "app-live" {
		t.Fatalf("expected live app-live after corrupt cache, got %q", got)
	}
	if stub.calls != 1 {
		t.Fatalf("expected live lookup after corrupt cache, got %d calls", stub.calls)
	}
	if cached, ok := cachedAppIDForBundleID(scope, "com.example.app"); !ok || cached != "app-live" {
		t.Fatalf("expected corrupt entry replaced with app-live, got %q (ok=%t)", cached, ok)
	}
}

func TestResolveAppIDWithLookup_ScopeMismatchedEntryIsIgnored(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	enableAppLookupCacheForTest(t)

	scope := "issuer:issuer-a"
	writeCacheEntry(t, scope, "com.example.app", appLookupCacheEntry{
		AsOf:     time.Now().UTC(),
		Scope:    "issuer:issuer-other",
		BundleID: "com.example.app",
		AppID:    "app-wrong-scope",
	})

	stub := &scopedAppLookupStub{
		sequenceAppLookupStub: sequenceAppLookupStub{
			responses: []*asc.AppsResponse{
				appsResponseFromApps([]appFixture{{id: "app-live", name: "Live App"}}),
			},
		},
		issuerID: "issuer-a",
	}
	got, err := ResolveAppIDWithLookup(context.Background(), stub, "com.example.app")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "app-live" {
		t.Fatalf("expected live app-live for mismatched scope entry, got %q", got)
	}
	if stub.calls != 1 {
		t.Fatalf("expected live lookup for mismatched scope entry, got %d calls", stub.calls)
	}
}

func TestResolveAppIDWithLookup_NoCacheFlagBypassesCache(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	dir := enableAppLookupCacheForTest(t)

	scope := "issuer:issuer-a"
	writeCacheEntry(t, scope, "com.example.app", appLookupCacheEntry{
		AsOf:     time.Now().UTC(),
		Scope:    scope,
		BundleID: "com.example.app",
		AppID:    "app-cached",
	})

	noCache = true
	stub := &scopedAppLookupStub{
		sequenceAppLookupStub: sequenceAppLookupStub{
			responses: []*asc.AppsResponse{
				appsResponseFromApps([]appFixture{{id: "app-live", name: "Live App"}}),
			},
		},
		issuerID: "issuer-a",
	}
	got, err := ResolveAppIDWithLookup(context.Background(), stub, "com.example.app")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "app-live" {
		t.Fatalf("expected live app-live with --no-cache, got %q", got)
	}
	if stub.calls != 1 {
		t.Fatalf("expected live lookup with --no-cache, got %d calls", stub.calls)
	}

	// The bypassed run must not leave the older answer available to a later
	// cache-enabled invocation.
	noCache = false
	if cached, ok := cachedAppIDForBundleID(scope, "com.example.app"); ok {
		t.Fatalf("expected bypassed live lookup to invalidate stale entry, got %q", cached)
	}
	if files := cacheFiles(t, dir); len(files) != 0 {
		t.Fatalf("expected empty cache after bypassed live lookup, got %v", files)
	}
}

func TestResolveAppIDWithLookup_NoCacheEnvBypassesCache(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	enableAppLookupCacheForTest(t)
	t.Setenv(noCacheEnvVar, "1")

	scope := "issuer:issuer-a"
	writeCacheEntry(t, scope, "com.example.app", appLookupCacheEntry{
		AsOf:     time.Now().UTC(),
		Scope:    scope,
		BundleID: "com.example.app",
		AppID:    "app-cached",
	})

	stub := &scopedAppLookupStub{
		sequenceAppLookupStub: sequenceAppLookupStub{
			responses: []*asc.AppsResponse{
				appsResponseFromApps([]appFixture{{id: "app-live", name: "Live App"}}),
			},
		},
		issuerID: "issuer-a",
	}
	got, err := ResolveAppIDWithLookup(context.Background(), stub, "com.example.app")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "app-live" {
		t.Fatalf("expected live app-live with ASC_NO_CACHE=1, got %q", got)
	}
	if stub.calls != 1 {
		t.Fatalf("expected live lookup with ASC_NO_CACHE=1, got %d calls", stub.calls)
	}
	t.Setenv(noCacheEnvVar, "0")
	if cached, ok := cachedAppIDForBundleID(scope, "com.example.app"); ok {
		t.Fatalf("expected env-bypassed lookup to invalidate stale entry, got %q", cached)
	}
}

func TestNoCacheRequestedWarnsOnceForInvalidEnvValue(t *testing.T) {
	enableAppLookupCacheForTest(t)
	t.Setenv(noCacheEnvVar, "invalid\x1b[31m")

	noCacheWarnMu.Lock()
	previousWarnings := noCacheWarned
	noCacheWarned = map[string]struct{}{}
	noCacheWarnMu.Unlock()
	t.Cleanup(func() {
		noCacheWarnMu.Lock()
		noCacheWarned = previousWarnings
		noCacheWarnMu.Unlock()
	})

	_, stderr := captureOutput(t, func() {
		if noCacheRequested() {
			t.Fatal("invalid ASC_NO_CACHE value must keep cache enabled")
		}
		_ = noCacheRequested()
	})
	if count := strings.Count(stderr, "invalid ASC_NO_CACHE value"); count != 1 {
		t.Fatalf("expected one invalid-value warning, got %d in %q", count, stderr)
	}
	if strings.Contains(stderr, "\x1b") {
		t.Fatalf("expected warning to sanitize terminal control characters, got %q", stderr)
	}
}

func TestResolveAppIDWithLookup_DropsEntryWhenAppNoLongerVisible(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	dir := enableAppLookupCacheForTest(t)

	scope := "issuer:issuer-a"
	path := writeCacheEntry(t, scope, "com.example.gone", appLookupCacheEntry{
		AsOf:     time.Now().UTC(),
		Scope:    scope,
		BundleID: "com.example.gone",
		AppID:    "app-gone",
	})

	// Bypass the cached entry so the resolver performs a live lookup that no
	// longer sees the app; the stale entry must be dropped.
	noCache = true
	stub := &scopedAppLookupStub{issuerID: "issuer-a"}
	_, err := ResolveAppIDWithLookup(context.Background(), stub, "com.example.gone")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("expected stale cache entry %q to be removed, stat err: %v", filepath.Base(path), statErr)
	}
	if files := cacheFiles(t, dir); len(files) != 0 {
		t.Fatalf("expected empty cache dir after invalidation, got %v", files)
	}
}

func TestResolveAppIDWithLookup_UnscopedClientDoesNotWriteCache(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	dir := enableAppLookupCacheForTest(t)

	stub := &sequenceAppLookupStub{
		responses: []*asc.AppsResponse{
			appsResponseFromApps([]appFixture{{id: "app-bundle", name: "Bundle App"}}),
		},
	}
	got, err := ResolveAppIDWithLookup(context.Background(), stub, "com.example.app")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "app-bundle" {
		t.Fatalf("expected app-bundle, got %q", got)
	}
	if files := cacheFiles(t, dir); len(files) != 0 {
		t.Fatalf("expected no cache files for unscoped client, got %v", files)
	}
}

func TestResolveAppIDWithLookup_CacheDisabledByDefault(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	dir := t.TempDir()
	appLookupCacheDirOverrideMu.Lock()
	previousDir := appLookupCacheDirOverride
	appLookupCacheDirOverride = dir
	appLookupCacheDirOverrideMu.Unlock()
	t.Cleanup(func() {
		appLookupCacheDirOverrideMu.Lock()
		appLookupCacheDirOverride = previousDir
		appLookupCacheDirOverrideMu.Unlock()
	})

	stub := &scopedAppLookupStub{
		sequenceAppLookupStub: sequenceAppLookupStub{
			responses: []*asc.AppsResponse{
				appsResponseFromApps([]appFixture{{id: "app-bundle", name: "Bundle App"}}),
			},
		},
		issuerID: "issuer-a",
	}
	if _, err := ResolveAppIDWithLookup(context.Background(), stub, "com.example.app"); err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if files := cacheFiles(t, dir); len(files) != 0 {
		t.Fatalf("expected no cache files while cache is disabled, got %v", files)
	}
}

func TestAppLookupCacheScope(t *testing.T) {
	if scope := appLookupCacheScope(&sequenceAppLookupStub{}); scope != "" {
		t.Fatalf("expected empty scope for unscoped client, got %q", scope)
	}
	if scope := appLookupCacheScope(&scopedAppLookupStub{issuerID: " issuer-a ", keyID: "KEY1"}); scope != "issuer:issuer-a" {
		t.Fatalf("expected issuer scope, got %q", scope)
	}
	if scope := appLookupCacheScope(&scopedAppLookupStub{keyID: "KEY1"}); scope != "key:KEY1" {
		t.Fatalf("expected key scope for individual keys, got %q", scope)
	}
	if scope := appLookupCacheScope(&scopedAppLookupStub{}); scope != "" {
		t.Fatalf("expected empty scope without identifiers, got %q", scope)
	}
}

func TestResolveAppIDWithLookup_RealClientCachesAcrossInvocations(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	enableAppLookupCacheForTest(t)

	requests := 0
	first := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		requests++
		return appResolutionJSONResponse(`{"data":[{"type":"apps","id":"6759231657","attributes":{"name":"Bundle App","bundleId":"com.example.app"}}]}`)
	})
	got, err := ResolveAppIDWithLookup(context.Background(), first, "com.example.app")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() error: %v", err)
	}
	if got != "6759231657" {
		t.Fatalf("expected 6759231657, got %q", got)
	}
	if requests != 1 {
		t.Fatalf("expected one live HTTP request, got %d", requests)
	}

	// A fresh client with the same credentials must resolve from disk.
	second := newAppResolutionTestClient(t, func(req *http.Request) (*http.Response, error) {
		t.Errorf("unexpected HTTP request %s %s for cached resolution", req.Method, req.URL)
		return appResolutionJSONResponse(`{"data":[]}`)
	})
	got, err = ResolveAppIDWithLookup(context.Background(), second, "com.example.app")
	if err != nil {
		t.Fatalf("ResolveAppIDWithLookup() cached error: %v", err)
	}
	if got != "6759231657" {
		t.Fatalf("expected cached 6759231657, got %q", got)
	}
}

func TestCachedAppIDForBundleIDRejectsBundleMismatch(t *testing.T) {
	enableAppLookupCacheForTest(t)

	scope := "issuer:issuer-a"
	writeCacheEntry(t, scope, "com.example.app", appLookupCacheEntry{
		AsOf:     time.Now().UTC(),
		Scope:    scope,
		BundleID: "com.example.other",
		AppID:    "app-other",
	})

	if cached, ok := cachedAppIDForBundleID(scope, "com.example.app"); ok {
		t.Fatalf("expected mismatched bundle entry to be rejected, got %q", cached)
	}
}

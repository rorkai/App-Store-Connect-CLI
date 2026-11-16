package shared

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func useTierCacheDirForTest(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	tierCacheDirOverrideMu.Lock()
	previous := tierCacheDirOverride
	tierCacheDirOverride = dir
	tierCacheDirOverrideMu.Unlock()

	t.Cleanup(func() {
		tierCacheDirOverrideMu.Lock()
		tierCacheDirOverride = previous
		tierCacheDirOverrideMu.Unlock()
	})
	return dir
}

func TestTierCacheRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tiers := []TierEntry{
		{Tier: 1, PricePointID: "pp-1", CustomerPrice: "0.99", Proceeds: "0.70"},
		{Tier: 2, PricePointID: "pp-2", CustomerPrice: "1.99", Proceeds: "1.40"},
	}

	if err := SaveTierCache("app123", "USA", tiers); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadTierCache("app123", "USA")
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 tiers, got %d", len(loaded))
	}
	if loaded[0].PricePointID != "pp-1" {
		t.Fatalf("expected pp-1, got %s", loaded[0].PricePointID)
	}
	if loaded[1].CustomerPrice != "1.99" {
		t.Fatalf("expected 1.99, got %s", loaded[1].CustomerPrice)
	}
}

func TestTierCacheSaveRejectsSymlinkAndPreservesTarget(t *testing.T) {
	cacheDir := useTierCacheDirForTest(t)

	sentinelPath := filepath.Join(t.TempDir(), "sentinel.txt")
	const sentinel = "keep this content"
	if err := os.WriteFile(sentinelPath, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	cachePath := filepath.Join(cacheDir, "tiers-app123-USA.json")
	if err := os.Symlink(sentinelPath, cachePath); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}

	err := SaveTierCache("app123", "USA", []TierEntry{{
		Tier:          1,
		PricePointID:  "pp-1",
		CustomerPrice: "0.99",
	}})
	if err == nil {
		t.Error("SaveTierCache() error = nil, want symlink rejection")
	}

	linkTarget, readlinkErr := os.Readlink(cachePath)
	if readlinkErr != nil {
		t.Fatalf("read cache symlink: %v", readlinkErr)
	}
	if linkTarget != sentinelPath {
		t.Errorf("cache symlink target = %q, want %q", linkTarget, sentinelPath)
	}

	got, readErr := os.ReadFile(sentinelPath)
	if readErr != nil {
		t.Fatalf("read sentinel: %v", readErr)
	}
	if string(got) != sentinel {
		t.Errorf("sentinel content = %q, want %q", got, sentinel)
	}
}

func TestTierCacheMiss(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, err := LoadTierCache("nonexistent", "USA")
	if err == nil {
		t.Fatal("expected error for missing cache")
	}
}

func TestTierCacheExpired(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".asc", "cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}

	cache := tierCacheFile{
		AsOf:      time.Now().Add(-25 * time.Hour),
		AppID:     "app123",
		Territory: "USA",
		Tiers:     []TierEntry{{Tier: 1, PricePointID: "pp-1", CustomerPrice: "0.99"}},
	}
	data, _ := json.Marshal(cache)
	path := filepath.Join(dir, "tiers-app123-USA.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write error: %v", err)
	}

	_, err := LoadTierCache("app123", "USA")
	if err == nil {
		t.Fatal("expected error for expired cache")
	}
}

func TestTierCacheDifferentTerritory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	usa := []TierEntry{{Tier: 1, PricePointID: "pp-usa", CustomerPrice: "0.99"}}
	jpn := []TierEntry{{Tier: 1, PricePointID: "pp-jpn", CustomerPrice: "120"}}

	if err := SaveTierCache("app1", "USA", usa); err != nil {
		t.Fatal(err)
	}
	if err := SaveTierCache("app1", "JPN", jpn); err != nil {
		t.Fatal(err)
	}

	loadedUSA, _ := LoadTierCache("app1", "USA")
	loadedJPN, _ := LoadTierCache("app1", "JPN")

	if loadedUSA[0].PricePointID != "pp-usa" {
		t.Fatalf("expected pp-usa, got %s", loadedUSA[0].PricePointID)
	}
	if loadedJPN[0].PricePointID != "pp-jpn" {
		t.Fatalf("expected pp-jpn, got %s", loadedJPN[0].PricePointID)
	}
}

func TestResetTierCacheForTestUsesIsolatedTempDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	resetTierCacheDirOverrideForTest()
	t.Cleanup(resetTierCacheDirOverrideForTest)

	realCacheDir := filepath.Join(home, ".asc", "cache")
	if err := os.MkdirAll(realCacheDir, 0o755); err != nil {
		t.Fatalf("mkdir real cache dir: %v", err)
	}

	sentinelPath := filepath.Join(realCacheDir, "sentinel.txt")
	if err := os.WriteFile(sentinelPath, []byte("keep"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	ResetTierCacheForTest(filepath.Join(t.TempDir(), "tier-cache"))

	if _, err := os.Stat(sentinelPath); err != nil {
		t.Fatalf("expected real cache sentinel to remain, got %v", err)
	}

	tiers := []TierEntry{{Tier: 1, PricePointID: "pp-1", CustomerPrice: "0.99"}}
	if err := SaveTierCache("app123", "USA", tiers); err != nil {
		t.Fatalf("save error: %v", err)
	}

	realCachePath := filepath.Join(realCacheDir, "tiers-app123-USA.json")
	if _, err := os.Stat(realCachePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected real cache path to stay untouched, got err=%v", err)
	}

	overridePath, err := tierCachePath("app123", "USA")
	if err != nil {
		t.Fatalf("tierCachePath error: %v", err)
	}
	if overridePath == realCachePath {
		t.Fatalf("expected isolated tier cache path, got real cache path %q", overridePath)
	}

	loaded, err := LoadTierCache("app123", "USA")
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(loaded) != 1 || loaded[0].PricePointID != "pp-1" {
		t.Fatalf("unexpected loaded tiers: %+v", loaded)
	}
}

package shared

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

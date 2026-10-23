package pppindex

import (
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
)

func TestEmbeddedPriceLevelsAreUsableTerritoryIndex(t *testing.T) {
	levels, err := priceLevels()
	if err != nil {
		t.Fatalf("embedded price levels: %v", err)
	}
	if usa, ok := PriceLevel("usa"); !ok || usa.RatString() != "1" {
		t.Fatalf("USA price level = %v, %t; want 1", usa, ok)
	}
	for territory := range levels {
		if normalized, err := ascterritory.Normalize(territory); err != nil || normalized != territory {
			t.Errorf("territory %s is not an App Store Connect territory ID (normalized %q, err %v)", territory, normalized, err)
		}
	}
}

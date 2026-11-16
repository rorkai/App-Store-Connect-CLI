package shared

import (
	"encoding/json"
	"testing"
)

func TestPrintResolvedPrices_JSON(t *testing.T) {
	result := &ResolvedPricesResult{
		Prices: []ResolvedPriceRow{
			{Territory: "USA", PriceID: "price-2", PricePointID: "pp-2"},
			{Territory: "GBR", PriceID: "price-1", PricePointID: "pp-1"},
		},
	}

	stdout, _ := captureOutput(t, func() {
		if err := DefaultDeps().PrintResolvedPrices(result, "json", false); err != nil {
			t.Fatalf("PrintResolvedPrices(json) error = %v", err)
		}
	})

	var parsed ResolvedPricesResult
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, stdout = %q", err, stdout)
	}
	if len(parsed.Prices) != 2 {
		t.Fatalf("expected 2 resolved rows, got %+v", parsed.Prices)
	}
	if parsed.Prices[0].Territory != "USA" || parsed.Prices[1].Territory != "GBR" {
		t.Fatalf("expected JSON output to preserve input order, got %+v", parsed.Prices)
	}
}

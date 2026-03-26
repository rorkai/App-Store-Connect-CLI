package shared

import (
	"fmt"
	"testing"
)

func TestValidatePriceSelectionFlags_NoneSet(t *testing.T) {
	err := ValidatePriceSelectionFlags("", 0, "")
	if err == nil {
		t.Fatal("expected error when no flags set")
	}
	expected := "one of --price-point, --tier, or --price is required"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestValidatePriceSelectionFlags_NoneSet_WithFreeSupport(t *testing.T) {
	err := ValidatePriceSelectionFlags("", 0, "", false)
	if err == nil {
		t.Fatal("expected error when no flags set")
	}
	expected := "one of --price-point, --tier, --price, or --free is required"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestValidatePriceSelectionFlags_NegativeTier(t *testing.T) {
	err := ValidatePriceSelectionFlags("", -1, "")
	if err == nil {
		t.Fatal("expected error for negative tier")
	}
	expected := "--tier must be a positive integer"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestValidatePriceSelectionFlags_OneSet(t *testing.T) {
	tests := []struct {
		name       string
		pricePoint string
		tier       int
		price      string
		free       bool
	}{
		{"price-point only", "PP", 0, "", false},
		{"tier only", "", 5, "", false},
		{"price only", "", 0, "4.99", false},
		{"free only", "", 0, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidatePriceSelectionFlags(tc.pricePoint, tc.tier, tc.price, tc.free); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePriceSelectionFlags_MultipleSet(t *testing.T) {
	tests := []struct {
		name       string
		pricePoint string
		tier       int
		price      string
	}{
		{"price-point and tier", "PP", 5, ""},
		{"price-point and price", "PP", 0, "4.99"},
		{"tier and price", "", 5, "4.99"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePriceSelectionFlags(tc.pricePoint, tc.tier, tc.price)
			if err == nil {
				t.Fatal("expected error for multiple flags")
			}
			expected := "--price-point, --tier, and --price are mutually exclusive"
			if err.Error() != expected {
				t.Fatalf("expected %q, got %q", expected, err.Error())
			}
		})
	}
}

func TestValidatePriceSelectionFlags_MultipleSet_WithFreeSupport(t *testing.T) {
	tests := []struct {
		name       string
		pricePoint string
		tier       int
		price      string
		free       bool
	}{
		{"price-point and tier", "PP", 5, "", false},
		{"price-point and price", "PP", 0, "4.99", false},
		{"tier and price", "", 5, "4.99", false},
		{"price-point and free", "PP", 0, "", true},
		{"tier and free", "", 5, "", true},
		{"price and free", "", 0, "4.99", true},
		{"all four", "PP", 5, "4.99", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePriceSelectionFlags(tc.pricePoint, tc.tier, tc.price, tc.free)
			if err == nil {
				t.Fatal("expected error for multiple flags")
			}
			expected := "--price-point, --tier, --price, and --free are mutually exclusive"
			if err.Error() != expected {
				t.Fatalf("expected %q, got %q", expected, err.Error())
			}
		})
	}
}

func TestResolvePricePointByTier(t *testing.T) {
	tiers := []TierEntry{
		{Tier: 1, PricePointID: "pp-1", CustomerPrice: "0.99"},
		{Tier: 2, PricePointID: "pp-2", CustomerPrice: "1.99"},
		{Tier: 3, PricePointID: "pp-3", CustomerPrice: "2.99"},
	}

	id, err := ResolvePricePointByTier(tiers, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "pp-2" {
		t.Fatalf("expected pp-2, got %s", id)
	}
}

func TestResolvePricePointByTier_NotFound(t *testing.T) {
	tiers := []TierEntry{
		{Tier: 1, PricePointID: "pp-1", CustomerPrice: "0.99"},
	}

	_, err := ResolvePricePointByTier(tiers, 99)
	if err == nil {
		t.Fatal("expected error for invalid tier")
	}
}

func TestResolvePricePointByPrice(t *testing.T) {
	tiers := []TierEntry{
		{Tier: 1, PricePointID: "pp-1", CustomerPrice: "0.99"},
		{Tier: 2, PricePointID: "pp-2", CustomerPrice: "1.99"},
		{Tier: 3, PricePointID: "pp-3", CustomerPrice: "2.99"},
	}

	id, err := ResolvePricePointByPrice(tiers, "1.99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "pp-2" {
		t.Fatalf("expected pp-2, got %s", id)
	}
}

func TestResolvePricePointByPrice_FuzzyMatch(t *testing.T) {
	tiers := []TierEntry{
		{Tier: 1, PricePointID: "pp-1", CustomerPrice: "0.99"},
	}

	id, err := ResolvePricePointByPrice(tiers, "0.990")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "pp-1" {
		t.Fatalf("expected pp-1, got %s", id)
	}
}

func TestResolvePricePointByPrice_NotFound(t *testing.T) {
	tiers := []TierEntry{
		{Tier: 1, PricePointID: "pp-1", CustomerPrice: "0.99"},
	}

	_, err := ResolvePricePointByPrice(tiers, "999.99")
	if err == nil {
		t.Fatal("expected error for unmatched price")
	}
}

func TestResolvePricePointByPrice_InvalidInput(t *testing.T) {
	tiers := []TierEntry{
		{Tier: 1, PricePointID: "pp-1", CustomerPrice: "0.99"},
	}

	_, err := ResolvePricePointByPrice(tiers, "not-a-number")
	if err == nil {
		t.Fatal("expected error for non-numeric price")
	}
}

func TestResolveFreeAppPricePointWithFetcher_PaginatesUntilMatch(t *testing.T) {
	callCount := 0
	id, err := resolveFreeAppPricePointWithFetcher("USA", func(nextURL string) (freePricePointPage, error) {
		callCount++
		switch callCount {
		case 1:
			if nextURL != "" {
				t.Fatalf("expected empty nextURL on first page, got %q", nextURL)
			}
			return freePricePointPage{nextURL: "page=2"}, nil
		case 2:
			if nextURL != "page=2" {
				t.Fatalf("expected page=2 nextURL, got %q", nextURL)
			}
			return freePricePointPage{pricePointID: "free-pp", found: true}, nil
		default:
			t.Fatalf("unexpected extra page fetch %d", callCount)
			return freePricePointPage{}, nil
		}
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "free-pp" {
		t.Fatalf("expected free-pp, got %q", id)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 page fetches, got %d", callCount)
	}
}

func TestResolveFreeAppPricePointWithFetcher_ReturnsNotFoundWhenAbsent(t *testing.T) {
	_, err := resolveFreeAppPricePointWithFetcher("USA", func(nextURL string) (freePricePointPage, error) {
		return freePricePointPage{}, nil
	})
	if err == nil {
		t.Fatal("expected error when free price point is absent")
	}
	expected := "no free ($0) price point found for territory USA"
	if err.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, err.Error())
	}
}

func TestResolveFreeAppPricePointWithFetcher_PropagatesFetchErrors(t *testing.T) {
	expectedErr := fmt.Errorf("fetch price points: boom")
	_, err := resolveFreeAppPricePointWithFetcher("USA", func(nextURL string) (freePricePointPage, error) {
		return freePricePointPage{}, expectedErr
	})
	if err == nil {
		t.Fatal("expected fetch error")
	}
	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected %q, got %q", expectedErr.Error(), err.Error())
	}
}

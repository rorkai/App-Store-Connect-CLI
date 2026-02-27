package shared

import (
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
	}{
		{"price-point only", "PP", 0, ""},
		{"tier only", "", 5, ""},
		{"price only", "", 0, "4.99"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidatePriceSelectionFlags(tc.pricePoint, tc.tier, tc.price); err != nil {
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
		{"all three", "PP", 5, "4.99"},
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

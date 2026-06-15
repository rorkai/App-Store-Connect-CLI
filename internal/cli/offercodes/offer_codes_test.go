package offercodes

import (
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestOfferCodesCommandConstructors(t *testing.T) {
	constructors := []func() any{
		func() any { return OfferCodeCustomCodesCommand() },
		func() any { return OfferCodePricesCommand() },
		func() any { return OfferCodesGenerateCommand() },
		func() any { return OfferCodesValuesCommand() },
	}
	for _, ctor := range constructors {
		if got := ctor(); got == nil {
			t.Fatal("expected constructor to return command")
		}
	}
}

func TestParseOfferCodePrices(t *testing.T) {
	prices, err := parseOfferCodePrices("US:pp-1, France:pp-2", asc.SubscriptionOfferModePayAsYouGo)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(prices) != 2 {
		t.Fatalf("expected 2 prices, got %d", len(prices))
	}
	if prices[0].TerritoryID != "USA" || prices[0].PricePointID != "pp-1" {
		t.Fatalf("unexpected first price: %+v", prices[0])
	}
	if prices[1].TerritoryID != "FRA" || prices[1].PricePointID != "pp-2" {
		t.Fatalf("unexpected second price: %+v", prices[1])
	}

	prices, err = parseOfferCodePrices("Moldova, Republic of:pp-1,Bolivia, Plurinational State of:pp-2", asc.SubscriptionOfferModePayUpFront)
	if err != nil {
		t.Fatalf("unexpected parse error for comma-containing territory names: %v", err)
	}
	if len(prices) != 2 {
		t.Fatalf("expected 2 comma-name prices, got %d", len(prices))
	}
	if prices[0].TerritoryID != "MDA" || prices[0].PricePointID != "pp-1" {
		t.Fatalf("unexpected first comma-name price: %+v", prices[0])
	}
	if prices[1].TerritoryID != "BOL" || prices[1].PricePointID != "pp-2" {
		t.Fatalf("unexpected second comma-name price: %+v", prices[1])
	}

	freeTrialPrices, err := parseOfferCodePrices("DE, France", asc.SubscriptionOfferModeFreeTrial)
	if err != nil {
		t.Fatalf("unexpected FREE_TRIAL parse error: %v", err)
	}
	if len(freeTrialPrices) != 2 {
		t.Fatalf("expected 2 FREE_TRIAL prices, got %d", len(freeTrialPrices))
	}
	if freeTrialPrices[0].TerritoryID != "DEU" || freeTrialPrices[0].PricePointID != "" {
		t.Fatalf("unexpected first FREE_TRIAL price: %+v", freeTrialPrices[0])
	}
	if freeTrialPrices[1].TerritoryID != "FRA" || freeTrialPrices[1].PricePointID != "" {
		t.Fatalf("unexpected second FREE_TRIAL price: %+v", freeTrialPrices[1])
	}

	if _, err := parseOfferCodePrices("usa-pp-1", asc.SubscriptionOfferModePayAsYouGo); err == nil {
		t.Fatal("expected parse error for malformed prices")
	}
	if _, err := parseOfferCodePrices("Atlantis:pp-1", asc.SubscriptionOfferModePayAsYouGo); err == nil {
		t.Fatal("expected parse error for invalid territory")
	}
	if _, err := parseOfferCodePrices("USA:pp-1", asc.SubscriptionOfferModeFreeTrial); err == nil {
		t.Fatal("expected FREE_TRIAL price point rejection")
	}
}

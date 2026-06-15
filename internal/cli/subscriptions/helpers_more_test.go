package subscriptions

import (
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestNormalizeSubscriptionEnums(t *testing.T) {
	if got, err := normalizeSubscriptionPeriod("one_month", true); err != nil || got != asc.SubscriptionPeriodOneMonth {
		t.Fatalf("expected ONE_MONTH, got %q err=%v", got, err)
	}
	if _, err := normalizeSubscriptionPeriod("", true); err == nil {
		t.Fatal("expected required error for empty period")
	}
	if _, err := normalizeSubscriptionPeriod("bad", true); err == nil {
		t.Fatal("expected validation error for period")
	}

	if got, err := normalizeSubscriptionOfferDuration("one_year"); err != nil || got != asc.SubscriptionOfferDurationOneYear {
		t.Fatalf("expected ONE_YEAR, got %q err=%v", got, err)
	}
	if _, err := normalizeSubscriptionOfferDuration("bad"); err == nil {
		t.Fatal("expected validation error for offer duration")
	}

	if got, err := normalizeSubscriptionOfferMode("free_trial"); err != nil || got != asc.SubscriptionOfferModeFreeTrial {
		t.Fatalf("expected FREE_TRIAL, got %q err=%v", got, err)
	}
	if _, err := normalizeSubscriptionOfferMode("bad"); err == nil {
		t.Fatal("expected validation error for offer mode")
	}

	if got, err := normalizeSubscriptionOfferEligibility("replace_intro_offers", true); err != nil || got != asc.SubscriptionOfferEligibilityReplaceIntroOffers {
		t.Fatalf("expected REPLACE_INTRO_OFFERS, got %q err=%v", got, err)
	}
	if _, err := normalizeSubscriptionOfferEligibility("bad", true); err == nil {
		t.Fatal("expected validation error for offer eligibility")
	}

	if got, err := normalizeSubscriptionGracePeriodRenewalType("all_renewals", true); err != nil || got != asc.SubscriptionGracePeriodRenewalTypeAllRenewals {
		t.Fatalf("expected ALL_RENEWALS, got %q err=%v", got, err)
	}
	if _, err := normalizeSubscriptionGracePeriodRenewalType("bad", true); err == nil {
		t.Fatal("expected validation error for renewal type")
	}
}

func TestNormalizeSubscriptionCustomerEligibilities(t *testing.T) {
	got, err := normalizeSubscriptionCustomerEligibilities("new,existing,expired")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 eligibilities, got %d", len(got))
	}
	if got[0] != asc.SubscriptionCustomerEligibilityNew {
		t.Fatalf("unexpected first eligibility: %q", got[0])
	}

	if _, err := normalizeSubscriptionCustomerEligibilities(""); err == nil {
		t.Fatal("expected required error for empty eligibilities")
	}
	if _, err := normalizeSubscriptionCustomerEligibilities("new,bad"); err == nil {
		t.Fatal("expected validation error for invalid eligibility")
	}
}

func TestParseSubscriptionOfferCodePrices(t *testing.T) {
	prices, err := parseSubscriptionOfferCodePrices("US:pp-1, France:pp-2", asc.SubscriptionOfferModePayAsYouGo)
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

	prices, err = parseSubscriptionOfferCodePrices("Moldova, Republic of:pp-1,Bolivia, Plurinational State of:pp-2", asc.SubscriptionOfferModePayUpFront)
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

	freeTrialPrices, err := parseSubscriptionOfferCodePrices("DE, France", asc.SubscriptionOfferModeFreeTrial)
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

	if _, err := parseSubscriptionOfferCodePrices("usa-pp-1", asc.SubscriptionOfferModePayAsYouGo); err == nil {
		t.Fatal("expected parse error for malformed input")
	}
	if _, err := parseSubscriptionOfferCodePrices("usa:", asc.SubscriptionOfferModePayAsYouGo); err == nil {
		t.Fatal("expected parse error for missing price point id")
	}
	if _, err := parseSubscriptionOfferCodePrices("Atlantis:pp-1", asc.SubscriptionOfferModePayAsYouGo); err == nil {
		t.Fatal("expected parse error for invalid territory")
	}
	if _, err := parseSubscriptionOfferCodePrices("USA:pp-1", asc.SubscriptionOfferModeFreeTrial); err == nil {
		t.Fatal("expected FREE_TRIAL price point rejection")
	}
}

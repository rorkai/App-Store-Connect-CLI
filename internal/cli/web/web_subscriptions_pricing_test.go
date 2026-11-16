package web

import (
	"context"
	"fmt"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestExistingMonthlyAvailabilityOnlyRejectsConfirmedMissingTerritory(t *testing.T) {
	unloaded := webcore.SubscriptionPlanAvailability{
		ID:                         "plan-monthly",
		PlanType:                   "MONTHLY",
		AvailableTerritoriesLoaded: false,
	}
	if availabilityExcludesTerritory(unloaded, "NOR") {
		t.Fatal("unloaded relationship must not be treated as confirmed missing")
	}

	capped := unloaded
	capped.AvailableTerritoriesLoaded = true
	capped.AvailableTerritories = make([]string, 200)
	for i := range capped.AvailableTerritories {
		capped.AvailableTerritories[i] = fmt.Sprintf("T%03d", i)
	}
	if availabilityExcludesTerritory(capped, "NOR") {
		t.Fatal("territory relationship at the response cap must not be treated as complete")
	}

	loaded := unloaded
	loaded.AvailableTerritoriesLoaded = true
	loaded.AvailableTerritories = []string{"DEU"}
	if !availabilityExcludesTerritory(loaded, "NOR") {
		t.Fatal("loaded relationship should confirm the territory is missing")
	}
}

func TestVerifyMonthlyCommitmentBootstrapRejectsStalePricePoints(t *testing.T) {
	origListAvailability := listWebSubscriptionPlanAvailabilitiesFn
	origListPrices := listWebSubscriptionPricesFn
	t.Cleanup(func() {
		listWebSubscriptionPlanAvailabilitiesFn = origListAvailability
		listWebSubscriptionPricesFn = origListPrices
	})

	listWebSubscriptionPlanAvailabilitiesFn = func(ctx context.Context, client *webcore.Client, subscriptionID string) ([]webcore.SubscriptionPlanAvailability, error) {
		return []webcore.SubscriptionPlanAvailability{{
			ID:                         "plan-monthly",
			PlanType:                   "MONTHLY",
			AvailableTerritories:       []string{"NOR"},
			AvailableTerritoriesLoaded: true,
		}}, nil
	}
	listWebSubscriptionPricesFn = func(ctx context.Context, client *webcore.Client, subscriptionID, territory string) ([]webcore.SubscriptionPrice, error) {
		return []webcore.SubscriptionPrice{{
			PlanType:     "UPFRONT",
			Territory:    "NOR",
			PricePointID: "stale-upfront",
		}, {
			PlanType:     "MONTHLY",
			Territory:    "NOR",
			PricePointID: "stale-monthly",
		}}, nil
	}

	d, _, _ := newWebTestDeps()
	err := verifyMonthlyCommitmentBootstrap(context.Background(), d, &webcore.Client{}, asc.WebSubscriptionMonthlyCommitmentBootstrapResult{
		SubscriptionID:      "sub-1",
		Territory:           "NOR",
		PlanAvailabilityID:  "plan-monthly",
		UpfrontPricePointID: "upfront-point",
		MonthlyPricePointID: "monthly-point",
	})
	if err == nil {
		t.Fatal("expected stale price points to fail verification")
	}
}

func TestVerifyMonthlyCommitmentBootstrapRejectsConfirmedMissingTerritory(t *testing.T) {
	origListAvailability := listWebSubscriptionPlanAvailabilitiesFn
	origListPrices := listWebSubscriptionPricesFn
	t.Cleanup(func() {
		listWebSubscriptionPlanAvailabilitiesFn = origListAvailability
		listWebSubscriptionPricesFn = origListPrices
	})
	listWebSubscriptionPricesFn = func(ctx context.Context, client *webcore.Client, subscriptionID, territory string) ([]webcore.SubscriptionPrice, error) {
		t.Fatal("price readback should not run when the territory is confirmed missing")
		return nil, nil
	}
	listWebSubscriptionPlanAvailabilitiesFn = func(ctx context.Context, client *webcore.Client, subscriptionID string) ([]webcore.SubscriptionPlanAvailability, error) {
		return []webcore.SubscriptionPlanAvailability{{
			ID:                         "plan-monthly",
			PlanType:                   "MONTHLY",
			AvailableTerritories:       []string{"DEU"},
			AvailableTerritoriesLoaded: true,
		}}, nil
	}
	d, _, _ := newWebTestDeps()
	err := verifyMonthlyCommitmentBootstrap(context.Background(), d, &webcore.Client{}, asc.WebSubscriptionMonthlyCommitmentBootstrapResult{
		SubscriptionID:     "sub-1",
		Territory:          "NOR",
		PlanAvailabilityID: "plan-monthly",
	})
	if err == nil {
		t.Fatal("expected unverified result when loaded territories confirm the requested one is missing")
	}
}

func TestVerifyMonthlyCommitmentBootstrapTreatsCappedTerritoryReadAsInconclusive(t *testing.T) {
	origListAvailability := listWebSubscriptionPlanAvailabilitiesFn
	origListPrices := listWebSubscriptionPricesFn
	t.Cleanup(func() {
		listWebSubscriptionPlanAvailabilitiesFn = origListAvailability
		listWebSubscriptionPricesFn = origListPrices
	})
	territories := make([]string, webcore.SubscriptionPlanAvailabilityTerritoryLimit)
	for i := range territories {
		territories[i] = fmt.Sprintf("T%03d", i)
	}
	listWebSubscriptionPlanAvailabilitiesFn = func(ctx context.Context, client *webcore.Client, subscriptionID string) ([]webcore.SubscriptionPlanAvailability, error) {
		return []webcore.SubscriptionPlanAvailability{{
			ID:                         "plan-monthly",
			PlanType:                   "MONTHLY",
			AvailableTerritories:       territories,
			AvailableTerritoriesLoaded: true,
		}}, nil
	}
	listWebSubscriptionPricesFn = func(ctx context.Context, client *webcore.Client, subscriptionID, territory string) ([]webcore.SubscriptionPrice, error) {
		return []webcore.SubscriptionPrice{{
			PlanType: "UPFRONT", Territory: "NOR", PricePointID: "upfront-point",
		}, {
			PlanType: "MONTHLY", Territory: "NOR", PricePointID: "monthly-point",
		}}, nil
	}
	d, _, _ := newWebTestDeps()
	err := verifyMonthlyCommitmentBootstrap(context.Background(), d, &webcore.Client{}, asc.WebSubscriptionMonthlyCommitmentBootstrapResult{
		SubscriptionID:      "sub-1",
		Territory:           "NOR",
		PlanAvailabilityID:  "plan-monthly",
		UpfrontPricePointID: "upfront-point",
		MonthlyPricePointID: "monthly-point",
	})
	if err != nil {
		t.Fatalf("capped territory include must not fail post-write verification: %v", err)
	}
}

func TestVerifyMonthlyCommitmentBootstrapIgnoresCreatePreserveOption(t *testing.T) {
	origListAvailability := listWebSubscriptionPlanAvailabilitiesFn
	origListPrices := listWebSubscriptionPricesFn
	t.Cleanup(func() {
		listWebSubscriptionPlanAvailabilitiesFn = origListAvailability
		listWebSubscriptionPricesFn = origListPrices
	})
	listWebSubscriptionPlanAvailabilitiesFn = func(ctx context.Context, client *webcore.Client, subscriptionID string) ([]webcore.SubscriptionPlanAvailability, error) {
		return []webcore.SubscriptionPlanAvailability{{
			ID:                         "plan-monthly",
			PlanType:                   "MONTHLY",
			AvailableTerritories:       []string{"NOR"},
			AvailableTerritoriesLoaded: true,
		}}, nil
	}
	listWebSubscriptionPricesFn = func(ctx context.Context, client *webcore.Client, subscriptionID, territory string) ([]webcore.SubscriptionPrice, error) {
		return []webcore.SubscriptionPrice{{
			PlanType:     "UPFRONT",
			Territory:    "NOR",
			PricePointID: "upfront-point",
			StartDate:    "2026-07-01",
			Preserved:    false,
		}, {
			PlanType:     "MONTHLY",
			Territory:    "NOR",
			PricePointID: "monthly-point",
			StartDate:    "2026-07-01",
			Preserved:    false,
		}}, nil
	}
	d, _, _ := newWebTestDeps()
	err := verifyMonthlyCommitmentBootstrap(context.Background(), d, &webcore.Client{}, asc.WebSubscriptionMonthlyCommitmentBootstrapResult{
		SubscriptionID:       "sub-1",
		Territory:            "NOR",
		PlanAvailabilityID:   "plan-monthly",
		UpfrontPricePointID:  "upfront-point",
		MonthlyPricePointID:  "monthly-point",
		StartDate:            "2026-07-01",
		PreserveCurrentPrice: true,
	})
	if err != nil {
		t.Fatalf("preserveCurrentPrice create option must not be compared with preserved state: %v", err)
	}
}

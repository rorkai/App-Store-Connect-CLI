package subscriptions

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

func TestSubscriptionSetupPriceMatchesTargetRequiresPlanType(t *testing.T) {
	relationships, err := json.Marshal(subscriptionSetupPriceRelationships{
		SubscriptionPricePoint: &asc.Relationship{Data: asc.ResourceData{Type: asc.ResourceTypeSubscriptionPricePoints, ID: "pp-1"}},
		Territory:              &asc.Relationship{Data: asc.ResourceData{Type: asc.ResourceTypeTerritories, ID: "USA"}},
	})
	if err != nil {
		t.Fatalf("marshal relationships: %v", err)
	}

	price := asc.Resource[asc.SubscriptionPriceAttributes]{
		ID:            "price-1",
		Relationships: relationships,
		Attributes: asc.SubscriptionPriceAttributes{
			PlanType: asc.SubscriptionPlanTypeMonthly,
		},
	}

	if subscriptionSetupPriceMatchesTarget(price, "pp-1", "USA", asc.SubscriptionPriceCreateAttributes{PlanType: asc.SubscriptionPlanTypeUpfront}) {
		t.Fatal("monthly price should not satisfy an upfront setup price")
	}

	price.Attributes.PlanType = asc.SubscriptionPlanTypeUpfront
	if !subscriptionSetupPriceMatchesTarget(price, "pp-1", "USA", asc.SubscriptionPriceCreateAttributes{PlanType: asc.SubscriptionPlanTypeUpfront}) {
		t.Fatal("upfront price should satisfy an upfront setup price")
	}
}

func TestSubscriptionSetupPriceCoverageFindsEveryMissingAvailabilityTerritory(t *testing.T) {
	priced, missing := subscriptionSetupPriceCoverage([]subscriptionPriceImportState{
		{territoryID: "USA"},
		{territoryID: "JPN"},
		{territoryID: "USA"},
	}, []string{"CAN", "USA", "FRA"})

	if !reflect.DeepEqual(priced, []string{"JPN", "USA"}) {
		t.Fatalf("unexpected priced territories: %v", priced)
	}
	if !reflect.DeepEqual(missing, []string{"CAN", "FRA"}) {
		t.Fatalf("unexpected missing territories: %v", missing)
	}
}

func TestSubscriptionSetupHasPricedTerritory(t *testing.T) {
	states := []subscriptionPriceImportState{{territoryID: "USA"}, {territoryID: "can"}}
	if !subscriptionSetupHasPricedTerritory(states, "CAN") {
		t.Fatal("expected territory lookup to be case-insensitive")
	}
	if subscriptionSetupHasPricedTerritory(states, "FRA") {
		t.Fatal("did not expect an absent territory to be reported as priced")
	}
}

func TestSubscriptionSetupStateIsComplete(t *testing.T) {
	for _, state := range []string{"READY_TO_SUBMIT", "WAITING_FOR_REVIEW", "IN_REVIEW", "PENDING_BINARY_APPROVAL", "APPROVED"} {
		if !subscriptionSetupStateIsComplete(state) {
			t.Fatalf("expected %s to be complete", state)
		}
	}
	for _, state := range []string{"", "MISSING_METADATA", "DEVELOPER_ACTION_NEEDED", "REJECTED", "REMOVED_FROM_SALE", "UNEXPECTED"} {
		if subscriptionSetupStateIsComplete(state) {
			t.Fatalf("did not expect %s to be complete", state)
		}
	}
}

func TestSubscriptionsSetupDiagnosticRowsExposeDeepDiagnostics(t *testing.T) {
	diagnostics := []validation.SubscriptionDiagnostics{{
		SubscriptionID: "sub-1",
		Conclusion:     "known_blocker",
		Rows: []validation.SubscriptionDiagnosticRow{{
			Label:       "Review screenshot delivery",
			Status:      validation.DiagnosticStatusNo,
			Blocking:    true,
			Evidence:    "asset_delivery_state=FAILED errors=IMAGE_INCORRECT_DIMENSIONS",
			Remediation: "Delete and re-upload the screenshot.",
		}},
	}}

	headers, rows := subscriptionsSetupDiagnosticRows(diagnostics)
	if len(headers) != 7 || len(rows) != 1 {
		t.Fatalf("unexpected diagnostics table: headers=%v rows=%v", headers, rows)
	}
	if !strings.Contains(strings.Join(rows[0], " "), "IMAGE_INCORRECT_DIMENSIONS") {
		t.Fatalf("expected diagnostics evidence in table row, got %v", rows[0])
	}
}

func TestValidateExistingSubscriptionSetupLocalizationComparesEmptyDescription(t *testing.T) {
	localization := asc.Resource[asc.SubscriptionLocalizationAttributes]{
		ID: "loc-1",
		Attributes: asc.SubscriptionLocalizationAttributes{
			Locale:      "en-US",
			Name:        "Pro Monthly",
			Description: "Old description.",
		},
	}
	opts := subscriptionsSetupOptions{
		Locale:      "en-US",
		DisplayName: "Pro Monthly",
	}

	err := validateExistingSubscriptionSetupLocalization(localization, opts)
	if err == nil || !strings.Contains(err.Error(), "different description") {
		t.Fatalf("expected different description error, got %v", err)
	}
}

func TestValidateExistingSubscriptionSetupGroupLocalizationIgnoresUnspecifiedCustomAppName(t *testing.T) {
	localization := asc.Resource[asc.SubscriptionGroupLocalizationAttributes]{
		ID: "group-loc-1",
		Attributes: asc.SubscriptionGroupLocalizationAttributes{
			Locale:        "en-US",
			Name:          "Premium",
			CustomAppName: "Existing App Name",
		},
	}
	opts := subscriptionsSetupOptions{
		GroupLocale:      "en-US",
		GroupDisplayName: "Premium",
	}

	if err := validateExistingSubscriptionSetupGroupLocalization(localization, opts); err != nil {
		t.Fatalf("unspecified custom app name should not reject an existing value: %v", err)
	}
}

func TestValidateExistingSubscriptionSetupGroupLocalizationComparesSpecifiedCustomAppName(t *testing.T) {
	localization := asc.Resource[asc.SubscriptionGroupLocalizationAttributes]{
		ID: "group-loc-1",
		Attributes: asc.SubscriptionGroupLocalizationAttributes{
			Locale:        "en-US",
			Name:          "Premium",
			CustomAppName: "Existing App Name",
		},
	}
	opts := subscriptionsSetupOptions{
		GroupLocale:        "en-US",
		GroupDisplayName:   "Premium",
		GroupCustomAppName: "Requested App Name",
	}

	err := validateExistingSubscriptionSetupGroupLocalization(localization, opts)
	if err == nil || !strings.Contains(err.Error(), "different custom app name") {
		t.Fatalf("expected different custom app name error, got %v", err)
	}
}

package subscriptions

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestConsumeResolvedSubscriptionPricePage_SelectsLatestActivePerTerritory(t *testing.T) {
	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)

	page := &asc.SubscriptionPricesResponse{
		Data: []asc.Resource[asc.SubscriptionPriceAttributes]{
			newResolvedSubscriptionPriceResource("price-old-usa", "USA", "pp-old-usa", "2024-01-01", false),
			newResolvedSubscriptionPriceResource("price-current-usa", "USA", "pp-current-usa", "2025-01-01", false),
			newResolvedSubscriptionPriceResource("price-future-usa", "USA", "pp-future-usa", "2030-01-01", false),
			newResolvedSubscriptionPriceResource("price-current-gbr", "GBR", "pp-current-gbr", "2025-06-01", false),
		},
		Included: mustMarshalJSON(t, []map[string]any{
			subscriptionPricePointIncluded("pp-old-usa", "1.99", "1.40", "1.60"),
			subscriptionPricePointIncluded("pp-current-usa", "9.99", "7.00", "8.49"),
			subscriptionPricePointIncluded("pp-future-usa", "12.99", "10.00", "11.00"),
			subscriptionPricePointIncluded("pp-current-gbr", "7.99", "5.60", "6.40"),
			territoryIncluded("USA", "USD"),
			territoryIncluded("GBR", "GBP"),
		}),
	}

	candidates := make(map[string]resolvedSubscriptionPriceCandidate)
	if err := consumeResolvedSubscriptionPricePage(candidates, page, now, ""); err != nil {
		t.Fatalf("consumeResolvedSubscriptionPricePage() error = %v", err)
	}

	rows := resolvedSubscriptionRows(candidates)
	shared.SortResolvedPrices(rows)

	if len(rows) != 2 {
		t.Fatalf("expected 2 resolved rows, got %d", len(rows))
	}
	if rows[0].Territory != "GBR" || rows[0].CustomerPrice != "7.99" {
		t.Fatalf("unexpected GBR row: %+v", rows[0])
	}
	if rows[1].Territory != "USA" || rows[1].CustomerPrice != "9.99" {
		t.Fatalf("unexpected USA row: %+v", rows[1])
	}
}

func TestConsumeResolvedSubscriptionPricePage_PrefersNonPreservedSameDay(t *testing.T) {
	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)

	page := &asc.SubscriptionPricesResponse{
		Data: []asc.Resource[asc.SubscriptionPriceAttributes]{
			newResolvedSubscriptionPriceResource("price-preserved", "USA", "pp-preserved", "2025-01-01", true),
			newResolvedSubscriptionPriceResource("price-standard", "USA", "pp-standard", "2025-01-01", false),
		},
		Included: mustMarshalJSON(t, []map[string]any{
			subscriptionPricePointIncluded("pp-preserved", "4.99", "3.49", "3.99"),
			subscriptionPricePointIncluded("pp-standard", "9.99", "7.00", "8.49"),
			territoryIncluded("USA", "USD"),
		}),
	}

	candidates := make(map[string]resolvedSubscriptionPriceCandidate)
	if err := consumeResolvedSubscriptionPricePage(candidates, page, now, ""); err != nil {
		t.Fatalf("consumeResolvedSubscriptionPricePage() error = %v", err)
	}

	row := candidates["USA"].row
	if row.CustomerPrice != "9.99" {
		t.Fatalf("expected non-preserved row to win, got %+v", row)
	}
	if row.Preserved == nil || *row.Preserved {
		t.Fatalf("expected preserved=false, got %+v", row.Preserved)
	}
}

func TestConsumeResolvedSubscriptionPricePage_UsesUndatedCurrentPricesPerPlanType(t *testing.T) {
	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)

	monthly := newResolvedSubscriptionPriceResource("price-monthly", "USA", "pp-monthly", "", false)
	monthly.Attributes.PlanType = asc.SubscriptionPlanTypeMonthly
	upfront := newResolvedSubscriptionPriceResource("price-upfront", "USA", "pp-upfront", "", false)
	upfront.Attributes.PlanType = asc.SubscriptionPlanTypeUpfront

	page := &asc.SubscriptionPricesResponse{
		Data: []asc.Resource[asc.SubscriptionPriceAttributes]{
			newResolvedSubscriptionPriceResource("price-future", "USA", "pp-future", "2030-01-01", false),
			monthly,
			upfront,
		},
		Included: mustMarshalJSON(t, []map[string]any{
			subscriptionPricePointIncluded("pp-future", "12.99", "10.00", "11.00"),
			subscriptionPricePointIncluded("pp-monthly", "8.99", "6.20", "7.10"),
			subscriptionPricePointIncluded("pp-upfront", "49.99", "34.90", "39.90"),
			territoryIncluded("USA", "USD"),
		}),
	}

	candidates := make(map[string]resolvedSubscriptionPriceCandidate)
	if err := consumeResolvedSubscriptionPricePage(candidates, page, now, ""); err != nil {
		t.Fatalf("consumeResolvedSubscriptionPricePage() error = %v", err)
	}

	rows := resolvedSubscriptionRows(candidates)
	shared.SortResolvedPrices(rows)

	if len(rows) != 2 {
		t.Fatalf("expected one row per plan type, got %+v", rows)
	}
	if rows[0].PlanType != "MONTHLY" || rows[0].PriceID != "price-monthly" || rows[0].CustomerPrice != "8.99" {
		t.Fatalf("unexpected monthly row: %+v", rows[0])
	}
	if rows[1].PlanType != "UPFRONT" || rows[1].PriceID != "price-upfront" || rows[1].CustomerPrice != "49.99" {
		t.Fatalf("unexpected upfront row: %+v", rows[1])
	}
}

func TestConsumeResolvedSubscriptionPricePage_AcceptsUndatedMonthlyPrice(t *testing.T) {
	now := time.Date(2026, time.June, 13, 12, 0, 0, 0, time.UTC)

	page := &asc.SubscriptionPricesResponse{
		Data: []asc.Resource[asc.SubscriptionPriceAttributes]{
			newResolvedSubscriptionPriceResource("price-monthly", "NOR", "pp-monthly", "", false),
		},
		Included: mustMarshalJSON(t, []map[string]any{
			subscriptionPricePointIncluded("pp-monthly", "5.0", "3.4", "3.4"),
			territoryIncluded("NOR", "NOK"),
		}),
	}

	candidates := make(map[string]resolvedSubscriptionPriceCandidate)
	if err := consumeResolvedSubscriptionPricePage(candidates, page, now, asc.SubscriptionPlanTypeMonthly); err != nil {
		t.Fatalf("consumeResolvedSubscriptionPricePage() error = %v", err)
	}

	rows := resolvedSubscriptionRows(candidates)
	if len(rows) != 1 {
		t.Fatal("expected undated MONTHLY price to resolve as the active price")
	}
	row := rows[0]
	if row.CustomerPrice != "5.0" {
		t.Fatalf("expected MONTHLY customer price 5.0, got %+v", row)
	}
	if row.PlanType != "MONTHLY" {
		t.Fatalf("expected MONTHLY plan type, got %+v", row)
	}
}

func TestConsumeResolvedSubscriptionPricePage_AcceptsUndatedUpfrontPrice(t *testing.T) {
	now := time.Date(2026, time.June, 13, 12, 0, 0, 0, time.UTC)

	page := &asc.SubscriptionPricesResponse{
		Data: []asc.Resource[asc.SubscriptionPriceAttributes]{
			newResolvedSubscriptionPriceResource("price-upfront", "NOR", "pp-upfront", "", false),
		},
		Included: mustMarshalJSON(t, []map[string]any{
			subscriptionPricePointIncluded("pp-upfront", "49.0", "34.0", "34.0"),
			territoryIncluded("NOR", "NOK"),
		}),
	}

	candidates := make(map[string]resolvedSubscriptionPriceCandidate)
	if err := consumeResolvedSubscriptionPricePage(candidates, page, now, asc.SubscriptionPlanTypeUpfront); err != nil {
		t.Fatalf("consumeResolvedSubscriptionPricePage() error = %v", err)
	}

	rows := resolvedSubscriptionRows(candidates)
	if len(rows) != 1 {
		t.Fatal("expected undated UPFRONT price to resolve as the active price")
	}
	row := rows[0]
	if row.CustomerPrice != "49.0" {
		t.Fatalf("expected UPFRONT customer price 49.0, got %+v", row)
	}
	if row.PlanType != "UPFRONT" {
		t.Fatalf("expected UPFRONT plan type, got %+v", row)
	}
}

func TestConsumeResolvedSubscriptionPricePage_PrefersDatedCurrentOverUndatedFallback(t *testing.T) {
	now := time.Date(2026, time.June, 13, 12, 0, 0, 0, time.UTC)

	page := &asc.SubscriptionPricesResponse{
		Data: []asc.Resource[asc.SubscriptionPriceAttributes]{
			newResolvedSubscriptionPriceResource("price-initial", "NOR", "pp-initial", "", false),
			newResolvedSubscriptionPriceResource("price-current", "NOR", "pp-current", "2026-01-01", false),
		},
		Included: mustMarshalJSON(t, []map[string]any{
			subscriptionPricePointIncluded("pp-initial", "49.0", "34.0", "34.0"),
			subscriptionPricePointIncluded("pp-current", "59.0", "41.0", "41.0"),
			territoryIncluded("NOR", "NOK"),
		}),
	}

	candidates := make(map[string]resolvedSubscriptionPriceCandidate)
	if err := consumeResolvedSubscriptionPricePage(candidates, page, now, asc.SubscriptionPlanTypeUpfront); err != nil {
		t.Fatalf("consumeResolvedSubscriptionPricePage() error = %v", err)
	}

	rows := resolvedSubscriptionRows(candidates)
	if len(rows) != 1 {
		t.Fatal("expected a resolved UPFRONT price")
	}
	row := rows[0]
	if row.PriceID != "price-current" || row.CustomerPrice != "59.0" {
		t.Fatalf("expected latest dated price to beat the undated initial fallback, got %+v", row)
	}
}

func newResolvedSubscriptionPriceResource(
	priceID string,
	territoryID string,
	pricePointID string,
	startDate string,
	preserved bool,
) asc.Resource[asc.SubscriptionPriceAttributes] {
	relationships := map[string]any{
		"territory": map[string]any{
			"data": map[string]any{
				"type": "territories",
				"id":   territoryID,
			},
		},
		"subscriptionPricePoint": map[string]any{
			"data": map[string]any{
				"type": "subscriptionPricePoints",
				"id":   pricePointID,
			},
		},
	}

	return asc.Resource[asc.SubscriptionPriceAttributes]{
		Type:          asc.ResourceTypeSubscriptionPrices,
		ID:            priceID,
		Attributes:    asc.SubscriptionPriceAttributes{StartDate: startDate, Preserved: preserved},
		Relationships: mustMarshalJSONValue(relationships),
	}
}

func subscriptionPricePointIncluded(id, customerPrice, proceeds, proceedsYear2 string) map[string]any {
	return map[string]any{
		"type": "subscriptionPricePoints",
		"id":   id,
		"attributes": map[string]any{
			"customerPrice": customerPrice,
			"proceeds":      proceeds,
			"proceedsYear2": proceedsYear2,
		},
	}
}

func territoryIncluded(id, currency string) map[string]any {
	return map[string]any{
		"type": "territories",
		"id":   id,
		"attributes": map[string]any{
			"currency": currency,
		},
	}
}

func resolvedSubscriptionRows(candidates map[string]resolvedSubscriptionPriceCandidate) []shared.ResolvedPriceRow {
	rows := make([]shared.ResolvedPriceRow, 0, len(candidates))
	for _, candidate := range candidates {
		rows = append(rows, candidate.row)
	}
	return rows
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

func mustMarshalJSONValue(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

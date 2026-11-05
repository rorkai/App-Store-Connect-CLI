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

func TestConsumeResolvedSubscriptionPricePage_AcceptsUndatedPrice(t *testing.T) {
	now := time.Date(2026, time.June, 13, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		planType      asc.SubscriptionPlanType
		customerPrice string
		proceeds      string
	}{
		{planType: asc.SubscriptionPlanTypeMonthly, customerPrice: "5.0", proceeds: "3.4"},
		{planType: asc.SubscriptionPlanTypeUpfront, customerPrice: "49.0", proceeds: "34.0"},
	}
	for _, test := range tests {
		t.Run(string(test.planType), func(t *testing.T) {
			page := &asc.SubscriptionPricesResponse{
				Data: []asc.Resource[asc.SubscriptionPriceAttributes]{
					newResolvedSubscriptionPriceResource("price-1", "NOR", "pp-1", "", false),
				},
				Included: mustMarshalJSON(t, []map[string]any{
					subscriptionPricePointIncluded("pp-1", test.customerPrice, test.proceeds, test.proceeds),
					territoryIncluded("NOR", "NOK"),
				}),
			}

			candidates := make(map[string]resolvedSubscriptionPriceCandidate)
			if err := consumeResolvedSubscriptionPricePage(candidates, page, now, test.planType); err != nil {
				t.Fatalf("consumeResolvedSubscriptionPricePage() error = %v", err)
			}

			rows := resolvedSubscriptionRows(candidates)
			if len(rows) != 1 {
				t.Fatalf("expected undated %s price to resolve as the active price", test.planType)
			}
			if row := rows[0]; row.CustomerPrice != test.customerPrice || row.PlanType != string(test.planType) {
				t.Fatalf("expected %s customer price %s, got %+v", test.planType, test.customerPrice, row)
			}
		})
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

func TestConsumeResolvedSubscriptionPricePage_EndsPriceOnSuccessorStartDate(t *testing.T) {
	// Subscription prices carry only a startDate: each price ends on the date
	// the next one starts, the end-equals-next-start shape App Store Connect
	// uses for app and in-app purchase schedules.
	page := &asc.SubscriptionPricesResponse{
		Data: []asc.Resource[asc.SubscriptionPriceAttributes]{
			newResolvedSubscriptionPriceResource("price-old", "USA", "pp-old", "2026-01-01", false),
			newResolvedSubscriptionPriceResource("price-new", "USA", "pp-new", "2026-10-01", false),
		},
		Included: mustMarshalJSON(t, []map[string]any{
			subscriptionPricePointIncluded("pp-old", "0.99", "0.84", "0.84"),
			subscriptionPricePointIncluded("pp-new", "1.99", "1.69", "1.69"),
			territoryIncluded("USA", "USD"),
		}),
	}

	tests := []struct {
		date time.Time
		want string
	}{
		{date: time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC), want: "0.99"},
		{date: time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC), want: "1.99"},
	}
	for _, test := range tests {
		t.Run(test.date.Format("2006-01-02"), func(t *testing.T) {
			candidates := make(map[string]resolvedSubscriptionPriceCandidate)
			if err := consumeResolvedSubscriptionPricePage(candidates, page, test.date, ""); err != nil {
				t.Fatalf("consumeResolvedSubscriptionPricePage() error = %v", err)
			}
			if got := candidates["USA"].row.CustomerPrice; got != test.want {
				t.Fatalf("customerPrice = %q, want %q", got, test.want)
			}
		})
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

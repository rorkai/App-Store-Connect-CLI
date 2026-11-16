package iap

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestConsumeResolvedIAPPricePage_DateBoundaries(t *testing.T) {
	page := &asc.InAppPurchasePricesResponse{
		Data: []asc.Resource[asc.InAppPurchasePriceAttributes]{
			newResolvedIAPPriceResource("old-price", "pp-old", "2026-01-01", "2026-10-01", true),
			newResolvedIAPPriceResource("new-price", "pp-new", "2026-10-01", "", true),
			newResolvedIAPPriceResource("ending-price", "pp-ending", "2026-01-01", "2026-10-01", true),
		},
		Included: mustMarshalResolvedIAPJSON(t, []map[string]any{
			inAppPurchasePricePointIncluded("pp-old", "0.99", "0.84"),
			inAppPurchasePricePointIncluded("pp-new", "1.99", "1.69"),
			inAppPurchasePricePointIncluded("pp-ending", "0.99", "0.84"),
			resolvedTerritoryIncluded("USA", "USD"),
		}),
	}

	tests := []struct {
		now  time.Time
		want string
	}{
		// 17:30 PDT on 2026-09-30: the prices ending 2026-10-01 still apply.
		{now: time.Date(2026, time.October, 1, 0, 30, 0, 0, time.UTC), want: "0.99"},
		// Pacific midnight: prices end on their end date.
		{now: time.Date(2026, time.October, 1, 7, 0, 0, 0, time.UTC), want: "1.99"},
	}
	for _, test := range tests {
		t.Run(test.now.Format(time.RFC3339), func(t *testing.T) {
			candidates := make(map[string]resolvedIAPPriceCandidate)
			if err := consumeResolvedIAPPricePage(candidates, page, test.now, false); err != nil {
				t.Fatalf("consumeResolvedIAPPricePage() error = %v", err)
			}
			if got := candidates["USA"].row.CustomerPrice; got != test.want {
				t.Fatalf("customerPrice = %q, want %q", got, test.want)
			}
		})
	}
}

func TestConsumeResolvedIAPPricePage_PrefersManualSameDay(t *testing.T) {
	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)

	page := &asc.InAppPurchasePricesResponse{
		Data: []asc.Resource[asc.InAppPurchasePriceAttributes]{
			newResolvedIAPPriceResource("auto-price", "pp-auto", "2025-01-01", "", false),
			newResolvedIAPPriceResource("manual-price", "pp-manual", "2025-01-01", "", true),
		},
		Included: mustMarshalResolvedIAPJSON(t, []map[string]any{
			inAppPurchasePricePointIncluded("pp-auto", "4.99", "3.49"),
			inAppPurchasePricePointIncluded("pp-manual", "9.99", "8.49"),
			resolvedTerritoryIncluded("USA", "USD"),
		}),
	}

	candidates := make(map[string]resolvedIAPPriceCandidate)
	if err := consumeResolvedIAPPricePage(candidates, page, now, false); err != nil {
		t.Fatalf("consumeResolvedIAPPricePage() error = %v", err)
	}

	row := candidates["USA"].row
	if row.CustomerPrice != "9.99" {
		t.Fatalf("expected manual row to win, got %+v", row)
	}
	if row.Manual == nil || !*row.Manual {
		t.Fatalf("expected manual=true, got %+v", row.Manual)
	}
}

func TestConsumeResolvedIAPPricePage_SkipsFutureAndExpiredRows(t *testing.T) {
	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.UTC)

	page := &asc.InAppPurchasePricesResponse{
		Data: []asc.Resource[asc.InAppPurchasePriceAttributes]{
			newResolvedIAPPriceResource("expired-price", "pp-expired", "2024-01-01", "2025-01-01", true),
			newResolvedIAPPriceResource("future-price", "pp-future", "2030-01-01", "", true),
		},
		Included: mustMarshalResolvedIAPJSON(t, []map[string]any{
			inAppPurchasePricePointIncluded("pp-expired", "1.99", "1.40"),
			inAppPurchasePricePointIncluded("pp-future", "12.99", "11.04"),
			resolvedTerritoryIncluded("USA", "USD"),
		}),
	}

	for _, included := range []json.RawMessage{page.Included, nil} {
		page.Included = included
		candidates := make(map[string]resolvedIAPPriceCandidate)
		if err := consumeResolvedIAPPricePage(candidates, page, now, true); err != nil {
			t.Fatalf("consumeResolvedIAPPricePage() error = %v", err)
		}

		if len(candidates) != 0 {
			t.Fatalf("expected no resolved rows, got %+v", candidates)
		}
	}
}

// ASC encodes the price tier ("p") in IDs, and the same tier exists in every territory.
func TestConsumeResolvedIAPPricePage_SameTierInSeveralTerritories(t *testing.T) {
	now := time.Date(2026, time.November, 16, 12, 0, 0, 0, time.UTC)
	const iapID = "6758862939"
	pricePoint := func(territory string) string {
		return ascEncodedID(t, map[string]any{"s": iapID, "t": territory, "p": "10054"})
	}
	price := func(territory string) asc.Resource[asc.InAppPurchasePriceAttributes] {
		id := ascEncodedID(t, map[string]any{"s": iapID, "t": territory, "p": "10054", "sd": 0, "ed": 0})
		resource := newResolvedIAPPriceResource(id, pricePoint(territory), "", "", true)
		resource.Relationships = mustMarshalResolvedIAPValue(map[string]any{
			"territory":               map[string]any{"data": map[string]any{"type": "territories", "id": territory}},
			"inAppPurchasePricePoint": map[string]any{"data": map[string]any{"type": "inAppPurchasePricePoints", "id": pricePoint(territory)}},
		})
		return resource
	}

	page := &asc.InAppPurchasePricesResponse{
		Data: []asc.Resource[asc.InAppPurchasePriceAttributes]{price("IND"), price("DEU")},
		Included: mustMarshalResolvedIAPJSON(t, []map[string]any{
			inAppPurchasePricePointIncluded(pricePoint("IND"), "185.0", "146.81"),
			inAppPurchasePricePointIncluded(pricePoint("DEU"), "4.39", "3.12"),
			resolvedTerritoryIncluded("IND", "INR"),
			resolvedTerritoryIncluded("DEU", "EUR"),
		}),
	}

	for _, relationships := range []bool{true, false} {
		if !relationships {
			for i := range page.Data {
				page.Data[i].Relationships = nil
			}
		}
		candidates := make(map[string]resolvedIAPPriceCandidate)
		if err := consumeResolvedIAPPricePage(candidates, page, now, true); err != nil {
			t.Fatalf("consumeResolvedIAPPricePage() error = %v", err)
		}
		for territory, want := range map[string]string{"IND": "185.0 INR", "DEU": "4.39 EUR"} {
			row := candidates[territory].row
			if got := row.CustomerPrice + " " + row.Currency; got != want {
				t.Fatalf("relationships=%v %s = %q, want %q", relationships, territory, got, want)
			}
		}
	}
}

func ascEncodedID(t *testing.T, payload map[string]any) string {
	t.Helper()
	return base64.RawURLEncoding.EncodeToString(mustMarshalResolvedIAPJSON(t, payload))
}

func newResolvedIAPPriceResource(
	priceID string,
	pricePointID string,
	startDate string,
	endDate string,
	manual bool,
) asc.Resource[asc.InAppPurchasePriceAttributes] {
	relationships := map[string]any{
		"territory": map[string]any{
			"data": map[string]any{
				"type": "territories",
				"id":   "USA",
			},
		},
		"inAppPurchasePricePoint": map[string]any{
			"data": map[string]any{
				"type": "inAppPurchasePricePoints",
				"id":   pricePointID,
			},
		},
	}

	return asc.Resource[asc.InAppPurchasePriceAttributes]{
		Type:          asc.ResourceTypeInAppPurchasePrices,
		ID:            priceID,
		Attributes:    asc.InAppPurchasePriceAttributes{StartDate: startDate, EndDate: endDate, Manual: manual},
		Relationships: mustMarshalResolvedIAPValue(relationships),
	}
}

func inAppPurchasePricePointIncluded(id, customerPrice, proceeds string) map[string]any {
	return map[string]any{
		"type": "inAppPurchasePricePoints",
		"id":   id,
		"attributes": map[string]any{
			"customerPrice": customerPrice,
			"proceeds":      proceeds,
		},
	}
}

func resolvedTerritoryIncluded(id, currency string) map[string]any {
	return map[string]any{
		"type": "territories",
		"id":   id,
		"attributes": map[string]any{
			"currency": currency,
		},
	}
}

func mustMarshalResolvedIAPJSON(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

func mustMarshalResolvedIAPValue(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

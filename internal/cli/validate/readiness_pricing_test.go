package validate

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestCurrentAppPaidPricingEvidenceUsesLatestActiveManualPrice(t *testing.T) {
	page := &asc.AppPricesResponse{
		Data: []asc.Resource[asc.AppPriceV2Attributes]{
			{ID: "old", Attributes: asc.AppPriceV2Attributes{StartDate: "2025-01-01", EndDate: "2025-12-31"}, Relationships: json.RawMessage(`{"appPricePoint":{"data":{"id":"free"}}}`)},
			{ID: "current", Attributes: asc.AppPriceV2Attributes{StartDate: "2026-01-01"}, Relationships: json.RawMessage(`{"appPricePoint":{"data":{"id":"paid"}}}`)},
		},
		Included: json.RawMessage(`[
			{"type":"appPricePoints","id":"free","attributes":{"customerPrice":"0.00"}},
			{"type":"appPricePoints","id":"paid","attributes":{"customerPrice":"4.99"}}
		]`),
	}

	paid, known := currentAppPaidPricingEvidence([]*asc.AppPricesResponse{page}, time.Date(2026, time.August, 29, 0, 0, 0, 0, time.UTC))
	if !known || !paid {
		t.Fatalf("pricing evidence = paid %t known %t, want paid and known", paid, known)
	}
}

func TestCurrentAppPaidPricingEvidenceRecognizesCurrentFreePrice(t *testing.T) {
	page := &asc.AppPricesResponse{
		Data: []asc.Resource[asc.AppPriceV2Attributes]{
			{ID: "current", Attributes: asc.AppPriceV2Attributes{StartDate: "2026-01-01"}, Relationships: json.RawMessage(`{"appPricePoint":{"data":{"id":"free"}}}`)},
		},
		Included: json.RawMessage(`[
			{"type":"appPricePoints","id":"free","attributes":{"customerPrice":"0.00"}}
		]`),
	}

	paid, known := currentAppPaidPricingEvidence([]*asc.AppPricesResponse{page}, time.Date(2026, time.August, 29, 0, 0, 0, 0, time.UTC))
	if !known || paid {
		t.Fatalf("pricing evidence = paid %t known %t, want free and known", paid, known)
	}
}

func TestCurrentAppPaidPricingEvidenceFailsClosedOnIncompleteCurrentPrice(t *testing.T) {
	page := &asc.AppPricesResponse{
		Data: []asc.Resource[asc.AppPriceV2Attributes]{
			{ID: "current", Attributes: asc.AppPriceV2Attributes{StartDate: "2026-01-01"}, Relationships: json.RawMessage(`{"appPricePoint":{"data":{"id":"missing"}}}`)},
		},
	}

	paid, known := currentAppPaidPricingEvidence([]*asc.AppPricesResponse{page}, time.Date(2026, time.August, 29, 0, 0, 0, 0, time.UTC))
	if paid || known {
		t.Fatalf("pricing evidence = paid %t known %t, want unknown", paid, known)
	}
}

func TestCurrentAppPaidPricingEvidenceDateBoundaries(t *testing.T) {
	page := &asc.AppPricesResponse{
		Data: []asc.Resource[asc.AppPriceV2Attributes]{
			{ID: "old", Attributes: asc.AppPriceV2Attributes{StartDate: "2026-01-01", EndDate: "2026-08-29"}, Relationships: json.RawMessage(`{"appPricePoint":{"data":{"id":"paid"}}}`)},
			{ID: "new", Attributes: asc.AppPriceV2Attributes{StartDate: "2026-08-29"}, Relationships: json.RawMessage(`{"appPricePoint":{"data":{"id":"free"}}}`)},
		},
		Included: json.RawMessage(`[
			{"type":"appPricePoints","id":"free","attributes":{"customerPrice":"0.00"}},
			{"type":"appPricePoints","id":"paid","attributes":{"customerPrice":"4.99"}}
		]`),
	}

	tests := []struct {
		name     string
		now      time.Time
		wantPaid bool
	}{
		// 17:30 PDT on 2026-08-28: the paid price ending 2026-08-29 still applies.
		{name: "UTC midnight is still the previous Pacific day", now: time.Date(2026, time.August, 29, 0, 30, 0, 0, time.UTC), wantPaid: true},
		// The paid price ends on its end date, so only the free price applies.
		{name: "price ending today does not apply", now: time.Date(2026, time.August, 29, 20, 0, 0, 0, time.UTC), wantPaid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paid, known := currentAppPaidPricingEvidence([]*asc.AppPricesResponse{page}, test.now)
			if !known || paid != test.wantPaid {
				t.Fatalf("pricing evidence = paid %t known %t, want paid %t and known", paid, known, test.wantPaid)
			}
		})
	}
}

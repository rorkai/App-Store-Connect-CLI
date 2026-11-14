package asc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPrintTable_SubscriptionPrices(t *testing.T) {
	relationships := json.RawMessage(`{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"PRICE_POINT_1"}}}`)
	resp := &SubscriptionPricesResponse{
		Data: []Resource[SubscriptionPriceAttributes]{
			{
				ID:            "price-1",
				Relationships: relationships,
				Attributes: SubscriptionPriceAttributes{
					StartDate: "2026-01-01",
					Preserved: true,
					PlanType:  SubscriptionPlanTypeMonthly,
				},
			},
		},
		Included: json.RawMessage(`[
			{"type":"territories","id":"USA","attributes":{"currency":"USD"}},
			{"type":"subscriptionPricePoints","id":"PRICE_POINT_1","attributes":{"customerPrice":"9.99","proceeds":"6.99","proceedsYear2":"5.99"}}
		]`),
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Territory") || !strings.Contains(output, "Price Point") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "USA") {
		t.Fatalf("expected territory in output, got: %s", output)
	}
	if !strings.Contains(output, "Plan Type") || !strings.Contains(output, string(SubscriptionPlanTypeMonthly)) {
		t.Fatalf("expected plan type in output, got: %s", output)
	}
	for _, want := range []string{"Currency", "USD", "Customer Price", "9.99", "Proceeds", "6.99", "Proceeds Y2", "5.99"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected included value %q in output, got: %s", want, output)
		}
	}
}

func TestPrintMarkdown_SubscriptionPrices(t *testing.T) {
	relationships := json.RawMessage(`{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"PRICE_POINT_1"}}}`)
	resp := &SubscriptionPricesResponse{
		Data: []Resource[SubscriptionPriceAttributes]{
			{
				ID:            "price-1",
				Relationships: relationships,
				Attributes: SubscriptionPriceAttributes{
					StartDate: "2026-01-01",
					Preserved: true,
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Territory") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "PRICE_POINT_1") {
		t.Fatalf("expected price point in output, got: %s", output)
	}
}

func TestPrintTable_SubscriptionPricesIncludesPricePointRelationships(t *testing.T) {
	relationships := json.RawMessage(`{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"PRICE_POINT_1"}}}`)
	resp := &SubscriptionPricesResponse{
		Data: []Resource[SubscriptionPriceAttributes]{
			{
				ID:            "price-1",
				Relationships: relationships,
				Attributes: SubscriptionPriceAttributes{
					StartDate: "2026-01-01",
				},
			},
		},
		Included: json.RawMessage(`[
			{"type":"subscriptionPricePoints","id":"PRICE_POINT_1","relationships":{
				"territory":{"data":{"type":"territories","id":"GBR"}},
				"equalizations":{"links":{"self":"/v1/subscriptionPricePoints/PRICE_POINT_1/relationships/equalizations","related":"/v1/subscriptionPricePoints/PRICE_POINT_1/equalizations"}},
				"adjustedEqualizations":{"links":{"self":"/v1/subscriptionPricePoints/PRICE_POINT_1/adjustedEqualizations"}}
			}}
		]`),
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	for _, want := range []string{
		"Price Point Territory ID", "GBR",
		"Equalizations URL", "/v1/subscriptionPricePoints/PRICE_POINT_1/equalizations",
		"Adjusted Equalizations URL", "/v1/subscriptionPricePoints/PRICE_POINT_1/adjustedEqualizations",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in table output, got: %s", want, output)
		}
	}
}

func TestPrintMarkdown_SubscriptionPricesIncludesPricePointRelationships(t *testing.T) {
	relationships := json.RawMessage(`{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"PRICE_POINT_1"}}}`)
	resp := &SubscriptionPricesResponse{
		Data: []Resource[SubscriptionPriceAttributes]{
			{
				ID:            "price-1",
				Relationships: relationships,
				Attributes:    SubscriptionPriceAttributes{StartDate: "2026-01-01"},
			},
		},
		Included: json.RawMessage(`[
			{"type":"subscriptionPricePoints","id":"PRICE_POINT_1","relationships":{
				"territory":{"data":{"type":"territories","id":"GBR"}},
				"equalizations":{"links":{"related":"/v1/subscriptionPricePoints/PRICE_POINT_1/equalizations"}},
				"adjustedEqualizations":{"links":{"self":"/v1/subscriptionPricePoints/PRICE_POINT_1/adjustedEqualizations"}}
			}}
		]`),
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	for _, want := range []string{
		"Price Point Territory ID", "GBR",
		"Equalizations URL", "/v1/subscriptionPricePoints/PRICE_POINT_1/equalizations",
		"Adjusted Equalizations URL", "/v1/subscriptionPricePoints/PRICE_POINT_1/adjustedEqualizations",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in markdown output, got: %s", want, output)
		}
	}
}

func TestSubscriptionPricesRowsLeavesOmittedPreservedBlank(t *testing.T) {
	var resp SubscriptionPricesResponse
	if err := json.Unmarshal([]byte(`{"data":[{"type":"subscriptionPrices","id":"price-1","attributes":{"startDate":"2026-01-01"}}],"links":{}}`), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	_, rows, err := subscriptionPricesRows(&resp)
	if err != nil {
		t.Fatalf("subscriptionPricesRows() error: %v", err)
	}
	if got := rows[0][5]; got != "" {
		t.Fatalf("preserved = %q, want blank for an omitted attribute", got)
	}
}

func TestSubscriptionPricesRowsPreservesExplicitFalse(t *testing.T) {
	var resp SubscriptionPricesResponse
	if err := json.Unmarshal([]byte(`{"data":[{"type":"subscriptionPrices","id":"price-1","attributes":{"preserved":false}}],"links":{}}`), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	_, rows, err := subscriptionPricesRows(&resp)
	if err != nil {
		t.Fatalf("subscriptionPricesRows() error: %v", err)
	}
	if got := rows[0][5]; got != "false" {
		t.Fatalf("preserved = %q, want explicit false", got)
	}
}

func TestPrintTable_SubscriptionIntroductoryOfferCreateSummary(t *testing.T) {
	result := &SubscriptionIntroductoryOfferCreateSummary{
		SubscriptionID: "sub-1",
		AvailabilityID: "availability-1",
		AllTerritories: true,
		DryRun:         true,
		Total:          3,
		Created:        1,
		Skipped:        1,
		Failed:         1,
		Skips: []SubscriptionIntroductoryOfferCreateSummarySkip{{
			Territory: "USA",
			Reason:    "already exists",
		}},
		Failures: []SubscriptionIntroductoryOfferCreateSummaryFailure{{
			Territory: "CAN",
			Error:     "provider rejected request",
		}},
	}

	output := captureStdout(t, func() error { return PrintTable(result) })
	for _, want := range []string{
		"Subscription ID", "sub-1", "Availability ID", "availability-1",
		"Dry Run", "true", "Total", "3", "Created", "1", "Skipped", "1", "Failed", "1",
		"Skipped Territory", "USA", "already exists", "Failed Territory", "CAN", "provider rejected request",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output, got: %s", want, output)
		}
	}
	if strings.Contains(output, "Territory\n") {
		t.Fatalf("all-territories summary should not expose a single-territory column: %s", output)
	}
}

func TestPrintMarkdown_SubscriptionIntroductoryOfferCreateSummary(t *testing.T) {
	result := &SubscriptionIntroductoryOfferCreateSummary{
		SubscriptionID: "sub-1",
		Territory:      "USA",
		DryRun:         true,
		Total:          1,
		Created:        1,
	}

	output := captureStdout(t, func() error { return PrintMarkdown(result) })
	for _, want := range []string{
		"Subscription ID", "sub-1", "Territory", "USA", "Dry Run", "true",
		"Total", "1", "Created", "1", "Skipped", "0", "Failed", "0",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output, got: %s", want, output)
		}
	}
	if strings.Contains(output, "Availability ID") {
		t.Fatalf("single-territory summary should not expose availability column: %s", output)
	}
}

func TestPrintTable_SubscriptionGracePeriod(t *testing.T) {
	resp := &SubscriptionGracePeriodResponse{
		Data: Resource[SubscriptionGracePeriodAttributes]{
			ID: "grace-1",
			Attributes: SubscriptionGracePeriodAttributes{
				OptIn:        true,
				SandboxOptIn: false,
				Duration:     "DAY_16",
				RenewalType:  "ALL_RENEWALS",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Renewal Type") || !strings.Contains(output, "DAY_16") {
		t.Fatalf("expected grace period fields in output, got: %s", output)
	}
}

func TestPrintMarkdown_SubscriptionGracePeriod(t *testing.T) {
	resp := &SubscriptionGracePeriodResponse{
		Data: Resource[SubscriptionGracePeriodAttributes]{
			ID: "grace-1",
			Attributes: SubscriptionGracePeriodAttributes{
				OptIn:        true,
				SandboxOptIn: true,
				Duration:     "DAY_28",
				RenewalType:  "PAID_TO_PAID_ONLY",
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Opt In") || !strings.Contains(output, "DAY_28") {
		t.Fatalf("expected grace period fields in output, got: %s", output)
	}
}

func TestPrintTable_SubscriptionGroupVersions(t *testing.T) {
	resp := &SubscriptionGroupVersionsResponse{Data: []Resource[SubscriptionGroupVersionAttributes]{
		{ID: "version-1", Attributes: SubscriptionGroupVersionAttributes{Version: 2, State: "READY_FOR_REVIEW"}},
	}}

	output := captureStdout(t, func() error { return PrintTable(resp) })
	if !strings.Contains(output, "Version") || !strings.Contains(output, "READY_FOR_REVIEW") || !strings.Contains(output, "version-1") {
		t.Fatalf("expected version fields in output, got: %s", output)
	}
}

func TestPrintMarkdown_SubscriptionGroupLocalizationV2(t *testing.T) {
	resp := &SubscriptionGroupLocalizationV2Response{Data: Resource[SubscriptionGroupLocalizationV2Attributes]{
		ID: "loc-1", Attributes: SubscriptionGroupLocalizationV2Attributes{Locale: "en-US", Name: "Premium", CustomAppName: "Example"},
	}}

	output := captureStdout(t, func() error { return PrintMarkdown(resp) })
	if !strings.Contains(output, "Locale") || !strings.Contains(output, "en-US") || !strings.Contains(output, "Premium") {
		t.Fatalf("expected localization fields in output, got: %s", output)
	}
	if strings.Contains(output, "State") {
		t.Fatalf("v2 localization output exposed legacy-only state: %s", output)
	}
}

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
				},
			},
		},
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

	if !strings.Contains(output, "| ID | Territory | Price Point |") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "PRICE_POINT_1") {
		t.Fatalf("expected price point in output, got: %s", output)
	}
}

func TestPrintTable_SubscriptionPriceDeleteResult(t *testing.T) {
	result := &SubscriptionPriceDeleteResult{ID: "price-1", Deleted: true}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected deleted column in output, got: %s", output)
	}
}

func TestPrintMarkdown_SubscriptionPriceDeleteResult(t *testing.T) {
	result := &SubscriptionPriceDeleteResult{ID: "price-1", Deleted: true}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "| ID | Deleted |") {
		t.Fatalf("expected markdown header, got: %s", output)
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

	if !strings.Contains(output, "| ID | Opt In |") || !strings.Contains(output, "DAY_28") {
		t.Fatalf("expected grace period fields in output, got: %s", output)
	}
}

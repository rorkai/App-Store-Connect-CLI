package asc

import (
	"strings"
	"testing"
)

func TestPrintTable_Webhooks(t *testing.T) {
	resp := &WebhooksResponse{
		Data: []Resource[WebhookAttributes]{
			{
				ID: "wh-1",
				Attributes: WebhookAttributes{
					Name:       "Build Updates",
					URL:        "https://example.com/webhook",
					Enabled:    true,
					EventTypes: []WebhookEventType{WebhookEventBuildUploadStateUpdated},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "Events") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "Build Updates") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintMarkdown_Webhooks(t *testing.T) {
	resp := &WebhooksResponse{
		Data: []Resource[WebhookAttributes]{
			{
				ID: "wh-1",
				Attributes: WebhookAttributes{
					Name:       "Build Updates",
					URL:        "https://example.com/webhook",
					Enabled:    true,
					EventTypes: []WebhookEventType{WebhookEventBuildUploadStateUpdated},
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Events") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "Build Updates") {
		t.Fatalf("expected name in output, got: %s", output)
	}
}

func TestPrintTable_WebhookDeliveries(t *testing.T) {
	resp := &WebhookDeliveriesResponse{
		Data: []Resource[WebhookDeliveryAttributes]{
			{
				ID: "deliv-1",
				Attributes: WebhookDeliveryAttributes{
					DeliveryState: "SUCCEEDED",
					CreatedDate:   "2026-01-20T10:00:00Z",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "State") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "deliv-1") {
		t.Fatalf("expected delivery id in output, got: %s", output)
	}
}

func TestPrintMarkdown_WebhookDeliveries(t *testing.T) {
	resp := &WebhookDeliveriesResponse{
		Data: []Resource[WebhookDeliveryAttributes]{
			{
				ID: "deliv-1",
				Attributes: WebhookDeliveryAttributes{
					DeliveryState: "SUCCEEDED",
					CreatedDate:   "2026-01-20T10:00:00Z",
				},
			},
		},
	}

	output := captureStdout(t, func() error {
		return PrintMarkdown(resp)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "State") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "deliv-1") {
		t.Fatalf("expected delivery id in output, got: %s", output)
	}
}

func TestPrintTable_WebhookDeleteResult(t *testing.T) {
	result := &WebhookDeleteResult{ID: "wh-1", Deleted: true}

	output := captureStdout(t, func() error {
		return PrintTable(result)
	})

	if !strings.Contains(output, "Deleted") {
		t.Fatalf("expected header in output, got: %s", output)
	}
	if !strings.Contains(output, "wh-1") {
		t.Fatalf("expected webhook id in output, got: %s", output)
	}
}

func TestPrintMarkdown_WebhookDeleteResult(t *testing.T) {
	result := &WebhookDeleteResult{ID: "wh-1", Deleted: true}

	output := captureStdout(t, func() error {
		return PrintMarkdown(result)
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Deleted") {
		t.Fatalf("expected markdown header, got: %s", output)
	}
	if !strings.Contains(output, "wh-1") {
		t.Fatalf("expected webhook id in output, got: %s", output)
	}
}

func TestPrintTable_WebhookPing(t *testing.T) {
	resp := &WebhookPingResponse{
		Data: Resource[struct{}]{
			ID: "ping-1",
		},
	}

	output := captureStdout(t, func() error {
		return PrintTable(resp)
	})

	if !strings.Contains(output, "ping-1") {
		t.Fatalf("expected ping id in output, got: %s", output)
	}
}

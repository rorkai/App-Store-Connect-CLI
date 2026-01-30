package webhooks

import "testing"

func TestNormalizeWebhookEvents(t *testing.T) {
	values, err := normalizeWebhookEvents("build_upload_state_updated, build_beta_detail_external_build_state_updated")
	if err != nil {
		t.Fatalf("normalizeWebhookEvents() error: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(values))
	}
	if string(values[0]) != "BUILD_UPLOAD_STATE_UPDATED" {
		t.Fatalf("expected normalized event, got %q", values[0])
	}
}

func TestExtractWebhookIDFromNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/webhooks/wh-123/relationships/deliveries?cursor=abc"
	got, err := extractWebhookIDFromNextURL(next)
	if err != nil {
		t.Fatalf("extractWebhookIDFromNextURL() error: %v", err)
	}
	if got != "wh-123" {
		t.Fatalf("expected webhook id wh-123, got %q", got)
	}
}

func TestExtractWebhookIDFromNextURL_Invalid(t *testing.T) {
	_, err := extractWebhookIDFromNextURL("https://api.appstoreconnect.apple.com/v1/webhooks")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

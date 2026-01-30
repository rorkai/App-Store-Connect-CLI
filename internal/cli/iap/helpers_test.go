package iap

import (
	"encoding/json"
	"testing"
)

func TestRelationshipResourceID(t *testing.T) {
	relationships := json.RawMessage(`{"inAppPurchase":{"data":{"type":"inAppPurchases","id":"iap-1"}}}`)

	id, err := relationshipResourceID(relationships, "inAppPurchase")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if id != "iap-1" {
		t.Fatalf("expected id iap-1, got %s", id)
	}
}

func TestRelationshipResourceIDMissingID(t *testing.T) {
	relationships := json.RawMessage(`{"inAppPurchase":{"data":{"type":"inAppPurchases"}}}`)

	_, err := relationshipResourceID(relationships, "inAppPurchase")
	if err == nil {
		t.Fatalf("expected error for missing relationship id")
	}
}

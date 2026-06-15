package asc

import (
	"encoding/json"
	"testing"
)

func TestOfferCodePriceRelationshipIDsWithoutPricePoint(t *testing.T) {
	raw := json.RawMessage(`{
		"territory": {
			"data": {
				"type": "territories",
				"id": "DEU"
			}
		}
	}`)

	territoryID, pricePointID, err := offerCodePriceRelationshipIDs(raw)
	if err != nil {
		t.Fatalf("offerCodePriceRelationshipIDs() error: %v", err)
	}
	if territoryID != "DEU" {
		t.Fatalf("expected territory DEU, got %q", territoryID)
	}
	if pricePointID != "" {
		t.Fatalf("expected empty price point, got %q", pricePointID)
	}
}

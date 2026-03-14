package validate

import (
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestMapIAPsResponse_MapsItems(t *testing.T) {
	iaps, err := mapIAPsResponse(&asc.InAppPurchasesV2Response{
		Data: []asc.Resource[asc.InAppPurchaseV2Attributes]{
			{
				ID: "iap-1",
				Attributes: asc.InAppPurchaseV2Attributes{
					Name:              "Coins",
					ProductID:         "com.example.coins",
					InAppPurchaseType: "CONSUMABLE",
					State:             "MISSING_METADATA",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(iaps) != 1 {
		t.Fatalf("expected 1 IAP, got %d", len(iaps))
	}
	if iaps[0].ID != "iap-1" || iaps[0].Name != "Coins" || iaps[0].ProductID != "com.example.coins" || iaps[0].Type != "CONSUMABLE" || iaps[0].State != "MISSING_METADATA" {
		t.Fatalf("unexpected mapped IAP: %+v", iaps[0])
	}
}

func TestMapIAPsResponse_ReturnsErrorForNilResponse(t *testing.T) {
	_, err := mapIAPsResponse(nil)
	if err == nil {
		t.Fatal("expected error for nil pagination response")
	}
	if !strings.Contains(err.Error(), "unexpected nil") {
		t.Fatalf("expected nil-response error, got %v", err)
	}
}

func TestMapIAPsResponse_ReturnsErrorForUnexpectedResponseType(t *testing.T) {
	_, err := mapIAPsResponse(&asc.SubscriptionsResponse{})
	if err == nil {
		t.Fatal("expected error for unexpected pagination response type")
	}
	if !strings.Contains(err.Error(), "unexpected in-app purchases pagination response type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

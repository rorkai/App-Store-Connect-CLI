package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSetSubscriptionPriceMatrix_MultiRowOrderingAndDedupe(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"sub-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptions/sub-1" {
			t.Fatalf("expected path /v1/subscriptions/sub-1, got %s", req.URL.Path)
		}

		var payload SubscriptionUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Prices == nil {
			t.Fatal("expected prices relationship")
		}
		if got := len(payload.Data.Relationships.Prices.Data); got != 2 {
			t.Fatalf("expected 2 relationship rows after dedupe, got %d", got)
		}
		if got := len(payload.Included); got != 2 {
			t.Fatalf("expected 2 included rows after dedupe, got %d", got)
		}

		for i, wantID := range []string{"${price-1}", "${price-2}"} {
			linkage := payload.Data.Relationships.Prices.Data[i]
			if linkage.Type != ResourceTypeSubscriptionPrices || linkage.ID != wantID {
				t.Fatalf("relationship row %d = %+v, want subscriptionPrices %s", i, linkage, wantID)
			}
			if payload.Included[i].ID != wantID {
				t.Fatalf("included row %d ID = %q, want %q", i, payload.Included[i].ID, wantID)
			}
		}

		canada := payload.Included[0]
		if got := canada.Relationships.Territory.Data.ID; got != "CAN" {
			t.Fatalf("first territory = %q, want CAN", got)
		}
		if got := canada.Relationships.SubscriptionPricePoint.Data.ID; got != "point-can" {
			t.Fatalf("CAN price point = %q, want point-can", got)
		}
		if canada.Attributes == nil || canada.Attributes.StartDate != "2026-08-01" {
			t.Fatalf("CAN attributes = %+v", canada.Attributes)
		}

		usa := payload.Included[1]
		if got := usa.Relationships.Territory.Data.ID; got != "USA" {
			t.Fatalf("second territory = %q, want USA", got)
		}
		if got := usa.Relationships.SubscriptionPricePoint.Data.ID; got != "point-usa" {
			t.Fatalf("USA price point = %q, want point-usa", got)
		}
		if usa.Attributes != nil {
			t.Fatalf("expected empty USA attributes to be omitted, got %+v", usa.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	rows := []SubscriptionInlinePrice{
		{PricePointID: " point-usa ", TerritoryID: " usa "},
		{PricePointID: "point-can", TerritoryID: "CAN", Attributes: SubscriptionPriceCreateAttributes{StartDate: "2026-08-01"}},
		{PricePointID: " point-can ", TerritoryID: " can ", Attributes: SubscriptionPriceCreateAttributes{StartDate: "2026-08-01"}},
	}
	if _, err := client.SetSubscriptionPriceMatrix(context.Background(), " sub-1 ", rows); err != nil {
		t.Fatalf("SetSubscriptionPriceMatrix() error: %v", err)
	}
}

func TestSetSubscriptionPriceMatrix_RejectsConflictingDuplicateTerritory(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
	}, jsonResponse(http.StatusOK, `{}`))

	_, err := client.SetSubscriptionPriceMatrix(context.Background(), "sub-1", []SubscriptionInlinePrice{
		{PricePointID: "point-1", TerritoryID: "USA"},
		{PricePointID: "point-2", TerritoryID: " usa "},
	})
	if err == nil {
		t.Fatal("expected duplicate territory error")
	}
	if !strings.Contains(err.Error(), `duplicate territory "USA"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetSubscriptionPriceMatrix_ValidatesRequiredInput(t *testing.T) {
	tests := []struct {
		name  string
		subID string
		rows  []SubscriptionInlinePrice
		want  string
	}{
		{name: "subscription", rows: []SubscriptionInlinePrice{{PricePointID: "point-1", TerritoryID: "USA"}}, want: "subscription ID is required"},
		{name: "rows", subID: "sub-1", want: "at least one subscription price is required"},
		{name: "price point", subID: "sub-1", rows: []SubscriptionInlinePrice{{TerritoryID: "USA"}}, want: "price point ID is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
			}, jsonResponse(http.StatusOK, `{}`))

			_, err := client.SetSubscriptionPriceMatrix(context.Background(), tt.subID, tt.rows)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

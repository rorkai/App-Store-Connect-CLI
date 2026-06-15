package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSubscriptionPlanPricesBuildsExpectedInlinePatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/iris/v1/subscriptions/sub-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			Data     jsonAPIResource   `json:"data"`
			Included []jsonAPIResource `json:"included"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		prices := parseRelationshipRefs(body.Data.Relationships["prices"].Data)
		if len(prices) != 2 || len(body.Included) != 2 {
			t.Fatalf("expected two inline prices, got refs=%#v included=%#v", prices, body.Included)
		}
		if got := stringAttr(body.Included[0].Attributes, "planType"); got != "UPFRONT" {
			t.Fatalf("expected UPFRONT first, got %q", got)
		}
		if got := stringAttr(body.Included[1].Attributes, "planType"); got != "MONTHLY" {
			t.Fatalf("expected MONTHLY second, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"subscriptions","id":"sub-1"}}`))
	}))
	t.Cleanup(server.Close)

	client := &Client{httpClient: server.Client(), baseURL: server.URL + "/iris/v1"}
	got, err := client.CreateSubscriptionPlanPrices(context.Background(), "sub-1", "upfront-point", "monthly-point")
	if err != nil {
		t.Fatalf("CreateSubscriptionPlanPrices() error = %v", err)
	}
	if got.SubscriptionID != "sub-1" || got.UpfrontPricePointID != "upfront-point" || got.MonthlyPricePointID != "monthly-point" {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestSetSubscriptionPlanPricesIncludesScheduleAttributes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Included []jsonAPIResource `json:"included"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(body.Included) != 2 {
			t.Fatalf("expected two included prices, got %d", len(body.Included))
		}
		for _, price := range body.Included {
			if got := stringAttr(price.Attributes, "startDate"); got != "2026-07-01" {
				t.Fatalf("startDate = %q", got)
			}
			if got, ok := price.Attributes["preserveCurrentPrice"].(bool); !ok || !got {
				t.Fatalf("preserveCurrentPrice = %#v", price.Attributes["preserveCurrentPrice"])
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"subscriptions","id":"sub-1"}}`))
	}))
	t.Cleanup(server.Close)

	client := &Client{httpClient: server.Client(), baseURL: server.URL + "/iris/v1"}
	_, err := client.SetSubscriptionPlanPrices(context.Background(), "sub-1", []SubscriptionPlanPrice{
		{PlanType: "UPFRONT", PricePointID: "upfront", StartDate: "2026-07-01", PreserveCurrentPrice: true},
		{PlanType: "MONTHLY", PricePointID: "monthly", StartDate: "2026-07-01", PreserveCurrentPrice: true},
	})
	if err != nil {
		t.Fatalf("SetSubscriptionPlanPrices() error = %v", err)
	}
}

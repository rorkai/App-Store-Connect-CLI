package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateSubscriptionPlanAvailability(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-1","attributes":{"planType":"MONTHLY","availableInNewTerritories":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionPlanAvailabilities" {
			t.Fatalf("expected path /v1/subscriptionPlanAvailabilities, got %s", req.URL.Path)
		}
		var payload SubscriptionPlanAvailabilityCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionPlanAvailabilities {
			t.Fatalf("expected type subscriptionPlanAvailabilities, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.PlanType != SubscriptionPlanTypeMonthly {
			t.Fatalf("expected MONTHLY plan type, got %q", payload.Data.Attributes.PlanType)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Subscription == nil || payload.Data.Relationships.AvailableTerritories == nil {
			t.Fatalf("expected subscription and territory relationships")
		}
		if payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
			t.Fatalf("unexpected subscription relationship: %+v", payload.Data.Relationships.Subscription.Data)
		}
		got := payload.Data.Relationships.AvailableTerritories.Data
		if len(got) != 2 || got[0].ID != "NOR" || got[1].ID != "DEU" {
			t.Fatalf("expected NOR,DEU territories, got %#v", got)
		}
		assertAuthorized(t, req)
	}, response)

	availableInNew := true
	attrs := SubscriptionPlanAvailabilityAttributes{
		AvailableInNewTerritories: &availableInNew,
		PlanType:                  SubscriptionPlanTypeMonthly,
	}
	if _, err := client.CreateSubscriptionPlanAvailability(context.Background(), "sub-1", []string{"NOR", "DEU"}, attrs); err != nil {
		t.Fatalf("CreateSubscriptionPlanAvailability() error: %v", err)
	}
}

func TestUpdateSubscriptionPlanAvailability(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-1","attributes":{"planType":"MONTHLY","availableInNewTerritories":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionPlanAvailabilities/plan-1" {
			t.Fatalf("expected path /v1/subscriptionPlanAvailabilities/plan-1, got %s", req.URL.Path)
		}
		var payload SubscriptionPlanAvailabilityUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "plan-1" || payload.Data.Type != ResourceTypeSubscriptionPlanAvailabilities {
			t.Fatalf("unexpected update data: %+v", payload.Data)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.AvailableTerritories == nil {
			t.Fatalf("expected availableTerritories relationship")
		}
		if len(payload.Data.Relationships.AvailableTerritories.Data) != 0 {
			t.Fatalf("expected empty territory list, got %#v", payload.Data.Relationships.AvailableTerritories.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.UpdateSubscriptionPlanAvailability(context.Background(), "plan-1", nil, nil); err != nil {
		t.Fatalf("UpdateSubscriptionPlanAvailability() error: %v", err)
	}
}

func TestGetSubscriptionPlanAvailabilitiesForSubscriptionFiltersPlanType(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[
		{"type":"subscriptionPlanAvailabilities","id":"plan-monthly","attributes":{"planType":"MONTHLY","availableInNewTerritories":true}},
		{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":false}}
	],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/planAvailabilities?cursor=abc"},"meta":{"paging":{"total":2,"limit":50}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptions/sub-1/planAvailabilities" {
			t.Fatalf("expected path /v1/subscriptions/sub-1/planAvailabilities, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetSubscriptionPlanAvailabilitiesForSubscription(
		context.Background(),
		"sub-1",
		WithSubscriptionPlanAvailabilitiesPlanTypes(SubscriptionPlanTypeMonthly),
	)
	if err != nil {
		t.Fatalf("GetSubscriptionPlanAvailabilitiesForSubscription() error: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "plan-monthly" {
		t.Fatalf("expected only monthly plan availability, got %#v", resp.Data)
	}
	if resp.Links.Next != "" {
		t.Fatalf("expected next link to be cleared after filtering, got %q", resp.Links.Next)
	}
	if got := ParsePagingTotal(resp.Meta); got != 1 {
		t.Fatalf("expected paging total 1 after filtering, got %d", got)
	}
}

func TestAdjustFilteredPagingMetadata(t *testing.T) {
	t.Parallel()

	updated := adjustFilteredPagingMetadata(json.RawMessage(`{"paging":{"total":2,"limit":50}}`), 1)
	if got := ParsePagingTotal(updated); got != 1 {
		t.Fatalf("expected paging total 1, got %d", got)
	}

	unchanged := adjustFilteredPagingMetadata(json.RawMessage(`{"paging":{"limit":50}}`), 1)
	if got := ParsePagingTotal(unchanged); got != 0 {
		t.Fatalf("expected unchanged metadata without total, got %d", got)
	}
}

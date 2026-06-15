package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListSubscriptionPlanAvailabilitiesBuildsExpectedRequest(t *testing.T) {
	var gotPath string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "plan-1",
				"type": "subscriptionPlanAvailabilities",
				"attributes": {"availableInNewTerritories": true, "planType": "UPFRONT"},
				"relationships": {
					"availableTerritories": {
						"data": [{"type": "territories", "id": "USA"}]
					}
				}
			}]
		}`))
	}))
	t.Cleanup(server.Close)

	client := &Client{httpClient: server.Client(), baseURL: server.URL + "/iris/v1"}
	got, err := client.ListSubscriptionPlanAvailabilities(context.Background(), "sub-1")
	if err != nil {
		t.Fatalf("ListSubscriptionPlanAvailabilities() error = %v", err)
	}

	if gotPath != "/iris/v1/subscriptions/sub-1/planAvailabilities" {
		t.Fatalf("expected plan availabilities path, got %q", gotPath)
	}
	if !strings.Contains(gotQuery, "include=availableTerritories") || !strings.Contains(gotQuery, "limit%5BavailableTerritories%5D=200") {
		t.Fatalf("unexpected query: %q", gotQuery)
	}
	if len(got) != 1 || got[0].ID != "plan-1" || got[0].PlanType != "UPFRONT" || !got[0].AvailableInNewTerritories {
		t.Fatalf("unexpected decoded plan availability: %#v", got)
	}
	if len(got[0].AvailableTerritories) != 1 || got[0].AvailableTerritories[0] != "USA" {
		t.Fatalf("unexpected decoded territories: %#v", got[0].AvailableTerritories)
	}
	if !got[0].AvailableTerritoriesLoaded {
		t.Fatalf("expected available territories relationship to be loaded: %#v", got[0])
	}
}

func TestRemoveSubscriptionPlanAvailabilityFromSaleBuildsExpectedRequest(t *testing.T) {
	var payload struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				AvailableInNewTerritories bool `json:"availableInNewTerritories"`
			} `json:"attributes"`
			Relationships struct {
				AvailableTerritories struct {
					Data []any `json:"data"`
				} `json:"availableTerritories"`
			} `json:"relationships"`
		} `json:"data"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/iris/v1/subscriptionPlanAvailabilities/plan-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "plan-1",
				"type": "subscriptionPlanAvailabilities",
				"attributes": {"availableInNewTerritories": false, "planType": "UPFRONT"}
			}
		}`))
	}))
	t.Cleanup(server.Close)

	client := &Client{httpClient: server.Client(), baseURL: server.URL + "/iris/v1"}
	got, err := client.RemoveSubscriptionPlanAvailabilityFromSale(context.Background(), "plan-1")
	if err != nil {
		t.Fatalf("RemoveSubscriptionPlanAvailabilityFromSale() error = %v", err)
	}

	if payload.Data.Type != "subscriptionPlanAvailabilities" || payload.Data.ID != "plan-1" {
		t.Fatalf("unexpected payload identity: %#v", payload.Data)
	}
	if payload.Data.Attributes.AvailableInNewTerritories {
		t.Fatal("expected availableInNewTerritories=false")
	}
	if len(payload.Data.Relationships.AvailableTerritories.Data) != 0 {
		t.Fatalf("expected availableTerritories.data to be empty, got %#v", payload.Data.Relationships.AvailableTerritories.Data)
	}
	if got.ID != "plan-1" || got.AvailableInNewTerritories {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestCreateSubscriptionPlanAvailabilityBuildsExpectedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/iris/v1/subscriptionPlanAvailabilities" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			Data jsonAPIResource `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got := stringAttr(body.Data.Attributes, "planType"); got != "MONTHLY" {
			t.Fatalf("expected MONTHLY, got %q", got)
		}
		subscription := parseRelationshipRefs(body.Data.Relationships["subscription"].Data)
		if len(subscription) != 1 || subscription[0].ID != "sub-1" {
			t.Fatalf("unexpected subscription relationship: %#v", subscription)
		}
		territories := parseRelationshipRefs(body.Data.Relationships["availableTerritories"].Data)
		if len(territories) != 1 || territories[0].ID != "NOR" {
			t.Fatalf("unexpected territories: %#v", territories)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-monthly","attributes":{"planType":"MONTHLY","availableInNewTerritories":false},"relationships":{"availableTerritories":{"data":[{"type":"territories","id":"NOR"}]}}}}`))
	}))
	t.Cleanup(server.Close)

	client := &Client{httpClient: server.Client(), baseURL: server.URL + "/iris/v1"}
	got, err := client.CreateSubscriptionPlanAvailability(context.Background(), "sub-1", "MONTHLY", []string{"nor"}, false)
	if err != nil {
		t.Fatalf("CreateSubscriptionPlanAvailability() error = %v", err)
	}
	if got.ID != "plan-monthly" || got.PlanType != "MONTHLY" {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestSubscriptionPlanAvailabilityRequiresIDs(t *testing.T) {
	client := &Client{}
	if _, err := client.ListSubscriptionPlanAvailabilities(context.Background(), " "); err == nil || !strings.Contains(err.Error(), "subscription id is required") {
		t.Fatalf("expected subscription id error, got %v", err)
	}
	if _, err := client.RemoveSubscriptionPlanAvailabilityFromSale(context.Background(), " "); err == nil || !strings.Contains(err.Error(), "subscription plan availability id is required") {
		t.Fatalf("expected plan availability id error, got %v", err)
	}
}

package validate

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestFetchSubscriptionPriceTerritories_DeduplicatesAndSortsTerritories(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return buildsJSONResponse(http.StatusMethodNotAllowed, `{"errors":[{"status":"405"}]}`)
		}
		if req.URL.Path != "/v1/subscriptions/sub-1/prices" {
			t.Fatalf("expected subscription prices path, got %q", req.URL.Path)
		}
		if got := req.URL.Query().Get("include"); got != "territory" {
			t.Fatalf("expected include=territory query, got %q", got)
		}
		if got := req.URL.Query().Get("limit"); got != "200" {
			t.Fatalf("expected limit=200 query, got %q", got)
		}
		return buildsJSONResponse(http.StatusOK, `{
			"data": [
				{
					"type": "subscriptionPrices",
					"id": "price-1",
					"relationships": {
						"territory": {
							"data": {"type": "territories", "id": "USA"}
						}
					}
				},
				{
					"type": "subscriptionPrices",
					"id": "price-2",
					"relationships": {
						"territory": {
							"data": {"type": "territories", "id": "JPN"}
						}
					}
				},
				{
					"type": "subscriptionPrices",
					"id": "price-3",
					"relationships": {
						"territory": {
							"data": {"type": "territories", "id": "USA"}
						}
					}
				}
			]
		}`)
	}))

	territoryIDs, status, err := fetchSubscriptionPriceTerritories(context.Background(), client, "sub-1")
	if err != nil {
		t.Fatalf("fetchSubscriptionPriceTerritories() error = %v", err)
	}
	if !status.Verified || status.SkipReason != "" {
		t.Fatalf("expected verified status without skip reason, got %+v", status)
	}
	if len(territoryIDs) != 2 {
		t.Fatalf("expected 2 unique territory IDs, got %d (%v)", len(territoryIDs), territoryIDs)
	}
	if territoryIDs[0] != "JPN" || territoryIDs[1] != "USA" {
		t.Fatalf("expected sorted unique territory IDs [JPN USA], got %v", territoryIDs)
	}
}

func TestFetchSubscriptionPriceTerritories_SkipsWhenRelationshipsCannotBeDecoded(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return buildsJSONResponse(http.StatusOK, `{
			"data": [
				{
					"type": "subscriptionPrices",
					"id": "price-1",
					"relationships": "not-an-object"
				}
			]
		}`)
	}))

	territoryIDs, status, err := fetchSubscriptionPriceTerritories(context.Background(), client, "sub-1")
	if err != nil {
		t.Fatalf("fetchSubscriptionPriceTerritories() error = %v", err)
	}
	if territoryIDs != nil {
		t.Fatalf("expected no territory IDs when verification is skipped, got %v", territoryIDs)
	}
	if status.Verified {
		t.Fatalf("expected skipped status, got %+v", status)
	}
	if !strings.Contains(status.SkipReason, "relationships could not be decoded") {
		t.Fatalf("expected relationships decode skip reason, got %q", status.SkipReason)
	}
}

func TestFetchSubscriptionPriceTerritories_SkipsWhenTerritoryRelationshipMissing(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return buildsJSONResponse(http.StatusOK, `{
			"data": [
				{
					"type": "subscriptionPrices",
					"id": "price-1",
					"relationships": {}
				}
			]
		}`)
	}))

	territoryIDs, status, err := fetchSubscriptionPriceTerritories(context.Background(), client, "sub-1")
	if err != nil {
		t.Fatalf("fetchSubscriptionPriceTerritories() error = %v", err)
	}
	if territoryIDs != nil {
		t.Fatalf("expected no territory IDs when verification is skipped, got %v", territoryIDs)
	}
	if status.Verified {
		t.Fatalf("expected skipped status, got %+v", status)
	}
	if !strings.Contains(status.SkipReason, "omitted territory relationships") {
		t.Fatalf("expected missing-territory skip reason, got %q", status.SkipReason)
	}
}

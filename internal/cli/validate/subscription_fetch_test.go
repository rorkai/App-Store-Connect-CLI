package validate

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestFetchSubscriptions_BoundsGroupAndMetadataFanOutAndSortsDeterministically(t *testing.T) {
	const delay = 50 * time.Millisecond
	tracker := &requestConcurrencyTracker{}
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/v1/apps/app-1/subscriptionGroups" {
			return buildsJSONResponse(http.StatusOK, `{"data":[
				{"type":"subscriptionGroups","id":"group-z","attributes":{"referenceName":"Group Z"}},
				{"type":"subscriptionGroups","id":"group-a","attributes":{"referenceName":"Group A"}}
			]}`)
		}
		if err := tracker.wait(req.Context(), delay); err != nil {
			return nil, err
		}

		switch {
		case strings.HasPrefix(req.URL.Path, "/v1/subscriptionGroups/") && strings.HasSuffix(req.URL.Path, "/subscriptions"):
			groupID := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1/subscriptionGroups/"), "/subscriptions")
			subscriptionID := strings.TrimPrefix(groupID, "group-")
			return buildsJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-`+subscriptionID+`","attributes":{"name":"Subscription `+subscriptionID+`","productId":"product.`+subscriptionID+`","state":"MISSING_METADATA"}}]}`)
		case strings.HasPrefix(req.URL.Path, "/v1/subscriptionGroups/") && strings.HasSuffix(req.URL.Path, "/subscriptionGroupLocalizations"):
			return buildsJSONResponse(http.StatusOK, `{"data":[]}`)
		case strings.HasSuffix(req.URL.Path, "/appStoreReviewScreenshot"), strings.HasSuffix(req.URL.Path, "/subscriptionAvailability"):
			return buildsJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`)
		case strings.HasSuffix(req.URL.Path, "/images"),
			strings.HasSuffix(req.URL.Path, "/prices"),
			strings.HasSuffix(req.URL.Path, "/planAvailabilities"),
			strings.HasSuffix(req.URL.Path, "/subscriptionLocalizations"),
			strings.HasSuffix(req.URL.Path, "/introductoryOffers"),
			strings.HasSuffix(req.URL.Path, "/promotionalOffers"),
			strings.HasSuffix(req.URL.Path, "/winBackOffers"):
			return buildsJSONResponse(http.StatusOK, `{"data":[]}`)
		default:
			return buildsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_REQUEST"}]}`)
		}
	}))

	started := time.Now()
	subscriptions, err := fetchSubscriptions(context.Background(), client, "app-1")
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("fetchSubscriptions() error = %v", err)
	}
	if got := tracker.max.Load(); got != readinessConcurrencyLimit {
		t.Fatalf("maximum in-flight requests = %d, want exactly %d", got, readinessConcurrencyLimit)
	}
	if elapsed >= 800*time.Millisecond {
		t.Fatalf("bounded subscription fan-out took %s; serial delayed fixture time is %s", elapsed, 20*delay)
	}
	if len(subscriptions) != 2 {
		t.Fatalf("got %d subscriptions, want 2", len(subscriptions))
	}
	if subscriptions[0].ID != "sub-a" || subscriptions[0].GroupID != "group-a" {
		t.Fatalf("first subscription is not stably sorted: %+v", subscriptions[0])
	}
	if subscriptions[1].ID != "sub-z" || subscriptions[1].GroupID != "group-z" {
		t.Fatalf("second subscription is not stably sorted: %+v", subscriptions[1])
	}
}

func TestFetchSubscriptionPlanAvailabilitiesPaginatesPlansAndTerritories(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/sub-1/planAvailabilities" && req.URL.Query().Get("cursor") == "next":
			return buildsJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-monthly","attributes":{"planType":"MONTHLY","availableInNewTerritories":false}}]}`)
		case req.URL.Path == "/v1/subscriptions/sub-1/planAvailabilities":
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Fatalf("expected plan limit=200, got %q", got)
			}
			return buildsJSONResponse(http.StatusOK, `{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT","availableInNewTerritories":true}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/planAvailabilities?cursor=next"}}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront/relationships/availableTerritories" && req.URL.Query().Get("cursor") == "next":
			return buildsJSONResponse(http.StatusOK, `{"data":[{"type":"territories","id":"CAN"}]}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-upfront/relationships/availableTerritories":
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Fatalf("expected territory limit=200, got %q", got)
			}
			return buildsJSONResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptionPlanAvailabilities/plan-upfront/relationships/availableTerritories?cursor=next"}}`)
		case req.URL.Path == "/v1/subscriptionPlanAvailabilities/plan-monthly/relationships/availableTerritories":
			return buildsJSONResponse(http.StatusOK, `{"data":[]}`)
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	}))

	plans, status, err := fetchSubscriptionPlanAvailabilities(context.Background(), client, "sub-1")
	if err != nil {
		t.Fatalf("fetchSubscriptionPlanAvailabilities() error = %v", err)
	}
	if !status.Verified || status.SkipReason != "" {
		t.Fatalf("expected verified status, got %+v", status)
	}
	if len(plans) != 2 {
		t.Fatalf("expected two plans, got %+v", plans)
	}
	if plans[0].PlanType != "MONTHLY" || len(plans[0].Territories) != 0 {
		t.Fatalf("unexpected monthly plan: %+v", plans[0])
	}
	if plans[1].PlanType != "UPFRONT" || strings.Join(plans[1].Territories, ",") != "CAN,USA" {
		t.Fatalf("unexpected upfront plan: %+v", plans[1])
	}
}

func TestFetchSubscriptionPlanAvailabilitiesPreservesUnverifiedFallback(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return buildsJSONResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)
	}))

	plans, status, err := fetchSubscriptionPlanAvailabilities(context.Background(), client, "sub-1")
	if err != nil {
		t.Fatalf("fetchSubscriptionPlanAvailabilities() error = %v", err)
	}
	if plans != nil || status.Verified || !strings.Contains(status.SkipReason, "subscription plan availabilities") {
		t.Fatalf("expected unverified fallback with endpoint context, got plans=%+v status=%+v", plans, status)
	}
}

func TestFetchSubscriptionAvailabilityTerritoriesNormalizesTerritoryIDs(t *testing.T) {
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/subscriptions/sub-1/subscriptionAvailability":
			return buildsJSONResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":true}}}`)
		case "/v1/subscriptionAvailabilities/availability-1/availableTerritories":
			return buildsJSONResponse(http.StatusOK, `{"data":[{"type":"territories","id":" can "},{"type":"territories","id":"fra"}]}`)
		default:
			t.Fatalf("unexpected request: %s", req.URL.String())
			return nil, nil
		}
	}))

	id, territories, availableInNew, status, err := fetchSubscriptionAvailabilityTerritories(context.Background(), client, "sub-1")
	if err != nil {
		t.Fatalf("fetchSubscriptionAvailabilityTerritories() error = %v", err)
	}
	if id != "availability-1" || strings.Join(territories, ",") != "CAN,FRA" {
		t.Fatalf("unexpected normalized availability: id=%q territories=%v", id, territories)
	}
	if availableInNew == nil || !*availableInNew || !status.Verified {
		t.Fatalf("unexpected availability metadata: availableInNew=%v status=%+v", availableInNew, status)
	}
}

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
		if got := req.URL.Query().Get("filter[planType]"); got != "UPFRONT" {
			t.Fatalf("expected filter[planType]=UPFRONT query, got %q", got)
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

func TestFetchSubscriptionReviewScreenshot_ReportsAssetDeliveryState(t *testing.T) {
	tests := []struct {
		name         string
		state        string
		errors       string
		wantDetails  []string
		wantVerified bool
		wantReason   string
	}{
		{name: "complete", state: "COMPLETE", wantVerified: true},
		{name: "failed", state: "FAILED", errors: `,"errors":[{"code":"IMAGE_INCORRECT_DIMENSIONS","message":"The image dimensions are invalid."}]`, wantDetails: []string{"IMAGE_INCORRECT_DIMENSIONS: The image dimensions are invalid."}, wantVerified: true},
		{name: "processing", state: "PROCESSING", wantReason: "PROCESSING"},
		{name: "upload complete", state: "UPLOAD_COMPLETE", wantReason: "UPLOAD_COMPLETE"},
		{name: "unknown", state: "", wantReason: "did not return"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if got := req.URL.Query().Get("fields[subscriptionAppStoreReviewScreenshots]"); got != "assetDeliveryState" {
					t.Fatalf("expected assetDeliveryState field request, got %q", got)
				}
				attributes := `{}`
				if test.state != "" {
					attributes = `{"assetDeliveryState":{"state":"` + test.state + `"` + test.errors + `}}`
				}
				return buildsJSONResponse(http.StatusOK, `{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":`+attributes+`}}`)
			}))

			id, state, details, status, err := fetchSubscriptionReviewScreenshot(context.Background(), client, "sub-1")
			if err != nil {
				t.Fatalf("fetchSubscriptionReviewScreenshot() error = %v", err)
			}
			if id != "shot-1" || state != test.state {
				t.Fatalf("got id=%q state=%q, want id=shot-1 state=%q", id, state, test.state)
			}
			if !slices.Equal(details, test.wantDetails) {
				t.Fatalf("details=%v, want %v", details, test.wantDetails)
			}
			if status.Verified != test.wantVerified {
				t.Fatalf("verified=%t, want %t; status=%+v", status.Verified, test.wantVerified, status)
			}
			if test.wantReason != "" && !strings.Contains(status.SkipReason, test.wantReason) {
				t.Fatalf("skip reason %q does not contain %q", status.SkipReason, test.wantReason)
			}
		})
	}
}

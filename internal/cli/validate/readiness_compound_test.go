package validate

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

func TestBuildReadinessReport_UsesCompoundReadsWithoutFallbacks(t *testing.T) {
	var mu sync.Mutex
	requests := make(map[string]int)
	queries := make(map[string]string)

	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		requests[req.URL.Path]++
		queries[req.URL.Path] = req.URL.Query().Encode()
		mu.Unlock()

		switch req.URL.Path {
		case "/v1/appStoreVersions/ver-1":
			return buildsJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "ver-1",
					"attributes": {"platform":"IOS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION","copyright":"2026 Test"},
					"relationships": {
						"app": {"data":{"type":"apps","id":"app-1"}},
						"appStoreVersionLocalizations": {"data":[{"type":"appStoreVersionLocalizations","id":"ver-loc-1"}],"meta":{"paging":{"total":1,"limit":50}}},
						"build": {"data":{"type":"builds","id":"build-1"}},
						"appStoreReviewDetail": {"data":{"type":"appStoreReviewDetails","id":"review-1"}}
					}
				},
				"included": [
					{"type":"appStoreVersionLocalizations","id":"ver-loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","whatsNew":"Notes","supportUrl":"https://example.com/support"}},
					{"type":"builds","id":"build-1","attributes":{"version":"1.0","processingState":"VALID","expired":false,"usesNonExemptEncryption":false}},
					{"type":"appStoreReviewDetails","id":"review-1","attributes":{"contactFirstName":"A","contactLastName":"B","contactEmail":"a@example.com","contactPhone":"123"}}
				]
			}`)
		case "/v1/apps/app-1/appInfos":
			return buildsJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "appInfos",
					"id": "info-1",
					"attributes": {"state":"PREPARE_FOR_SUBMISSION"},
					"relationships": {
						"app": {"data":{"type":"apps","id":"app-1"}},
						"ageRatingDeclaration": {"data":{"type":"ageRatingDeclarations","id":"age-1"}},
						"appInfoLocalizations": {"data":[{"type":"appInfoLocalizations","id":"info-loc-1"}],"meta":{"paging":{"total":1,"limit":50}}},
						"primaryCategory": {"data":{"type":"appCategories","id":"cat-1"}}
					}
				}],
				"included": [
					{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US","contentRightsDeclaration":"DOES_NOT_USE_THIRD_PARTY_CONTENT"}},
					{"type":"ageRatingDeclarations","id":"age-1","attributes":{"advertising":false}},
					{"type":"appInfoLocalizations","id":"info-loc-1","attributes":{"locale":"en-US","name":"My App","privacyPolicyUrl":"https://example.com/privacy"}},
					{"type":"appCategories","id":"cat-1","attributes":{}}
				]
			}`)
		case "/v1/apps/app-1/appPriceSchedule":
			return buildsJSONResponse(http.StatusOK, `{"data":{"type":"appPriceSchedules","id":"schedule-1"}}`)
		case "/v1/apps/app-1":
			return buildsJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US","contentRightsDeclaration":"DOES_NOT_USE_THIRD_PARTY_CONTENT"}}}`)
		case "/v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
			return buildsJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"ver-loc-1","attributes":{"locale":"en-US"}}]}`)
		case "/v1/appInfos/info-1/appInfoLocalizations":
			return buildsJSONResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"info-loc-1","attributes":{"locale":"en-US"}}]}`)
		case "/v1/appInfos/info-1/relationships/primaryCategory":
			return buildsJSONResponse(http.StatusOK, `{"data":{"type":"appCategories","id":"cat-1"}}`)
		case "/v1/appInfos/info-1/ageRatingDeclaration":
			return buildsJSONResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"age-1","attributes":{"advertising":false}}}`)
		case "/v1/appStoreVersions/ver-1/appStoreReviewDetail":
			return buildsJSONResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"review-1","attributes":{}}}`)
		case "/v1/appStoreVersions/ver-1/build":
			return buildsJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"usesNonExemptEncryption":false}}}`)
		default:
			return buildsJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`)
		}
	}))

	restoreClient := SetClientFactory(func() (*asc.Client, error) { return client, nil })
	t.Cleanup(restoreClient)
	restoreAvailability := SetFetchAvailableTerritoriesFunc(func(context.Context, *asc.Client, string) (string, int, error) {
		return "availability-1", 1, nil
	})
	t.Cleanup(restoreAvailability)
	pricingTerritoryFetches := 0
	restorePricingTerritories := SetFetchPricingTerritoriesFunc(func(context.Context, *asc.Client) ([]string, error) {
		pricingTerritoryFetches++
		return []string{"USA", "CAN"}, nil
	})
	t.Cleanup(restorePricingTerritories)
	restoreScreenshots := SetFetchScreenshotSetsFunc(func(context.Context, *asc.Client, []asc.Resource[asc.AppStoreVersionLocalizationAttributes]) ([]validation.ScreenshotSet, error) {
		return nil, nil
	})
	t.Cleanup(restoreScreenshots)
	restoreSubscriptions := SetFetchSubscriptionsFunc(func(context.Context, *asc.Client, string) ([]validation.Subscription, error) {
		return nil, nil
	})
	t.Cleanup(restoreSubscriptions)
	restoreIAPs := SetFetchIAPsFunc(func(context.Context, *asc.Client, string) ([]validation.IAP, error) {
		return nil, nil
	})
	t.Cleanup(restoreIAPs)

	if _, err := BuildReadinessReport(context.Background(), ReadinessOptions{AppID: "app-1", VersionID: "ver-1"}); err != nil {
		t.Fatalf("BuildReadinessReport() error = %v", err)
	}

	if got := queries["/v1/appStoreVersions/ver-1"]; got != "include=appStoreVersionLocalizations%2Cbuild%2CappStoreReviewDetail&limit%5BappStoreVersionLocalizations%5D=50" {
		t.Fatalf("unexpected version compound query: %q", got)
	}
	if got := queries["/v1/apps/app-1/appInfos"]; got != "include=app%2CageRatingDeclaration%2CappInfoLocalizations%2CprimaryCategory&limit%5BappInfoLocalizations%5D=50" {
		t.Fatalf("unexpected app-info compound query: %q", got)
	}
	totalRequests := 0
	for _, count := range requests {
		totalRequests += count
	}
	if totalRequests != 3 {
		t.Fatalf("compound readiness request count = %d, want 3 (version, app infos, price schedule)", totalRequests)
	}
	if pricingTerritoryFetches != 1 {
		t.Fatalf("pricing territory fetches = %d, want 1", pricingTerritoryFetches)
	}

	for _, path := range []string{
		"/v1/apps/app-1",
		"/v1/appStoreVersions/ver-1/appStoreVersionLocalizations",
		"/v1/appInfos/info-1/appInfoLocalizations",
		"/v1/appInfos/info-1/relationships/primaryCategory",
		"/v1/appInfos/info-1/ageRatingDeclaration",
		"/v1/appStoreVersions/ver-1/appStoreReviewDetail",
		"/v1/appStoreVersions/ver-1/build",
	} {
		if got := requests[path]; got != 0 {
			t.Fatalf("expected compound response to avoid %s, got %d request(s)", path, got)
		}
	}
}

func TestFetchVersionReadinessData_MissingOrAmbiguousMemberFallsBackOnlyForReviewDetails(t *testing.T) {
	tests := []struct {
		name     string
		included string
	}{
		{name: "missing included member", included: `[]`},
		{
			name: "duplicate included member",
			included: `[
				{"type":"appStoreReviewDetails","id":"review-1","attributes":{}},
				{"type":"appStoreReviewDetails","id":"review-1","attributes":{}}
			]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var mu sync.Mutex
			requests := make(map[string]int)
			queries := make(map[string]string)
			client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				mu.Lock()
				requests[req.URL.Path]++
				queries[req.URL.Path] = req.URL.Query().Encode()
				mu.Unlock()

				switch req.URL.Path {
				case "/v1/appStoreVersions/ver-1":
					return buildsJSONResponse(http.StatusOK, fmt.Sprintf(`{
						"data": {
							"type":"appStoreVersions",
							"id":"ver-1",
							"attributes":{"platform":"IOS","versionString":"1.0"},
							"relationships": {
								"appStoreVersionLocalizations":{"data":[],"meta":{"paging":{"total":0,"limit":50}}},
								"build":{"data":null},
								"appStoreReviewDetail":{"data":{"type":"appStoreReviewDetails","id":"review-1"}}
							}
						},
						"included": %s
					}`, test.included))
				case "/v1/appStoreVersions/ver-1/appStoreReviewDetail":
					return buildsJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND"}]}`)
				default:
					return buildsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_REQUEST"}]}`)
				}
			}))

			result, err := fetchVersionReadinessData(withReadinessRequestGate(context.Background()), client, "ver-1", nil)
			if err != nil {
				t.Fatalf("fetchVersionReadinessData() error = %v", err)
			}
			if result.reviewDetails != nil {
				t.Fatalf("expected NotFound fallback to preserve absent review details, got %+v", result.reviewDetails)
			}
			if got := queries["/v1/appStoreVersions/ver-1"]; got != "include=appStoreVersionLocalizations%2Cbuild%2CappStoreReviewDetail&limit%5BappStoreVersionLocalizations%5D=50" {
				t.Fatalf("unexpected version compound query: %q", got)
			}
			if got := requests["/v1/appStoreVersions/ver-1/appStoreReviewDetail"]; got != 1 {
				t.Fatalf("expected one review-detail fallback, got %d", got)
			}
			for _, path := range []string{
				"/v1/appStoreVersions/ver-1/appStoreVersionLocalizations",
				"/v1/appStoreVersions/ver-1/build",
			} {
				if got := requests[path]; got != 0 {
					t.Fatalf("expected no unrelated fallback for %s, got %d request(s)", path, got)
				}
			}
		})
	}
}

func TestFetchAppInfoReadinessData_MissingIncludedAgeRatingFallsBackOnlyForAgeRating(t *testing.T) {
	var mu sync.Mutex
	requests := make(map[string]int)
	queries := make(map[string]string)
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		requests[req.URL.Path]++
		queries[req.URL.Path] = req.URL.Query().Encode()
		mu.Unlock()

		switch req.URL.Path {
		case "/v1/apps/app-1/appInfos":
			return buildsJSONResponse(http.StatusOK, `{
				"data": [{
					"type":"appInfos",
					"id":"info-1",
					"attributes":{"state":"PREPARE_FOR_SUBMISSION"},
					"relationships": {
						"app":{"data":{"type":"apps","id":"app-1"}},
						"ageRatingDeclaration":{"data":{"type":"ageRatingDeclarations","id":"age-1"}},
						"appInfoLocalizations":{"data":[],"meta":{"paging":{"total":0,"limit":50}}},
						"primaryCategory":{"data":null}
					}
				}],
				"included":[{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US"}}]
			}`)
		case "/v1/appInfos/info-1/ageRatingDeclaration":
			return buildsJSONResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"age-1","attributes":{"advertising":false}}}`)
		default:
			return buildsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_REQUEST"}]}`)
		}
	}))

	result, err := fetchAppInfoReadinessData(withReadinessRequestGate(context.Background()), client, "app-1")
	if err != nil {
		t.Fatalf("fetchAppInfoReadinessData() error = %v", err)
	}
	if result.ageRatingDeclaration == nil || result.ageRatingDeclaration.Advertising == nil || *result.ageRatingDeclaration.Advertising {
		t.Fatalf("expected age-rating fallback data, got %+v", result.ageRatingDeclaration)
	}
	if got := queries["/v1/apps/app-1/appInfos"]; got != "include=app%2CageRatingDeclaration%2CappInfoLocalizations%2CprimaryCategory&limit%5BappInfoLocalizations%5D=50" {
		t.Fatalf("unexpected app-info compound query: %q", got)
	}
	if got := requests["/v1/appInfos/info-1/ageRatingDeclaration"]; got != 1 {
		t.Fatalf("expected one age-rating fallback, got %d", got)
	}
	for _, path := range []string{
		"/v1/apps/app-1",
		"/v1/appInfos/info-1/appInfoLocalizations",
		"/v1/appInfos/info-1/relationships/primaryCategory",
	} {
		if got := requests[path]; got != 0 {
			t.Fatalf("expected no unrelated fallback for %s, got %d request(s)", path, got)
		}
	}
}

func TestFetchVersionReadinessData_TruncatedLocalizationIncludeFallsBackOnlyForLocalizations(t *testing.T) {
	var mu sync.Mutex
	requests := make(map[string]int)
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		requests[req.URL.Path]++
		mu.Unlock()

		switch req.URL.Path {
		case "/v1/appStoreVersions/ver-1":
			return buildsJSONResponse(http.StatusOK, `{
				"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0"},"relationships":{
					"appStoreVersionLocalizations":{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1"}],"meta":{"paging":{"total":2,"limit":50,"nextCursor":"next"}}},
					"build":{"data":null},
					"appStoreReviewDetail":{"data":null}
				}},
				"included":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]
			}`)
		case "/v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
			return buildsJSONResponse(http.StatusOK, `{"data":[
				{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}},
				{"type":"appStoreVersionLocalizations","id":"loc-2","attributes":{"locale":"fr-FR"}}
			]}`)
		default:
			return buildsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_REQUEST"}]}`)
		}
	}))

	result, err := fetchVersionReadinessData(withReadinessRequestGate(context.Background()), client, "ver-1", nil)
	if err != nil {
		t.Fatalf("fetchVersionReadinessData() error = %v", err)
	}
	if len(result.localizations) != 2 || result.localizations[0].ID != "loc-1" || result.localizations[1].ID != "loc-2" {
		t.Fatalf("unexpected fallback localizations: %+v", result.localizations)
	}
	if got := requests["/v1/appStoreVersions/ver-1/appStoreVersionLocalizations"]; got != 1 {
		t.Fatalf("expected one localization fallback, got %d", got)
	}
	for _, path := range []string{
		"/v1/appStoreVersions/ver-1/build",
		"/v1/appStoreVersions/ver-1/appStoreReviewDetail",
	} {
		if got := requests[path]; got != 0 {
			t.Fatalf("expected no unrelated fallback for %s, got %d request(s)", path, got)
		}
	}
}

func TestResolveMultipleAppInfoAgeRating_UsesVersionStateSelection(t *testing.T) {
	requests := 0
	client := newBuildsTestClient(t, buildsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Path != "/v1/apps/app-1/appInfos" {
			return buildsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"UNEXPECTED_REQUEST"}]}`)
		}
		return buildsJSONResponse(http.StatusOK, `{
			"data":[
				{"type":"appInfos","id":"info-draft","attributes":{"state":"PREPARE_FOR_SUBMISSION"},"relationships":{
					"app":{"data":{"type":"apps","id":"app-1"}},
					"ageRatingDeclaration":{"data":{"type":"ageRatingDeclarations","id":"age-draft"}},
					"appInfoLocalizations":{"data":[],"meta":{"paging":{"total":0,"limit":50}}},
					"primaryCategory":{"data":null}
				}},
				{"type":"appInfos","id":"info-live","attributes":{"state":"READY_FOR_DISTRIBUTION"},"relationships":{
					"ageRatingDeclaration":{"data":{"type":"ageRatingDeclarations","id":"age-live"}}
				}}
			],
			"included":[
				{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US"}},
				{"type":"ageRatingDeclarations","id":"age-draft","attributes":{"advertising":false}},
				{"type":"ageRatingDeclarations","id":"age-live","attributes":{"advertising":true}}
			]
		}`)
	}))

	ctx := withReadinessRequestGate(context.Background())
	result, err := fetchAppInfoReadinessData(ctx, client, "app-1")
	if err != nil {
		t.Fatalf("fetchAppInfoReadinessData() error = %v", err)
	}
	if result.appInfoID != "info-draft" {
		t.Fatalf("metadata app info = %q, want info-draft", result.appInfoID)
	}
	if result.ageRatingDeclaration != nil {
		t.Fatalf("expected multi-app-info age rating to remain deferred, got %+v", result.ageRatingDeclaration)
	}
	if err := resolveMultipleAppInfoAgeRating(ctx, client, &result, "app-1", "READY_FOR_SALE"); err != nil {
		t.Fatalf("resolveMultipleAppInfoAgeRating() error = %v", err)
	}
	if result.ageRatingDeclaration == nil || result.ageRatingDeclaration.Advertising == nil || !*result.ageRatingDeclaration.Advertising {
		t.Fatalf("expected live app-info age rating, got %+v", result.ageRatingDeclaration)
	}
	if requests != 1 {
		t.Fatalf("request count = %d, want only the compound app-info request", requests)
	}
}

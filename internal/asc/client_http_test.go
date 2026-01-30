package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newTestClient(t *testing.T, check func(*http.Request), response *http.Response) *Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if check != nil {
			check(req)
		}
		return response, nil
	})

	return &Client{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func assertAuthorized(t *testing.T, req *http.Request) {
	t.Helper()

	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		t.Fatalf("expected Authorization bearer token, got %q", auth)
	}
}

func TestGetApps_RateLimitedIncludesRetryAfter(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")

	response := jsonResponse(http.StatusTooManyRequests, `{"errors":[{"title":"Rate limit","detail":"Too many requests"}]}`)
	response.Header.Set("Retry-After", "120")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetApps(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsRetryable(err) {
		t.Fatalf("expected retryable error, got %v", err)
	}
	if got := GetRetryAfter(err); got != 2*time.Minute {
		t.Fatalf("expected retry-after 2m, got %s", got)
	}
}

func TestGetApps_WithSortAndLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"1","attributes":{"name":"Demo","bundleId":"com.example.demo","sku":"SKU1"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps" {
			t.Fatalf("expected path /v1/apps, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("sort") != "-name" {
			t.Fatalf("expected sort=-name, got %q", values.Get("sort"))
		}
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetApps(context.Background(), WithAppsLimit(10), WithAppsSort("-name")); err != nil {
		t.Fatalf("GetApps() error: %v", err)
	}
}

func TestGetApps_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/apps?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetApps(context.Background(), WithAppsLimit(5), WithAppsSort("name"), WithAppsNextURL(next)); err != nil {
		t.Fatalf("GetApps() error: %v", err)
	}
}

func TestGetApps_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"1","attributes":{"name":"Demo","bundleId":"com.example.demo","sku":"SKU1"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps" {
			t.Fatalf("expected path /v1/apps, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[bundleId]") != "com.example.demo,com.example.other" {
			t.Fatalf("expected filter[bundleId] to be set, got %q", values.Get("filter[bundleId]"))
		}
		if values.Get("filter[name]") != "Demo App" {
			t.Fatalf("expected filter[name]=Demo App, got %q", values.Get("filter[name]"))
		}
		if values.Get("filter[sku]") != "SKU1,SKU2" {
			t.Fatalf("expected filter[sku]=SKU1,SKU2, got %q", values.Get("filter[sku]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetApps(
		context.Background(),
		WithAppsBundleIDs([]string{"com.example.demo", "com.example.other"}),
		WithAppsNames([]string{"Demo App"}),
		WithAppsSKUs([]string{"SKU1", "SKU2"}),
	); err != nil {
		t.Fatalf("GetApps() error: %v", err)
	}
}

func TestGetApp(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"123","attributes":{"name":"Demo","bundleId":"com.example.demo","sku":"SKU1"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/123" {
			t.Fatalf("expected path /v1/apps/123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetApp(context.Background(), "123"); err != nil {
		t.Fatalf("GetApp() error: %v", err)
	}
}

func TestGetAppTags_WithFiltersAndSort(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appTags","id":"tag-1","attributes":{"name":"Games","visibleInAppStore":true}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/123/appTags" {
			t.Fatalf("expected path /v1/apps/123/appTags, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[visibleInAppStore]") != "true" {
			t.Fatalf("expected filter[visibleInAppStore]=true, got %q", values.Get("filter[visibleInAppStore]"))
		}
		if values.Get("sort") != "name" {
			t.Fatalf("expected sort=name, got %q", values.Get("sort"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		if values.Get("fields[appTags]") != "name,visibleInAppStore" {
			t.Fatalf("expected fields[appTags]=name,visibleInAppStore, got %q", values.Get("fields[appTags]"))
		}
		if values.Get("include") != "territories" {
			t.Fatalf("expected include=territories, got %q", values.Get("include"))
		}
		if values.Get("fields[territories]") != "currency" {
			t.Fatalf("expected fields[territories]=currency, got %q", values.Get("fields[territories]"))
		}
		if values.Get("limit[territories]") != "25" {
			t.Fatalf("expected limit[territories]=25, got %q", values.Get("limit[territories]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppTags(
		context.Background(),
		"123",
		WithAppTagsVisibleInAppStore([]string{"true"}),
		WithAppTagsSort("name"),
		WithAppTagsLimit(5),
		WithAppTagsFields([]string{"name", "visibleInAppStore"}),
		WithAppTagsInclude([]string{"territories"}),
		WithAppTagsTerritoryFields([]string{"currency"}),
		WithAppTagsTerritoryLimit(25),
	); err != nil {
		t.Fatalf("GetAppTags() error: %v", err)
	}
}

func TestGetAppTags_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/apps/123/appTags?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppTags(context.Background(), "123", WithAppTagsLimit(5), WithAppTagsNextURL(next)); err != nil {
		t.Fatalf("GetAppTags() error: %v", err)
	}
}

func TestGetAppTagTerritories_WithFieldsAndLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appTags/tag-1/territories" {
			t.Fatalf("expected path /v1/appTags/tag-1/territories, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("fields[territories]") != "currency" {
			t.Fatalf("expected fields[territories]=currency, got %q", values.Get("fields[territories]"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppTagTerritories(
		context.Background(),
		"tag-1",
		WithTerritoriesFields([]string{"currency"}),
		WithTerritoriesLimit(5),
	); err != nil {
		t.Fatalf("GetAppTagTerritories() error: %v", err)
	}
}

func TestGetAppTagTerritories_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/appTags/tag-1/territories?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppTagTerritories(context.Background(), "tag-1", WithTerritoriesNextURL(next)); err != nil {
		t.Fatalf("GetAppTagTerritories() error: %v", err)
	}
}

func TestGetAppTagTerritoriesRelationships_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appTags/tag-1/relationships/territories" {
			t.Fatalf("expected path /v1/appTags/tag-1/relationships/territories, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppTagTerritoriesRelationships(context.Background(), "tag-1", WithLinkagesLimit(5)); err != nil {
		t.Fatalf("GetAppTagTerritoriesRelationships() error: %v", err)
	}
}

func TestGetAppTagsRelationshipsForApp_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appTags","id":"tag-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/relationships/appTags" {
			t.Fatalf("expected path /v1/apps/app-1/relationships/appTags, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppTagsRelationshipsForApp(context.Background(), "app-1", WithLinkagesLimit(5)); err != nil {
		t.Fatalf("GetAppTagsRelationshipsForApp() error: %v", err)
	}
}

func TestGetAppPromotedPurchases_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"promotedPurchases","id":"promo-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/promotedPurchases" {
			t.Fatalf("expected path /v1/apps/app-1/promotedPurchases, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppPromotedPurchases(context.Background(), "app-1", WithPromotedPurchasesLimit(5)); err != nil {
		t.Fatalf("GetAppPromotedPurchases() error: %v", err)
	}
}

func TestGetAppPromotedPurchasesRelationships_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"promotedPurchases","id":"promo-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/relationships/promotedPurchases" {
			t.Fatalf("expected path /v1/apps/app-1/relationships/promotedPurchases, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppPromotedPurchasesRelationships(context.Background(), "app-1", WithLinkagesLimit(5)); err != nil {
		t.Fatalf("GetAppPromotedPurchasesRelationships() error: %v", err)
	}
}

func TestSetAppPromotedPurchases_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/relationships/promotedPurchases" {
			t.Fatalf("expected path /v1/apps/app-1/relationships/promotedPurchases, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 relationships, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypePromotedPurchases {
			t.Fatalf("expected type promotedPurchases, got %q", payload.Data[0].Type)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.SetAppPromotedPurchases(context.Background(), "app-1", []string{"promo-1", "promo-2"}); err != nil {
		t.Fatalf("SetAppPromotedPurchases() error: %v", err)
	}
}

func TestSetAppPromotedPurchases_EmptyListClears(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/relationships/promotedPurchases" {
			t.Fatalf("expected path /v1/apps/app-1/relationships/promotedPurchases, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data == nil {
			t.Fatalf("expected data to be an empty array, got nil")
		}
		if len(payload.Data) != 0 {
			t.Fatalf("expected 0 relationships, got %d", len(payload.Data))
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.SetAppPromotedPurchases(context.Background(), "app-1", nil); err != nil {
		t.Fatalf("SetAppPromotedPurchases() error: %v", err)
	}
}

func TestUpdateAppTag_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appTags","id":"tag-1","attributes":{"name":"Games","visibleInAppStore":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appTags/tag-1" {
			t.Fatalf("expected path /v1/appTags/tag-1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AppTagUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppTags {
			t.Fatalf("expected type appTags, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "tag-1" {
			t.Fatalf("expected id tag-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.VisibleInAppStore == nil {
			t.Fatalf("expected visibleInAppStore attribute to be set")
		}
		if !*payload.Data.Attributes.VisibleInAppStore {
			t.Fatalf("expected visibleInAppStore=true, got false")
		}
		assertAuthorized(t, req)
	}, response)

	visible := true
	attrs := AppTagUpdateAttributes{VisibleInAppStore: &visible}
	if _, err := client.UpdateAppTag(context.Background(), "tag-1", attrs); err != nil {
		t.Fatalf("UpdateAppTag() error: %v", err)
	}
}

func TestUpdateApp_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{"name":"Example","bundleId":"com.example.app","sku":"SKU-1","primaryLocale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1" {
			t.Fatalf("expected path /v1/apps/app-1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AppUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeApps {
			t.Fatalf("expected type apps, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "app-1" {
			t.Fatalf("expected id app-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.BundleID == nil || payload.Data.Attributes.PrimaryLocale == nil {
			t.Fatalf("expected bundleId and primaryLocale attributes to be set")
		}
		assertAuthorized(t, req)
	}, response)

	bundleID := "com.example.app"
	primaryLocale := "en-US"
	attrs := AppUpdateAttributes{
		BundleID:      &bundleID,
		PrimaryLocale: &primaryLocale,
	}
	if _, err := client.UpdateApp(context.Background(), "app-1", attrs); err != nil {
		t.Fatalf("UpdateApp() error: %v", err)
	}
}

func TestGetSubscriptionOfferCodeOneTimeUseCodes_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionOfferCodeOneTimeUseCodes","id":"1","attributes":{"numberOfCodes":5}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionOfferCodes/123/oneTimeUseCodes" {
			t.Fatalf("expected path /v1/subscriptionOfferCodes/123/oneTimeUseCodes, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionOfferCodeOneTimeUseCodes(context.Background(), "123", WithSubscriptionOfferCodeOneTimeUseCodesLimit(5)); err != nil {
		t.Fatalf("GetSubscriptionOfferCodeOneTimeUseCodes() error: %v", err)
	}
}

func TestGetSubscriptionOfferCodeOneTimeUseCodes_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/subscriptionOfferCodes/123/oneTimeUseCodes?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionOfferCodeOneTimeUseCodes(context.Background(), "123", WithSubscriptionOfferCodeOneTimeUseCodesNextURL(next)); err != nil {
		t.Fatalf("GetSubscriptionOfferCodeOneTimeUseCodes() error: %v", err)
	}
}

func TestGetSubscriptionOfferCodeOneTimeUseCode(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionOfferCodeOneTimeUseCodes","id":"code-1","attributes":{"numberOfCodes":5}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionOfferCodeOneTimeUseCodes/code-1" {
			t.Fatalf("expected path /v1/subscriptionOfferCodeOneTimeUseCodes/code-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionOfferCodeOneTimeUseCode(context.Background(), "code-1"); err != nil {
		t.Fatalf("GetSubscriptionOfferCodeOneTimeUseCode() error: %v", err)
	}
}

func TestCreateSubscriptionOfferCodeOneTimeUseCode_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionOfferCodeOneTimeUseCodes","id":"code-1","attributes":{"numberOfCodes":5}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionOfferCodeOneTimeUseCodes" {
			t.Fatalf("expected path /v1/subscriptionOfferCodeOneTimeUseCodes, got %s", req.URL.Path)
		}
		var payload struct {
			Data struct {
				Type       string `json:"type"`
				Attributes struct {
					NumberOfCodes  int    `json:"numberOfCodes"`
					ExpirationDate string `json:"expirationDate"`
				} `json:"attributes"`
				Relationships struct {
					OfferCode struct {
						Data struct {
							Type string `json:"type"`
							ID   string `json:"id"`
						} `json:"data"`
					} `json:"offerCode"`
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if payload.Data.Type != "subscriptionOfferCodeOneTimeUseCodes" {
			t.Fatalf("expected type=subscriptionOfferCodeOneTimeUseCodes, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.NumberOfCodes != 5 {
			t.Fatalf("expected numberOfCodes=5, got %d", payload.Data.Attributes.NumberOfCodes)
		}
		if payload.Data.Attributes.ExpirationDate != "2026-02-01" {
			t.Fatalf("expected expirationDate=2026-02-01, got %q", payload.Data.Attributes.ExpirationDate)
		}
		if payload.Data.Relationships.OfferCode.Data.Type != "subscriptionOfferCodes" {
			t.Fatalf("expected offerCode type=subscriptionOfferCodes, got %q", payload.Data.Relationships.OfferCode.Data.Type)
		}
		if payload.Data.Relationships.OfferCode.Data.ID != "OFFER_CODE_ID" {
			t.Fatalf("expected offerCode id=OFFER_CODE_ID, got %q", payload.Data.Relationships.OfferCode.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	req := SubscriptionOfferCodeOneTimeUseCodeCreateRequest{
		Data: SubscriptionOfferCodeOneTimeUseCodeCreateData{
			Type: ResourceTypeSubscriptionOfferCodeOneTimeUseCodes,
			Attributes: SubscriptionOfferCodeOneTimeUseCodeCreateAttributes{
				NumberOfCodes:  5,
				ExpirationDate: "2026-02-01",
			},
			Relationships: SubscriptionOfferCodeOneTimeUseCodeCreateRelationships{
				OfferCode: Relationship{
					Data: ResourceData{
						Type: ResourceTypeSubscriptionOfferCodes,
						ID:   "OFFER_CODE_ID",
					},
				},
			},
		},
	}
	if _, err := client.CreateSubscriptionOfferCodeOneTimeUseCode(context.Background(), req); err != nil {
		t.Fatalf("CreateSubscriptionOfferCodeOneTimeUseCode() error: %v", err)
	}
}

func TestGetSubscriptionOfferCodeOneTimeUseCodeValues(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("code\nABC123\nDEF456\n")),
		Header:     http.Header{},
	}
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionOfferCodeOneTimeUseCodes/code-1/values" {
			t.Fatalf("expected path /v1/subscriptionOfferCodeOneTimeUseCodes/code-1/values, got %s", req.URL.Path)
		}
		if req.Header.Get("Accept") != "text/csv" {
			t.Fatalf("expected Accept=text/csv, got %q", req.Header.Get("Accept"))
		}
		assertAuthorized(t, req)
	}, response)

	values, err := client.GetSubscriptionOfferCodeOneTimeUseCodeValues(context.Background(), "code-1")
	if err != nil {
		t.Fatalf("GetSubscriptionOfferCodeOneTimeUseCodeValues() error: %v", err)
	}
	if len(values) != 2 || values[0] != "ABC123" || values[1] != "DEF456" {
		t.Fatalf("expected codes to parse, got %v", values)
	}
}

func TestGetBuilds_WithSortAndLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"builds","id":"1","attributes":{"version":"1.0","uploadedDate":"2026-01-20T00:00:00Z"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		// When sorting or limiting, we use /v1/builds?filter[app]=APP_ID
		if req.URL.Path != "/v1/builds" {
			t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[app]") != "123" {
			t.Fatalf("expected filter[app]=123, got %q", values.Get("filter[app]"))
		}
		if values.Get("sort") != "-uploadedDate" {
			t.Fatalf("expected sort=-uploadedDate, got %q", values.Get("sort"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBuilds(context.Background(), "123", WithBuildsLimit(5), WithBuildsSort("-uploadedDate")); err != nil {
		t.Fatalf("GetBuilds() error: %v", err)
	}
}

func TestGetBuilds_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/builds?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBuilds(context.Background(), "123", WithBuildsLimit(5), WithBuildsSort("uploadedDate"), WithBuildsNextURL(next)); err != nil {
		t.Fatalf("GetBuilds() error: %v", err)
	}
}

func TestGetBuilds_WithPreReleaseVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"1.0","uploadedDate":"2026-01-20T00:00:00Z"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		// When filtering by preReleaseVersion, we use /v1/builds endpoint
		if req.URL.Path != "/v1/builds" {
			t.Fatalf("expected path /v1/builds, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[app]") != "123" {
			t.Fatalf("expected filter[app]=123, got %q", values.Get("filter[app]"))
		}
		if values.Get("filter[preReleaseVersion]") != "prv-456" {
			t.Fatalf("expected filter[preReleaseVersion]=prv-456, got %q", values.Get("filter[preReleaseVersion]"))
		}
		if values.Get("sort") != "-uploadedDate" {
			t.Fatalf("expected sort=-uploadedDate, got %q", values.Get("sort"))
		}
		if values.Get("limit") != "1" {
			t.Fatalf("expected limit=1, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	builds, err := client.GetBuilds(context.Background(), "123",
		WithBuildsLimit(1),
		WithBuildsSort("-uploadedDate"),
		WithBuildsPreReleaseVersion("prv-456"),
	)
	if err != nil {
		t.Fatalf("GetBuilds() error: %v", err)
	}
	if len(builds.Data) != 1 {
		t.Fatalf("expected 1 build, got %d", len(builds.Data))
	}
	if builds.Data[0].ID != "build-1" {
		t.Fatalf("expected build ID build-1, got %s", builds.Data[0].ID)
	}
}

func TestGetAppStoreVersions_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"1","attributes":{"versionString":"1.0.0","platform":"IOS"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/123/appStoreVersions" {
			t.Fatalf("expected path /v1/apps/123/appStoreVersions, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[platform]") != "IOS" {
			t.Fatalf("expected filter[platform]=IOS, got %q", values.Get("filter[platform]"))
		}
		if values.Get("filter[versionString]") != "1.0.0" {
			t.Fatalf("expected filter[versionString]=1.0.0, got %q", values.Get("filter[versionString]"))
		}
		if values.Get("filter[appStoreState]") != "READY_FOR_REVIEW" {
			t.Fatalf("expected filter[appStoreState]=READY_FOR_REVIEW, got %q", values.Get("filter[appStoreState]"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersions(
		context.Background(),
		"123",
		WithAppStoreVersionsLimit(5),
		WithAppStoreVersionsPlatforms([]string{"IOS"}),
		WithAppStoreVersionsVersionStrings([]string{"1.0.0"}),
		WithAppStoreVersionsStates([]string{"READY_FOR_REVIEW"}),
	); err != nil {
		t.Fatalf("GetAppStoreVersions() error: %v", err)
	}
}

func TestGetAppStoreVersions_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/apps/123/appStoreVersions?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersions(context.Background(), "123", WithAppStoreVersionsNextURL(next)); err != nil {
		t.Fatalf("GetAppStoreVersions() error: %v", err)
	}
}

func TestGetPreReleaseVersions_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"1","attributes":{"version":"1.0.0","platform":"IOS"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/preReleaseVersions" {
			t.Fatalf("expected path /v1/preReleaseVersions, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[app]") != "123" {
			t.Fatalf("expected filter[app]=123, got %q", values.Get("filter[app]"))
		}
		if values.Get("filter[platform]") != "IOS" {
			t.Fatalf("expected filter[platform]=IOS, got %q", values.Get("filter[platform]"))
		}
		if values.Get("filter[version]") != "1.0.0" {
			t.Fatalf("expected filter[version]=1.0.0, got %q", values.Get("filter[version]"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetPreReleaseVersions(
		context.Background(),
		"123",
		WithPreReleaseVersionsLimit(5),
		WithPreReleaseVersionsPlatform("ios"),
		WithPreReleaseVersionsVersion("1.0.0"),
	); err != nil {
		t.Fatalf("GetPreReleaseVersions() error: %v", err)
	}
}

func TestGetPreReleaseVersions_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/preReleaseVersions?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetPreReleaseVersions(context.Background(), "123", WithPreReleaseVersionsNextURL(next)); err != nil {
		t.Fatalf("GetPreReleaseVersions() error: %v", err)
	}
}

func TestGetPreReleaseVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"pr-1","attributes":{"version":"1.0.0","platform":"IOS"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/preReleaseVersions/pr-1" {
			t.Fatalf("expected path /v1/preReleaseVersions/pr-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetPreReleaseVersion(context.Background(), "pr-1"); err != nil {
		t.Fatalf("GetPreReleaseVersion() error: %v", err)
	}
}

func TestGetAppStoreVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"1","attributes":{"versionString":"1.0.0","platform":"IOS"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersions/1" {
			t.Fatalf("expected path /v1/appStoreVersions/1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersion(context.Background(), "1"); err != nil {
		t.Fatalf("GetAppStoreVersion() error: %v", err)
	}
}

func TestCreateAppStoreVersion(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"1.0.0","platform":"IOS"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersions" {
			t.Fatalf("expected path /v1/appStoreVersions, got %s", req.URL.Path)
		}
		var payload AppStoreVersionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppStoreVersions {
			t.Fatalf("unexpected resource type: %s", payload.Data.Type)
		}
		if payload.Data.Attributes.VersionString != "1.0.0" || payload.Data.Attributes.Platform != PlatformIOS {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship")
		}
		if payload.Data.Relationships.App.Data.Type != ResourceTypeApps || payload.Data.Relationships.App.Data.ID != "APP_123" {
			t.Fatalf("unexpected app relationship: %+v", payload.Data.Relationships.App.Data)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.CreateAppStoreVersion(context.Background(), "APP_123", AppStoreVersionCreateAttributes{
		Platform:      PlatformIOS,
		VersionString: "1.0.0",
	})
	if err != nil {
		t.Fatalf("CreateAppStoreVersion() error: %v", err)
	}
	if result.Data.ID != "VERSION_123" {
		t.Fatalf("expected version ID VERSION_123, got %s", result.Data.ID)
	}
}

func TestAttachBuildToVersion(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersions/1/relationships/build" {
			t.Fatalf("expected path /v1/appStoreVersions/1/relationships/build, got %s", req.URL.Path)
		}
		var body AppStoreVersionBuildRelationshipUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if body.Data.Type != ResourceTypeBuilds || body.Data.ID != "BUILD_123" {
			t.Fatalf("unexpected request payload: %+v", body.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.AttachBuildToVersion(context.Background(), "1", "BUILD_123"); err != nil {
		t.Fatalf("AttachBuildToVersion() error: %v", err)
	}
}

func TestAddBetaGroupsToBuild_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
			t.Fatalf("expected path /v1/builds/build-1/relationships/betaGroups, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 relationships, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeBetaGroups || payload.Data[0].ID != "group-1" {
			t.Fatalf("unexpected relationship[0]: %+v", payload.Data[0])
		}
		if payload.Data[1].Type != ResourceTypeBetaGroups || payload.Data[1].ID != "group-2" {
			t.Fatalf("unexpected relationship[1]: %+v", payload.Data[1])
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.AddBetaGroupsToBuild(context.Background(), "build-1", []string{"group-1", "group-2"}); err != nil {
		t.Fatalf("AddBetaGroupsToBuild() error: %v", err)
	}
}

func TestAddBetaGroupsToBuildWithNotify_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
			t.Fatalf("expected path /v1/builds/build-1/relationships/betaGroups, got %s", req.URL.Path)
		}
		if req.URL.RawQuery != "notify=true" {
			t.Fatalf("expected notify query, got %q", req.URL.RawQuery)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.Data) != 1 {
			t.Fatalf("expected 1 relationship, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeBetaGroups || payload.Data[0].ID != "group-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data[0])
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.AddBetaGroupsToBuildWithNotify(context.Background(), "build-1", []string{"group-1"}, true); err != nil {
		t.Fatalf("AddBetaGroupsToBuildWithNotify() error: %v", err)
	}
}

func TestRemoveBetaGroupsFromBuild_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/build-1/relationships/betaGroups" {
			t.Fatalf("expected path /v1/builds/build-1/relationships/betaGroups, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		if len(body) == 0 {
			t.Fatalf("expected request body for delete")
		}
		var payload RelationshipRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if len(payload.Data) != 1 {
			t.Fatalf("expected 1 relationship, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeBetaGroups || payload.Data[0].ID != "group-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data[0])
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.RemoveBetaGroupsFromBuild(context.Background(), "build-1", []string{"group-1"}); err != nil {
		t.Fatalf("RemoveBetaGroupsFromBuild() error: %v", err)
	}
}

func TestGetAppStoreVersionBuild(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"1.0.0","uploadedDate":"2026-01-20T00:00:00Z"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersions/1/build" {
			t.Fatalf("expected path /v1/appStoreVersions/1/build, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersionBuild(context.Background(), "1"); err != nil {
		t.Fatalf("GetAppStoreVersionBuild() error: %v", err)
	}
}

func TestGetAppStoreVersionSubmissionResource(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"SUBMIT_123","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"VERSION_123"}}}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionSubmissions/SUBMIT_123" {
			t.Fatalf("expected path /v1/appStoreVersionSubmissions/SUBMIT_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersionSubmissionResource(context.Background(), "SUBMIT_123"); err != nil {
		t.Fatalf("GetAppStoreVersionSubmissionResource() error: %v", err)
	}
}

func TestGetAppStoreVersionSubmissionForVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"SUBMIT_123","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"VERSION_123"}}}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersions/VERSION_123/appStoreVersionSubmission" {
			t.Fatalf("expected path /v1/appStoreVersions/VERSION_123/appStoreVersionSubmission, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersionSubmissionForVersion(context.Background(), "VERSION_123"); err != nil {
		t.Fatalf("GetAppStoreVersionSubmissionForVersion() error: %v", err)
	}
}

func TestGetBetaGroups_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"1","attributes":{"name":"Beta"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/123/betaGroups" {
			t.Fatalf("expected path /v1/apps/123/betaGroups, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaGroups(context.Background(), "123", WithBetaGroupsLimit(10)); err != nil {
		t.Fatalf("GetBetaGroups() error: %v", err)
	}
}

func TestGetBetaGroups_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/apps/123/betaGroups?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaGroups(context.Background(), "123", WithBetaGroupsLimit(5), WithBetaGroupsNextURL(next)); err != nil {
		t.Fatalf("GetBetaGroups() error: %v", err)
	}
}

func TestGetBetaGroupBuilds_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"1.0","uploadedDate":"2026-01-20T00:00:00Z"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/group-1/builds" {
			t.Fatalf("expected path /v1/betaGroups/group-1/builds, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaGroupBuilds(context.Background(), "group-1", WithBetaGroupBuildsLimit(50)); err != nil {
		t.Fatalf("GetBetaGroupBuilds() error: %v", err)
	}
}

func TestGetBetaGroupBuilds_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/betaGroups/group-1/builds?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaGroupBuilds(context.Background(), "group-1", WithBetaGroupBuildsNextURL(next)); err != nil {
		t.Fatalf("GetBetaGroupBuilds() error: %v", err)
	}
}

func TestGetBetaGroupTesters_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/group-1/betaTesters" {
			t.Fatalf("expected path /v1/betaGroups/group-1/betaTesters, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "20" {
			t.Fatalf("expected limit=20, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaGroupTesters(context.Background(), "group-1", WithBetaGroupTestersLimit(20)); err != nil {
		t.Fatalf("GetBetaGroupTesters() error: %v", err)
	}
}

func TestGetBetaGroupTesters_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/betaGroups/group-1/betaTesters?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaGroupTesters(context.Background(), "group-1", WithBetaGroupTestersNextURL(next)); err != nil {
		t.Fatalf("GetBetaGroupTesters() error: %v", err)
	}
}

func TestGetBetaTesters_WithAppFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaTesters","id":"1","attributes":{"email":"tester@example.com"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters" {
			t.Fatalf("expected path /v1/betaTesters, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[apps]") != "123" {
			t.Fatalf("expected filter[apps]=123, got %q", values.Get("filter[apps]"))
		}
		if values.Get("filter[email]") != "tester@example.com" {
			t.Fatalf("expected filter[email]=tester@example.com, got %q", values.Get("filter[email]"))
		}
		if values.Get("filter[betaGroups]") != "group-1,group-2" {
			t.Fatalf("expected filter[betaGroups]=group-1,group-2, got %q", values.Get("filter[betaGroups]"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaTesters(
		context.Background(),
		"123",
		WithBetaTestersEmail("tester@example.com"),
		WithBetaTestersGroupIDs([]string{"group-1", "group-2"}),
		WithBetaTestersLimit(5),
	); err != nil {
		t.Fatalf("GetBetaTesters() error: %v", err)
	}
}

func TestGetBetaTesters_WithBuildFilter(t *testing.T) {
	// API only allows one relationship filter, so builds takes precedence over apps
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaTesters","id":"1","attributes":{"email":"tester@example.com"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters" {
			t.Fatalf("expected path /v1/betaTesters, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		// When build filter is provided, apps filter should NOT be present
		if values.Get("filter[apps]") != "" {
			t.Fatalf("expected no filter[apps] when filter[builds] is set, got %q", values.Get("filter[apps]"))
		}
		if values.Get("filter[builds]") != "build-1" {
			t.Fatalf("expected filter[builds]=build-1, got %q", values.Get("filter[builds]"))
		}
		if values.Get("filter[email]") != "tester@example.com" {
			t.Fatalf("expected filter[email]=tester@example.com, got %q", values.Get("filter[email]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaTesters(
		context.Background(),
		"123", // appID provided but should be ignored when build filter is set
		WithBetaTestersEmail("tester@example.com"),
		WithBetaTestersBuildID("build-1"),
	); err != nil {
		t.Fatalf("GetBetaTesters() error: %v", err)
	}
}

func TestGetBetaTesters_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/betaTesters?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaTesters(context.Background(), "123", WithBetaTestersLimit(5), WithBetaTestersNextURL(next)); err != nil {
		t.Fatalf("GetBetaTesters() error: %v", err)
	}
}

func TestGetBuild_ByID(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"123","attributes":{"version":"1.0","uploadedDate":"2026-01-20T00:00:00Z","expired":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/123" {
			t.Fatalf("expected path /v1/builds/123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBuild(context.Background(), "123"); err != nil {
		t.Fatalf("GetBuild() error: %v", err)
	}
}

func TestGetApp_ByID(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1","attributes":{"name":"Demo","bundleId":"com.example.demo","sku":"SKU1"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1" {
			t.Fatalf("expected path /v1/apps/app-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetApp(context.Background(), "app-1"); err != nil {
		t.Fatalf("GetApp() error: %v", err)
	}
}

func TestGetBuildAppStoreVersion_ByBuildID(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/123/appStoreVersion" {
			t.Fatalf("expected path /v1/builds/123/appStoreVersion, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBuildAppStoreVersion(context.Background(), "123"); err != nil {
		t.Fatalf("GetBuildAppStoreVersion() error: %v", err)
	}
}

func TestExpireBuild_SendsPatch(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"builds","id":"123","attributes":{"version":"1.0","uploadedDate":"2026-01-20T00:00:00Z","expired":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/123" {
			t.Fatalf("expected path /v1/builds/123, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload struct {
			Data struct {
				Type       string `json:"type"`
				ID         string `json:"id"`
				Attributes struct {
					Expired bool `json:"expired"`
				} `json:"attributes"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != "builds" {
			t.Fatalf("expected type builds, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "123" {
			t.Fatalf("expected id 123, got %q", payload.Data.ID)
		}
		if !payload.Data.Attributes.Expired {
			t.Fatalf("expected expired true")
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.ExpireBuild(context.Background(), "123"); err != nil {
		t.Fatalf("ExpireBuild() error: %v", err)
	}
}

func TestCreateBetaGroup_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"betaGroups","id":"bg1","attributes":{"name":"Beta"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups" {
			t.Fatalf("expected path /v1/betaGroups, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BetaGroupCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBetaGroups {
			t.Fatalf("expected type betaGroups, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Name != "Beta" {
			t.Fatalf("expected name Beta, got %q", payload.Data.Attributes.Name)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship to be set")
		}
		if payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("expected app id app-1, got %q", payload.Data.Relationships.App.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateBetaGroup(context.Background(), "app-1", "Beta"); err != nil {
		t.Fatalf("CreateBetaGroup() error: %v", err)
	}
}

func TestGetBetaGroup_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaGroups","id":"bg1","attributes":{"name":"Beta Testers","isInternalGroup":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/bg1" {
			t.Fatalf("expected path /v1/betaGroups/bg1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaGroup(context.Background(), "bg1"); err != nil {
		t.Fatalf("GetBetaGroup() error: %v", err)
	}
}

func TestGetBetaTester_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaTesters","id":"bt1","attributes":{"email":"tester@example.com","firstName":"Test","lastName":"User","state":"INVITED","inviteType":"EMAIL"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters/bt1" {
			t.Fatalf("expected path /v1/betaTesters/bt1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	tester, err := client.GetBetaTester(context.Background(), "bt1")
	if err != nil {
		t.Fatalf("GetBetaTester() error: %v", err)
	}
	if tester.Data.ID != "bt1" {
		t.Fatalf("expected tester id bt1, got %q", tester.Data.ID)
	}
	if tester.Data.Attributes.Email != "tester@example.com" {
		t.Fatalf("expected tester email tester@example.com, got %q", tester.Data.Attributes.Email)
	}
	if tester.Data.Attributes.State != BetaTesterStateInvited {
		t.Fatalf("expected state %q, got %q", BetaTesterStateInvited, tester.Data.Attributes.State)
	}
	if tester.Data.Attributes.InviteType != BetaInviteTypeEmail {
		t.Fatalf("expected invite type %q, got %q", BetaInviteTypeEmail, tester.Data.Attributes.InviteType)
	}
}

func TestUpdateBetaGroup_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaGroups","id":"bg1","attributes":{"name":"Updated Beta Testers","publicLinkEnabled":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/bg1" {
			t.Fatalf("expected path /v1/betaGroups/bg1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BetaGroupUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBetaGroups {
			t.Fatalf("expected type betaGroups, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "bg1" {
			t.Fatalf("expected id bg1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil {
			t.Fatalf("expected attributes to be set")
		}
		if payload.Data.Attributes.Name != "Updated Beta Testers" {
			t.Fatalf("expected name Updated Beta Testers, got %q", payload.Data.Attributes.Name)
		}
		assertAuthorized(t, req)
	}, response)

	req := BetaGroupUpdateRequest{
		Data: BetaGroupUpdateData{
			Type:       ResourceTypeBetaGroups,
			ID:         "bg1",
			Attributes: &BetaGroupUpdateAttributes{Name: "Updated Beta Testers"},
		},
	}
	if _, err := client.UpdateBetaGroup(context.Background(), "bg1", req); err != nil {
		t.Fatalf("UpdateBetaGroup() error: %v", err)
	}
}

func TestDeleteBetaGroup_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/bg1" {
			t.Fatalf("expected path /v1/betaGroups/bg1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteBetaGroup(context.Background(), "bg1"); err != nil {
		t.Fatalf("DeleteBetaGroup() error: %v", err)
	}
}

func TestAddBetaTestersToGroup_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/bg1/relationships/betaTesters" {
			t.Fatalf("expected path /v1/betaGroups/bg1/relationships/betaTesters, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 tester relationships, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeBetaTesters || payload.Data[0].ID != "tester-1" {
			t.Fatalf("unexpected tester data: %+v", payload.Data[0])
		}
		if payload.Data[1].Type != ResourceTypeBetaTesters || payload.Data[1].ID != "tester-2" {
			t.Fatalf("unexpected tester data: %+v", payload.Data[1])
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.AddBetaTestersToGroup(context.Background(), "bg1", []string{"tester-1", "tester-2"}); err != nil {
		t.Fatalf("AddBetaTestersToGroup() error: %v", err)
	}
}

func TestRemoveBetaTestersFromGroup_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/bg1/relationships/betaTesters" {
			t.Fatalf("expected path /v1/betaGroups/bg1/relationships/betaTesters, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.Data) != 1 {
			t.Fatalf("expected 1 tester relationship, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeBetaTesters || payload.Data[0].ID != "tester-1" {
			t.Fatalf("unexpected tester data: %+v", payload.Data[0])
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.RemoveBetaTestersFromGroup(context.Background(), "bg1", []string{"tester-1"}); err != nil {
		t.Fatalf("RemoveBetaTestersFromGroup() error: %v", err)
	}
}

func TestCreateBetaTester_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"betaTesters","id":"bt1","attributes":{"email":"tester@example.com"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters" {
			t.Fatalf("expected path /v1/betaTesters, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BetaTesterCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBetaTesters {
			t.Fatalf("expected type betaTesters, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Email != "tester@example.com" {
			t.Fatalf("expected email tester@example.com, got %q", payload.Data.Attributes.Email)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.BetaGroups == nil {
			t.Fatalf("expected betaGroups relationship to be set")
		}
		if len(payload.Data.Relationships.BetaGroups.Data) != 2 {
			t.Fatalf("expected 2 beta group relationships, got %d", len(payload.Data.Relationships.BetaGroups.Data))
		}
		if payload.Data.Relationships.BetaGroups.Data[0].ID != "group-1" {
			t.Fatalf("expected group-1, got %q", payload.Data.Relationships.BetaGroups.Data[0].ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateBetaTester(context.Background(), "tester@example.com", "Test", "User", []string{"group-1", "group-2"}); err != nil {
		t.Fatalf("CreateBetaTester() error: %v", err)
	}
}

func TestBetaGroupTesterRelationshipMethods_ErrorResponse(t *testing.T) {
	ctx := context.Background()
	errorBody := `{"errors":[{"code":"BAD_REQUEST","title":"Bad Request","detail":"nope"}]}`

	tests := []struct {
		name string
		call func(*Client) error
	}{
		{
			name: "AddBetaTestersToGroup",
			call: func(c *Client) error {
				return c.AddBetaTestersToGroup(ctx, "bg1", []string{"tester-1"})
			},
		},
		{
			name: "RemoveBetaTestersFromGroup",
			call: func(c *Client) error {
				return c.RemoveBetaTestersFromGroup(ctx, "bg1", []string{"tester-1"})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, nil, jsonResponse(http.StatusBadRequest, errorBody))
			if err := test.call(client); err == nil {
				t.Fatalf("expected error")
			} else if !errors.Is(err, ErrBadRequest) {
				t.Fatalf("expected bad request error, got %v", err)
			}
		})
	}
}

func TestDeleteBetaTester_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters/bt-1" {
			t.Fatalf("expected path /v1/betaTesters/bt-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteBetaTester(context.Background(), "bt-1"); err != nil {
		t.Fatalf("DeleteBetaTester() error: %v", err)
	}
}

func TestAddBetaTesterToGroups_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters/bt-1/relationships/betaGroups" {
			t.Fatalf("expected path /v1/betaTesters/bt-1/relationships/betaGroups, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload RelationshipList
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 beta group relationships, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeBetaGroups {
			t.Fatalf("expected type betaGroups, got %q", payload.Data[0].Type)
		}
		if payload.Data[0].ID != "group-1" {
			t.Fatalf("expected group-1, got %q", payload.Data[0].ID)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.AddBetaTesterToGroups(context.Background(), "bt-1", []string{"group-1", "group-2"}); err != nil {
		t.Fatalf("AddBetaTesterToGroups() error: %v", err)
	}
}

func TestRemoveBetaTesterFromGroups_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters/bt-1/relationships/betaGroups" {
			t.Fatalf("expected path /v1/betaTesters/bt-1/relationships/betaGroups, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload RelationshipList
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 beta group relationships, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeBetaGroups {
			t.Fatalf("expected type betaGroups, got %q", payload.Data[0].Type)
		}
		if payload.Data[1].ID != "group-2" {
			t.Fatalf("expected group-2, got %q", payload.Data[1].ID)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.RemoveBetaTesterFromGroups(context.Background(), "bt-1", []string{"group-1", "group-2"}); err != nil {
		t.Fatalf("RemoveBetaTesterFromGroups() error: %v", err)
	}
}

func TestCreateBetaTesterInvitation_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"betaTesterInvitations","id":"invite-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesterInvitations" {
			t.Fatalf("expected path /v1/betaTesterInvitations, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BetaTesterInvitationCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBetaTesterInvitations {
			t.Fatalf("expected type betaTesterInvitations, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship to be set")
		}
		if payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("expected app id app-1, got %q", payload.Data.Relationships.App.Data.ID)
		}
		if payload.Data.Relationships.BetaTester == nil || payload.Data.Relationships.BetaTester.Data.ID != "tester-1" {
			t.Fatalf("expected beta tester id tester-1")
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateBetaTesterInvitation(context.Background(), "app-1", "tester-1"); err != nil {
		t.Fatalf("CreateBetaTesterInvitation() error: %v", err)
	}
}

func TestGetAppStoreVersionLocalizations_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
			t.Fatalf("expected path /v1/appStoreVersions/version-1/appStoreVersionLocalizations, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[locale]") != "en-US" {
			t.Fatalf("expected filter[locale]=en-US, got %q", values.Get("filter[locale]"))
		}
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersionLocalizations(
		context.Background(),
		"version-1",
		WithAppStoreVersionLocalizationLocales([]string{"en-US"}),
		WithAppStoreVersionLocalizationsLimit(10),
	); err != nil {
		t.Fatalf("GetAppStoreVersionLocalizations() error: %v", err)
	}
}

func TestGetAppStoreVersionLocalization_ByID(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1" {
			t.Fatalf("expected path /v1/appStoreVersionLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersionLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetAppStoreVersionLocalization() error: %v", err)
	}
}

func TestCreateAppStoreVersionLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionLocalizations" {
			t.Fatalf("expected path /v1/appStoreVersionLocalizations, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AppStoreVersionLocalizationCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppStoreVersionLocalizations {
			t.Fatalf("expected type appStoreVersionLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Locale != "en-US" {
			t.Fatalf("expected locale en-US, got %q", payload.Data.Attributes.Locale)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.AppStoreVersion == nil {
			t.Fatalf("expected appStoreVersion relationship")
		}
		if payload.Data.Relationships.AppStoreVersion.Data.ID != "version-1" {
			t.Fatalf("expected version id version-1, got %q", payload.Data.Relationships.AppStoreVersion.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppStoreVersionLocalizationAttributes{
		Locale:      "en-US",
		Description: "Hello",
	}
	if _, err := client.CreateAppStoreVersionLocalization(context.Background(), "version-1", attrs); err != nil {
		t.Fatalf("CreateAppStoreVersionLocalization() error: %v", err)
	}
}

func TestUpdateAppStoreVersionLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"description":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1" {
			t.Fatalf("expected path /v1/appStoreVersionLocalizations/loc-1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AppStoreVersionLocalizationUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppStoreVersionLocalizations {
			t.Fatalf("expected type appStoreVersionLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "loc-1" {
			t.Fatalf("expected id loc-1, got %q", payload.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppStoreVersionLocalizationAttributes{
		Description: "Updated",
	}
	if _, err := client.UpdateAppStoreVersionLocalization(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateAppStoreVersionLocalization() error: %v", err)
	}
}

func TestDeleteAppStoreVersionLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1" {
			t.Fatalf("expected path /v1/appStoreVersionLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppStoreVersionLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteAppStoreVersionLocalization() error: %v", err)
	}
}

func TestGetBetaBuildLocalizations_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/build-1/betaBuildLocalizations" {
			t.Fatalf("expected path /v1/builds/build-1/betaBuildLocalizations, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[locale]") != "en-US" {
			t.Fatalf("expected filter[locale]=en-US, got %q", values.Get("filter[locale]"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaBuildLocalizations(
		context.Background(),
		"build-1",
		WithBetaBuildLocalizationLocales([]string{"en-US"}),
		WithBetaBuildLocalizationsLimit(5),
	); err != nil {
		t.Fatalf("GetBetaBuildLocalizations() error: %v", err)
	}
}

func TestGetBetaBuildLocalization_ByID(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaBuildLocalizations/loc-1" {
			t.Fatalf("expected path /v1/betaBuildLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaBuildLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetBetaBuildLocalization() error: %v", err)
	}
}

func TestCreateBetaBuildLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaBuildLocalizations" {
			t.Fatalf("expected path /v1/betaBuildLocalizations, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BetaBuildLocalizationCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBetaBuildLocalizations {
			t.Fatalf("expected type betaBuildLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Locale != "en-US" {
			t.Fatalf("expected locale en-US, got %q", payload.Data.Attributes.Locale)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Build == nil {
			t.Fatalf("expected build relationship")
		}
		if payload.Data.Relationships.Build.Data.ID != "build-1" {
			t.Fatalf("expected build id build-1, got %q", payload.Data.Relationships.Build.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := BetaBuildLocalizationAttributes{
		Locale:   "en-US",
		WhatsNew: "Test the new feature",
	}
	if _, err := client.CreateBetaBuildLocalization(context.Background(), "build-1", attrs); err != nil {
		t.Fatalf("CreateBetaBuildLocalization() error: %v", err)
	}
}

func TestUpdateBetaBuildLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaBuildLocalizations","id":"loc-1","attributes":{"whatsNew":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaBuildLocalizations/loc-1" {
			t.Fatalf("expected path /v1/betaBuildLocalizations/loc-1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BetaBuildLocalizationUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBetaBuildLocalizations {
			t.Fatalf("expected type betaBuildLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "loc-1" {
			t.Fatalf("expected id loc-1, got %q", payload.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := BetaBuildLocalizationAttributes{
		WhatsNew: "Updated",
	}
	if _, err := client.UpdateBetaBuildLocalization(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateBetaBuildLocalization() error: %v", err)
	}
}

func TestDeleteBetaBuildLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaBuildLocalizations/loc-1" {
			t.Fatalf("expected path /v1/betaBuildLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteBetaBuildLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteBetaBuildLocalization() error: %v", err)
	}
}

func TestGetBetaAppLocalizations_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaAppLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppLocalizations" {
			t.Fatalf("expected path /v1/betaAppLocalizations, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[app]") != "app-1" {
			t.Fatalf("expected filter[app]=app-1, got %q", values.Get("filter[app]"))
		}
		if values.Get("filter[locale]") != "en-US" {
			t.Fatalf("expected filter[locale]=en-US, got %q", values.Get("filter[locale]"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaAppLocalizations(
		context.Background(),
		WithBetaAppLocalizationAppIDs([]string{"app-1"}),
		WithBetaAppLocalizationLocales([]string{"en-US"}),
		WithBetaAppLocalizationsLimit(5),
	); err != nil {
		t.Fatalf("GetBetaAppLocalizations() error: %v", err)
	}
}

func TestGetBetaAppLocalization_ByID(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaAppLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppLocalizations/loc-1" {
			t.Fatalf("expected path /v1/betaAppLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaAppLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetBetaAppLocalization() error: %v", err)
	}
}

func TestCreateBetaAppLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"betaAppLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppLocalizations" {
			t.Fatalf("expected path /v1/betaAppLocalizations, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BetaAppLocalizationCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBetaAppLocalizations {
			t.Fatalf("expected type betaAppLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Locale != "en-US" {
			t.Fatalf("expected locale en-US, got %q", payload.Data.Attributes.Locale)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship")
		}
		if payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("expected app id app-1, got %q", payload.Data.Relationships.App.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := BetaAppLocalizationAttributes{
		Locale:        "en-US",
		Description:   "Test details",
		FeedbackEmail: "tester@example.com",
	}
	if _, err := client.CreateBetaAppLocalization(context.Background(), "app-1", attrs); err != nil {
		t.Fatalf("CreateBetaAppLocalization() error: %v", err)
	}
}

func TestUpdateBetaAppLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaAppLocalizations","id":"loc-1","attributes":{"description":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppLocalizations/loc-1" {
			t.Fatalf("expected path /v1/betaAppLocalizations/loc-1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BetaAppLocalizationUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBetaAppLocalizations {
			t.Fatalf("expected type betaAppLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "loc-1" {
			t.Fatalf("expected id loc-1, got %q", payload.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := BetaAppLocalizationUpdateAttributes{
		Description: func() *string { v := "Updated"; return &v }(),
	}
	if _, err := client.UpdateBetaAppLocalization(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateBetaAppLocalization() error: %v", err)
	}
}

func TestDeleteBetaAppLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppLocalizations/loc-1" {
			t.Fatalf("expected path /v1/betaAppLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteBetaAppLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteBetaAppLocalization() error: %v", err)
	}
}

func TestGetAppInfoLocalizations_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appInfos/app-info-1/appInfoLocalizations" {
			t.Fatalf("expected path /v1/appInfos/app-info-1/appInfoLocalizations, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[locale]") != "en-US" {
			t.Fatalf("expected filter[locale]=en-US, got %q", values.Get("filter[locale]"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppInfoLocalizations(
		context.Background(),
		"app-info-1",
		WithAppInfoLocalizationLocales([]string{"en-US"}),
		WithAppInfoLocalizationsLimit(5),
	); err != nil {
		t.Fatalf("GetAppInfoLocalizations() error: %v", err)
	}
}

func TestCreateAppInfoLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appInfoLocalizations" {
			t.Fatalf("expected path /v1/appInfoLocalizations, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AppInfoLocalizationCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppInfoLocalizations {
			t.Fatalf("expected type appInfoLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Locale != "en-US" {
			t.Fatalf("expected locale en-US, got %q", payload.Data.Attributes.Locale)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.AppInfo == nil {
			t.Fatalf("expected appInfo relationship")
		}
		if payload.Data.Relationships.AppInfo.Data.ID != "app-info-1" {
			t.Fatalf("expected appInfo id app-info-1, got %q", payload.Data.Relationships.AppInfo.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppInfoLocalizationAttributes{
		Locale: "en-US",
		Name:   "Demo App",
	}
	if _, err := client.CreateAppInfoLocalization(context.Background(), "app-info-1", attrs); err != nil {
		t.Fatalf("CreateAppInfoLocalization() error: %v", err)
	}
}

func TestUpdateAppInfoLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"name":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appInfoLocalizations/loc-1" {
			t.Fatalf("expected path /v1/appInfoLocalizations/loc-1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AppInfoLocalizationUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppInfoLocalizations {
			t.Fatalf("expected type appInfoLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "loc-1" {
			t.Fatalf("expected id loc-1, got %q", payload.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppInfoLocalizationAttributes{
		Name: "Updated",
	}
	if _, err := client.UpdateAppInfoLocalization(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateAppInfoLocalization() error: %v", err)
	}
}

func TestGetAppInfos(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appInfos","id":"info-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/appInfos" {
			t.Fatalf("expected path /v1/apps/app-1/appInfos, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppInfos(context.Background(), "app-1"); err != nil {
		t.Fatalf("GetAppInfos() error: %v", err)
	}
}

func TestGetAgeRatingDeclarationForAppInfo(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"age-1","attributes":{"gambling":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appInfos/info-1/ageRatingDeclaration" {
			t.Fatalf("expected path /v1/appInfos/info-1/ageRatingDeclaration, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAgeRatingDeclarationForAppInfo(context.Background(), "info-1"); err != nil {
		t.Fatalf("GetAgeRatingDeclarationForAppInfo() error: %v", err)
	}
}

func TestGetAgeRatingDeclarationForAppStoreVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"age-2","attributes":{"gambling":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersions/ver-1/ageRatingDeclaration" {
			t.Fatalf("expected path /v1/appStoreVersions/ver-1/ageRatingDeclaration, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAgeRatingDeclarationForAppStoreVersion(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetAgeRatingDeclarationForAppStoreVersion() error: %v", err)
	}
}

func TestUpdateAgeRatingDeclaration(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ageRatingDeclarations","id":"age-3","attributes":{"gambling":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ageRatingDeclarations/age-3" {
			t.Fatalf("expected path /v1/ageRatingDeclarations/age-3, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload AgeRatingDeclarationUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeAgeRatingDeclarations {
			t.Fatalf("expected type ageRatingDeclarations, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "age-3" {
			t.Fatalf("expected id age-3, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes.Gambling == nil || !*payload.Data.Attributes.Gambling {
			t.Fatalf("expected gambling=true in request")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AgeRatingDeclarationAttributes{
		Gambling: func() *bool { value := true; return &value }(),
	}
	if _, err := client.UpdateAgeRatingDeclaration(context.Background(), "age-3", attrs); err != nil {
		t.Fatalf("UpdateAgeRatingDeclaration() error: %v", err)
	}
}

func TestGetFeedback_BuildsQuery(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaFeedbackScreenshotSubmissions","id":"1","attributes":{"createdDate":"2026-01-20T00:00:00Z","comment":"Nice","email":"tester@example.com"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/apps/123/betaFeedbackScreenshotSubmissions" {
			t.Fatalf("expected feedback path, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[deviceModel]") != "iPhone15,3" {
			t.Fatalf("expected deviceModel filter, got %q", values.Get("filter[deviceModel]"))
		}
		if values.Get("filter[osVersion]") != "17.2" {
			t.Fatalf("expected osVersion filter, got %q", values.Get("filter[osVersion]"))
		}
		if values.Get("filter[appPlatform]") != "IOS,MAC_OS" {
			t.Fatalf("expected appPlatform filter, got %q", values.Get("filter[appPlatform]"))
		}
		if values.Get("filter[devicePlatform]") != "TV_OS" {
			t.Fatalf("expected devicePlatform filter, got %q", values.Get("filter[devicePlatform]"))
		}
		if values.Get("filter[build]") != "build-1" {
			t.Fatalf("expected build filter, got %q", values.Get("filter[build]"))
		}
		if values.Get("filter[build.preReleaseVersion]") != "pre-1" {
			t.Fatalf("expected preRelease filter, got %q", values.Get("filter[build.preReleaseVersion]"))
		}
		if values.Get("filter[tester]") != "tester-1" {
			t.Fatalf("expected tester filter, got %q", values.Get("filter[tester]"))
		}
		if values.Get("sort") != "-createdDate" {
			t.Fatalf("expected sort=-createdDate, got %q", values.Get("sort"))
		}
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	opts := []FeedbackOption{
		WithFeedbackDeviceModels([]string{"iPhone15,3"}),
		WithFeedbackOSVersions([]string{"17.2"}),
		WithFeedbackAppPlatforms([]string{"ios", "mac_os"}),
		WithFeedbackDevicePlatforms([]string{"tv_os"}),
		WithFeedbackBuildIDs([]string{"build-1"}),
		WithFeedbackBuildPreReleaseVersionIDs([]string{"pre-1"}),
		WithFeedbackTesterIDs([]string{"tester-1"}),
		WithFeedbackLimit(10),
		WithFeedbackSort("-createdDate"),
	}

	if _, err := client.GetFeedback(context.Background(), "123", opts...); err != nil {
		t.Fatalf("GetFeedback() error: %v", err)
	}
}

func TestGetFeedback_IncludesScreenshots(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaFeedbackScreenshotSubmissions","id":"1","attributes":{"createdDate":"2026-01-20T00:00:00Z","comment":"Nice","email":"tester@example.com","screenshots":[{"url":"https://example.com/shot.png","width":320,"height":640,"expirationDate":"2026-01-21T00:00:00Z"}]}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		values := req.URL.Query()
		expected := "createdDate,comment,email,deviceModel,osVersion,appPlatform,devicePlatform,screenshots"
		if values.Get("fields[betaFeedbackScreenshotSubmissions]") != expected {
			t.Fatalf("expected screenshot fields, got %q", values.Get("fields[betaFeedbackScreenshotSubmissions]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetFeedback(context.Background(), "123", WithFeedbackIncludeScreenshots()); err != nil {
		t.Fatalf("GetFeedback() error: %v", err)
	}
}

func TestGetCrashes_BuildsQuery(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaFeedbackCrashSubmissions","id":"1","attributes":{"createdDate":"2026-01-20T00:00:00Z","comment":"Crash","email":"tester@example.com","crashLog":"stack"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/apps/123/betaFeedbackCrashSubmissions" {
			t.Fatalf("expected crashes path, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[deviceModel]") != "iPhone16,1" {
			t.Fatalf("expected deviceModel filter, got %q", values.Get("filter[deviceModel]"))
		}
		if values.Get("filter[osVersion]") != "18.0" {
			t.Fatalf("expected osVersion filter, got %q", values.Get("filter[osVersion]"))
		}
		if values.Get("filter[appPlatform]") != "IOS" {
			t.Fatalf("expected appPlatform filter, got %q", values.Get("filter[appPlatform]"))
		}
		if values.Get("filter[devicePlatform]") != "MAC_OS" {
			t.Fatalf("expected devicePlatform filter, got %q", values.Get("filter[devicePlatform]"))
		}
		if values.Get("filter[build]") != "build-2" {
			t.Fatalf("expected build filter, got %q", values.Get("filter[build]"))
		}
		if values.Get("filter[build.preReleaseVersion]") != "pre-2" {
			t.Fatalf("expected preRelease filter, got %q", values.Get("filter[build.preReleaseVersion]"))
		}
		if values.Get("filter[tester]") != "tester-2" {
			t.Fatalf("expected tester filter, got %q", values.Get("filter[tester]"))
		}
		if values.Get("sort") != "createdDate" {
			t.Fatalf("expected sort=createdDate, got %q", values.Get("sort"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	opts := []CrashOption{
		WithCrashDeviceModels([]string{"iPhone16,1"}),
		WithCrashOSVersions([]string{"18.0"}),
		WithCrashAppPlatforms([]string{"ios"}),
		WithCrashDevicePlatforms([]string{"mac_os"}),
		WithCrashBuildIDs([]string{"build-2"}),
		WithCrashBuildPreReleaseVersionIDs([]string{"pre-2"}),
		WithCrashTesterIDs([]string{"tester-2"}),
		WithCrashLimit(5),
		WithCrashSort("createdDate"),
	}

	if _, err := client.GetCrashes(context.Background(), "123", opts...); err != nil {
		t.Fatalf("GetCrashes() error: %v", err)
	}
}

func TestGetReviews_BuildsQuery(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"customerReviews","id":"1","attributes":{"rating":5,"title":"Great","body":"Nice","reviewerNickname":"Tester","createdDate":"2026-01-20T00:00:00Z","territory":"US"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/apps/123/customerReviews" {
			t.Fatalf("expected reviews path, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[rating]") != "5" {
			t.Fatalf("expected rating filter, got %q", values.Get("filter[rating]"))
		}
		if values.Get("filter[territory]") != "US" {
			t.Fatalf("expected territory filter, got %q", values.Get("filter[territory]"))
		}
		if values.Get("sort") != "-createdDate" {
			t.Fatalf("expected sort=-createdDate, got %q", values.Get("sort"))
		}
		if values.Get("limit") != "25" {
			t.Fatalf("expected limit=25, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	opts := []ReviewOption{
		WithRating(5),
		WithTerritory("us"),
		WithReviewSort("-createdDate"),
		WithLimit(25),
	}

	if _, err := client.GetReviews(context.Background(), "123", opts...); err != nil {
		t.Fatalf("GetReviews() error: %v", err)
	}
}

func TestGetEndpoints_ReturnsAPIError(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client) error
	}{
		{
			name: "apps",
			call: func(c *Client) error {
				_, err := c.GetApps(context.Background())
				return err
			},
		},
		{
			name: "builds",
			call: func(c *Client) error {
				_, err := c.GetBuilds(context.Background(), "123")
				return err
			},
		},
		{
			name: "feedback",
			call: func(c *Client) error {
				_, err := c.GetFeedback(context.Background(), "123")
				return err
			},
		},
		{
			name: "crashes",
			call: func(c *Client) error {
				_, err := c.GetCrashes(context.Background(), "123")
				return err
			},
		},
		{
			name: "reviews",
			call: func(c *Client) error {
				_, err := c.GetReviews(context.Background(), "123")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := jsonResponse(http.StatusForbidden, `{"errors":[{"code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)
			client := newTestClient(t, nil, response)
			err := tt.call(client)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, ErrForbidden) {
				t.Fatalf("expected forbidden error, got %v", err)
			}
		})
	}
}

func TestGetEndpoints_ReturnsParseError(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client) error
	}{
		{
			name: "apps",
			call: func(c *Client) error {
				_, err := c.GetApps(context.Background())
				return err
			},
		},
		{
			name: "builds",
			call: func(c *Client) error {
				_, err := c.GetBuilds(context.Background(), "123")
				return err
			},
		},
		{
			name: "feedback",
			call: func(c *Client) error {
				_, err := c.GetFeedback(context.Background(), "123")
				return err
			},
		},
		{
			name: "crashes",
			call: func(c *Client) error {
				_, err := c.GetCrashes(context.Background(), "123")
				return err
			},
		},
		{
			name: "reviews",
			call: func(c *Client) error {
				_, err := c.GetReviews(context.Background(), "123")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := jsonResponse(http.StatusOK, `{"data":[}`)
			client := newTestClient(t, nil, response)
			err := tt.call(client)
			if err == nil {
				t.Fatal("expected error")
			}
			var syntaxErr *json.SyntaxError
			if !errors.As(err, &syntaxErr) {
				t.Fatalf("expected JSON parse error, got %v", err)
			}
		})
	}
}

func TestIsNotFoundAndUnauthorized(t *testing.T) {
	if !IsNotFound(&APIError{Code: "NOT_FOUND", Title: "The specified resource does not exist"}) {
		t.Fatal("expected IsNotFound to return true")
	}
	if IsNotFound(fmt.Errorf("something else")) {
		t.Fatal("expected IsNotFound to return false")
	}
	if !IsUnauthorized(&APIError{Code: "UNAUTHORIZED", Title: "Unauthorized"}) {
		t.Fatal("expected IsUnauthorized to return true")
	}
	if IsUnauthorized(fmt.Errorf("something else")) {
		t.Fatal("expected IsUnauthorized to return false")
	}
}

func TestCreateBuildUpload(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploads","id":"UPLOAD_123","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"123","platform":"IOS"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildUploads" {
			t.Fatalf("expected path /v1/buildUploads, got %s", req.URL.Path)
		}
		if !strings.Contains(req.Header.Get("Content-Type"), "application/json") {
			t.Fatalf("expected Content-Type application/json")
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.CreateBuildUpload(context.Background(), BuildUploadCreateRequest{
		Data: BuildUploadCreateData{
			Type: ResourceTypeBuildUploads,
			Attributes: BuildUploadAttributes{
				CFBundleShortVersionString: "1.0.0",
				CFBundleVersion:            "123",
				Platform:                   PlatformIOS,
			},
			Relationships: &BuildUploadRelationships{
				App: &Relationship{
					Data: ResourceData{Type: ResourceTypeApps, ID: "APP_123"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBuildUpload() error: %v", err)
	}
	if result.Data.ID != "UPLOAD_123" {
		t.Fatalf("expected upload ID UPLOAD_123, got %s", result.Data.ID)
	}
}

func TestGetBuildUpload(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"UPLOAD_123","attributes":{"cfBundleShortVersionString":"1.0.0","cfBundleVersion":"123","platform":"IOS"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildUploads/UPLOAD_123" {
			t.Fatalf("expected path /v1/buildUploads/UPLOAD_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.GetBuildUpload(context.Background(), "UPLOAD_123")
	if err != nil {
		t.Fatalf("GetBuildUpload() error: %v", err)
	}
	if result.Data.ID != "UPLOAD_123" {
		t.Fatalf("expected upload ID UPLOAD_123, got %s", result.Data.ID)
	}
}

func TestCreateBuildUploadFile(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"buildUploadFiles","id":"FILE_123","attributes":{"fileName":"app.ipa","fileSize":1024000,"uti":"com.apple.ipa","assetType":"ASSET","uploadOperations":[{"method":"PUT","url":"https://example.com/upload","length":1024000,"offset":0}]}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildUploadFiles" {
			t.Fatalf("expected path /v1/buildUploadFiles, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.CreateBuildUploadFile(context.Background(), BuildUploadFileCreateRequest{
		Data: BuildUploadFileCreateData{
			Type: ResourceTypeBuildUploadFiles,
			Attributes: BuildUploadFileAttributes{
				FileName:  "app.ipa",
				FileSize:  1024000,
				UTI:       UTIIPA,
				AssetType: AssetTypeAsset,
			},
			Relationships: &BuildUploadFileRelationships{
				BuildUpload: &Relationship{
					Data: ResourceData{Type: ResourceTypeBuildUploads, ID: "UPLOAD_123"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateBuildUploadFile() error: %v", err)
	}
	if result.Data.ID != "FILE_123" {
		t.Fatalf("expected file ID FILE_123, got %s", result.Data.ID)
	}
}

func TestUpdateBuildUploadFile(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"FILE_123","attributes":{"uploaded":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildUploadFiles/FILE_123" {
			t.Fatalf("expected path /v1/buildUploadFiles/FILE_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	uploaded := true
	result, err := client.UpdateBuildUploadFile(context.Background(), "FILE_123", BuildUploadFileUpdateRequest{
		Data: BuildUploadFileUpdateData{
			Type: ResourceTypeBuildUploadFiles,
			ID:   "FILE_123",
			Attributes: &BuildUploadFileUpdateAttributes{
				Uploaded: &uploaded,
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateBuildUploadFile() error: %v", err)
	}
	if result.Data.ID != "FILE_123" {
		t.Fatalf("expected file ID FILE_123, got %s", result.Data.ID)
	}
}

func TestGetBuildUploads_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"buildUploads","id":"UPLOAD_123","attributes":{"cfBundleShortVersionString":"1.0.0"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/buildUploads" {
			t.Fatalf("expected path /v1/apps/app-1/buildUploads, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[cfBundleShortVersionString]") != "1.0.0,1.0.1" {
			t.Fatalf("expected filter[cfBundleShortVersionString]=1.0.0,1.0.1, got %q", values.Get("filter[cfBundleShortVersionString]"))
		}
		if values.Get("filter[cfBundleVersion]") != "100" {
			t.Fatalf("expected filter[cfBundleVersion]=100, got %q", values.Get("filter[cfBundleVersion]"))
		}
		if values.Get("filter[platform]") != "IOS,MAC_OS" {
			t.Fatalf("expected filter[platform]=IOS,MAC_OS, got %q", values.Get("filter[platform]"))
		}
		if values.Get("filter[state]") != "UPLOADED,FAILED" {
			t.Fatalf("expected filter[state]=UPLOADED,FAILED, got %q", values.Get("filter[state]"))
		}
		if values.Get("sort") != "uploadedDate" {
			t.Fatalf("expected sort=uploadedDate, got %q", values.Get("sort"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetBuildUploads(context.Background(), "app-1",
		WithBuildUploadsCFBundleShortVersionStrings([]string{"1.0.0", "1.0.1"}),
		WithBuildUploadsCFBundleVersions([]string{"100"}),
		WithBuildUploadsPlatforms([]string{"ios", "mac_os"}),
		WithBuildUploadsStates([]string{"uploaded", "failed"}),
		WithBuildUploadsSort("uploadedDate"),
		WithBuildUploadsLimit(5),
	)
	if err != nil {
		t.Fatalf("GetBuildUploads() error: %v", err)
	}
}

func TestDeleteBuildUpload(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildUploads/UPLOAD_123" {
			t.Fatalf("expected path /v1/buildUploads/UPLOAD_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteBuildUpload(context.Background(), "UPLOAD_123"); err != nil {
		t.Fatalf("DeleteBuildUpload() error: %v", err)
	}
}

func TestGetBuildUploadFiles_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"buildUploadFiles","id":"FILE_123","attributes":{"fileName":"app.ipa","fileSize":1234,"uti":"com.apple.ipa","assetType":"ASSET"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildUploads/UPLOAD_123/buildUploadFiles" {
			t.Fatalf("expected path /v1/buildUploads/UPLOAD_123/buildUploadFiles, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "15" {
			t.Fatalf("expected limit=15, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBuildUploadFiles(context.Background(), "UPLOAD_123", WithBuildUploadFilesLimit(15)); err != nil {
		t.Fatalf("GetBuildUploadFiles() error: %v", err)
	}
}

func TestGetBuildUploadFile(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"buildUploadFiles","id":"FILE_123","attributes":{"fileName":"app.ipa","fileSize":1234,"uti":"com.apple.ipa","assetType":"ASSET"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildUploadFiles/FILE_123" {
			t.Fatalf("expected path /v1/buildUploadFiles/FILE_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBuildUploadFile(context.Background(), "FILE_123"); err != nil {
		t.Fatalf("GetBuildUploadFile() error: %v", err)
	}
}

func TestGetBuildIndividualTesters_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaTesters","id":"TESTER_1","attributes":{"email":"tester@example.com"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/BUILD_123/individualTesters" {
			t.Fatalf("expected path /v1/builds/BUILD_123/individualTesters, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBuildIndividualTesters(context.Background(), "BUILD_123", WithBuildIndividualTestersLimit(10)); err != nil {
		t.Fatalf("GetBuildIndividualTesters() error: %v", err)
	}
}

func TestAddIndividualTestersToBuild(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/BUILD_123/relationships/individualTesters" {
			t.Fatalf("expected path /v1/builds/BUILD_123/relationships/individualTesters, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload RelationshipRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 data entries, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeBetaTesters || payload.Data[0].ID != "TESTER_1" {
			t.Fatalf("unexpected first tester linkage: %+v", payload.Data[0])
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.AddIndividualTestersToBuild(context.Background(), "BUILD_123", []string{"TESTER_1", "TESTER_2"}); err != nil {
		t.Fatalf("AddIndividualTestersToBuild() error: %v", err)
	}
}

func TestRemoveIndividualTestersFromBuild(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/BUILD_123/relationships/individualTesters" {
			t.Fatalf("expected path /v1/builds/BUILD_123/relationships/individualTesters, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload RelationshipRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if len(payload.Data) != 1 || payload.Data[0].ID != "TESTER_1" {
			t.Fatalf("unexpected relationship payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.RemoveIndividualTestersFromBuild(context.Background(), "BUILD_123", []string{"TESTER_1"}); err != nil {
		t.Fatalf("RemoveIndividualTestersFromBuild() error: %v", err)
	}
}

func TestGetBuildBetaUsagesMetrics_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"value":"1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/builds/build-1/metrics/betaBuildUsages" {
			t.Fatalf("expected path /v1/builds/build-1/metrics/betaBuildUsages, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "7" {
			t.Fatalf("expected limit=7, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetBuildBetaUsagesMetrics(context.Background(), "build-1", WithBetaBuildUsagesLimit(7))
	if err != nil {
		t.Fatalf("GetBuildBetaUsagesMetrics() error: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatalf("expected response data")
	}
}

func TestGetBetaTesterUsagesMetrics_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"value":"1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaTesters/tester-1/metrics/betaTesterUsages" {
			t.Fatalf("expected path /v1/betaTesters/tester-1/metrics/betaTesterUsages, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("period") != "P30D" {
			t.Fatalf("expected period=P30D, got %q", values.Get("period"))
		}
		if values.Get("filter[apps]") != "app-1" {
			t.Fatalf("expected filter[apps]=app-1, got %q", values.Get("filter[apps]"))
		}
		if values.Get("limit") != "20" {
			t.Fatalf("expected limit=20, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetBetaTesterUsagesMetrics(context.Background(), "tester-1",
		WithBetaTesterUsagesPeriod("P30D"),
		WithBetaTesterUsagesAppID("app-1"),
		WithBetaTesterUsagesLimit(20),
	)
	if err != nil {
		t.Fatalf("GetBetaTesterUsagesMetrics() error: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatalf("expected response data")
	}
}

func TestGetAppScreenshotSets(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"SET_123","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets" {
			t.Fatalf("expected path /v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.GetAppScreenshotSets(context.Background(), "LOC_123")
	if err != nil {
		t.Fatalf("GetAppScreenshotSets() error: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 set, got %d", len(result.Data))
	}
}

func TestCreateAppScreenshotSet(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appScreenshotSets","id":"SET_123","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appScreenshotSets" {
			t.Fatalf("expected path /v1/appScreenshotSets, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.CreateAppScreenshotSet(context.Background(), "LOC_123", "APP_IPHONE_65")
	if err != nil {
		t.Fatalf("CreateAppScreenshotSet() error: %v", err)
	}
	if result.Data.ID != "SET_123" {
		t.Fatalf("expected set ID SET_123, got %s", result.Data.ID)
	}
}

func TestDeleteAppScreenshotSet(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appScreenshotSets/SET_123" {
			t.Fatalf("expected path /v1/appScreenshotSets/SET_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppScreenshotSet(context.Background(), "SET_123"); err != nil {
		t.Fatalf("DeleteAppScreenshotSet() error: %v", err)
	}
}

func TestGetAppScreenshots(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"SHOT_123","attributes":{"fileName":"shot.png","fileSize":1024}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appScreenshotSets/SET_123/appScreenshots" {
			t.Fatalf("expected path /v1/appScreenshotSets/SET_123/appScreenshots, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.GetAppScreenshots(context.Background(), "SET_123")
	if err != nil {
		t.Fatalf("GetAppScreenshots() error: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 screenshot, got %d", len(result.Data))
	}
}

func TestGetAppScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"SHOT_123","attributes":{"fileName":"shot.png","fileSize":1024}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appScreenshots/SHOT_123" {
			t.Fatalf("expected path /v1/appScreenshots/SHOT_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.GetAppScreenshot(context.Background(), "SHOT_123")
	if err != nil {
		t.Fatalf("GetAppScreenshot() error: %v", err)
	}
	if result.Data.ID != "SHOT_123" {
		t.Fatalf("expected screenshot ID SHOT_123, got %s", result.Data.ID)
	}
}

func TestCreateAppScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appScreenshots","id":"SHOT_123","attributes":{"fileName":"shot.png","fileSize":1024,"uploadOperations":[{"method":"PUT","url":"https://example.com/upload","length":1024,"offset":0}]}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appScreenshots" {
			t.Fatalf("expected path /v1/appScreenshots, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.CreateAppScreenshot(context.Background(), "SET_123", "shot.png", 1024)
	if err != nil {
		t.Fatalf("CreateAppScreenshot() error: %v", err)
	}
	if result.Data.ID != "SHOT_123" {
		t.Fatalf("expected screenshot ID SHOT_123, got %s", result.Data.ID)
	}
}

func TestUpdateAppScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"SHOT_123","attributes":{"uploaded":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appScreenshots/SHOT_123" {
			t.Fatalf("expected path /v1/appScreenshots/SHOT_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	uploaded := true
	result, err := client.UpdateAppScreenshot(context.Background(), "SHOT_123", uploaded, "abc123")
	if err != nil {
		t.Fatalf("UpdateAppScreenshot() error: %v", err)
	}
	if result.Data.ID != "SHOT_123" {
		t.Fatalf("expected screenshot ID SHOT_123, got %s", result.Data.ID)
	}
}

func TestDeleteAppScreenshot(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appScreenshots/SHOT_123" {
			t.Fatalf("expected path /v1/appScreenshots/SHOT_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppScreenshot(context.Background(), "SHOT_123"); err != nil {
		t.Fatalf("DeleteAppScreenshot() error: %v", err)
	}
}

func TestGetAppPreviewSets(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"SET_123","attributes":{"previewType":"IPHONE_65"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionLocalizations/LOC_123/appPreviewSets" {
			t.Fatalf("expected path /v1/appStoreVersionLocalizations/LOC_123/appPreviewSets, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.GetAppPreviewSets(context.Background(), "LOC_123")
	if err != nil {
		t.Fatalf("GetAppPreviewSets() error: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 set, got %d", len(result.Data))
	}
}

func TestCreateAppPreviewSet(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appPreviewSets","id":"SET_123","attributes":{"previewType":"IPHONE_65"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appPreviewSets" {
			t.Fatalf("expected path /v1/appPreviewSets, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.CreateAppPreviewSet(context.Background(), "LOC_123", "IPHONE_65")
	if err != nil {
		t.Fatalf("CreateAppPreviewSet() error: %v", err)
	}
	if result.Data.ID != "SET_123" {
		t.Fatalf("expected set ID SET_123, got %s", result.Data.ID)
	}
}

func TestDeleteAppPreviewSet(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appPreviewSets/SET_123" {
			t.Fatalf("expected path /v1/appPreviewSets/SET_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppPreviewSet(context.Background(), "SET_123"); err != nil {
		t.Fatalf("DeleteAppPreviewSet() error: %v", err)
	}
}

func TestGetAppPreviews(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appPreviews","id":"PREVIEW_123","attributes":{"fileName":"preview.mov","fileSize":2048}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appPreviewSets/SET_123/appPreviews" {
			t.Fatalf("expected path /v1/appPreviewSets/SET_123/appPreviews, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.GetAppPreviews(context.Background(), "SET_123")
	if err != nil {
		t.Fatalf("GetAppPreviews() error: %v", err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 preview, got %d", len(result.Data))
	}
}

func TestGetAppPreview(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appPreviews","id":"PREVIEW_123","attributes":{"fileName":"preview.mov","fileSize":2048}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appPreviews/PREVIEW_123" {
			t.Fatalf("expected path /v1/appPreviews/PREVIEW_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.GetAppPreview(context.Background(), "PREVIEW_123")
	if err != nil {
		t.Fatalf("GetAppPreview() error: %v", err)
	}
	if result.Data.ID != "PREVIEW_123" {
		t.Fatalf("expected preview ID PREVIEW_123, got %s", result.Data.ID)
	}
}

func TestCreateAppPreview(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appPreviews","id":"PREVIEW_123","attributes":{"fileName":"preview.mov","fileSize":2048,"mimeType":"video/quicktime","uploadOperations":[{"method":"PUT","url":"https://example.com/upload","length":2048,"offset":0}]}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appPreviews" {
			t.Fatalf("expected path /v1/appPreviews, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.CreateAppPreview(context.Background(), "SET_123", "preview.mov", 2048, "video/quicktime")
	if err != nil {
		t.Fatalf("CreateAppPreview() error: %v", err)
	}
	if result.Data.ID != "PREVIEW_123" {
		t.Fatalf("expected preview ID PREVIEW_123, got %s", result.Data.ID)
	}
}

func TestUpdateAppPreview(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appPreviews","id":"PREVIEW_123","attributes":{"uploaded":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appPreviews/PREVIEW_123" {
			t.Fatalf("expected path /v1/appPreviews/PREVIEW_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	uploaded := true
	result, err := client.UpdateAppPreview(context.Background(), "PREVIEW_123", uploaded, "def456")
	if err != nil {
		t.Fatalf("UpdateAppPreview() error: %v", err)
	}
	if result.Data.ID != "PREVIEW_123" {
		t.Fatalf("expected preview ID PREVIEW_123, got %s", result.Data.ID)
	}
}

func TestDeleteAppPreview(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appPreviews/PREVIEW_123" {
			t.Fatalf("expected path /v1/appPreviews/PREVIEW_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppPreview(context.Background(), "PREVIEW_123"); err != nil {
		t.Fatalf("DeleteAppPreview() error: %v", err)
	}
}

func TestCreateAppStoreVersionSubmission(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreVersionSubmissions","id":"SUBMIT_123","attributes":{"createdDate":"2026-01-20T00:00:00Z"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionSubmissions" {
			t.Fatalf("expected path /v1/appStoreVersionSubmissions, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.CreateAppStoreVersionSubmission(context.Background(), AppStoreVersionSubmissionCreateRequest{
		Data: AppStoreVersionSubmissionCreateData{
			Type: ResourceTypeAppStoreVersionSubmissions,
			Relationships: &AppStoreVersionSubmissionRelationships{
				AppStoreVersion: &Relationship{
					Data: ResourceData{Type: ResourceTypeAppStoreVersions, ID: "VERSION_123"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateAppStoreVersionSubmission() error: %v", err)
	}
	if result.Data.ID != "SUBMIT_123" {
		t.Fatalf("expected submission ID SUBMIT_123, got %s", result.Data.ID)
	}
}

func TestGetAppStoreVersionSubmission(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"SUBMIT_123","attributes":{"createdDate":"2026-01-20T00:00:00Z"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionSubmissions/SUBMIT_123" {
			t.Fatalf("expected path /v1/appStoreVersionSubmissions/SUBMIT_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	result, err := client.GetAppStoreVersionSubmission(context.Background(), "SUBMIT_123")
	if err != nil {
		t.Fatalf("GetAppStoreVersionSubmission() error: %v", err)
	}
	if result.Data.ID != "SUBMIT_123" {
		t.Fatalf("expected submission ID SUBMIT_123, got %s", result.Data.ID)
	}
}

func TestDeleteAppStoreVersionSubmission(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersionSubmissions/SUBMIT_123" {
			t.Fatalf("expected path /v1/appStoreVersionSubmissions/SUBMIT_123, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	err := client.DeleteAppStoreVersionSubmission(context.Background(), "SUBMIT_123")
	if err != nil {
		t.Fatalf("DeleteAppStoreVersionSubmission() error: %v", err)
	}
}

func TestBuildUploadMethods_ErrorResponse(t *testing.T) {
	ctx := context.Background()
	errorBody := `{"errors":[{"code":"BAD_REQUEST","title":"Bad Request","detail":"nope"}]}`

	tests := []struct {
		name string
		call func(*Client) error
	}{
		{
			name: "CreateBuildUpload",
			call: func(c *Client) error {
				_, err := c.CreateBuildUpload(ctx, BuildUploadCreateRequest{
					Data: BuildUploadCreateData{
						Type: ResourceTypeBuildUploads,
						Attributes: BuildUploadAttributes{
							CFBundleShortVersionString: "1.0.0",
							CFBundleVersion:            "123",
							Platform:                   PlatformIOS,
						},
						Relationships: &BuildUploadRelationships{
							App: &Relationship{
								Data: ResourceData{Type: ResourceTypeApps, ID: "APP_123"},
							},
						},
					},
				})
				return err
			},
		},
		{
			name: "GetBuildUpload",
			call: func(c *Client) error {
				_, err := c.GetBuildUpload(ctx, "UPLOAD_123")
				return err
			},
		},
		{
			name: "CreateBuildUploadFile",
			call: func(c *Client) error {
				_, err := c.CreateBuildUploadFile(ctx, BuildUploadFileCreateRequest{
					Data: BuildUploadFileCreateData{
						Type: ResourceTypeBuildUploadFiles,
						Attributes: BuildUploadFileAttributes{
							FileName:  "app.ipa",
							FileSize:  1024000,
							UTI:       UTIIPA,
							AssetType: AssetTypeAsset,
						},
						Relationships: &BuildUploadFileRelationships{
							BuildUpload: &Relationship{
								Data: ResourceData{Type: ResourceTypeBuildUploads, ID: "UPLOAD_123"},
							},
						},
					},
				})
				return err
			},
		},
		{
			name: "UpdateBuildUploadFile",
			call: func(c *Client) error {
				uploaded := true
				_, err := c.UpdateBuildUploadFile(ctx, "FILE_123", BuildUploadFileUpdateRequest{
					Data: BuildUploadFileUpdateData{
						Type: ResourceTypeBuildUploadFiles,
						ID:   "FILE_123",
						Attributes: &BuildUploadFileUpdateAttributes{
							Uploaded: &uploaded,
						},
					},
				})
				return err
			},
		},
		{
			name: "CreateAppStoreVersionSubmission",
			call: func(c *Client) error {
				_, err := c.CreateAppStoreVersionSubmission(ctx, AppStoreVersionSubmissionCreateRequest{
					Data: AppStoreVersionSubmissionCreateData{
						Type: ResourceTypeAppStoreVersionSubmissions,
						Relationships: &AppStoreVersionSubmissionRelationships{
							AppStoreVersion: &Relationship{
								Data: ResourceData{Type: ResourceTypeAppStoreVersions, ID: "VERSION_123"},
							},
						},
					},
				})
				return err
			},
		},
		{
			name: "GetAppStoreVersionSubmission",
			call: func(c *Client) error {
				_, err := c.GetAppStoreVersionSubmission(ctx, "SUBMIT_123")
				return err
			},
		},
		{
			name: "DeleteAppStoreVersionSubmission",
			call: func(c *Client) error {
				return c.DeleteAppStoreVersionSubmission(ctx, "SUBMIT_123")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, nil, jsonResponse(http.StatusBadRequest, errorBody))
			if err := test.call(client); err == nil {
				t.Fatalf("expected error")
			} else if !errors.Is(err, ErrBadRequest) {
				t.Fatalf("expected bad request error, got %v", err)
			}
		})
	}
}

func TestGetCiProducts_WithAppFilterAndLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"ciProducts","id":"prod-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciProducts" {
			t.Fatalf("expected path /v1/ciProducts, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[app]") != "app-1" {
			t.Fatalf("expected filter[app]=app-1, got %q", values.Get("filter[app]"))
		}
		if values.Get("limit") != "25" {
			t.Fatalf("expected limit=25, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiProducts(context.Background(), WithCiProductsAppID("app-1"), WithCiProductsLimit(25)); err != nil {
		t.Fatalf("GetCiProducts() error: %v", err)
	}
}

func TestGetCiProduct(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ciProducts","id":"prod-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciProducts/prod-1" {
			t.Fatalf("expected path /v1/ciProducts/prod-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiProduct(context.Background(), "prod-1"); err != nil {
		t.Fatalf("GetCiProduct() error: %v", err)
	}
}

func TestGetCiProductApp(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciProducts/prod-1/app" {
			t.Fatalf("expected path /v1/ciProducts/prod-1/app, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiProductApp(context.Background(), "prod-1"); err != nil {
		t.Fatalf("GetCiProductApp() error: %v", err)
	}
}

func TestGetCiProductBuildRuns_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"ciBuildRuns","id":"run-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciProducts/prod-1/buildRuns" {
			t.Fatalf("expected path /v1/ciProducts/prod-1/buildRuns, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiProductBuildRuns(context.Background(), "prod-1", WithCiBuildRunsLimit(50)); err != nil {
		t.Fatalf("GetCiProductBuildRuns() error: %v", err)
	}
}

func TestGetCiProductPrimaryRepositories_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"scmRepositories","id":"repo-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciProducts/prod-1/primaryRepositories" {
			t.Fatalf("expected path /v1/ciProducts/prod-1/primaryRepositories, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiProductPrimaryRepositories(context.Background(), "prod-1", WithCiProductRepositoriesLimit(10)); err != nil {
		t.Fatalf("GetCiProductPrimaryRepositories() error: %v", err)
	}
}

func TestGetCiProductAdditionalRepositories_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"scmRepositories","id":"repo-2"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciProducts/prod-1/additionalRepositories" {
			t.Fatalf("expected path /v1/ciProducts/prod-1/additionalRepositories, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiProductAdditionalRepositories(context.Background(), "prod-1", WithCiProductRepositoriesLimit(5)); err != nil {
		t.Fatalf("GetCiProductAdditionalRepositories() error: %v", err)
	}
}

func TestDeleteCiProduct(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciProducts/prod-1" {
			t.Fatalf("expected path /v1/ciProducts/prod-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteCiProduct(context.Background(), "prod-1"); err != nil {
		t.Fatalf("DeleteCiProduct() error: %v", err)
	}
}

func TestGetCiWorkflows_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/ciProducts/prod-1/workflows?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiWorkflows(context.Background(), "prod-1", WithCiWorkflowsNextURL(next)); err != nil {
		t.Fatalf("GetCiWorkflows() error: %v", err)
	}
}

func TestGetCiWorkflow(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ciWorkflows","id":"wf-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciWorkflows/wf-1" {
			t.Fatalf("expected path /v1/ciWorkflows/wf-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiWorkflow(context.Background(), "wf-1"); err != nil {
		t.Fatalf("GetCiWorkflow() error: %v", err)
	}
}

func TestCreateCiWorkflow(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"ciWorkflows","id":"wf-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciWorkflows" {
			t.Fatalf("expected path /v1/ciWorkflows, got %s", req.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		data, ok := payload["data"].(map[string]interface{})
		if !ok || data["type"] != "ciWorkflows" {
			t.Fatalf("expected data.type=ciWorkflows")
		}
		assertAuthorized(t, req)
	}, response)

	body := json.RawMessage(`{"data":{"type":"ciWorkflows"}}`)
	if _, err := client.CreateCiWorkflow(context.Background(), body); err != nil {
		t.Fatalf("CreateCiWorkflow() error: %v", err)
	}
}

func TestUpdateCiWorkflow(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ciWorkflows","id":"wf-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciWorkflows/wf-1" {
			t.Fatalf("expected path /v1/ciWorkflows/wf-1, got %s", req.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		data, ok := payload["data"].(map[string]interface{})
		if !ok || data["type"] != "ciWorkflows" {
			t.Fatalf("expected data.type=ciWorkflows")
		}
		assertAuthorized(t, req)
	}, response)

	body := json.RawMessage(`{"data":{"type":"ciWorkflows","id":"wf-1"}}`)
	if _, err := client.UpdateCiWorkflow(context.Background(), "wf-1", body); err != nil {
		t.Fatalf("UpdateCiWorkflow() error: %v", err)
	}
}

func TestDeleteCiWorkflow(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciWorkflows/wf-1" {
			t.Fatalf("expected path /v1/ciWorkflows/wf-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteCiWorkflow(context.Background(), "wf-1"); err != nil {
		t.Fatalf("DeleteCiWorkflow() error: %v", err)
	}
}

func TestGetScmProviders_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"scmProviders","id":"provider-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/scmProviders" {
			t.Fatalf("expected path /v1/scmProviders, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("limit"); got != "25" {
			t.Fatalf("expected limit=25, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetScmProviders(context.Background(), WithScmProvidersLimit(25)); err != nil {
		t.Fatalf("GetScmProviders() error: %v", err)
	}
}

func TestGetScmProviders_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/scmProviders?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetScmProviders(context.Background(), WithScmProvidersNextURL(next)); err != nil {
		t.Fatalf("GetScmProviders() error: %v", err)
	}
}

func TestGetScmProvider(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"scmProviders","id":"provider-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/scmProviders/provider-1" {
			t.Fatalf("expected path /v1/scmProviders/provider-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetScmProvider(context.Background(), "provider-1"); err != nil {
		t.Fatalf("GetScmProvider() error: %v", err)
	}
}

func TestGetScmProviderRepositories_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"scmRepositories","id":"repo-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/scmProviders/provider-1/repositories" {
			t.Fatalf("expected path /v1/scmProviders/provider-1/repositories, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("limit"); got != "10" {
			t.Fatalf("expected limit=10, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetScmProviderRepositories(context.Background(), "provider-1", WithScmRepositoriesLimit(10)); err != nil {
		t.Fatalf("GetScmProviderRepositories() error: %v", err)
	}
}

func TestGetScmRepositories_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"scmRepositories","id":"repo-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/scmRepositories" {
			t.Fatalf("expected path /v1/scmRepositories, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("limit"); got != "10" {
			t.Fatalf("expected limit=10, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetScmRepositories(context.Background(), WithScmRepositoriesLimit(10)); err != nil {
		t.Fatalf("GetScmRepositories() error: %v", err)
	}
}

func TestGetScmRepositories_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/scmRepositories?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetScmRepositories(context.Background(), WithScmRepositoriesNextURL(next)); err != nil {
		t.Fatalf("GetScmRepositories() error: %v", err)
	}
}

func TestGetScmRepository(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"scmRepositories","id":"repo-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/scmRepositories/repo-1" {
			t.Fatalf("expected path /v1/scmRepositories/repo-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetScmRepository(context.Background(), "repo-1"); err != nil {
		t.Fatalf("GetScmRepository() error: %v", err)
	}
}

func TestGetScmGitReference(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"scmGitReferences","id":"ref-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/scmGitReferences/ref-1" {
			t.Fatalf("expected path /v1/scmGitReferences/ref-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetScmGitReference(context.Background(), "ref-1"); err != nil {
		t.Fatalf("GetScmGitReference() error: %v", err)
	}
}

func TestGetScmRepositoryGitReferencesRelationships_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/scmRepositories/repo-1/relationships/gitReferences" {
			t.Fatalf("expected path /v1/scmRepositories/repo-1/relationships/gitReferences, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("limit"); got != "5" {
			t.Fatalf("expected limit=5, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetScmRepositoryGitReferencesRelationships(context.Background(), "repo-1", WithLinkagesLimit(5)); err != nil {
		t.Fatalf("GetScmRepositoryGitReferencesRelationships() error: %v", err)
	}
}

func TestGetScmRepositoryPullRequests_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"scmPullRequests","id":"pr-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/scmRepositories/repo-1/pullRequests" {
			t.Fatalf("expected path /v1/scmRepositories/repo-1/pullRequests, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("limit"); got != "20" {
			t.Fatalf("expected limit=20, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetScmRepositoryPullRequests(context.Background(), "repo-1", WithScmPullRequestsLimit(20)); err != nil {
		t.Fatalf("GetScmRepositoryPullRequests() error: %v", err)
	}
}

func TestGetScmRepositoryPullRequestsRelationships_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/scmRepositories/repo-1/relationships/pullRequests" {
			t.Fatalf("expected path /v1/scmRepositories/repo-1/relationships/pullRequests, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("limit"); got != "20" {
			t.Fatalf("expected limit=20, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetScmRepositoryPullRequestsRelationships(context.Background(), "repo-1", WithLinkagesLimit(20)); err != nil {
		t.Fatalf("GetScmRepositoryPullRequestsRelationships() error: %v", err)
	}
}

func TestGetScmPullRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"scmPullRequests","id":"pr-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/scmPullRequests/pr-1" {
			t.Fatalf("expected path /v1/scmPullRequests/pr-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetScmPullRequest(context.Background(), "pr-1"); err != nil {
		t.Fatalf("GetScmPullRequest() error: %v", err)
	}
}

func TestGetScmGitReferences_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"scmGitReferences","id":"ref-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/scmRepositories/repo-1/gitReferences" {
			t.Fatalf("expected path /v1/scmRepositories/repo-1/gitReferences, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "100" {
			t.Fatalf("expected limit=100, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetScmGitReferences(context.Background(), "repo-1", WithScmGitReferencesLimit(100)); err != nil {
		t.Fatalf("GetScmGitReferences() error: %v", err)
	}
}

func TestGetCiBuildRuns_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"ciBuildRuns","id":"run-1","attributes":{"number":1}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciWorkflows/wf-1/buildRuns" {
			t.Fatalf("expected path /v1/ciWorkflows/wf-1/buildRuns, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiBuildRuns(context.Background(), "wf-1", WithCiBuildRunsLimit(50)); err != nil {
		t.Fatalf("GetCiBuildRuns() error: %v", err)
	}
}

func TestGetCiBuildRun(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":1}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciBuildRuns/run-1" {
			t.Fatalf("expected path /v1/ciBuildRuns/run-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiBuildRun(context.Background(), "run-1"); err != nil {
		t.Fatalf("GetCiBuildRun() error: %v", err)
	}
}

func TestGetCiBuildRunBuilds_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciBuildRuns/run-1/builds" {
			t.Fatalf("expected path /v1/ciBuildRuns/run-1/builds, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiBuildRunBuilds(context.Background(), "run-1", WithCiBuildRunBuildsLimit(10)); err != nil {
		t.Fatalf("GetCiBuildRunBuilds() error: %v", err)
	}
}

func TestCreateCiBuildRun(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":1}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciBuildRuns" {
			t.Fatalf("expected path /v1/ciBuildRuns, got %s", req.URL.Path)
		}
		var payload CiBuildRunCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeCiBuildRuns {
			t.Fatalf("expected type ciBuildRuns, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Workflow == nil || payload.Data.Relationships.SourceBranchOrTag == nil {
			t.Fatalf("expected workflow and sourceBranchOrTag relationships")
		}
		assertAuthorized(t, req)
	}, response)

	req := CiBuildRunCreateRequest{
		Data: CiBuildRunCreateData{
			Type: ResourceTypeCiBuildRuns,
			Relationships: &CiBuildRunCreateRelationships{
				Workflow: &Relationship{
					Data: ResourceData{Type: ResourceTypeCiWorkflows, ID: "wf-1"},
				},
				SourceBranchOrTag: &Relationship{
					Data: ResourceData{Type: ResourceTypeScmGitReferences, ID: "ref-1"},
				},
			},
		},
	}
	if _, err := client.CreateCiBuildRun(context.Background(), req); err != nil {
		t.Fatalf("CreateCiBuildRun() error: %v", err)
	}
}

func TestGetCiBuildAction(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ciBuildActions","id":"action-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciBuildActions/action-1" {
			t.Fatalf("expected path /v1/ciBuildActions/action-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiBuildAction(context.Background(), "action-1"); err != nil {
		t.Fatalf("GetCiBuildAction() error: %v", err)
	}
}

func TestGetCiBuildActionBuildRun(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ciBuildRuns","id":"run-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciBuildActions/action-1/buildRun" {
			t.Fatalf("expected path /v1/ciBuildActions/action-1/buildRun, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiBuildActionBuildRun(context.Background(), "action-1"); err != nil {
		t.Fatalf("GetCiBuildActionBuildRun() error: %v", err)
	}
}

func TestGetCiBuildActionArtifacts_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"ciArtifacts","id":"art-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciBuildActions/action-1/artifacts" {
			t.Fatalf("expected path /v1/ciBuildActions/action-1/artifacts, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "20" {
			t.Fatalf("expected limit=20, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiBuildActionArtifacts(context.Background(), "action-1", WithCiArtifactsLimit(20)); err != nil {
		t.Fatalf("GetCiBuildActionArtifacts() error: %v", err)
	}
}

func TestGetCiArtifact(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ciArtifacts","id":"art-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciArtifacts/art-1" {
			t.Fatalf("expected path /v1/ciArtifacts/art-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiArtifact(context.Background(), "art-1"); err != nil {
		t.Fatalf("GetCiArtifact() error: %v", err)
	}
}

func TestGetCiBuildActionTestResults_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"ciTestResults","id":"test-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciBuildActions/action-1/testResults" {
			t.Fatalf("expected path /v1/ciBuildActions/action-1/testResults, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "30" {
			t.Fatalf("expected limit=30, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiBuildActionTestResults(context.Background(), "action-1", WithCiTestResultsLimit(30)); err != nil {
		t.Fatalf("GetCiBuildActionTestResults() error: %v", err)
	}
}

func TestGetCiTestResult(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ciTestResults","id":"test-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciTestResults/test-1" {
			t.Fatalf("expected path /v1/ciTestResults/test-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiTestResult(context.Background(), "test-1"); err != nil {
		t.Fatalf("GetCiTestResult() error: %v", err)
	}
}

func TestGetCiBuildActionIssues_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"ciIssues","id":"issue-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciBuildActions/action-1/issues" {
			t.Fatalf("expected path /v1/ciBuildActions/action-1/issues, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "40" {
			t.Fatalf("expected limit=40, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiBuildActionIssues(context.Background(), "action-1", WithCiIssuesLimit(40)); err != nil {
		t.Fatalf("GetCiBuildActionIssues() error: %v", err)
	}
}

func TestGetCiIssue(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ciIssues","id":"issue-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciIssues/issue-1" {
			t.Fatalf("expected path /v1/ciIssues/issue-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiIssue(context.Background(), "issue-1"); err != nil {
		t.Fatalf("GetCiIssue() error: %v", err)
	}
}

func TestGetCiMacOsVersions_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"ciMacOsVersions","id":"macos-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciMacOsVersions" {
			t.Fatalf("expected path /v1/ciMacOsVersions, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiMacOsVersions(context.Background(), WithCiMacOsVersionsLimit(5)); err != nil {
		t.Fatalf("GetCiMacOsVersions() error: %v", err)
	}
}

func TestGetCiMacOsVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ciMacOsVersions","id":"macos-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciMacOsVersions/macos-1" {
			t.Fatalf("expected path /v1/ciMacOsVersions/macos-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiMacOsVersion(context.Background(), "macos-1"); err != nil {
		t.Fatalf("GetCiMacOsVersion() error: %v", err)
	}
}

func TestGetCiMacOsVersionXcodeVersions_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"ciXcodeVersions","id":"xcode-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciMacOsVersions/macos-1/xcodeVersions" {
			t.Fatalf("expected path /v1/ciMacOsVersions/macos-1/xcodeVersions, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "20" {
			t.Fatalf("expected limit=20, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiMacOsVersionXcodeVersions(context.Background(), "macos-1", WithCiXcodeVersionsLimit(20)); err != nil {
		t.Fatalf("GetCiMacOsVersionXcodeVersions() error: %v", err)
	}
}

func TestGetCiXcodeVersions_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"ciXcodeVersions","id":"xcode-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciXcodeVersions" {
			t.Fatalf("expected path /v1/ciXcodeVersions, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiXcodeVersions(context.Background(), WithCiXcodeVersionsLimit(10)); err != nil {
		t.Fatalf("GetCiXcodeVersions() error: %v", err)
	}
}

func TestGetCiXcodeVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"ciXcodeVersions","id":"xcode-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciXcodeVersions/xcode-1" {
			t.Fatalf("expected path /v1/ciXcodeVersions/xcode-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiXcodeVersion(context.Background(), "xcode-1"); err != nil {
		t.Fatalf("GetCiXcodeVersion() error: %v", err)
	}
}

func TestGetCiXcodeVersionMacOsVersions_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"ciMacOsVersions","id":"macos-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/ciXcodeVersions/xcode-1/macOsVersions" {
			t.Fatalf("expected path /v1/ciXcodeVersions/xcode-1/macOsVersions, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "15" {
			t.Fatalf("expected limit=15, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCiXcodeVersionMacOsVersions(context.Background(), "xcode-1", WithCiMacOsVersionsLimit(15)); err != nil {
		t.Fatalf("GetCiXcodeVersionMacOsVersions() error: %v", err)
	}
}

func TestDownloadCiArtifact_NoAuthHeader(t *testing.T) {
	downloadURL := "https://appstoreconnect.apple.com/artifacts/artifact.zip"
	response := rawResponse(http.StatusOK, "artifact-data")
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != downloadURL {
			t.Fatalf("expected URL %q, got %q", downloadURL, req.URL.String())
		}
		if req.Header.Get("Authorization") != "" {
			t.Fatalf("expected no Authorization header")
		}
	}, response)

	download, err := client.DownloadCiArtifact(context.Background(), downloadURL)
	if err != nil {
		t.Fatalf("DownloadCiArtifact() error: %v", err)
	}
	_ = download.Body.Close()
}

func TestDownloadCiArtifact_ICloudContentHostAllowed(t *testing.T) {
	downloadURL := "https://cvws.icloud-content.com/artifact.zip"
	response := rawResponse(http.StatusOK, "artifact-data")
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != downloadURL {
			t.Fatalf("expected URL %q, got %q", downloadURL, req.URL.String())
		}
		if req.Header.Get("Authorization") != "" {
			t.Fatalf("expected no Authorization header")
		}
	}, response)

	download, err := client.DownloadCiArtifact(context.Background(), downloadURL)
	if err != nil {
		t.Fatalf("DownloadCiArtifact() error: %v", err)
	}
	_ = download.Body.Close()
}

func TestDownloadCiArtifact_UntrustedHost(t *testing.T) {
	downloadURL := "https://downloads.example.com/artifact.zip"
	client := newTestClient(t, nil, nil)

	if _, err := client.DownloadCiArtifact(context.Background(), downloadURL); err == nil {
		t.Fatal("expected error for untrusted host")
	}
}

func TestDownloadCiArtifact_InvalidScheme(t *testing.T) {
	client := newTestClient(t, nil, nil)
	if _, err := client.DownloadCiArtifact(context.Background(), "http://downloads.example.com/artifact.zip"); err == nil {
		t.Fatal("expected error for insecure scheme")
	}
}

func TestDownloadCiArtifact_EmptyHost(t *testing.T) {
	client := newTestClient(t, nil, nil)
	if _, err := client.DownloadCiArtifact(context.Background(), "https:///artifact.zip"); err == nil {
		t.Fatal("expected error for empty host")
	}
}

func TestDownloadCiArtifact_CDNHostRequiresSignature(t *testing.T) {
	downloadURL := "https://example.cloudfront.net/artifact.zip"
	client := newTestClient(t, nil, nil)

	if _, err := client.DownloadCiArtifact(context.Background(), downloadURL); err == nil {
		t.Fatal("expected error for unsigned CDN host")
	}
}

func TestDownloadCiArtifact_CDNHostWithSignature(t *testing.T) {
	downloadURL := "https://example.cloudfront.net/artifact.zip?Signature=abc&Key-Pair-Id=key"
	response := rawResponse(http.StatusOK, "artifact-data")
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != downloadURL {
			t.Fatalf("expected URL %q, got %q", downloadURL, req.URL.String())
		}
		if req.Header.Get("Authorization") != "" {
			t.Fatalf("expected no Authorization header")
		}
	}, response)

	download, err := client.DownloadCiArtifact(context.Background(), downloadURL)
	if err != nil {
		t.Fatalf("DownloadCiArtifact() error: %v", err)
	}
	_ = download.Body.Close()
}

func TestResolveCiWorkflowByName_CaseInsensitive(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"ciWorkflows","id":"wf-1","attributes":{"name":"CI Build"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/ciProducts/prod-1/workflows" {
			t.Fatalf("expected path /v1/ciProducts/prod-1/workflows, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	workflow, err := client.ResolveCiWorkflowByName(context.Background(), "prod-1", "ci build")
	if err != nil {
		t.Fatalf("ResolveCiWorkflowByName() error: %v", err)
	}
	if workflow.ID != "wf-1" {
		t.Fatalf("expected workflow ID wf-1, got %q", workflow.ID)
	}
}

func TestResolveCiWorkflowByName_NoMatch(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"ciWorkflows","id":"wf-1","attributes":{"name":"Deploy"}}]}`)
	client := newTestClient(t, nil, response)

	if _, err := client.ResolveCiWorkflowByName(context.Background(), "prod-1", "ci"); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveGitReferenceByName_CanonicalMatch(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"scmGitReferences","id":"ref-1","attributes":{"name":"main","canonicalName":"refs/heads/main","isDeleted":false}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/scmRepositories/repo-1/gitReferences" {
			t.Fatalf("expected path /v1/scmRepositories/repo-1/gitReferences, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	ref, err := client.ResolveGitReferenceByName(context.Background(), "repo-1", "main")
	if err != nil {
		t.Fatalf("ResolveGitReferenceByName() error: %v", err)
	}
	if ref.ID != "ref-1" {
		t.Fatalf("expected git reference ID ref-1, got %q", ref.ID)
	}
}

func TestResolveGitReferenceByName_SuffixMatchNotAllowed(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"scmGitReferences","id":"ref-1","attributes":{"name":"feature/main","canonicalName":"refs/heads/feature/main","isDeleted":false}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/scmRepositories/repo-1/gitReferences" {
			t.Fatalf("expected path /v1/scmRepositories/repo-1/gitReferences, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.ResolveGitReferenceByName(context.Background(), "repo-1", "main"); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveGitReferenceByName_NoMatch(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"scmGitReferences","id":"ref-1","attributes":{"name":"develop","canonicalName":"refs/heads/develop","isDeleted":false}}]}`)
	client := newTestClient(t, nil, response)

	if _, err := client.ResolveGitReferenceByName(context.Background(), "repo-1", "main"); err == nil {
		t.Fatal("expected error")
	}
}

func TestGetBundleIDs_WithIdentifierFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"bid-1","attributes":{"identifier":"com.example.app"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/bundleIds" {
			t.Fatalf("expected path /v1/bundleIds, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[identifier]") != "com.example.app" {
			t.Fatalf("expected filter[identifier]=com.example.app, got %q", values.Get("filter[identifier]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBundleIDs(context.Background(), WithBundleIDsFilterIdentifier("com.example.app")); err != nil {
		t.Fatalf("GetBundleIDs() error: %v", err)
	}
}

func TestGetInAppPurchasesV2_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"CONSUMABLE"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/123/inAppPurchasesV2" {
			t.Fatalf("expected path /v1/apps/123/inAppPurchasesV2, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchasesV2(context.Background(), "123", WithIAPLimit(10)); err != nil {
		t.Fatalf("GetInAppPurchasesV2() error: %v", err)
	}
}

func TestGetBundleIDs_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"bundleIds","id":"b1","attributes":{"name":"Demo","identifier":"com.example.demo","platform":"IOS"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/bundleIds" {
			t.Fatalf("expected path /v1/bundleIds, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBundleIDs(context.Background(), WithBundleIDsLimit(10)); err != nil {
		t.Fatalf("GetBundleIDs() error: %v", err)
	}
}

func TestGetBundleIDs_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/bundleIds?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBundleIDs(context.Background(), WithBundleIDsNextURL(next)); err != nil {
		t.Fatalf("GetBundleIDs() error: %v", err)
	}
}

func TestGetInAppPurchasesV2_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/apps/123/inAppPurchasesV2?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchasesV2(context.Background(), "123", WithIAPNextURL(next)); err != nil {
		t.Fatalf("GetInAppPurchasesV2() error: %v", err)
	}
}

func TestGetBundleID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"bundleIds","id":"b1","attributes":{"name":"Demo","identifier":"com.example.demo","platform":"IOS"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/bundleIds/b1" {
			t.Fatalf("expected path /v1/bundleIds/b1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBundleID(context.Background(), "b1"); err != nil {
		t.Fatalf("GetBundleID() error: %v", err)
	}
}

func TestCreateBundleID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"bundleIds","id":"b1","attributes":{"name":"Demo","identifier":"com.example.demo","platform":"IOS"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/bundleIds" {
			t.Fatalf("expected path /v1/bundleIds, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BundleIDCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBundleIds {
			t.Fatalf("expected type bundleIds, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Identifier != "com.example.demo" {
			t.Fatalf("expected identifier com.example.demo, got %q", payload.Data.Attributes.Identifier)
		}
		if payload.Data.Attributes.Platform != PlatformIOS {
			t.Fatalf("expected platform IOS, got %q", payload.Data.Attributes.Platform)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := BundleIDCreateAttributes{
		Name:       "Demo",
		Identifier: "com.example.demo",
		Platform:   PlatformIOS,
	}
	if _, err := client.CreateBundleID(context.Background(), attrs); err != nil {
		t.Fatalf("CreateBundleID() error: %v", err)
	}
}

func TestUpdateBundleID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"bundleIds","id":"b1","attributes":{"name":"Updated","identifier":"com.example.demo","platform":"IOS"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/bundleIds/b1" {
			t.Fatalf("expected path /v1/bundleIds/b1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BundleIDUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBundleIds {
			t.Fatalf("expected type bundleIds, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "b1" {
			t.Fatalf("expected id b1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name != "Updated" {
			t.Fatalf("expected name Updated, got %v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := BundleIDUpdateAttributes{Name: "Updated"}
	if _, err := client.UpdateBundleID(context.Background(), "b1", attrs); err != nil {
		t.Fatalf("UpdateBundleID() error: %v", err)
	}
}

func TestDeleteBundleID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/bundleIds/b1" {
			t.Fatalf("expected path /v1/bundleIds/b1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteBundleID(context.Background(), "b1"); err != nil {
		t.Fatalf("DeleteBundleID() error: %v", err)
	}
}

func TestGetMerchantIDs_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"merchantIds","id":"m1","attributes":{"name":"Example","identifier":"merchant.com.example"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/merchantIds" {
			t.Fatalf("expected path /v1/merchantIds, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[name]") != "Example" {
			t.Fatalf("expected filter[name]=Example, got %q", values.Get("filter[name]"))
		}
		if values.Get("filter[identifier]") != "merchant.com.example" {
			t.Fatalf("expected filter[identifier]=merchant.com.example, got %q", values.Get("filter[identifier]"))
		}
		if values.Get("sort") != "name" {
			t.Fatalf("expected sort=name, got %q", values.Get("sort"))
		}
		if values.Get("fields[merchantIds]") != "name,identifier" {
			t.Fatalf("expected fields[merchantIds], got %q", values.Get("fields[merchantIds]"))
		}
		if values.Get("fields[certificates]") != "certificateType,serialNumber" {
			t.Fatalf("expected fields[certificates], got %q", values.Get("fields[certificates]"))
		}
		if values.Get("include") != "certificates" {
			t.Fatalf("expected include=certificates, got %q", values.Get("include"))
		}
		if values.Get("limit[certificates]") != "10" {
			t.Fatalf("expected limit[certificates]=10, got %q", values.Get("limit[certificates]"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetMerchantIDs(
		context.Background(),
		WithMerchantIDsFilterName("Example"),
		WithMerchantIDsFilterIdentifier("merchant.com.example"),
		WithMerchantIDsSort("name"),
		WithMerchantIDsFields([]string{"name", "identifier"}),
		WithMerchantIDsCertificateFields([]string{"certificateType", "serialNumber"}),
		WithMerchantIDsInclude([]string{"certificates"}),
		WithMerchantIDsCertificatesLimit(10),
		WithMerchantIDsLimit(5),
	); err != nil {
		t.Fatalf("GetMerchantIDs() error: %v", err)
	}
}

func TestGetMerchantID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"merchantIds","id":"m1","attributes":{"name":"Example","identifier":"merchant.com.example"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/merchantIds/m1" {
			t.Fatalf("expected path /v1/merchantIds/m1, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("fields[merchantIds]") != "name" {
			t.Fatalf("expected fields[merchantIds]=name, got %q", values.Get("fields[merchantIds]"))
		}
		if values.Get("fields[certificates]") != "certificateType" {
			t.Fatalf("expected fields[certificates]=certificateType, got %q", values.Get("fields[certificates]"))
		}
		if values.Get("include") != "certificates" {
			t.Fatalf("expected include=certificates, got %q", values.Get("include"))
		}
		if values.Get("limit[certificates]") != "5" {
			t.Fatalf("expected limit[certificates]=5, got %q", values.Get("limit[certificates]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetMerchantID(
		context.Background(),
		"m1",
		WithMerchantIDsFields([]string{"name"}),
		WithMerchantIDsCertificateFields([]string{"certificateType"}),
		WithMerchantIDsInclude([]string{"certificates"}),
		WithMerchantIDsCertificatesLimit(5),
	); err != nil {
		t.Fatalf("GetMerchantID() error: %v", err)
	}
}

func TestCreateMerchantID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"merchantIds","id":"m1","attributes":{"name":"Example","identifier":"merchant.com.example"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/merchantIds" {
			t.Fatalf("expected path /v1/merchantIds, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload MerchantIDCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeMerchantIds {
			t.Fatalf("expected type merchantIds, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Identifier != "merchant.com.example" {
			t.Fatalf("expected identifier merchant.com.example, got %q", payload.Data.Attributes.Identifier)
		}
		if payload.Data.Attributes.Name != "Example" {
			t.Fatalf("expected name Example, got %q", payload.Data.Attributes.Name)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := MerchantIDCreateAttributes{
		Name:       "Example",
		Identifier: "merchant.com.example",
	}
	if _, err := client.CreateMerchantID(context.Background(), attrs); err != nil {
		t.Fatalf("CreateMerchantID() error: %v", err)
	}
}

func TestUpdateMerchantID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"merchantIds","id":"m1","attributes":{"name":"Updated","identifier":"merchant.com.example"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/merchantIds/m1" {
			t.Fatalf("expected path /v1/merchantIds/m1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload MerchantIDUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeMerchantIds {
			t.Fatalf("expected type merchantIds, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "m1" {
			t.Fatalf("expected id m1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name == nil || *payload.Data.Attributes.Name != "Updated" {
			t.Fatalf("expected name Updated, got %v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	attrs := MerchantIDUpdateAttributes{Name: &name}
	if _, err := client.UpdateMerchantID(context.Background(), "m1", attrs); err != nil {
		t.Fatalf("UpdateMerchantID() error: %v", err)
	}
}

func TestDeleteMerchantID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/merchantIds/m1" {
			t.Fatalf("expected path /v1/merchantIds/m1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteMerchantID(context.Background(), "m1"); err != nil {
		t.Fatalf("DeleteMerchantID() error: %v", err)
	}
}

func TestGetMerchantIDCertificates_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"c1","attributes":{"name":"Cert","certificateType":"APPLE_PAY"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/merchantIds/m1/certificates" {
			t.Fatalf("expected path /v1/merchantIds/m1/certificates, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[displayName]") != "Cert Name" {
			t.Fatalf("expected filter[displayName], got %q", values.Get("filter[displayName]"))
		}
		if values.Get("filter[certificateType]") != "PASS_TYPE_ID" {
			t.Fatalf("expected filter[certificateType], got %q", values.Get("filter[certificateType]"))
		}
		if values.Get("filter[serialNumber]") != "SN123" {
			t.Fatalf("expected filter[serialNumber], got %q", values.Get("filter[serialNumber]"))
		}
		if values.Get("filter[id]") != "c1" {
			t.Fatalf("expected filter[id]=c1, got %q", values.Get("filter[id]"))
		}
		if values.Get("sort") != "serialNumber" {
			t.Fatalf("expected sort=serialNumber, got %q", values.Get("sort"))
		}
		if values.Get("fields[certificates]") != "serialNumber" {
			t.Fatalf("expected fields[certificates], got %q", values.Get("fields[certificates]"))
		}
		if values.Get("fields[passTypeIds]") != "identifier" {
			t.Fatalf("expected fields[passTypeIds], got %q", values.Get("fields[passTypeIds]"))
		}
		if values.Get("include") != "passTypeId" {
			t.Fatalf("expected include=passTypeId, got %q", values.Get("include"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetMerchantIDCertificates(
		context.Background(),
		"m1",
		WithMerchantIDCertificatesFilterDisplayName("Cert Name"),
		WithMerchantIDCertificatesFilterCertificateTypes("PASS_TYPE_ID"),
		WithMerchantIDCertificatesFilterSerialNumbers("SN123"),
		WithMerchantIDCertificatesFilterIDs("c1"),
		WithMerchantIDCertificatesSort("serialNumber"),
		WithMerchantIDCertificatesFields([]string{"serialNumber"}),
		WithMerchantIDCertificatesPassTypeFields([]string{"identifier"}),
		WithMerchantIDCertificatesInclude([]string{"passTypeId"}),
		WithMerchantIDCertificatesLimit(5),
	); err != nil {
		t.Fatalf("GetMerchantIDCertificates() error: %v", err)
	}
}

func TestGetMerchantIDCertificatesRelationships_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"c1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/merchantIds/m1/relationships/certificates" {
			t.Fatalf("expected path /v1/merchantIds/m1/relationships/certificates, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetMerchantIDCertificatesRelationships(context.Background(), "m1", WithLinkagesLimit(5)); err != nil {
		t.Fatalf("GetMerchantIDCertificatesRelationships() error: %v", err)
	}
}

func TestGetBundleIDCapabilities_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"bundleIdCapabilities","id":"cap1","attributes":{"capabilityType":"ICLOUD"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/bundleIds/b1/bundleIdCapabilities" {
			t.Fatalf("expected path /v1/bundleIds/b1/bundleIdCapabilities, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBundleIDCapabilities(context.Background(), "b1"); err != nil {
		t.Fatalf("GetBundleIDCapabilities() error: %v", err)
	}
}

func TestCreateBundleIDCapability_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"bundleIdCapabilities","id":"cap1","attributes":{"capabilityType":"ICLOUD"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/bundleIdCapabilities" {
			t.Fatalf("expected path /v1/bundleIdCapabilities, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload BundleIDCapabilityCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeBundleIdCapabilities {
			t.Fatalf("expected type bundleIdCapabilities, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.CapabilityType != "ICLOUD" {
			t.Fatalf("expected capability ICLOUD, got %q", payload.Data.Attributes.CapabilityType)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.BundleID == nil {
			t.Fatalf("expected bundleId relationship")
		}
		if payload.Data.Relationships.BundleID.Data.ID != "b1" {
			t.Fatalf("expected bundleId b1, got %q", payload.Data.Relationships.BundleID.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	enabled := true
	attrs := BundleIDCapabilityCreateAttributes{
		CapabilityType: "ICLOUD",
		Settings: []CapabilitySetting{
			{
				Key: "ICLOUD_VERSION",
				Options: []CapabilityOption{
					{Key: "XCODE_13", Enabled: &enabled},
				},
			},
		},
	}
	if _, err := client.CreateBundleIDCapability(context.Background(), "b1", attrs); err != nil {
		t.Fatalf("CreateBundleIDCapability() error: %v", err)
	}
}

func TestDeleteBundleIDCapability_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/bundleIdCapabilities/cap1" {
			t.Fatalf("expected path /v1/bundleIdCapabilities/cap1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteBundleIDCapability(context.Background(), "cap1"); err != nil {
		t.Fatalf("DeleteBundleIDCapability() error: %v", err)
	}
}

func TestGetCertificates_WithFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"c1","attributes":{"name":"Cert","certificateType":"IOS_DISTRIBUTION"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/certificates" {
			t.Fatalf("expected path /v1/certificates, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[certificateType]") != "IOS_DISTRIBUTION,IOS_DEVELOPMENT" {
			t.Fatalf("expected filter[certificateType] to be set, got %q", values.Get("filter[certificateType]"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCertificates(
		context.Background(),
		WithCertificatesTypes([]string{"IOS_DISTRIBUTION", "IOS_DEVELOPMENT"}),
		WithCertificatesLimit(5),
	); err != nil {
		t.Fatalf("GetCertificates() error: %v", err)
	}
}

func TestGetCertificates_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/certificates?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCertificates(context.Background(), WithCertificatesNextURL(next)); err != nil {
		t.Fatalf("GetCertificates() error: %v", err)
	}
}

func TestCreateCertificate_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"certificates","id":"c1","attributes":{"name":"Cert","certificateType":"IOS_DISTRIBUTION"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/certificates" {
			t.Fatalf("expected path /v1/certificates, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload CertificateCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeCertificates {
			t.Fatalf("expected type certificates, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.CertificateType != "IOS_DISTRIBUTION" {
			t.Fatalf("expected certificate type IOS_DISTRIBUTION, got %q", payload.Data.Attributes.CertificateType)
		}
		if payload.Data.Attributes.CSRContent != "CSR_CONTENT" {
			t.Fatalf("expected csr content, got %q", payload.Data.Attributes.CSRContent)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateCertificate(context.Background(), "CSR_CONTENT", "IOS_DISTRIBUTION"); err != nil {
		t.Fatalf("CreateCertificate() error: %v", err)
	}
}

func TestGetCertificate_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"certificates","id":"c1","attributes":{"name":"Cert","certificateType":"IOS_DISTRIBUTION"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/certificates/c1" {
			t.Fatalf("expected path /v1/certificates/c1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCertificate(context.Background(), "c1"); err != nil {
		t.Fatalf("GetCertificate() error: %v", err)
	}
}

func TestGetCertificate_WithInclude(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"certificates","id":"c1","attributes":{"name":"Cert","certificateType":"PASS_TYPE_ID"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/certificates/c1" {
			t.Fatalf("expected path /v1/certificates/c1, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("include") != "passTypeId" {
			t.Fatalf("expected include=passTypeId, got %q", values.Get("include"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetCertificate(context.Background(), "c1", WithCertificatesInclude([]string{"passTypeId"})); err != nil {
		t.Fatalf("GetCertificate() error: %v", err)
	}
}

func TestRevokeCertificate_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/certificates/c1" {
			t.Fatalf("expected path /v1/certificates/c1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.RevokeCertificate(context.Background(), "c1"); err != nil {
		t.Fatalf("RevokeCertificate() error: %v", err)
	}
}

func TestGetDevices_WithFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"devices","id":"d1","attributes":{"name":"Device","udid":"UDID","platform":"IOS","status":"ENABLED"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/devices" {
			t.Fatalf("expected path /v1/devices, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[platform]") != "IOS,MAC_OS" {
			t.Fatalf("expected filter[platform] to be set, got %q", values.Get("filter[platform]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetDevices(context.Background(), WithDevicesPlatforms([]string{"IOS", "MAC_OS"})); err != nil {
		t.Fatalf("GetDevices() error: %v", err)
	}
}

func TestGetDevice_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"devices","id":"d1","attributes":{"name":"Device","udid":"UDID","platform":"IOS","status":"ENABLED"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/devices/d1" {
			t.Fatalf("expected path /v1/devices/d1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetDevice(context.Background(), "d1", nil); err != nil {
		t.Fatalf("GetDevice() error: %v", err)
	}
}

func TestRegisterDevice_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"devices","id":"d1","attributes":{"name":"Device","udid":"UDID","platform":"IOS","status":"ENABLED"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/devices" {
			t.Fatalf("expected path /v1/devices, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload DeviceCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeDevices {
			t.Fatalf("expected type devices, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.UDID != "UDID" {
			t.Fatalf("expected udid UDID, got %q", payload.Data.Attributes.UDID)
		}
		if payload.Data.Attributes.Platform != DevicePlatformIOS {
			t.Fatalf("expected platform IOS, got %q", payload.Data.Attributes.Platform)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := DeviceCreateAttributes{
		Name:     "Device",
		UDID:     "UDID",
		Platform: DevicePlatformIOS,
	}
	if _, err := client.RegisterDevice(context.Background(), attrs); err != nil {
		t.Fatalf("RegisterDevice() error: %v", err)
	}
}

func TestGetProfiles_WithFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"profiles","id":"p1","attributes":{"name":"Profile","profileType":"IOS_APP_DEVELOPMENT"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/profiles" {
			t.Fatalf("expected path /v1/profiles, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[profileType]") != "IOS_APP_DEVELOPMENT,IOS_APP_STORE" {
			t.Fatalf("expected filter[profileType] to be set, got %q", values.Get("filter[profileType]"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetProfiles(
		context.Background(),
		WithProfilesTypes([]string{"IOS_APP_DEVELOPMENT", "IOS_APP_STORE"}),
		WithProfilesLimit(5),
	); err != nil {
		t.Fatalf("GetProfiles() error: %v", err)
	}
}

func TestGetProfiles_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/profiles?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetProfiles(context.Background(), WithProfilesNextURL(next)); err != nil {
		t.Fatalf("GetProfiles() error: %v", err)
	}
}

func TestGetProfile_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"profiles","id":"p1","attributes":{"name":"Profile","profileType":"IOS_APP_DEVELOPMENT"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/profiles/p1" {
			t.Fatalf("expected path /v1/profiles/p1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetProfile(context.Background(), "p1"); err != nil {
		t.Fatalf("GetProfile() error: %v", err)
	}
}

func TestGetProfile_WithInclude(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"profiles","id":"p1","attributes":{"name":"Profile","profileType":"IOS_APP_DEVELOPMENT"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/profiles/p1" {
			t.Fatalf("expected path /v1/profiles/p1, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("include") != "bundleId,certificates" {
			t.Fatalf("expected include=bundleId,certificates, got %q", values.Get("include"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetProfile(context.Background(), "p1", WithProfilesInclude([]string{"bundleId", "certificates"})); err != nil {
		t.Fatalf("GetProfile() error: %v", err)
	}
}

func TestGetProfileBundleID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"bundleIds","id":"b1","attributes":{"name":"Bundle","identifier":"com.example"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/profiles/p1/bundleId" {
			t.Fatalf("expected path /v1/profiles/p1/bundleId, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetProfileBundleID(context.Background(), "p1"); err != nil {
		t.Fatalf("GetProfileBundleID() error: %v", err)
	}
}

func TestGetProfileCertificates_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"c1","attributes":{"name":"Cert","certificateType":"IOS_DISTRIBUTION"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/profiles/p1/certificates" {
			t.Fatalf("expected path /v1/profiles/p1/certificates, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetProfileCertificates(context.Background(), "p1", WithProfileCertificatesLimit(5)); err != nil {
		t.Fatalf("GetProfileCertificates() error: %v", err)
	}
}

func TestGetProfileDevices_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"devices","id":"d1","attributes":{"name":"Device","platform":"IOS","udid":"UDID","status":"ENABLED"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/profiles/p1/devices" {
			t.Fatalf("expected path /v1/profiles/p1/devices, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetProfileDevices(context.Background(), "p1", WithProfileDevicesLimit(10)); err != nil {
		t.Fatalf("GetProfileDevices() error: %v", err)
	}
}

func TestGetProfileBundleIDRelationship_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"bundleIds","id":"b1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/profiles/p1/relationships/bundleId" {
			t.Fatalf("expected path /v1/profiles/p1/relationships/bundleId, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetProfileBundleIDRelationship(context.Background(), "p1"); err != nil {
		t.Fatalf("GetProfileBundleIDRelationship() error: %v", err)
	}
}

func TestGetProfileCertificatesRelationships_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"certificates","id":"c1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/profiles/p1/relationships/certificates" {
			t.Fatalf("expected path /v1/profiles/p1/relationships/certificates, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "2" {
			t.Fatalf("expected limit=2, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetProfileCertificatesRelationships(context.Background(), "p1", WithLinkagesLimit(2)); err != nil {
		t.Fatalf("GetProfileCertificatesRelationships() error: %v", err)
	}
}

func TestGetProfileDevicesRelationships_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"devices","id":"d1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/profiles/p1/relationships/devices" {
			t.Fatalf("expected path /v1/profiles/p1/relationships/devices, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "3" {
			t.Fatalf("expected limit=3, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetProfileDevicesRelationships(context.Background(), "p1", WithLinkagesLimit(3)); err != nil {
		t.Fatalf("GetProfileDevicesRelationships() error: %v", err)
	}
}

func TestCreateProfile_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"profiles","id":"p1","attributes":{"name":"Profile","profileType":"IOS_APP_DEVELOPMENT"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/profiles" {
			t.Fatalf("expected path /v1/profiles, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload ProfileCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeProfiles {
			t.Fatalf("expected type profiles, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.ProfileType != "IOS_APP_DEVELOPMENT" {
			t.Fatalf("expected profile type IOS_APP_DEVELOPMENT, got %q", payload.Data.Attributes.ProfileType)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.BundleID == nil {
			t.Fatalf("expected bundleId relationship")
		}
		if payload.Data.Relationships.BundleID.Data.ID != "b1" {
			t.Fatalf("expected bundleId b1, got %q", payload.Data.Relationships.BundleID.Data.ID)
		}
		if payload.Data.Relationships.Certificates == nil || len(payload.Data.Relationships.Certificates.Data) != 2 {
			t.Fatalf("expected 2 certificate relationships")
		}
		if payload.Data.Relationships.Devices == nil || len(payload.Data.Relationships.Devices.Data) != 1 {
			t.Fatalf("expected 1 device relationship")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := ProfileCreateAttributes{
		Name:        "Profile",
		ProfileType: "IOS_APP_DEVELOPMENT",
	}
	if _, err := client.CreateProfile(context.Background(), attrs, "b1", []string{"c1", "c2"}, []string{"d1"}); err != nil {
		t.Fatalf("CreateProfile() error: %v", err)
	}
}

func TestDeleteProfile_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/profiles/p1" {
			t.Fatalf("expected path /v1/profiles/p1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteProfile(context.Background(), "p1"); err != nil {
		t.Fatalf("DeleteProfile() error: %v", err)
	}
}

func TestGetInAppPurchaseV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"CONSUMABLE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/inAppPurchases/iap-1" {
			t.Fatalf("expected path /v2/inAppPurchases/iap-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseV2(context.Background(), "iap-1"); err != nil {
		t.Fatalf("GetInAppPurchaseV2() error: %v", err)
	}
}

func TestCreateInAppPurchaseV2(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"CONSUMABLE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/inAppPurchases" {
			t.Fatalf("expected path /v2/inAppPurchases, got %s", req.URL.Path)
		}
		var payload InAppPurchaseV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeInAppPurchases {
			t.Fatalf("expected type inAppPurchases, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Name != "Pro" || payload.Data.Attributes.ProductID != "com.example.pro" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship")
		}
		if payload.Data.Relationships.App.Data.Type != ResourceTypeApps || payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data.Relationships.App.Data)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := InAppPurchaseV2CreateAttributes{
		Name:              "Pro",
		ProductID:         "com.example.pro",
		InAppPurchaseType: "CONSUMABLE",
	}
	if _, err := client.CreateInAppPurchaseV2(context.Background(), "app-1", attrs); err != nil {
		t.Fatalf("CreateInAppPurchaseV2() error: %v", err)
	}
}

func TestUpdateInAppPurchaseV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"CONSUMABLE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v2/inAppPurchases/iap-1" {
			t.Fatalf("expected path /v2/inAppPurchases/iap-1, got %s", req.URL.Path)
		}
		var payload InAppPurchaseV2UpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "iap-1" || payload.Data.Type != ResourceTypeInAppPurchases {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name == nil || *payload.Data.Attributes.Name != "New Name" {
			t.Fatalf("expected name update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	name := "New Name"
	attrs := InAppPurchaseV2UpdateAttributes{Name: &name}
	if _, err := client.UpdateInAppPurchaseV2(context.Background(), "iap-1", attrs); err != nil {
		t.Fatalf("UpdateInAppPurchaseV2() error: %v", err)
	}
}

func TestDeleteInAppPurchaseV2(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v2/inAppPurchases/iap-1" {
			t.Fatalf("expected path /v2/inAppPurchases/iap-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteInAppPurchaseV2(context.Background(), "iap-1"); err != nil {
		t.Fatalf("DeleteInAppPurchaseV2() error: %v", err)
	}
}

func TestGetInAppPurchaseLocalizations_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"inAppPurchaseLocalizations","id":"loc-1","attributes":{"name":"Pro","locale":"en-US"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/inAppPurchases/iap-1/inAppPurchaseLocalizations" {
			t.Fatalf("expected path /v2/inAppPurchases/iap-1/inAppPurchaseLocalizations, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetInAppPurchaseLocalizations(context.Background(), "iap-1", WithIAPLocalizationsLimit(5)); err != nil {
		t.Fatalf("GetInAppPurchaseLocalizations() error: %v", err)
	}
}

func TestGetSubscriptionGroups_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/subscriptionGroups" {
			t.Fatalf("expected path /v1/apps/app-1/subscriptionGroups, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "20" {
			t.Fatalf("expected limit=20, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionGroups(context.Background(), "app-1", WithSubscriptionGroupsLimit(20)); err != nil {
		t.Fatalf("GetSubscriptionGroups() error: %v", err)
	}
}

func TestGetSubscriptionGroups_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/apps/app-1/subscriptionGroups?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionGroups(context.Background(), "app-1", WithSubscriptionGroupsNextURL(next)); err != nil {
		t.Fatalf("GetSubscriptionGroups() error: %v", err)
	}
}

func TestCreateSubscriptionGroup(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionGroups" {
			t.Fatalf("expected path /v1/subscriptionGroups, got %s", req.URL.Path)
		}
		var payload SubscriptionGroupCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionGroups {
			t.Fatalf("expected type subscriptionGroups, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.ReferenceName != "Premium" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship")
		}
		if payload.Data.Relationships.App.Data.Type != ResourceTypeApps || payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data.Relationships.App.Data)
		}
		assertAuthorized(t, req)
	}, response)

	subAttrs := SubscriptionGroupCreateAttributes{ReferenceName: "Premium"}
	if _, err := client.CreateSubscriptionGroup(context.Background(), "app-1", subAttrs); err != nil {
		t.Fatalf("CreateSubscriptionGroup() error: %v", err)
	}
}

func TestUpdateSubscriptionGroup(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionGroups/group-1" {
			t.Fatalf("expected path /v1/subscriptionGroups/group-1, got %s", req.URL.Path)
		}
		var payload SubscriptionGroupUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "group-1" || payload.Data.Type != ResourceTypeSubscriptionGroups {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes.ReferenceName == nil || *payload.Data.Attributes.ReferenceName != "Updated" {
			t.Fatalf("expected reference name update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	refName := "Updated"
	updateAttrs := SubscriptionGroupUpdateAttributes{ReferenceName: &refName}
	if _, err := client.UpdateSubscriptionGroup(context.Background(), "group-1", updateAttrs); err != nil {
		t.Fatalf("UpdateSubscriptionGroup() error: %v", err)
	}
}

func TestGetSubscriptions_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.sub.monthly"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionGroups/group-1/subscriptions" {
			t.Fatalf("expected path /v1/subscriptionGroups/group-1/subscriptions, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptions(context.Background(), "group-1", WithSubscriptionsLimit(5)); err != nil {
		t.Fatalf("GetSubscriptions() error: %v", err)
	}
}

func TestCreateSubscription(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.sub.monthly"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptions" {
			t.Fatalf("expected path /v1/subscriptions, got %s", req.URL.Path)
		}
		var payload SubscriptionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptions {
			t.Fatalf("expected type subscriptions, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Name != "Monthly" || payload.Data.Attributes.ProductID != "com.example.sub.monthly" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Group == nil {
			t.Fatalf("expected group relationship")
		}
		if payload.Data.Relationships.Group.Data.Type != ResourceTypeSubscriptionGroups || payload.Data.Relationships.Group.Data.ID != "group-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data.Relationships.Group.Data)
		}
		assertAuthorized(t, req)
	}, response)

	createSubAttrs := SubscriptionCreateAttributes{
		Name:      "Monthly",
		ProductID: "com.example.sub.monthly",
	}
	if _, err := client.CreateSubscription(context.Background(), "group-1", createSubAttrs); err != nil {
		t.Fatalf("CreateSubscription() error: %v", err)
	}
}

func TestCreateSubscriptionPrice(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionPrices","id":"price-1","attributes":{"startDate":"2026-01-01","preserved":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionPrices" {
			t.Fatalf("expected path /v1/subscriptionPrices, got %s", req.URL.Path)
		}
		var payload SubscriptionPriceCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionPrices {
			t.Fatalf("expected type subscriptionPrices, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Subscription == nil || payload.Data.Relationships.SubscriptionPricePoint == nil {
			t.Fatalf("expected subscription and price point relationships")
		}
		if payload.Data.Relationships.Subscription.Data.ID != "sub-1" || payload.Data.Relationships.SubscriptionPricePoint.Data.ID != "price-point-1" {
			t.Fatalf("unexpected relationships: %+v", payload.Data.Relationships)
		}
		assertAuthorized(t, req)
	}, response)

	preserved := true
	priceAttrs := SubscriptionPriceCreateAttributes{
		StartDate: "2026-01-01",
		Preserved: &preserved,
	}
	if _, err := client.CreateSubscriptionPrice(context.Background(), "sub-1", "price-point-1", priceAttrs); err != nil {
		t.Fatalf("CreateSubscriptionPrice() error: %v", err)
	}
}

func TestCreateSubscriptionAvailability(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionAvailabilities" {
			t.Fatalf("expected path /v1/subscriptionAvailabilities, got %s", req.URL.Path)
		}
		var payload SubscriptionAvailabilityCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionAvailabilities {
			t.Fatalf("expected type subscriptionAvailabilities, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Subscription == nil || payload.Data.Relationships.AvailableTerritories == nil {
			t.Fatalf("expected subscription and territory relationships")
		}
		if payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
			t.Fatalf("unexpected subscription relationship: %+v", payload.Data.Relationships.Subscription.Data)
		}
		if len(payload.Data.Relationships.AvailableTerritories.Data) != 2 {
			t.Fatalf("expected 2 territories, got %d", len(payload.Data.Relationships.AvailableTerritories.Data))
		}
		assertAuthorized(t, req)
	}, response)

	availAttrs := SubscriptionAvailabilityAttributes{AvailableInNewTerritories: true}
	if _, err := client.CreateSubscriptionAvailability(context.Background(), "sub-1", []string{"USA", "CAN"}, availAttrs); err != nil {
		t.Fatalf("CreateSubscriptionAvailability() error: %v", err)
	}
}

// User management tests
func TestGetUsers_WithFiltersAndLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"users","id":"user-1","attributes":{"username":"user@example.com","firstName":"Jane","lastName":"Doe","roles":["ADMIN"],"allAppsVisible":true,"provisioningAllowed":false}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/users" {
			t.Fatalf("expected path /v1/users, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[username]") != "user@example.com" {
			t.Fatalf("expected filter[username]=user@example.com, got %q", values.Get("filter[username]"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetUsers(context.Background(), WithUsersEmail("user@example.com"), WithUsersLimit(5)); err != nil {
		t.Fatalf("GetUsers() error: %v", err)
	}
}

func TestGetUsers_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/users?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetUsers(context.Background(), WithUsersNextURL(next)); err != nil {
		t.Fatalf("GetUsers() error: %v", err)
	}
}

func TestUpdateUser_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"users","id":"user-1","attributes":{"username":"user@example.com","roles":["ADMIN"],"allAppsVisible":false,"provisioningAllowed":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/users/user-1" {
			t.Fatalf("expected path /v1/users/user-1, got %s", req.URL.Path)
		}
		var payload UserUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeUsers {
			t.Fatalf("expected type users, got %s", payload.Data.Type)
		}
		if payload.Data.ID != "user-1" {
			t.Fatalf("expected id user-1, got %s", payload.Data.ID)
		}
		if payload.Data.Attributes == nil {
			t.Fatalf("expected attributes to be set")
		}
		if len(payload.Data.Attributes.Roles) != 1 || payload.Data.Attributes.Roles[0] != "ADMIN" {
			t.Fatalf("unexpected roles: %+v", payload.Data.Attributes.Roles)
		}
		if payload.Data.Attributes.AllAppsVisible == nil || *payload.Data.Attributes.AllAppsVisible {
			t.Fatalf("expected allAppsVisible=false, got %+v", payload.Data.Attributes.AllAppsVisible)
		}
		assertAuthorized(t, req)
	}, response)

	allAppsVisible := false
	if _, err := client.UpdateUser(context.Background(), "user-1", UserUpdateAttributes{
		Roles:          []string{"ADMIN"},
		AllAppsVisible: &allAppsVisible,
	}); err != nil {
		t.Fatalf("UpdateUser() error: %v", err)
	}
}

func TestCreateUserInvitation_WithVisibleApps(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"userInvitations","id":"invite-1","attributes":{"email":"user@example.com","roles":["ADMIN"],"allAppsVisible":false,"provisioningAllowed":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/userInvitations" {
			t.Fatalf("expected path /v1/userInvitations, got %s", req.URL.Path)
		}
		var payload UserInvitationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeUserInvitations {
			t.Fatalf("expected type userInvitations, got %s", payload.Data.Type)
		}
		if payload.Data.Attributes.Email != "user@example.com" {
			t.Fatalf("expected email user@example.com, got %s", payload.Data.Attributes.Email)
		}
		if len(payload.Data.Attributes.Roles) != 1 || payload.Data.Attributes.Roles[0] != "ADMIN" {
			t.Fatalf("unexpected roles: %+v", payload.Data.Attributes.Roles)
		}
		if payload.Data.Attributes.AllAppsVisible == nil || *payload.Data.Attributes.AllAppsVisible {
			t.Fatalf("expected allAppsVisible=false, got %+v", payload.Data.Attributes.AllAppsVisible)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.VisibleApps == nil {
			t.Fatalf("expected visibleApps relationships")
		}
		if len(payload.Data.Relationships.VisibleApps.Data) != 2 {
			t.Fatalf("expected 2 visibleApps relationships, got %d", len(payload.Data.Relationships.VisibleApps.Data))
		}
		if payload.Data.Relationships.VisibleApps.Data[0].Type != ResourceTypeApps {
			t.Fatalf("unexpected relationship type: %s", payload.Data.Relationships.VisibleApps.Data[0].Type)
		}
		assertAuthorized(t, req)
	}, response)

	userAllAppsVisible := false
	userAttrs := UserInvitationCreateAttributes{
		Email:          "user@example.com",
		Roles:          []string{"ADMIN"},
		AllAppsVisible: &userAllAppsVisible,
	}
	if _, err := client.CreateUserInvitation(context.Background(), userAttrs, []string{"app-1", "app-2"}); err != nil {
		t.Fatalf("CreateUserInvitation() error: %v", err)
	}
}

func TestGetUser_WithInclude(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"users","id":"user-1","attributes":{"username":"user@example.com","firstName":"Jane","lastName":"Doe","roles":["ADMIN"],"allAppsVisible":true,"provisioningAllowed":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/users/user-1" {
			t.Fatalf("expected path /v1/users/user-1, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("include") != "visibleApps" {
			t.Fatalf("expected include=visibleApps, got %q", req.URL.Query().Get("include"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetUser(context.Background(), "user-1", WithUsersInclude([]string{"visibleApps"})); err != nil {
		t.Fatalf("GetUser() error: %v", err)
	}
}

func TestGetUserVisibleApps_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"app-1","attributes":{"name":"Demo","bundleId":"com.example.demo","sku":"SKU1"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/users/user-1/visibleApps" {
			t.Fatalf("expected path /v1/users/user-1/visibleApps, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetUserVisibleApps(context.Background(), "user-1"); err != nil {
		t.Fatalf("GetUserVisibleApps() error: %v", err)
	}
}

func TestGetUserVisibleAppsRelationships_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"apps","id":"app-1"}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/users/user-1/relationships/visibleApps" {
			t.Fatalf("expected path /v1/users/user-1/relationships/visibleApps, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "7" {
			t.Fatalf("expected limit=7, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetUserVisibleAppsRelationships(context.Background(), "user-1", WithLinkagesLimit(7)); err != nil {
		t.Fatalf("GetUserVisibleAppsRelationships() error: %v", err)
	}
}

func TestAddUserVisibleApps_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/users/user-1/relationships/visibleApps" {
			t.Fatalf("expected path /v1/users/user-1/relationships/visibleApps, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 relationships, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeApps || payload.Data[0].ID != "app-1" {
			t.Fatalf("unexpected relationship[0]: %+v", payload.Data[0])
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.AddUserVisibleApps(context.Background(), "user-1", []string{"app-1", "app-2"}); err != nil {
		t.Fatalf("AddUserVisibleApps() error: %v", err)
	}
}

func TestRemoveUserVisibleApps_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/users/user-1/relationships/visibleApps" {
			t.Fatalf("expected path /v1/users/user-1/relationships/visibleApps, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.Data) != 1 {
			t.Fatalf("expected 1 relationship, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeApps || payload.Data[0].ID != "app-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data[0])
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.RemoveUserVisibleApps(context.Background(), "user-1", []string{"app-1"}); err != nil {
		t.Fatalf("RemoveUserVisibleApps() error: %v", err)
	}
}

func TestSetUserVisibleApps_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, ``)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/users/user-1/relationships/visibleApps" {
			t.Fatalf("expected path /v1/users/user-1/relationships/visibleApps, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 relationships, got %d", len(payload.Data))
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.SetUserVisibleApps(context.Background(), "user-1", []string{"app-1", "app-2"}); err != nil {
		t.Fatalf("SetUserVisibleApps() error: %v", err)
	}
}

func TestGetBetaAppReviewDetails_WithAppFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaAppReviewDetails","id":"detail-1","attributes":{"contactEmail":"dev@example.com"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppReviewDetails" {
			t.Fatalf("expected path /v1/betaAppReviewDetails, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[app]") != "app-1" {
			t.Fatalf("expected filter[app]=app-1, got %q", values.Get("filter[app]"))
		}
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaAppReviewDetails(context.Background(), "app-1", WithBetaAppReviewDetailsLimit(10)); err != nil {
		t.Fatalf("GetBetaAppReviewDetails() error: %v", err)
	}
}

func TestGetBetaAppReviewDetails_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/betaAppReviewDetails?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaAppReviewDetails(context.Background(), "app-1", WithBetaAppReviewDetailsNextURL(next)); err != nil {
		t.Fatalf("GetBetaAppReviewDetails() error: %v", err)
	}
}

func TestGetBetaAppReviewDetail(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaAppReviewDetails","id":"detail-1","attributes":{"contactEmail":"dev@example.com"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppReviewDetails/detail-1" {
			t.Fatalf("expected path /v1/betaAppReviewDetails/detail-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaAppReviewDetail(context.Background(), "detail-1"); err != nil {
		t.Fatalf("GetBetaAppReviewDetail() error: %v", err)
	}
}

func TestUpdateBetaAppReviewDetail_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaAppReviewDetails","id":"detail-1","attributes":{"contactEmail":"dev@example.com"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppReviewDetails/detail-1" {
			t.Fatalf("expected path /v1/betaAppReviewDetails/detail-1, got %s", req.URL.Path)
		}
		var payload BetaAppReviewDetailUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeBetaAppReviewDetails {
			t.Fatalf("expected type betaAppReviewDetails, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "detail-1" {
			t.Fatalf("expected id detail-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.ContactEmail == nil {
			t.Fatalf("expected contact email attribute")
		}
		assertAuthorized(t, req)
	}, response)

	email := "dev@example.com"
	if _, err := client.UpdateBetaAppReviewDetail(context.Background(), "detail-1", BetaAppReviewDetailUpdateAttributes{
		ContactEmail: &email,
	}); err != nil {
		t.Fatalf("UpdateBetaAppReviewDetail() error: %v", err)
	}
}

func TestGetBetaAppReviewSubmissions_WithBuildFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaAppReviewSubmissions","id":"submission-1","attributes":{"betaReviewState":"IN_REVIEW"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppReviewSubmissions" {
			t.Fatalf("expected path /v1/betaAppReviewSubmissions, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[build]") != "build-1" {
			t.Fatalf("expected filter[build]=build-1, got %q", values.Get("filter[build]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaAppReviewSubmissions(context.Background(), WithBetaAppReviewSubmissionsBuildIDs([]string{"build-1"})); err != nil {
		t.Fatalf("GetBetaAppReviewSubmissions() error: %v", err)
	}
}

func TestCreateBetaAppReviewSubmission_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"betaAppReviewSubmissions","id":"submission-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppReviewSubmissions" {
			t.Fatalf("expected path /v1/betaAppReviewSubmissions, got %s", req.URL.Path)
		}
		var payload BetaAppReviewSubmissionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeBetaAppReviewSubmissions {
			t.Fatalf("expected type betaAppReviewSubmissions, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Build == nil {
			t.Fatalf("expected build relationship")
		}
		if payload.Data.Relationships.Build.Data.ID != "build-1" {
			t.Fatalf("expected build id build-1, got %q", payload.Data.Relationships.Build.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateBetaAppReviewSubmission(context.Background(), "build-1"); err != nil {
		t.Fatalf("CreateBetaAppReviewSubmission() error: %v", err)
	}
}

func TestGetBetaAppReviewSubmission(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaAppReviewSubmissions","id":"submission-1","attributes":{"betaReviewState":"IN_REVIEW"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaAppReviewSubmissions/submission-1" {
			t.Fatalf("expected path /v1/betaAppReviewSubmissions/submission-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaAppReviewSubmission(context.Background(), "submission-1"); err != nil {
		t.Fatalf("GetBetaAppReviewSubmission() error: %v", err)
	}
}

func TestGetBuildBetaDetails_WithBuildFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildBetaDetails" {
			t.Fatalf("expected path /v1/buildBetaDetails, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[build]") != "build-1" {
			t.Fatalf("expected filter[build]=build-1, got %q", values.Get("filter[build]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBuildBetaDetails(context.Background(), WithBuildBetaDetailsBuildIDs([]string{"build-1"})); err != nil {
		t.Fatalf("GetBuildBetaDetails() error: %v", err)
	}
}

func TestGetBuildBetaDetail(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildBetaDetails/detail-1" {
			t.Fatalf("expected path /v1/buildBetaDetails/detail-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBuildBetaDetail(context.Background(), "detail-1"); err != nil {
		t.Fatalf("GetBuildBetaDetail() error: %v", err)
	}
}

func TestUpdateBuildBetaDetail_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/buildBetaDetails/detail-1" {
			t.Fatalf("expected path /v1/buildBetaDetails/detail-1, got %s", req.URL.Path)
		}
		var payload BuildBetaDetailUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeBuildBetaDetails {
			t.Fatalf("expected type buildBetaDetails, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "detail-1" {
			t.Fatalf("expected id detail-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.AutoNotifyEnabled == nil {
			t.Fatalf("expected autoNotifyEnabled attribute")
		}
		assertAuthorized(t, req)
	}, response)

	enabled := true
	if _, err := client.UpdateBuildBetaDetail(context.Background(), "detail-1", BuildBetaDetailUpdateAttributes{
		AutoNotifyEnabled: &enabled,
	}); err != nil {
		t.Fatalf("UpdateBuildBetaDetail() error: %v", err)
	}
}

func TestGetBetaRecruitmentCriterionOptions_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaRecruitmentCriterionOptions","id":"opt-1","attributes":{"identifier":"OPTION_1","name":"Option 1"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaRecruitmentCriterionOptions" {
			t.Fatalf("expected path /v1/betaRecruitmentCriterionOptions, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaRecruitmentCriterionOptions(context.Background(), WithBetaRecruitmentCriterionOptionsLimit(5)); err != nil {
		t.Fatalf("GetBetaRecruitmentCriterionOptions() error: %v", err)
	}
}

func TestCreateBetaRecruitmentCriteria_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"betaRecruitmentCriteria","id":"criteria-1","attributes":{"lastModifiedDate":"2026-01-21T00:00:00Z"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaRecruitmentCriteria" {
			t.Fatalf("expected path /v1/betaRecruitmentCriteria, got %s", req.URL.Path)
		}
		var payload BetaRecruitmentCriteriaCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeBetaRecruitmentCriteria {
			t.Fatalf("expected type betaRecruitmentCriteria, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.BetaGroup == nil {
			t.Fatalf("expected betaGroup relationship")
		}
		if payload.Data.Relationships.BetaGroup.Data.ID != "group-1" {
			t.Fatalf("expected group id group-1, got %q", payload.Data.Relationships.BetaGroup.Data.ID)
		}
		if payload.Data.Relationships.BetaRecruitmentCriterionOptions == nil || len(payload.Data.Relationships.BetaRecruitmentCriterionOptions.Data) != 2 {
			t.Fatalf("expected 2 option relationships")
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateBetaRecruitmentCriteria(context.Background(), "group-1", []string{"opt-1", "opt-2"}); err != nil {
		t.Fatalf("CreateBetaRecruitmentCriteria() error: %v", err)
	}
}

func TestUpdateBetaRecruitmentCriteria_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"betaRecruitmentCriteria","id":"criteria-1","attributes":{"lastModifiedDate":"2026-01-21T00:00:00Z"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaRecruitmentCriteria/criteria-1" {
			t.Fatalf("expected path /v1/betaRecruitmentCriteria/criteria-1, got %s", req.URL.Path)
		}
		var payload BetaRecruitmentCriteriaUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeBetaRecruitmentCriteria {
			t.Fatalf("expected type betaRecruitmentCriteria, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "criteria-1" {
			t.Fatalf("expected id criteria-1, got %q", payload.Data.ID)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.BetaRecruitmentCriterionOptions == nil {
			t.Fatalf("expected option relationships")
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.UpdateBetaRecruitmentCriteria(context.Background(), "criteria-1", []string{"opt-1"}); err != nil {
		t.Fatalf("UpdateBetaRecruitmentCriteria() error: %v", err)
	}
}

func TestDeleteBetaRecruitmentCriteria_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaRecruitmentCriteria/criteria-1" {
			t.Fatalf("expected path /v1/betaRecruitmentCriteria/criteria-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteBetaRecruitmentCriteria(context.Background(), "criteria-1"); err != nil {
		t.Fatalf("DeleteBetaRecruitmentCriteria() error: %v", err)
	}
}

func TestGetBetaGroupPublicLinkUsages(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaGroupMetrics","id":"metric-1","attributes":{"installCount":5}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/group-1/metrics/publicLinkUsages" {
			t.Fatalf("expected path /v1/betaGroups/group-1/metrics/publicLinkUsages, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaGroupPublicLinkUsages(context.Background(), "group-1"); err != nil {
		t.Fatalf("GetBetaGroupPublicLinkUsages() error: %v", err)
	}
}

func TestGetBetaGroupTesterUsages(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"betaGroupMetrics","id":"metric-1","attributes":{"testerCount":12}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/betaGroups/group-1/metrics/betaTesterUsages" {
			t.Fatalf("expected path /v1/betaGroups/group-1/metrics/betaTesterUsages, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("groupBy") != "betaTesters" {
			t.Fatalf("expected groupBy=betaTesters, got %q", req.URL.Query().Get("groupBy"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetBetaGroupTesterUsages(context.Background(), "group-1"); err != nil {
		t.Fatalf("GetBetaGroupTesterUsages() error: %v", err)
	}
}

func TestGetDevices_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"devices","id":"device-1","attributes":{"udid":"UDID1","platform":"IOS","status":"ENABLED"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/devices" {
			t.Fatalf("expected path /v1/devices, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[udid]") != "UDID1,UDID2" {
			t.Fatalf("expected filter[udid]=UDID1,UDID2, got %q", values.Get("filter[udid]"))
		}
		if values.Get("filter[platform]") != "IOS" {
			t.Fatalf("expected filter[platform]=IOS, got %q", values.Get("filter[platform]"))
		}
		if values.Get("filter[status]") != "ENABLED" {
			t.Fatalf("expected filter[status]=ENABLED, got %q", values.Get("filter[status]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetDevices(context.Background(),
		WithDevicesFilterUDIDs([]string{"UDID1", "UDID2"}),
		WithDevicesFilterPlatforms([]string{"ios"}),
		WithDevicesFilterStatuses([]string{"enabled"}),
	); err != nil {
		t.Fatalf("GetDevices() error: %v", err)
	}
}

func TestGetDevices_WithFiltersAndLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"devices","id":"device-1","attributes":{"name":"My iPhone","platform":"IOS","udid":"UDID-1","status":"ENABLED"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/devices" {
			t.Fatalf("expected path /v1/devices, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[name]") != "My iPhone" {
			t.Fatalf("expected filter[name]=My iPhone, got %q", values.Get("filter[name]"))
		}
		if values.Get("filter[platform]") != "IOS" {
			t.Fatalf("expected filter[platform]=IOS, got %q", values.Get("filter[platform]"))
		}
		if values.Get("filter[status]") != "ENABLED" {
			t.Fatalf("expected filter[status]=ENABLED, got %q", values.Get("filter[status]"))
		}
		if values.Get("filter[udid]") != "UDID-1,UDID-2" {
			t.Fatalf("expected filter[udid] CSV, got %q", values.Get("filter[udid]"))
		}
		if values.Get("filter[id]") != "device-1" {
			t.Fatalf("expected filter[id]=device-1, got %q", values.Get("filter[id]"))
		}
		if values.Get("sort") != "-name" {
			t.Fatalf("expected sort=-name, got %q", values.Get("sort"))
		}
		if values.Get("fields[devices]") != "name,udid" {
			t.Fatalf("expected fields[devices]=name,udid, got %q", values.Get("fields[devices]"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetDevices(context.Background(),
		WithDevicesNames([]string{"My iPhone"}),
		WithDevicesPlatform("IOS"),
		WithDevicesStatus("ENABLED"),
		WithDevicesUDIDs([]string{"UDID-1", "UDID-2"}),
		WithDevicesIDs([]string{"device-1"}),
		WithDevicesSort("-name"),
		WithDevicesFields([]string{"name", "udid"}),
		WithDevicesLimit(5),
	); err != nil {
		t.Fatalf("GetDevices() error: %v", err)
	}
}

func TestGetDevices_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/devices?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetDevices(context.Background(), WithDevicesNextURL(next)); err != nil {
		t.Fatalf("GetDevices() error: %v", err)
	}
}

func TestGetDevice_WithFields(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"devices","id":"device-1","attributes":{"name":"My iPhone","platform":"IOS","udid":"UDID-1","status":"ENABLED"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/devices/device-1" {
			t.Fatalf("expected path /v1/devices/device-1, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("fields[devices]") != "name,udid" {
			t.Fatalf("expected fields[devices]=name,udid, got %q", req.URL.Query().Get("fields[devices]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetDevice(context.Background(), "device-1", []string{"name", "udid"}); err != nil {
		t.Fatalf("GetDevice() error: %v", err)
	}
}

func TestCreateDevice_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"devices","id":"device-1","attributes":{"name":"My iPhone","platform":"IOS","udid":"UDID-1","status":"ENABLED"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/devices" {
			t.Fatalf("expected path /v1/devices, got %s", req.URL.Path)
		}
		var payload DeviceCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeDevices {
			t.Fatalf("expected type devices, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Name != "My iPhone" {
			t.Fatalf("expected name My iPhone, got %q", payload.Data.Attributes.Name)
		}
		if payload.Data.Attributes.UDID != "UDID-1" {
			t.Fatalf("expected udid UDID-1, got %q", payload.Data.Attributes.UDID)
		}
		if payload.Data.Attributes.Platform != DevicePlatformIOS {
			t.Fatalf("expected platform IOS, got %q", payload.Data.Attributes.Platform)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateDevice(context.Background(), DeviceCreateAttributes{
		Name:     "My iPhone",
		UDID:     "UDID-1",
		Platform: DevicePlatformIOS,
	}); err != nil {
		t.Fatalf("CreateDevice() error: %v", err)
	}
}

func TestUpdateDevice_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"devices","id":"device-1","attributes":{"name":"Updated iPhone","status":"DISABLED"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/devices/device-1" {
			t.Fatalf("expected path /v1/devices/device-1, got %s", req.URL.Path)
		}
		var payload DeviceUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeDevices {
			t.Fatalf("expected type devices, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "device-1" {
			t.Fatalf("expected id device-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil {
			t.Fatalf("expected attributes to be set")
		}
		if payload.Data.Attributes.Name == nil || *payload.Data.Attributes.Name != "Updated iPhone" {
			t.Fatalf("expected name Updated iPhone, got %+v", payload.Data.Attributes.Name)
		}
		if payload.Data.Attributes.Status == nil || *payload.Data.Attributes.Status != DeviceStatusDisabled {
			t.Fatalf("expected status DISABLED, got %+v", payload.Data.Attributes.Status)
		}
		assertAuthorized(t, req)
	}, response)

	status := DeviceStatusDisabled
	name := "Updated iPhone"
	if _, err := client.UpdateDevice(context.Background(), "device-1", DeviceUpdateAttributes{
		Name:   &name,
		Status: &status,
	}); err != nil {
		t.Fatalf("UpdateDevice() error: %v", err)
	}
}

func TestGetAccessibilityDeclarations_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"accessibilityDeclarations","id":"decl-1","attributes":{"deviceFamily":"IPHONE","state":"DRAFT"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/accessibilityDeclarations" {
			t.Fatalf("expected path /v1/apps/app-1/accessibilityDeclarations, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[deviceFamily]") != "IPHONE,IPAD" {
			t.Fatalf("expected filter[deviceFamily]=IPHONE,IPAD, got %q", values.Get("filter[deviceFamily]"))
		}
		if values.Get("filter[state]") != "DRAFT" {
			t.Fatalf("expected filter[state]=DRAFT, got %q", values.Get("filter[state]"))
		}
		if values.Get("fields[accessibilityDeclarations]") != "deviceFamily,state" {
			t.Fatalf("expected fields[accessibilityDeclarations]=deviceFamily,state, got %q", values.Get("fields[accessibilityDeclarations]"))
		}
		if values.Get("limit") != "2" {
			t.Fatalf("expected limit=2, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAccessibilityDeclarations(
		context.Background(),
		"app-1",
		WithAccessibilityDeclarationsDeviceFamilies([]string{"iphone", "ipad"}),
		WithAccessibilityDeclarationsStates([]string{"draft"}),
		WithAccessibilityDeclarationsFields([]string{"deviceFamily", "state"}),
		WithAccessibilityDeclarationsLimit(2),
	); err != nil {
		t.Fatalf("GetAccessibilityDeclarations() error: %v", err)
	}
}

func TestGetAccessibilityDeclaration_WithFields(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"accessibilityDeclarations","id":"decl-1","attributes":{"deviceFamily":"IPHONE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/accessibilityDeclarations/decl-1" {
			t.Fatalf("expected path /v1/accessibilityDeclarations/decl-1, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("fields[accessibilityDeclarations]") != "deviceFamily,state" {
			t.Fatalf("expected fields[accessibilityDeclarations]=deviceFamily,state, got %q", req.URL.Query().Get("fields[accessibilityDeclarations]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAccessibilityDeclaration(context.Background(), "decl-1", []string{"deviceFamily", "state"}); err != nil {
		t.Fatalf("GetAccessibilityDeclaration() error: %v", err)
	}
}

func TestCreateAccessibilityDeclaration(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"accessibilityDeclarations","id":"decl-1","attributes":{"deviceFamily":"IPHONE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/accessibilityDeclarations" {
			t.Fatalf("expected path /v1/accessibilityDeclarations, got %s", req.URL.Path)
		}
		var payload AccessibilityDeclarationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeAccessibilityDeclarations {
			t.Fatalf("expected type accessibilityDeclarations, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.DeviceFamily != DeviceFamilyIPhone {
			t.Fatalf("expected device family IPHONE, got %q", payload.Data.Attributes.DeviceFamily)
		}
		if payload.Data.Attributes.SupportsVoiceover == nil || !*payload.Data.Attributes.SupportsVoiceover {
			t.Fatalf("expected supportsVoiceover true, got %+v", payload.Data.Attributes.SupportsVoiceover)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship")
		}
		if payload.Data.Relationships.App.Data.Type != ResourceTypeApps || payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data.Relationships.App.Data)
		}
		assertAuthorized(t, req)
	}, response)

	supportsVoiceover := true
	attrs := AccessibilityDeclarationCreateAttributes{
		DeviceFamily:      DeviceFamilyIPhone,
		SupportsVoiceover: &supportsVoiceover,
	}
	if _, err := client.CreateAccessibilityDeclaration(context.Background(), "app-1", attrs); err != nil {
		t.Fatalf("CreateAccessibilityDeclaration() error: %v", err)
	}
}

func TestUpdateAccessibilityDeclaration(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"accessibilityDeclarations","id":"decl-1","attributes":{"deviceFamily":"IPHONE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/accessibilityDeclarations/decl-1" {
			t.Fatalf("expected path /v1/accessibilityDeclarations/decl-1, got %s", req.URL.Path)
		}
		var payload AccessibilityDeclarationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeAccessibilityDeclarations || payload.Data.ID != "decl-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Publish == nil || !*payload.Data.Attributes.Publish {
			t.Fatalf("expected publish true, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	publish := true
	attrs := AccessibilityDeclarationUpdateAttributes{Publish: &publish}
	if _, err := client.UpdateAccessibilityDeclaration(context.Background(), "decl-1", attrs); err != nil {
		t.Fatalf("UpdateAccessibilityDeclaration() error: %v", err)
	}
}

func TestDeleteAccessibilityDeclaration(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/accessibilityDeclarations/decl-1" {
			t.Fatalf("expected path /v1/accessibilityDeclarations/decl-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAccessibilityDeclaration(context.Background(), "decl-1"); err != nil {
		t.Fatalf("DeleteAccessibilityDeclaration() error: %v", err)
	}
}

func TestGetAppStoreReviewAttachmentsForReviewDetail_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreReviewAttachments","id":"att-1","attributes":{"fileName":"review.pdf","fileSize":123}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreReviewDetails/detail-1/appStoreReviewAttachments" {
			t.Fatalf("expected path /v1/appStoreReviewDetails/detail-1/appStoreReviewAttachments, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("fields[appStoreReviewAttachments]") != "fileName,fileSize" {
			t.Fatalf("expected fields[appStoreReviewAttachments]=fileName,fileSize, got %q", values.Get("fields[appStoreReviewAttachments]"))
		}
		if values.Get("fields[appStoreReviewDetails]") != "contactEmail,notes" {
			t.Fatalf("expected fields[appStoreReviewDetails]=contactEmail,notes, got %q", values.Get("fields[appStoreReviewDetails]"))
		}
		if values.Get("include") != "appStoreReviewDetail" {
			t.Fatalf("expected include=appStoreReviewDetail, got %q", values.Get("include"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreReviewAttachmentsForReviewDetail(
		context.Background(),
		"detail-1",
		WithAppStoreReviewAttachmentsFields([]string{"fileName", "fileSize"}),
		WithAppStoreReviewAttachmentReviewDetailFields([]string{"contactEmail", "notes"}),
		WithAppStoreReviewAttachmentsInclude([]string{"appStoreReviewDetail"}),
		WithAppStoreReviewAttachmentsLimit(5),
	); err != nil {
		t.Fatalf("GetAppStoreReviewAttachmentsForReviewDetail() error: %v", err)
	}
}

func TestGetAppStoreReviewAttachment_WithFields(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewAttachments","id":"att-1","attributes":{"fileName":"review.pdf"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreReviewAttachments/att-1" {
			t.Fatalf("expected path /v1/appStoreReviewAttachments/att-1, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("fields[appStoreReviewAttachments]") != "fileName,fileSize" {
			t.Fatalf("expected fields[appStoreReviewAttachments]=fileName,fileSize, got %q", req.URL.Query().Get("fields[appStoreReviewAttachments]"))
		}
		if req.URL.Query().Get("include") != "appStoreReviewDetail" {
			t.Fatalf("expected include=appStoreReviewDetail, got %q", req.URL.Query().Get("include"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreReviewAttachment(
		context.Background(),
		"att-1",
		WithAppStoreReviewAttachmentsFields([]string{"fileName", "fileSize"}),
		WithAppStoreReviewAttachmentsInclude([]string{"appStoreReviewDetail"}),
	); err != nil {
		t.Fatalf("GetAppStoreReviewAttachment() error: %v", err)
	}
}

func TestCreateAppStoreReviewAttachment(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreReviewAttachments","id":"att-1","attributes":{"fileName":"review.pdf","fileSize":123}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreReviewAttachments" {
			t.Fatalf("expected path /v1/appStoreReviewAttachments, got %s", req.URL.Path)
		}
		var payload AppStoreReviewAttachmentCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppStoreReviewAttachments {
			t.Fatalf("expected type appStoreReviewAttachments, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.FileName != "review.pdf" || payload.Data.Attributes.FileSize != 123 {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.AppStoreReviewDetail == nil {
			t.Fatalf("expected review detail relationship")
		}
		if payload.Data.Relationships.AppStoreReviewDetail.Data.Type != ResourceTypeAppStoreReviewDetails {
			t.Fatalf("expected relationship type appStoreReviewDetails, got %q", payload.Data.Relationships.AppStoreReviewDetail.Data.Type)
		}
		if payload.Data.Relationships.AppStoreReviewDetail.Data.ID != "detail-1" {
			t.Fatalf("expected relationship id detail-1, got %q", payload.Data.Relationships.AppStoreReviewDetail.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAppStoreReviewAttachment(context.Background(), "detail-1", "review.pdf", 123); err != nil {
		t.Fatalf("CreateAppStoreReviewAttachment() error: %v", err)
	}
}

func TestUpdateAppStoreReviewAttachment(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewAttachments","id":"att-1","attributes":{"uploaded":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreReviewAttachments/att-1" {
			t.Fatalf("expected path /v1/appStoreReviewAttachments/att-1, got %s", req.URL.Path)
		}
		var payload AppStoreReviewAttachmentUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppStoreReviewAttachments || payload.Data.ID != "att-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Uploaded == nil || !*payload.Data.Attributes.Uploaded {
			t.Fatalf("expected uploaded true, got %+v", payload.Data.Attributes)
		}
		if payload.Data.Attributes.SourceFileChecksum == nil || *payload.Data.Attributes.SourceFileChecksum != "abcd1234" {
			t.Fatalf("expected checksum abcd1234, got %+v", payload.Data.Attributes.SourceFileChecksum)
		}
		assertAuthorized(t, req)
	}, response)

	uploaded := true
	checksum := "abcd1234"
	attrs := AppStoreReviewAttachmentUpdateAttributes{
		SourceFileChecksum: &checksum,
		Uploaded:           &uploaded,
	}
	if _, err := client.UpdateAppStoreReviewAttachment(context.Background(), "att-1", attrs); err != nil {
		t.Fatalf("UpdateAppStoreReviewAttachment() error: %v", err)
	}
}

func TestDeleteAppStoreReviewAttachment(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreReviewAttachments/att-1" {
			t.Fatalf("expected path /v1/appStoreReviewAttachments/att-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppStoreReviewAttachment(context.Background(), "att-1"); err != nil {
		t.Fatalf("DeleteAppStoreReviewAttachment() error: %v", err)
	}
}

func TestGetRoutingAppCoverage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"cover-1","attributes":{"fileName":"coverage.geojson"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/routingAppCoverages/cover-1" {
			t.Fatalf("expected path /v1/routingAppCoverages/cover-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetRoutingAppCoverage(context.Background(), "cover-1"); err != nil {
		t.Fatalf("GetRoutingAppCoverage() error: %v", err)
	}
}

func TestGetRoutingAppCoverageForVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"cover-1","attributes":{"fileName":"coverage.geojson"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersions/version-1/routingAppCoverage" {
			t.Fatalf("expected path /v1/appStoreVersions/version-1/routingAppCoverage, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetRoutingAppCoverageForVersion(context.Background(), "version-1"); err != nil {
		t.Fatalf("GetRoutingAppCoverageForVersion() error: %v", err)
	}
}

func TestCreateRoutingAppCoverage(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"routingAppCoverages","id":"cover-1","attributes":{"fileName":"coverage.geojson","fileSize":123}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/routingAppCoverages" {
			t.Fatalf("expected path /v1/routingAppCoverages, got %s", req.URL.Path)
		}
		var payload RoutingAppCoverageCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeRoutingAppCoverages {
			t.Fatalf("expected type routingAppCoverages, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.FileName != "coverage.geojson" || payload.Data.Attributes.FileSize != 123 {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.AppStoreVersion == nil {
			t.Fatalf("expected app store version relationship")
		}
		if payload.Data.Relationships.AppStoreVersion.Data.Type != ResourceTypeAppStoreVersions {
			t.Fatalf("expected relationship type appStoreVersions, got %q", payload.Data.Relationships.AppStoreVersion.Data.Type)
		}
		if payload.Data.Relationships.AppStoreVersion.Data.ID != "version-1" {
			t.Fatalf("expected relationship id version-1, got %q", payload.Data.Relationships.AppStoreVersion.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateRoutingAppCoverage(context.Background(), "version-1", "coverage.geojson", 123); err != nil {
		t.Fatalf("CreateRoutingAppCoverage() error: %v", err)
	}
}

func TestUpdateRoutingAppCoverage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"cover-1","attributes":{"uploaded":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/routingAppCoverages/cover-1" {
			t.Fatalf("expected path /v1/routingAppCoverages/cover-1, got %s", req.URL.Path)
		}
		var payload RoutingAppCoverageUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeRoutingAppCoverages || payload.Data.ID != "cover-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Uploaded == nil || !*payload.Data.Attributes.Uploaded {
			t.Fatalf("expected uploaded true, got %+v", payload.Data.Attributes)
		}
		if payload.Data.Attributes.SourceFileChecksum == nil || *payload.Data.Attributes.SourceFileChecksum != "abcd1234" {
			t.Fatalf("expected checksum abcd1234, got %+v", payload.Data.Attributes.SourceFileChecksum)
		}
		assertAuthorized(t, req)
	}, response)

	uploaded := true
	checksum := "abcd1234"
	attrs := RoutingAppCoverageUpdateAttributes{
		SourceFileChecksum: &checksum,
		Uploaded:           &uploaded,
	}
	if _, err := client.UpdateRoutingAppCoverage(context.Background(), "cover-1", attrs); err != nil {
		t.Fatalf("UpdateRoutingAppCoverage() error: %v", err)
	}
}

func TestDeleteRoutingAppCoverage(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/routingAppCoverages/cover-1" {
			t.Fatalf("expected path /v1/routingAppCoverages/cover-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteRoutingAppCoverage(context.Background(), "cover-1"); err != nil {
		t.Fatalf("DeleteRoutingAppCoverage() error: %v", err)
	}
}

func TestGetAppEncryptionDeclarationsForApp(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"appEncryptionDeclarations","id":"decl-1","attributes":{"appDescription":"Uses TLS"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEncryptionDeclarations" {
			t.Fatalf("expected path /v1/appEncryptionDeclarations, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[app]") != "app-1" {
			t.Fatalf("expected filter[app]=app-1, got %q", values.Get("filter[app]"))
		}
		if values.Get("filter[builds]") != "build-1,build-2" {
			t.Fatalf("expected filter[builds]=build-1,build-2, got %q", values.Get("filter[builds]"))
		}
		if values.Get("fields[appEncryptionDeclarations]") != "appDescription,exempt" {
			t.Fatalf("expected fields[appEncryptionDeclarations]=appDescription,exempt, got %q", values.Get("fields[appEncryptionDeclarations]"))
		}
		if values.Get("fields[appEncryptionDeclarationDocuments]") != "fileName,fileSize" {
			t.Fatalf("expected fields[appEncryptionDeclarationDocuments]=fileName,fileSize, got %q", values.Get("fields[appEncryptionDeclarationDocuments]"))
		}
		if values.Get("include") != "app,builds" {
			t.Fatalf("expected include=app,builds, got %q", values.Get("include"))
		}
		if values.Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", values.Get("limit"))
		}
		if values.Get("limit[builds]") != "10" {
			t.Fatalf("expected limit[builds]=10, got %q", values.Get("limit[builds]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEncryptionDeclarations(context.Background(), "app-1",
		WithAppEncryptionDeclarationsBuildIDs([]string{"build-1", "build-2"}),
		WithAppEncryptionDeclarationsFields([]string{"appDescription", "exempt"}),
		WithAppEncryptionDeclarationsDocumentFields([]string{"fileName", "fileSize"}),
		WithAppEncryptionDeclarationsInclude([]string{"app", "builds"}),
		WithAppEncryptionDeclarationsLimit(5),
		WithAppEncryptionDeclarationsBuildLimit(10),
	); err != nil {
		t.Fatalf("GetAppEncryptionDeclarations() error: %v", err)
	}
}

func TestGetAppEncryptionDeclaration(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appEncryptionDeclarations","id":"decl-1","attributes":{"appDescription":"Uses TLS"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEncryptionDeclarations/decl-1" {
			t.Fatalf("expected path /v1/appEncryptionDeclarations/decl-1, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("fields[appEncryptionDeclarations]") != "appDescription,exempt" {
			t.Fatalf("expected fields[appEncryptionDeclarations]=appDescription,exempt, got %q", values.Get("fields[appEncryptionDeclarations]"))
		}
		if values.Get("fields[appEncryptionDeclarationDocuments]") != "fileName,fileSize" {
			t.Fatalf("expected fields[appEncryptionDeclarationDocuments]=fileName,fileSize, got %q", values.Get("fields[appEncryptionDeclarationDocuments]"))
		}
		if values.Get("include") != "appEncryptionDeclarationDocument" {
			t.Fatalf("expected include=appEncryptionDeclarationDocument, got %q", values.Get("include"))
		}
		if values.Get("limit[builds]") != "20" {
			t.Fatalf("expected limit[builds]=20, got %q", values.Get("limit[builds]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEncryptionDeclaration(context.Background(), "decl-1",
		WithAppEncryptionDeclarationsFields([]string{"appDescription", "exempt"}),
		WithAppEncryptionDeclarationsDocumentFields([]string{"fileName", "fileSize"}),
		WithAppEncryptionDeclarationsInclude([]string{"appEncryptionDeclarationDocument"}),
		WithAppEncryptionDeclarationsBuildLimit(20),
	); err != nil {
		t.Fatalf("GetAppEncryptionDeclaration() error: %v", err)
	}
}

func TestCreateAppEncryptionDeclaration(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appEncryptionDeclarations","id":"decl-1","attributes":{"appDescription":"Uses TLS"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEncryptionDeclarations" {
			t.Fatalf("expected path /v1/appEncryptionDeclarations, got %s", req.URL.Path)
		}
		var payload AppEncryptionDeclarationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppEncryptionDeclarations {
			t.Fatalf("expected type appEncryptionDeclarations, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.AppDescription != "Uses TLS" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship")
		}
		if payload.Data.Relationships.App.Data.Type != ResourceTypeApps {
			t.Fatalf("expected relationship type apps, got %q", payload.Data.Relationships.App.Data.Type)
		}
		if payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("expected relationship id app-1, got %q", payload.Data.Relationships.App.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := AppEncryptionDeclarationCreateAttributes{
		AppDescription:                  "Uses TLS",
		ContainsProprietaryCryptography: false,
		ContainsThirdPartyCryptography:  true,
		AvailableOnFrenchStore:          true,
	}
	if _, err := client.CreateAppEncryptionDeclaration(context.Background(), "app-1", attrs); err != nil {
		t.Fatalf("CreateAppEncryptionDeclaration() error: %v", err)
	}
}

func TestAddBuildsToAppEncryptionDeclaration(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEncryptionDeclarations/decl-1/relationships/builds" {
			t.Fatalf("expected path /v1/appEncryptionDeclarations/decl-1/relationships/builds, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 relationship items, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeBuilds || payload.Data[0].ID != "build-1" {
			t.Fatalf("unexpected relationship data: %+v", payload.Data[0])
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.AddBuildsToAppEncryptionDeclaration(context.Background(), "decl-1", []string{"build-1", "build-2"}); err != nil {
		t.Fatalf("AddBuildsToAppEncryptionDeclaration() error: %v", err)
	}
}

func TestGetAppEncryptionDeclarationDocument(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appEncryptionDeclarationDocuments","id":"doc-1","attributes":{"fileName":"export.pdf"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEncryptionDeclarationDocuments/doc-1" {
			t.Fatalf("expected path /v1/appEncryptionDeclarationDocuments/doc-1, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("fields[appEncryptionDeclarationDocuments]") != "fileName,fileSize" {
			t.Fatalf("expected fields[appEncryptionDeclarationDocuments]=fileName,fileSize, got %q", req.URL.Query().Get("fields[appEncryptionDeclarationDocuments]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppEncryptionDeclarationDocument(context.Background(), "doc-1", []string{"fileName", "fileSize"}); err != nil {
		t.Fatalf("GetAppEncryptionDeclarationDocument() error: %v", err)
	}
}

func TestCreateAppEncryptionDeclarationDocument(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appEncryptionDeclarationDocuments","id":"doc-1","attributes":{"fileName":"export.pdf","fileSize":123}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEncryptionDeclarationDocuments" {
			t.Fatalf("expected path /v1/appEncryptionDeclarationDocuments, got %s", req.URL.Path)
		}
		var payload AppEncryptionDeclarationDocumentCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppEncryptionDeclarationDocuments {
			t.Fatalf("expected type appEncryptionDeclarationDocuments, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.FileName != "export.pdf" || payload.Data.Attributes.FileSize != 123 {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.AppEncryptionDeclaration == nil {
			t.Fatalf("expected declaration relationship")
		}
		if payload.Data.Relationships.AppEncryptionDeclaration.Data.Type != ResourceTypeAppEncryptionDeclarations {
			t.Fatalf("expected relationship type appEncryptionDeclarations, got %q", payload.Data.Relationships.AppEncryptionDeclaration.Data.Type)
		}
		if payload.Data.Relationships.AppEncryptionDeclaration.Data.ID != "decl-1" {
			t.Fatalf("expected relationship id decl-1, got %q", payload.Data.Relationships.AppEncryptionDeclaration.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateAppEncryptionDeclarationDocument(context.Background(), "decl-1", "export.pdf", 123); err != nil {
		t.Fatalf("CreateAppEncryptionDeclarationDocument() error: %v", err)
	}
}

func TestUpdateAppEncryptionDeclarationDocument(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appEncryptionDeclarationDocuments","id":"doc-1","attributes":{"uploaded":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appEncryptionDeclarationDocuments/doc-1" {
			t.Fatalf("expected path /v1/appEncryptionDeclarationDocuments/doc-1, got %s", req.URL.Path)
		}
		var payload AppEncryptionDeclarationDocumentUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppEncryptionDeclarationDocuments || payload.Data.ID != "doc-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Uploaded == nil || !*payload.Data.Attributes.Uploaded {
			t.Fatalf("expected uploaded true, got %+v", payload.Data.Attributes)
		}
		if payload.Data.Attributes.SourceFileChecksum == nil || *payload.Data.Attributes.SourceFileChecksum != "abcd1234" {
			t.Fatalf("expected checksum abcd1234, got %+v", payload.Data.Attributes.SourceFileChecksum)
		}
		assertAuthorized(t, req)
	}, response)

	uploaded := true
	checksum := "abcd1234"
	attrs := AppEncryptionDeclarationDocumentUpdateAttributes{
		SourceFileChecksum: &checksum,
		Uploaded:           &uploaded,
	}
	if _, err := client.UpdateAppEncryptionDeclarationDocument(context.Background(), "doc-1", attrs); err != nil {
		t.Fatalf("UpdateAppEncryptionDeclarationDocument() error: %v", err)
	}
}

func TestGetAppStoreReviewDetail(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"contactEmail":"dev@example.com"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreReviewDetails/detail-1" {
			t.Fatalf("expected path /v1/appStoreReviewDetails/detail-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreReviewDetail(context.Background(), "detail-1"); err != nil {
		t.Fatalf("GetAppStoreReviewDetail() error: %v", err)
	}
}

func TestGetAppStoreReviewDetailForVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"contactEmail":"dev@example.com"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersions/version-1/appStoreReviewDetail" {
			t.Fatalf("expected path /v1/appStoreVersions/version-1/appStoreReviewDetail, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreReviewDetailForVersion(context.Background(), "version-1"); err != nil {
		t.Fatalf("GetAppStoreReviewDetailForVersion() error: %v", err)
	}
}

func TestCreateAppStoreReviewDetail(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"contactEmail":"dev@example.com"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreReviewDetails" {
			t.Fatalf("expected path /v1/appStoreReviewDetails, got %s", req.URL.Path)
		}
		var payload AppStoreReviewDetailCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppStoreReviewDetails {
			t.Fatalf("expected type appStoreReviewDetails, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.AppStoreVersion == nil {
			t.Fatalf("expected app store version relationship")
		}
		if payload.Data.Relationships.AppStoreVersion.Data.Type != ResourceTypeAppStoreVersions {
			t.Fatalf("expected relationship type appStoreVersions, got %q", payload.Data.Relationships.AppStoreVersion.Data.Type)
		}
		if payload.Data.Relationships.AppStoreVersion.Data.ID != "version-1" {
			t.Fatalf("expected relationship id version-1, got %q", payload.Data.Relationships.AppStoreVersion.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.ContactEmail == nil || *payload.Data.Attributes.ContactEmail != "dev@example.com" {
			t.Fatalf("expected contactEmail dev@example.com, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	contactEmail := "dev@example.com"
	attrs := &AppStoreReviewDetailCreateAttributes{
		ContactEmail: &contactEmail,
	}
	if _, err := client.CreateAppStoreReviewDetail(context.Background(), "version-1", attrs); err != nil {
		t.Fatalf("CreateAppStoreReviewDetail() error: %v", err)
	}
}

func TestUpdateAppStoreReviewDetail(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreReviewDetails","id":"detail-1","attributes":{"contactPhone":"123"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreReviewDetails/detail-1" {
			t.Fatalf("expected path /v1/appStoreReviewDetails/detail-1, got %s", req.URL.Path)
		}
		var payload AppStoreReviewDetailUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeAppStoreReviewDetails || payload.Data.ID != "detail-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.ContactPhone == nil || *payload.Data.Attributes.ContactPhone != "123" {
			t.Fatalf("expected contactPhone 123, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	contactPhone := "123"
	attrs := AppStoreReviewDetailUpdateAttributes{ContactPhone: &contactPhone}
	if _, err := client.UpdateAppStoreReviewDetail(context.Background(), "detail-1", attrs); err != nil {
		t.Fatalf("UpdateAppStoreReviewDetail() error: %v", err)
	}
}

func TestGetPerfPowerMetricsForApp(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"version":"1","productData":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		assertAuthorized(t, req)
		if req.URL.Path != "/v1/apps/app-1/perfPowerMetrics" {
			t.Fatalf("expected path /v1/apps/app-1/perfPowerMetrics, got %s", req.URL.Path)
		}
		if req.Header.Get("Accept") != perfPowerMetricsAcceptHeader {
			t.Fatalf("expected Accept %q, got %q", perfPowerMetricsAcceptHeader, req.Header.Get("Accept"))
		}
		query := req.URL.Query()
		if query.Get("filter[platform]") != "IOS" {
			t.Fatalf("expected platform filter IOS, got %q", query.Get("filter[platform]"))
		}
		if query.Get("filter[metricType]") != "DISK,HANG" {
			t.Fatalf("expected metricType filter, got %q", query.Get("filter[metricType]"))
		}
		if query.Get("filter[deviceType]") != "iPhone15,2" {
			t.Fatalf("expected deviceType filter, got %q", query.Get("filter[deviceType]"))
		}
	}, response)

	_, err := client.GetPerfPowerMetricsForApp(
		context.Background(),
		"app-1",
		WithPerfPowerMetricsPlatforms([]string{"IOS"}),
		WithPerfPowerMetricsMetricTypes([]string{"DISK", "HANG"}),
		WithPerfPowerMetricsDeviceTypes([]string{"iPhone15,2"}),
	)
	if err != nil {
		t.Fatalf("GetPerfPowerMetricsForApp() error: %v", err)
	}
}

func TestGetDiagnosticSignaturesForBuild(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"diagnosticSignatures","id":"diag-1"}],"links":{"self":"https://example.com"}}`)
	client := newTestClient(t, func(req *http.Request) {
		assertAuthorized(t, req)
		if req.URL.Path != "/v1/builds/build-1/diagnosticSignatures" {
			t.Fatalf("expected path /v1/builds/build-1/diagnosticSignatures, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("filter[diagnosticType]") != "HANGS" {
			t.Fatalf("expected diagnosticType filter HANGS, got %q", query.Get("filter[diagnosticType]"))
		}
		if query.Get("fields[diagnosticSignatures]") != "diagnosticType,signature" {
			t.Fatalf("expected fields filter, got %q", query.Get("fields[diagnosticSignatures]"))
		}
		if query.Get("limit") != "50" {
			t.Fatalf("expected limit 50, got %q", query.Get("limit"))
		}
	}, response)

	_, err := client.GetDiagnosticSignaturesForBuild(
		context.Background(),
		"build-1",
		WithDiagnosticSignaturesDiagnosticTypes([]string{"HANGS"}),
		WithDiagnosticSignaturesFields([]string{"diagnosticType", "signature"}),
		WithDiagnosticSignaturesLimit(50),
	)
	if err != nil {
		t.Fatalf("GetDiagnosticSignaturesForBuild() error: %v", err)
	}
}

func TestGetDiagnosticSignatureLogs(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"version":"1","productData":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		assertAuthorized(t, req)
		if req.URL.Path != "/v1/diagnosticSignatures/diag-1/logs" {
			t.Fatalf("expected path /v1/diagnosticSignatures/diag-1/logs, got %s", req.URL.Path)
		}
		if req.Header.Get("Accept") != diagnosticLogsAcceptHeader {
			t.Fatalf("expected Accept %q, got %q", diagnosticLogsAcceptHeader, req.Header.Get("Accept"))
		}
		query := req.URL.Query()
		if query.Get("limit") != "20" {
			t.Fatalf("expected limit 20, got %q", query.Get("limit"))
		}
	}, response)

	_, err := client.GetDiagnosticSignatureLogs(context.Background(), "diag-1", WithDiagnosticLogsLimit(20))
	if err != nil {
		t.Fatalf("GetDiagnosticSignatureLogs() error: %v", err)
	}
}

func TestGetAndroidToIosAppMappingDetails(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"androidToIosAppMappingDetails","id":"map-1"}],"links":{"self":"https://example.com"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/androidToIosAppMappingDetails" {
			t.Fatalf("expected path /v1/apps/app-1/androidToIosAppMappingDetails, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("fields[androidToIosAppMappingDetails]") != "packageName" {
			t.Fatalf("expected fields filter, got %q", query.Get("fields[androidToIosAppMappingDetails]"))
		}
		if query.Get("limit") != "20" {
			t.Fatalf("expected limit 20, got %q", query.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetAndroidToIosAppMappingDetails(context.Background(), "app-1",
		WithAndroidToIosAppMappingDetailsFields([]string{"packageName"}),
		WithAndroidToIosAppMappingDetailsLimit(20),
	)
	if err != nil {
		t.Fatalf("GetAndroidToIosAppMappingDetails() error: %v", err)
	}
}

func TestGetAndroidToIosAppMappingDetail(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"androidToIosAppMappingDetails","id":"map-1"},"links":{"self":"https://example.com"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/androidToIosAppMappingDetails/map-1" {
			t.Fatalf("expected path /v1/androidToIosAppMappingDetails/map-1, got %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("fields[androidToIosAppMappingDetails]") != "packageName" {
			t.Fatalf("expected fields filter, got %q", query.Get("fields[androidToIosAppMappingDetails]"))
		}
		if query.Get("limit") != "" {
			t.Fatalf("expected no limit param, got %q", query.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetAndroidToIosAppMappingDetail(context.Background(), "map-1",
		WithAndroidToIosAppMappingDetailsFields([]string{"packageName"}),
		WithAndroidToIosAppMappingDetailsLimit(20),
	)
	if err != nil {
		t.Fatalf("GetAndroidToIosAppMappingDetail() error: %v", err)
	}
}

func TestCreateAndroidToIosAppMappingDetail(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"androidToIosAppMappingDetails","id":"map-1"},"links":{"self":"https://example.com"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/androidToIosAppMappingDetails" {
			t.Fatalf("expected path /v1/androidToIosAppMappingDetails, got %s", req.URL.Path)
		}
		var payload AndroidToIosAppMappingDetailCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeAndroidToIosAppMappingDetails {
			t.Fatalf("unexpected type: %s", payload.Data.Type)
		}
		if payload.Data.Attributes.PackageName != "com.example.android" {
			t.Fatalf("unexpected packageName: %s", payload.Data.Attributes.PackageName)
		}
		if len(payload.Data.Attributes.AppSigningKeyPublicCertificateSha256Fingerprints) != 2 {
			t.Fatalf("expected 2 fingerprints, got %d", len(payload.Data.Attributes.AppSigningKeyPublicCertificateSha256Fingerprints))
		}
		if payload.Data.Relationships.App == nil || payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("unexpected app relationship: %+v", payload.Data.Relationships.App)
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.CreateAndroidToIosAppMappingDetail(context.Background(), "app-1", AndroidToIosAppMappingDetailCreateAttributes{
		PackageName: "com.example.android",
		AppSigningKeyPublicCertificateSha256Fingerprints: []string{"sha1", "sha2"},
	})
	if err != nil {
		t.Fatalf("CreateAndroidToIosAppMappingDetail() error: %v", err)
	}
}

func TestUpdateAndroidToIosAppMappingDetail(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"androidToIosAppMappingDetails","id":"map-1"},"links":{"self":"https://example.com"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/androidToIosAppMappingDetails/map-1" {
			t.Fatalf("expected path /v1/androidToIosAppMappingDetails/map-1, got %s", req.URL.Path)
		}
		var payload AndroidToIosAppMappingDetailUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeAndroidToIosAppMappingDetails || payload.Data.ID != "map-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.PackageName == nil || payload.Data.Attributes.PackageName.Value == nil {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if *payload.Data.Attributes.PackageName.Value != "com.example.android.new" {
			t.Fatalf("unexpected packageName: %s", *payload.Data.Attributes.PackageName.Value)
		}
		assertAuthorized(t, req)
	}, response)

	packageValue := "com.example.android.new"
	_, err := client.UpdateAndroidToIosAppMappingDetail(context.Background(), "map-1", AndroidToIosAppMappingDetailUpdateAttributes{
		PackageName: &NullableString{Value: &packageValue},
	})
	if err != nil {
		t.Fatalf("UpdateAndroidToIosAppMappingDetail() error: %v", err)
	}
}

func TestDeleteAndroidToIosAppMappingDetail(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/androidToIosAppMappingDetails/map-1" {
			t.Fatalf("expected path /v1/androidToIosAppMappingDetails/map-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAndroidToIosAppMappingDetail(context.Background(), "map-1"); err != nil {
		t.Fatalf("DeleteAndroidToIosAppMappingDetail() error: %v", err)
	}
}

func TestGetSubscriptionWinBackOffers_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"winBackOffers","id":"offer-1"}],"links":{"self":"https://example.com"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptions/sub-1/winBackOffers" {
			t.Fatalf("expected path /v1/subscriptions/sub-1/winBackOffers, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionWinBackOffers(context.Background(), "sub-1", WithWinBackOffersLimit(5)); err != nil {
		t.Fatalf("GetSubscriptionWinBackOffers() error: %v", err)
	}
}

func TestGetSubscriptionWinBackOffers_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/winBackOffers?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionWinBackOffers(context.Background(), "sub-1", WithWinBackOffersNextURL(next)); err != nil {
		t.Fatalf("GetSubscriptionWinBackOffers() error: %v", err)
	}
}

func TestGetWinBackOffer(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"winBackOffers","id":"offer-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/winBackOffers/offer-1" {
			t.Fatalf("expected path /v1/winBackOffers/offer-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetWinBackOffer(context.Background(), "offer-1"); err != nil {
		t.Fatalf("GetWinBackOffer() error: %v", err)
	}
}

func TestCreateWinBackOffer_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"winBackOffers","id":"offer-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/winBackOffers" {
			t.Fatalf("expected path /v1/winBackOffers, got %s", req.URL.Path)
		}
		var payload WinBackOfferCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if payload.Data.Type != ResourceTypeWinBackOffers {
			t.Fatalf("expected type winBackOffers, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.ReferenceName != "Spring Offer" {
			t.Fatalf("expected referenceName, got %q", payload.Data.Attributes.ReferenceName)
		}
		if payload.Data.Attributes.OfferID != "OFFER-1" {
			t.Fatalf("expected offerId, got %q", payload.Data.Attributes.OfferID)
		}
		if payload.Data.Relationships.Subscription.Data.Type != ResourceTypeSubscriptions {
			t.Fatalf("expected subscription type subscriptions, got %q", payload.Data.Relationships.Subscription.Data.Type)
		}
		if payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
			t.Fatalf("expected subscription id sub-1, got %q", payload.Data.Relationships.Subscription.Data.ID)
		}
		if len(payload.Data.Relationships.Prices.Data) != 1 {
			t.Fatalf("expected 1 price relationship, got %d", len(payload.Data.Relationships.Prices.Data))
		}
		assertAuthorized(t, req)
	}, response)

	waitBetween := 6
	req := WinBackOfferCreateRequest{
		Data: WinBackOfferCreateData{
			Type: ResourceTypeWinBackOffers,
			Attributes: WinBackOfferCreateAttributes{
				ReferenceName: "Spring Offer",
				OfferID:       "OFFER-1",
				Duration:      SubscriptionOfferDurationOneMonth,
				OfferMode:     SubscriptionOfferModePayAsYouGo,
				PeriodCount:   1,
				CustomerEligibilityPaidSubscriptionDurationInMonths: 6,
				CustomerEligibilityTimeSinceLastSubscribedInMonths:  NewIntegerRange(3, 12),
				CustomerEligibilityWaitBetweenOffersInMonths:        &waitBetween,
				StartDate: "2026-02-01",
				Priority:  WinBackOfferPriorityHigh,
			},
			Relationships: WinBackOfferCreateRelationships{
				Subscription: Relationship{
					Data: ResourceData{Type: ResourceTypeSubscriptions, ID: "sub-1"},
				},
				Prices: RelationshipList{Data: []ResourceData{
					{Type: ResourceTypeWinBackOfferPrices, ID: "price-1"},
				}},
			},
		},
	}

	if _, err := client.CreateWinBackOffer(context.Background(), req); err != nil {
		t.Fatalf("CreateWinBackOffer() error: %v", err)
	}
}

func TestUpdateWinBackOffer_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"winBackOffers","id":"offer-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/winBackOffers/offer-1" {
			t.Fatalf("expected path /v1/winBackOffers/offer-1, got %s", req.URL.Path)
		}
		var payload WinBackOfferUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if payload.Data.Type != ResourceTypeWinBackOffers || payload.Data.ID != "offer-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes.Priority == nil || *payload.Data.Attributes.Priority != WinBackOfferPriorityNormal {
			t.Fatalf("expected priority NORMAL, got %+v", payload.Data.Attributes.Priority)
		}
		assertAuthorized(t, req)
	}, response)

	priority := WinBackOfferPriorityNormal
	attrs := WinBackOfferUpdateAttributes{
		Priority: &priority,
	}
	if _, err := client.UpdateWinBackOffer(context.Background(), "offer-1", attrs); err != nil {
		t.Fatalf("UpdateWinBackOffer() error: %v", err)
	}
}

func TestDeleteWinBackOffer_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/winBackOffers/offer-1" {
			t.Fatalf("expected path /v1/winBackOffers/offer-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteWinBackOffer(context.Background(), "offer-1"); err != nil {
		t.Fatalf("DeleteWinBackOffer() error: %v", err)
	}
}

func TestGetWinBackOfferPrices_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"winBackOfferPrices","id":"price-1"}],"links":{"self":"https://example.com"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/winBackOffers/offer-1/prices" {
			t.Fatalf("expected path /v1/winBackOffers/offer-1/prices, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetWinBackOfferPrices(context.Background(), "offer-1", WithWinBackOfferPricesLimit(10)); err != nil {
		t.Fatalf("GetWinBackOfferPrices() error: %v", err)
	}
}

func TestGetWinBackOfferPricesRelationships(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"winBackOfferPrices","id":"price-1"}],"links":{"self":"https://example.com"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/winBackOffers/offer-1/relationships/prices" {
			t.Fatalf("expected path /v1/winBackOffers/offer-1/relationships/prices, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "2" {
			t.Fatalf("expected limit=2, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetWinBackOfferPricesRelationships(context.Background(), "offer-1", WithLinkagesLimit(2)); err != nil {
		t.Fatalf("GetWinBackOfferPricesRelationships() error: %v", err)
	}
}

func TestGetSubscriptionWinBackOffersRelationships(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"winBackOffers","id":"offer-1"}],"links":{"self":"https://example.com"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/subscriptions/sub-1/relationships/winBackOffers" {
			t.Fatalf("expected path /v1/subscriptions/sub-1/relationships/winBackOffers, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "3" {
			t.Fatalf("expected limit=3, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionWinBackOffersRelationships(context.Background(), "sub-1", WithLinkagesLimit(3)); err != nil {
		t.Fatalf("GetSubscriptionWinBackOffersRelationships() error: %v", err)
	}
}

func TestGetPromotedPurchase_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"promotedPurchases","id":"promo-1","attributes":{"visibleForAllUsers":true,"enabled":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/promotedPurchases/promo-1" {
			t.Fatalf("expected path /v1/promotedPurchases/promo-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetPromotedPurchase(context.Background(), "promo-1"); err != nil {
		t.Fatalf("GetPromotedPurchase() error: %v", err)
	}
}

func TestCreatePromotedPurchase_Subscription_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"promotedPurchases","id":"promo-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/promotedPurchases" {
			t.Fatalf("expected path /v1/promotedPurchases, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload PromotedPurchaseCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypePromotedPurchases {
			t.Fatalf("expected type promotedPurchases, got %q", payload.Data.Type)
		}
		if !payload.Data.Attributes.VisibleForAllUsers {
			t.Fatalf("expected visibleForAllUsers true")
		}
		if payload.Data.Relationships.App.Data.Type != ResourceTypeApps || payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("expected app relationship, got %+v", payload.Data.Relationships.App.Data)
		}
		if payload.Data.Relationships.Subscription == nil || payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
			t.Fatalf("expected subscription relationship, got %+v", payload.Data.Relationships.Subscription)
		}
		if payload.Data.Relationships.InAppPurchaseV2 != nil {
			t.Fatalf("expected no inAppPurchaseV2 relationship")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := PromotedPurchaseCreateAttributes{VisibleForAllUsers: true}
	relationships := PromotedPurchaseCreateRelationships{
		App: Relationship{
			Data: ResourceData{Type: ResourceTypeApps, ID: "app-1"},
		},
		Subscription: &Relationship{
			Data: ResourceData{Type: ResourceTypeSubscriptions, ID: "sub-1"},
		},
	}

	if _, err := client.CreatePromotedPurchase(context.Background(), attrs, relationships); err != nil {
		t.Fatalf("CreatePromotedPurchase() error: %v", err)
	}
}

func TestCreatePromotedPurchase_InAppPurchase_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"promotedPurchases","id":"promo-2"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/promotedPurchases" {
			t.Fatalf("expected path /v1/promotedPurchases, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload PromotedPurchaseCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Relationships.InAppPurchaseV2 == nil || payload.Data.Relationships.InAppPurchaseV2.Data.ID != "iap-1" {
			t.Fatalf("expected inAppPurchaseV2 relationship, got %+v", payload.Data.Relationships.InAppPurchaseV2)
		}
		if payload.Data.Relationships.Subscription != nil {
			t.Fatalf("expected no subscription relationship")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := PromotedPurchaseCreateAttributes{VisibleForAllUsers: true}
	relationships := PromotedPurchaseCreateRelationships{
		App: Relationship{
			Data: ResourceData{Type: ResourceTypeApps, ID: "app-1"},
		},
		InAppPurchaseV2: &Relationship{
			Data: ResourceData{Type: ResourceTypeInAppPurchases, ID: "iap-1"},
		},
	}

	if _, err := client.CreatePromotedPurchase(context.Background(), attrs, relationships); err != nil {
		t.Fatalf("CreatePromotedPurchase() error: %v", err)
	}
}

func TestUpdatePromotedPurchase_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"promotedPurchases","id":"promo-1","attributes":{"enabled":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/promotedPurchases/promo-1" {
			t.Fatalf("expected path /v1/promotedPurchases/promo-1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload PromotedPurchaseUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypePromotedPurchases {
			t.Fatalf("expected type promotedPurchases, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "promo-1" {
			t.Fatalf("expected id promo-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Enabled == nil || !*payload.Data.Attributes.Enabled {
			t.Fatalf("expected enabled=true, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	enabled := true
	attrs := PromotedPurchaseUpdateAttributes{Enabled: &enabled}
	if _, err := client.UpdatePromotedPurchase(context.Background(), "promo-1", attrs); err != nil {
		t.Fatalf("UpdatePromotedPurchase() error: %v", err)
	}
}

func TestDeletePromotedPurchase_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/promotedPurchases/promo-1" {
			t.Fatalf("expected path /v1/promotedPurchases/promo-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeletePromotedPurchase(context.Background(), "promo-1"); err != nil {
		t.Fatalf("DeletePromotedPurchase() error: %v", err)
	}
}

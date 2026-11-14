package asc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetMarketplaceSearchDetailForApp_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"marketplaceSearchDetails","id":"detail-1","attributes":{"catalogUrl":"https://example.com/catalog"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/marketplaceSearchDetail" {
			t.Fatalf("expected path /v1/apps/app-1/marketplaceSearchDetail, got %s", req.URL.Path)
		}
		if req.URL.RawQuery != "" {
			t.Fatalf("expected no query params, got %s", req.URL.RawQuery)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetMarketplaceSearchDetailForApp(context.Background(), "app-1", nil); err != nil {
		t.Fatalf("GetMarketplaceSearchDetailForApp() error: %v", err)
	}
}

func TestGetMarketplaceSearchDetailForApp_WithFields(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"marketplaceSearchDetails","id":"detail-1","attributes":{"catalogUrl":"https://example.com/catalog"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/marketplaceSearchDetail" {
			t.Fatalf("expected path /v1/apps/app-1/marketplaceSearchDetail, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("fields[marketplaceSearchDetails]") != "catalogUrl" {
			t.Fatalf("expected fields catalogUrl, got %q", values.Get("fields[marketplaceSearchDetails]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetMarketplaceSearchDetailForApp(context.Background(), "app-1", []string{"catalogUrl"}); err != nil {
		t.Fatalf("GetMarketplaceSearchDetailForApp() error: %v", err)
	}
}

func TestCreateMarketplaceSearchDetail_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"marketplaceSearchDetails","id":"detail-1","attributes":{"catalogUrl":"https://example.com/catalog"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/marketplaceSearchDetails" {
			t.Fatalf("expected path /v1/marketplaceSearchDetails, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload MarketplaceSearchDetailCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeMarketplaceSearchDetails {
			t.Fatalf("expected type marketplaceSearchDetails, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.CatalogURL != "https://example.com/catalog" {
			t.Fatalf("expected catalog url https://example.com/catalog, got %q", payload.Data.Attributes.CatalogURL)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship to be set")
		}
		if payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("expected app id app-1, got %q", payload.Data.Relationships.App.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateMarketplaceSearchDetail(context.Background(), "app-1", "https://example.com/catalog"); err != nil {
		t.Fatalf("CreateMarketplaceSearchDetail() error: %v", err)
	}
}

func TestUpdateMarketplaceSearchDetail_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"marketplaceSearchDetails","id":"detail-1","attributes":{"catalogUrl":"https://example.com/new"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/marketplaceSearchDetails/detail-1" {
			t.Fatalf("expected path /v1/marketplaceSearchDetails/detail-1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload MarketplaceSearchDetailUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeMarketplaceSearchDetails {
			t.Fatalf("expected type marketplaceSearchDetails, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "detail-1" {
			t.Fatalf("expected id detail-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.CatalogURL == nil {
			t.Fatalf("expected catalog url attribute to be set")
		}
		if *payload.Data.Attributes.CatalogURL != "https://example.com/new" {
			t.Fatalf("expected catalog url https://example.com/new, got %q", *payload.Data.Attributes.CatalogURL)
		}
		assertAuthorized(t, req)
	}, response)

	urlValue := "https://example.com/new"
	if _, err := client.UpdateMarketplaceSearchDetail(context.Background(), "detail-1", MarketplaceSearchDetailUpdateAttributes{CatalogURL: &urlValue}); err != nil {
		t.Fatalf("UpdateMarketplaceSearchDetail() error: %v", err)
	}
}

func TestGetMarketplaceWebhooks_WithFields(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		values := req.URL.Query()
		if values.Get("fields[marketplaceWebhooks]") != "endpointUrl" {
			t.Fatalf("expected fields endpointUrl, got %q", values.Get("fields[marketplaceWebhooks]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetMarketplaceWebhooks(context.Background(), WithMarketplaceWebhooksFields([]string{"endpointUrl"})); err != nil {
		t.Fatalf("GetMarketplaceWebhooks() error: %v", err)
	}
}

func TestGetMarketplaceWebhook_FindsExactIDOnFirstCollectionPage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"marketplaceWebhooks","id":"wh-10","attributes":{"endpointUrl":"https://example.com/wrong"}},{"type":"marketplaceWebhooks","id":"wh-1","attributes":{"endpointUrl":"https://example.com/target"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/marketplaceWebhooks?cursor=unused"}}`)
	requests := 0
	client := newTestClient(t, func(req *http.Request) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/marketplaceWebhooks" {
			t.Fatalf("expected collection GET, got %s %s", req.Method, req.URL.String())
		}
		if req.URL.Query().Get("limit") != "200" {
			t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	webhook, err := client.GetMarketplaceWebhook(context.Background(), " wh-1 ")
	if err != nil {
		t.Fatalf("GetMarketplaceWebhook() error: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected one request, got %d", requests)
	}
	if webhook.Data.ID != "wh-1" || webhook.Data.Attributes.EndpointURL != "https://example.com/target" {
		t.Fatalf("unexpected webhook: %+v", webhook.Data)
	}
}

func TestGetMarketplaceWebhook_FindsExactIDOnSecondCollectionPage(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/marketplaceWebhooks?cursor=page-2"
	first := jsonResponse(http.StatusOK, `{"data":[{"type":"marketplaceWebhooks","id":"wh-other","attributes":{"endpointUrl":"https://example.com/other"}}],"links":{"next":"`+nextURL+`"}}`)
	second := jsonResponse(http.StatusOK, `{"data":[{"type":"marketplaceWebhooks","id":"wh-target","attributes":{"endpointUrl":"https://example.com/target"}}],"links":{}}`)
	requests := 0
	client := newTestClient(t, func(req *http.Request) {
		requests++
		switch requests {
		case 1:
			if req.URL.Path != "/v1/marketplaceWebhooks" || req.URL.Query().Get("limit") != "200" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
		case 2:
			if req.URL.String() != nextURL {
				t.Fatalf("expected next URL %q, got %q", nextURL, req.URL.String())
			}
		default:
			t.Fatalf("unexpected request %d: %s", requests, req.URL.String())
		}
		assertAuthorized(t, req)
	}, first, second)

	webhook, err := client.GetMarketplaceWebhook(context.Background(), "wh-target")
	if err != nil {
		t.Fatalf("GetMarketplaceWebhook() error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected two requests, got %d", requests)
	}
	if webhook.Data.ID != "wh-target" {
		t.Fatalf("expected wh-target, got %+v", webhook.Data)
	}
}

func TestGetMarketplaceWebhook_RejectsRepeatedNextURL(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/marketplaceWebhooks?cursor=repeated"
	first := jsonResponse(http.StatusOK, `{"data":[{"type":"marketplaceWebhooks","id":"wh-1"}],"links":{"next":"`+nextURL+`"}}`)
	second := jsonResponse(http.StatusOK, `{"data":[{"type":"marketplaceWebhooks","id":"wh-2"}],"links":{"next":"`+nextURL+`"}}`)
	requests := 0
	client := newTestClient(t, func(req *http.Request) {
		requests++
		assertAuthorized(t, req)
	}, first, second)

	_, err := client.GetMarketplaceWebhook(context.Background(), "wh-missing")
	if !errors.Is(err, ErrRepeatedPaginationURL) {
		t.Fatalf("expected ErrRepeatedPaginationURL, got %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected two requests before repeated-next failure, got %d", requests)
	}
}

func TestGetMarketplaceWebhook_NotFoundAfterAllCollectionPages(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/marketplaceWebhooks?cursor=page-2"
	first := jsonResponse(http.StatusOK, `{"data":[{"type":"marketplaceWebhooks","id":"wh-1"}],"links":{"next":"`+nextURL+`"}}`)
	second := jsonResponse(http.StatusOK, `{"data":[{"type":"marketplaceWebhooks","id":"wh-2"}],"links":{}}`)
	requests := 0
	client := newTestClient(t, func(req *http.Request) {
		requests++
		assertAuthorized(t, req)
	}, first, second)

	_, err := client.GetMarketplaceWebhook(context.Background(), "wh-missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), `marketplace webhook "wh-missing" not found`) {
		t.Fatalf("expected contextual not-found error, got %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected all pages to be fetched, got %d requests", requests)
	}
}

func TestGetMarketplaceWebhook_EmptyCollectionIsNotFound(t *testing.T) {
	client := newTestClient(t, nil, jsonResponse(http.StatusOK, `{"data":[],"links":{}}`))

	_, err := client.GetMarketplaceWebhook(context.Background(), "wh-missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetMarketplaceWebhook_PropagatesCollectionAPIError(t *testing.T) {
	requests := 0
	client := newTestClient(t, func(req *http.Request) {
		requests++
		if req.URL.Path != "/v1/marketplaceWebhooks" {
			t.Fatalf("expected collection path, got %s", req.URL.Path)
		}
	}, jsonResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"INVALID_REQUEST","title":"Invalid Request","detail":"request rejected"}]}`))

	_, err := client.GetMarketplaceWebhook(context.Background(), "wh-1")
	if err == nil || !strings.Contains(err.Error(), "request rejected") {
		t.Fatalf("expected API error, got %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 APIError, got %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected one request, got %d", requests)
	}
}

func TestGetMarketplaceWebhook_RequiresID(t *testing.T) {
	client := &Client{}
	_, err := client.GetMarketplaceWebhook(context.Background(), " ")
	if err == nil || err.Error() != "webhookID is required" {
		t.Fatalf("expected webhookID validation error, got %v", err)
	}
}

func TestCreateMarketplaceWebhook_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"marketplaceWebhooks","id":"wh-1","attributes":{"endpointUrl":"https://example.com/webhook"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/marketplaceWebhooks" {
			t.Fatalf("expected path /v1/marketplaceWebhooks, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload MarketplaceWebhookCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeMarketplaceWebhooks {
			t.Fatalf("expected type marketplaceWebhooks, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.EndpointURL != "https://example.com/webhook" {
			t.Fatalf("expected endpoint url https://example.com/webhook, got %q", payload.Data.Attributes.EndpointURL)
		}
		if payload.Data.Attributes.Secret != "placeholder123" {
			t.Fatalf("expected secret placeholder123, got %q", payload.Data.Attributes.Secret)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateMarketplaceWebhook(context.Background(), "https://example.com/webhook", "placeholder123"); err != nil {
		t.Fatalf("CreateMarketplaceWebhook() error: %v", err)
	}
}

func TestUpdateMarketplaceWebhook_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"marketplaceWebhooks","id":"wh-1","attributes":{"endpointUrl":"https://example.com/new"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/marketplaceWebhooks/wh-1" {
			t.Fatalf("expected path /v1/marketplaceWebhooks/wh-1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload MarketplaceWebhookUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeMarketplaceWebhooks {
			t.Fatalf("expected type marketplaceWebhooks, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "wh-1" {
			t.Fatalf("expected id wh-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.EndpointURL == nil {
			t.Fatalf("expected endpoint url attribute to be set")
		}
		if *payload.Data.Attributes.EndpointURL != "https://example.com/new" {
			t.Fatalf("expected endpoint url https://example.com/new, got %q", *payload.Data.Attributes.EndpointURL)
		}
		assertAuthorized(t, req)
	}, response)

	urlValue := "https://example.com/new"
	if _, err := client.UpdateMarketplaceWebhook(context.Background(), "wh-1", MarketplaceWebhookUpdateAttributes{EndpointURL: &urlValue}); err != nil {
		t.Fatalf("UpdateMarketplaceWebhook() error: %v", err)
	}
}

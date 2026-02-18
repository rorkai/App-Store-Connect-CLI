package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetInAppPurchaseLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1","attributes":{"name":"Name","locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseLocalizations/loc-1" {
			t.Fatalf("expected path /v1/inAppPurchaseLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetInAppPurchaseLocalization(context.Background(), "loc-1")
	if err != nil {
		t.Fatalf("GetInAppPurchaseLocalization() error: %v", err)
	}
	if resp.Data.ID != "loc-1" {
		t.Fatalf("expected id loc-1, got %q", resp.Data.ID)
	}
}

func TestUpdateInAppPurchaseOfferCodeCustomCode_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseOfferCodeCustomCodes","id":"cc-1","attributes":{"active":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseOfferCodeCustomCodes/cc-1" {
			t.Fatalf("expected path /v1/inAppPurchaseOfferCodeCustomCodes/cc-1, got %s", req.URL.Path)
		}

		var payload InAppPurchaseOfferCodeCustomCodeUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Type != ResourceTypeInAppPurchaseOfferCodeCustomCodes {
			t.Fatalf("expected type %s, got %q", ResourceTypeInAppPurchaseOfferCodeCustomCodes, payload.Data.Type)
		}
		if payload.Data.ID != "cc-1" {
			t.Fatalf("expected id cc-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Active == nil || !*payload.Data.Attributes.Active {
			t.Fatalf("expected active=true in request payload")
		}

		assertAuthorized(t, req)
	}, response)

	active := true
	if _, err := client.UpdateInAppPurchaseOfferCodeCustomCode(context.Background(), "cc-1", InAppPurchaseOfferCodeCustomCodeUpdateAttributes{
		Active: &active,
	}); err != nil {
		t.Fatalf("UpdateInAppPurchaseOfferCodeCustomCode() error: %v", err)
	}
}

func TestUpdateInAppPurchaseOfferCodeOneTimeUseCode_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseOfferCodeOneTimeUseCodes","id":"otuc-1","attributes":{"active":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchaseOfferCodeOneTimeUseCodes/otuc-1" {
			t.Fatalf("expected path /v1/inAppPurchaseOfferCodeOneTimeUseCodes/otuc-1, got %s", req.URL.Path)
		}

		var payload InAppPurchaseOfferCodeOneTimeUseCodeUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload error: %v", err)
		}
		if payload.Data.Type != ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes {
			t.Fatalf("expected type %s, got %q", ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes, payload.Data.Type)
		}
		if payload.Data.ID != "otuc-1" {
			t.Fatalf("expected id otuc-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Active == nil || *payload.Data.Attributes.Active {
			t.Fatalf("expected active=false in request payload")
		}

		assertAuthorized(t, req)
	}, response)

	active := false
	if _, err := client.UpdateInAppPurchaseOfferCodeOneTimeUseCode(context.Background(), "otuc-1", InAppPurchaseOfferCodeOneTimeUseCodeUpdateAttributes{
		Active: &active,
	}); err != nil {
		t.Fatalf("UpdateInAppPurchaseOfferCodeOneTimeUseCode() error: %v", err)
	}
}

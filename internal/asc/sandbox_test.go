package asc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestGetSandboxTesters_WithFilters(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"sandboxTesters","id":"1","attributes":{"acAccountName":"tester@example.com","firstName":"Test","lastName":"User","territory":"USA"}},{"type":"sandboxTesters","id":"2","attributes":{"acAccountName":"other@example.com","firstName":"Other","lastName":"User","territory":"JPN"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/sandboxTesters" {
			t.Fatalf("expected path /v2/sandboxTesters, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetSandboxTesters(
		context.Background(),
		WithSandboxTestersEmail("tester@example.com"),
		WithSandboxTestersTerritory("usa"),
		Limit[SandboxTestersOption](10),
	)
	if err != nil {
		t.Fatalf("GetSandboxTesters() error: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 sandbox tester, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != "1" {
		t.Fatalf("expected tester ID 1, got %q", resp.Data[0].ID)
	}
}

func TestGetSandboxTesters_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/sandboxTesters?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected next URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSandboxTesters(context.Background(), NextURL[SandboxTestersOption](next)); err != nil {
		t.Fatalf("GetSandboxTesters() error: %v", err)
	}
}

func TestGetSandboxTester_ByID(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"sandboxTesters","id":"tester-1","attributes":{"acAccountName":"tester@example.com","territory":"USA"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/sandboxTesters" {
			t.Fatalf("expected path /v2/sandboxTesters, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("limit"); got != "200" {
			t.Fatalf("expected limit=200, got %q", got)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSandboxTester(context.Background(), "tester-1"); err != nil {
		t.Fatalf("GetSandboxTester() error: %v", err)
	}
}

func TestGetSandboxTester_FollowsPagination(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v2/sandboxTesters?cursor=abc"
	client := newTestClient(
		t, func(req *http.Request) {
			assertAuthorized(t, req)
		},
		jsonResponse(http.StatusOK, `{"data":[{"type":"sandboxTesters","id":"tester-1","attributes":{"acAccountName":"one@example.com","territory":"USA"}}],"links":{"next":"`+next+`"}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"sandboxTesters","id":"tester-2","attributes":{"acAccountName":"two@example.com","territory":"USA"}}]}`),
	)

	resp, err := client.GetSandboxTester(context.Background(), "tester-2")
	if err != nil {
		t.Fatalf("GetSandboxTester() error: %v", err)
	}
	if resp.Data.ID != "tester-2" {
		t.Fatalf("expected tester-2, got %q", resp.Data.ID)
	}
}

func TestGetSandboxTester_RejectsRepeatedNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v2/sandboxTesters?cursor=repeat"
	requests := 0
	client := newTestClient(
		t, func(req *http.Request) {
			requests++
			assertAuthorized(t, req)
		},
		jsonResponse(http.StatusOK, `{"data":[{"type":"sandboxTesters","id":"tester-1","attributes":{"acAccountName":"one@example.com","territory":"USA"}}],"links":{"next":"`+next+`"}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"sandboxTesters","id":"tester-2","attributes":{"acAccountName":"two@example.com","territory":"USA"}}],"links":{"next":"`+next+`"}}`),
	)

	_, err := client.GetSandboxTester(context.Background(), "missing-tester")
	if !errors.Is(err, ErrRepeatedPaginationURL) {
		t.Fatalf("expected ErrRepeatedPaginationURL, got %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected repeated next URL to stop after two requests, got %d", requests)
	}
}

func TestUpdateSandboxTester_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"sandboxTesters","id":"tester-1","attributes":{"acAccountName":"tester@example.com","territory":"USA"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v2/sandboxTesters/tester-1" {
			t.Fatalf("expected path /v2/sandboxTesters/tester-1, got %s", req.URL.Path)
		}

		var body map[string]map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		data, ok := body["data"]
		if !ok {
			t.Fatalf("expected data in request body")
		}
		if data["type"] != "sandboxTesters" {
			t.Fatalf("expected type sandboxTesters, got %v", data["type"])
		}
		if data["id"] != "tester-1" {
			t.Fatalf("expected id tester-1, got %v", data["id"])
		}
		attributes, ok := data["attributes"].(map[string]any)
		if !ok {
			t.Fatalf("expected attributes in request body")
		}
		if attributes["territory"] != "USA" {
			t.Fatalf("expected territory USA, got %v", attributes["territory"])
		}
		if attributes["interruptPurchases"] != true {
			t.Fatalf("expected interruptPurchases true, got %v", attributes["interruptPurchases"])
		}
		if attributes["subscriptionRenewalRate"] != string(SandboxTesterRenewalEveryOneHour) {
			t.Fatalf("expected subscriptionRenewalRate %s, got %v", SandboxTesterRenewalEveryOneHour, attributes["subscriptionRenewalRate"])
		}
		assertAuthorized(t, req)
	}, response)

	territory := "USA"
	interrupt := true
	rate := SandboxTesterRenewalEveryOneHour
	attrs := SandboxTesterUpdateAttributes{
		Territory:               &territory,
		InterruptPurchases:      &interrupt,
		SubscriptionRenewalRate: &rate,
	}
	if _, err := client.UpdateSandboxTester(context.Background(), "tester-1", attrs); err != nil {
		t.Fatalf("UpdateSandboxTester() error: %v", err)
	}
}

func TestClearSandboxTesterPurchaseHistory(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"sandboxTestersClearPurchaseHistoryRequest","id":"request-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/sandboxTestersClearPurchaseHistoryRequest" {
			t.Fatalf("expected path /v2/sandboxTestersClearPurchaseHistoryRequest, got %s", req.URL.Path)
		}

		var body map[string]map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		data, ok := body["data"]
		if !ok {
			t.Fatalf("expected data in request body")
		}
		if data["type"] != "sandboxTestersClearPurchaseHistoryRequest" {
			t.Fatalf("expected type sandboxTestersClearPurchaseHistoryRequest, got %v", data["type"])
		}
		relationships, ok := data["relationships"].(map[string]any)
		if !ok {
			t.Fatalf("expected relationships in request body")
		}
		sandboxTesters, ok := relationships["sandboxTesters"].(map[string]any)
		if !ok {
			t.Fatalf("expected sandboxTesters relationship in request body")
		}
		dataList, ok := sandboxTesters["data"].([]any)
		if !ok || len(dataList) != 1 {
			t.Fatalf("expected sandboxTesters data array with one element")
		}
		item, ok := dataList[0].(map[string]any)
		if !ok {
			t.Fatalf("expected sandboxTesters data item to be object")
		}
		if item["id"] != "tester-1" || item["type"] != "sandboxTesters" {
			t.Fatalf("expected sandbox tester id/type, got %v", item)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.ClearSandboxTesterPurchaseHistory(context.Background(), "tester-1"); err != nil {
		t.Fatalf("ClearSandboxTesterPurchaseHistory() error: %v", err)
	}
}

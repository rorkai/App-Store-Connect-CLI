package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestGetAppInfoLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appInfoLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appInfoLocalizations/loc-1" {
			t.Fatalf("expected path /v1/appInfoLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetAppInfoLocalization(context.Background(), "loc-1")
	if err != nil {
		t.Fatalf("GetAppInfoLocalization() error: %v", err)
	}
	if resp.Data.ID != "loc-1" {
		t.Fatalf("expected id loc-1, got %q", resp.Data.ID)
	}
}

func TestDeleteAppInfoLocalization_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appInfoLocalizations/loc-1" {
			t.Fatalf("expected path /v1/appInfoLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteAppInfoLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteAppInfoLocalization() error: %v", err)
	}
}

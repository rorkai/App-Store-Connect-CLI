package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestGetAppCategoryParentRelationship_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appCategories","id":"cat-0"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appCategories/cat-1/relationships/parent" {
			t.Fatalf("expected path /v1/appCategories/cat-1/relationships/parent, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppCategoryParentRelationship(context.Background(), "cat-1"); err != nil {
		t.Fatalf("GetAppCategoryParentRelationship() error: %v", err)
	}
}

func TestGetAppCategorySubcategoriesRelationships_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appCategories/cat-1/relationships/subcategories" {
			t.Fatalf("expected path /v1/appCategories/cat-1/relationships/subcategories, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppCategorySubcategoriesRelationships(context.Background(), "cat-1", WithLinkagesLimit(5)); err != nil {
		t.Fatalf("GetAppCategorySubcategoriesRelationships() error: %v", err)
	}
}

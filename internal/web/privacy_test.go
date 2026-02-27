package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListAppDataUsagesParsesRelationships(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/app-123/dataUsages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("include"); got != appDataUsagesInclude {
			t.Fatalf("unexpected include query: %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != defaultAppDataUsagePageLimit {
			t.Fatalf("unexpected limit query: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{
					"id": "usage-1",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"NAME"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"APP_FUNCTIONALITY"}},
						"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_LINKED_TO_YOU"}}
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.ListAppDataUsages(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("ListAppDataUsages() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one usage, got %d", len(got))
	}
	if got[0].ID != "usage-1" || got[0].Category != "NAME" || got[0].Purpose != "APP_FUNCTIONALITY" || got[0].DataProtection != "DATA_LINKED_TO_YOU" {
		t.Fatalf("unexpected usage payload: %#v", got[0])
	}
}

func TestListAppDataUsagesFollowsPagination(t *testing.T) {
	nextLink := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/iris/v1/apps/app-123/dataUsages" && r.URL.Path != "/apps/app-123/dataUsages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "page-2" {
			_, _ = w.Write([]byte(`{
				"data": [{
					"id": "usage-2",
					"type": "appDataUsages",
					"relationships": {
						"category": {"data": {"type":"appDataUsageCategories","id":"EMAIL_ADDRESS"}},
						"purpose": {"data": {"type":"appDataUsagePurposes","id":"ANALYTICS"}},
						"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_NOT_LINKED_TO_YOU"}}
					}
				}]
			}`))
			return
		}

		_, _ = w.Write([]byte(strings.ReplaceAll(`{
			"data": [{
				"id": "usage-1",
				"type": "appDataUsages",
				"relationships": {
					"category": {"data": {"type":"appDataUsageCategories","id":"NAME"}},
					"purpose": {"data": {"type":"appDataUsagePurposes","id":"APP_FUNCTIONALITY"}},
					"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_LINKED_TO_YOU"}}
				}
			}],
			"links": {
				"next": "__NEXT__"
			}
		}`, "__NEXT__", nextLink)))
	}))
	defer server.Close()

	nextLink = server.URL + "/iris/v1/apps/app-123/dataUsages?cursor=page-2"

	client := testWebClient(server)
	client.baseURL = server.URL + "/iris/v1"

	got, err := client.ListAppDataUsages(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("ListAppDataUsages() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected two usages, got %d", len(got))
	}
}

func TestListAppDataUsageCategories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/appDataUsageCategories" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("include"); got != appDataUsageCategoriesInclude {
			t.Fatalf("unexpected include query: %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != defaultCatalogPageLimit {
			t.Fatalf("unexpected limit query: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "NAME",
				"type": "appDataUsageCategories",
				"attributes": {"deleted": false},
				"relationships": {"grouping": {"data": {"type":"appDataUsageGroupings","id":"CONTACT_INFO"}}}
			}]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.ListAppDataUsageCategories(context.Background())
	if err != nil {
		t.Fatalf("ListAppDataUsageCategories() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "NAME" || got[0].Grouping != "CONTACT_INFO" {
		t.Fatalf("unexpected categories payload: %#v", got)
	}
}

func TestListAppDataUsagePurposes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/appDataUsagePurposes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "APP_FUNCTIONALITY",
				"type": "appDataUsagePurposes",
				"attributes": {"deleted": false}
			}]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.ListAppDataUsagePurposes(context.Background())
	if err != nil {
		t.Fatalf("ListAppDataUsagePurposes() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "APP_FUNCTIONALITY" {
		t.Fatalf("unexpected purposes payload: %#v", got)
	}
}

func TestListAppDataUsageDataProtections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/appDataUsageDataProtections" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "DATA_NOT_LINKED_TO_YOU",
				"type": "appDataUsageDataProtections",
				"attributes": {"deleted": false}
			}]
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	got, err := client.ListAppDataUsageDataProtections(context.Background())
	if err != nil {
		t.Fatalf("ListAppDataUsageDataProtections() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "DATA_NOT_LINKED_TO_YOU" {
		t.Fatalf("unexpected protections payload: %#v", got)
	}
}

func TestCreateAppDataUsageBuildsExpectedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/appDataUsages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		data, ok := body["data"].(map[string]any)
		if !ok {
			t.Fatalf("missing data payload: %#v", body)
		}
		relationships, ok := data["relationships"].(map[string]any)
		if !ok {
			t.Fatalf("missing relationships: %#v", data)
		}
		if _, ok := relationships["app"]; !ok {
			t.Fatalf("expected app relationship: %#v", relationships)
		}
		if _, ok := relationships["dataProtection"]; !ok {
			t.Fatalf("expected dataProtection relationship: %#v", relationships)
		}
		if _, ok := relationships["category"]; !ok {
			t.Fatalf("expected category relationship: %#v", relationships)
		}
		if _, ok := relationships["purpose"]; !ok {
			t.Fatalf("expected purpose relationship: %#v", relationships)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "usage-new",
				"type": "appDataUsages",
				"relationships": {
					"category": {"data": {"type":"appDataUsageCategories","id":"NAME"}},
					"purpose": {"data": {"type":"appDataUsagePurposes","id":"APP_FUNCTIONALITY"}},
					"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_LINKED_TO_YOU"}}
				}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	created, err := client.CreateAppDataUsage(context.Background(), "app-123", DataUsageTuple{
		Category:       "NAME",
		Purpose:        "APP_FUNCTIONALITY",
		DataProtection: "DATA_LINKED_TO_YOU",
	})
	if err != nil {
		t.Fatalf("CreateAppDataUsage() error = %v", err)
	}
	if created == nil {
		t.Fatal("expected created usage")
	}
	if created.ID != "usage-new" || created.Category != "NAME" || created.Purpose != "APP_FUNCTIONALITY" || created.DataProtection != "DATA_LINKED_TO_YOU" {
		t.Fatalf("unexpected created usage: %#v", created)
	}
}

func TestUpdateAppDataUsageRequiresID(t *testing.T) {
	client := &Client{httpClient: &http.Client{}, baseURL: "https://example.invalid"}
	_, err := client.UpdateAppDataUsage(context.Background(), " ", DataUsageTuple{
		Category:       "NAME",
		Purpose:        "APP_FUNCTIONALITY",
		DataProtection: "DATA_LINKED_TO_YOU",
	})
	if err == nil {
		t.Fatal("expected missing id error")
	}
}

func TestUpdateAppDataUsageBuildsExpectedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/appDataUsages/usage-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPatch {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		data, ok := body["data"].(map[string]any)
		if !ok {
			t.Fatalf("missing data payload: %#v", body)
		}
		if got, _ := data["id"].(string); got != "usage-1" {
			t.Fatalf("expected id usage-1, got %#v", data["id"])
		}
		relationships, ok := data["relationships"].(map[string]any)
		if !ok {
			t.Fatalf("missing relationships: %#v", data)
		}
		if _, ok := relationships["dataProtection"]; !ok {
			t.Fatalf("expected dataProtection relationship: %#v", relationships)
		}
		if _, ok := relationships["category"]; !ok {
			t.Fatalf("expected category relationship: %#v", relationships)
		}
		if _, ok := relationships["purpose"]; !ok {
			t.Fatalf("expected purpose relationship: %#v", relationships)
		}
		if _, ok := relationships["app"]; ok {
			t.Fatalf("did not expect app relationship in patch payload: %#v", relationships)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "usage-1",
				"type": "appDataUsages",
				"relationships": {
					"category": {"data": {"type":"appDataUsageCategories","id":"NAME"}},
					"purpose": {"data": {"type":"appDataUsagePurposes","id":"APP_FUNCTIONALITY"}},
					"dataProtection": {"data": {"type":"appDataUsageDataProtections","id":"DATA_NOT_LINKED_TO_YOU"}}
				}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	updated, err := client.UpdateAppDataUsage(context.Background(), "usage-1", DataUsageTuple{
		Category:       "NAME",
		Purpose:        "APP_FUNCTIONALITY",
		DataProtection: "DATA_NOT_LINKED_TO_YOU",
	})
	if err != nil {
		t.Fatalf("UpdateAppDataUsage() error = %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated usage")
	}
	if updated.ID != "usage-1" || updated.Category != "NAME" || updated.Purpose != "APP_FUNCTIONALITY" || updated.DataProtection != "DATA_NOT_LINKED_TO_YOU" {
		t.Fatalf("unexpected updated usage: %#v", updated)
	}
}

func TestDeleteAppDataUsageRequiresID(t *testing.T) {
	client := &Client{httpClient: &http.Client{}, baseURL: "https://example.invalid"}
	if err := client.DeleteAppDataUsage(context.Background(), " "); err == nil {
		t.Fatal("expected missing id error")
	}
}

func TestDeleteAppDataUsageCallsEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/appDataUsages/usage-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := testWebClient(server)
	if err := client.DeleteAppDataUsage(context.Background(), "usage-1"); err != nil {
		t.Fatalf("DeleteAppDataUsage() error = %v", err)
	}
}

func TestGetAppDataUsagesPublishStateParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apps/app-123/dataUsagePublishState" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "publish-state-1",
				"type": "appDataUsagesPublishState",
				"attributes": {
					"published": false
				}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	state, err := client.GetAppDataUsagesPublishState(context.Background(), "app-123")
	if err != nil {
		t.Fatalf("GetAppDataUsagesPublishState() error = %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil publish state")
	}
	if state.ID != "publish-state-1" || state.Published {
		t.Fatalf("unexpected publish state: %#v", state)
	}
}

func TestSetAppDataUsagesPublishedBuildsPatchRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/appDataUsagesPublishState/publish-state-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPatch {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		data, ok := body["data"].(map[string]any)
		if !ok {
			t.Fatalf("missing data payload: %#v", body)
		}
		attributes, ok := data["attributes"].(map[string]any)
		if !ok {
			t.Fatalf("missing attributes payload: %#v", data)
		}
		if got, ok := attributes["published"].(bool); !ok || !got {
			t.Fatalf("expected published=true payload, got %#v", attributes["published"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "publish-state-1",
				"type": "appDataUsagesPublishState",
				"attributes": {
					"published": true
				}
			}
		}`))
	}))
	defer server.Close()

	client := testWebClient(server)
	state, err := client.SetAppDataUsagesPublished(context.Background(), "publish-state-1", true)
	if err != nil {
		t.Fatalf("SetAppDataUsagesPublished() error = %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil publish state")
	}
	if state.ID != "publish-state-1" || !state.Published {
		t.Fatalf("unexpected publish state: %#v", state)
	}
}

func TestCreateAppDataUsageRejectsMissingDataProtection(t *testing.T) {
	client := &Client{httpClient: &http.Client{}, baseURL: "https://example.invalid"}
	_, err := client.CreateAppDataUsage(context.Background(), "app-123", DataUsageTuple{
		Category: "NAME",
		Purpose:  "APP_FUNCTIONALITY",
	})
	if err == nil {
		t.Fatal("expected missing data protection error")
	}
	if !strings.Contains(err.Error(), "data protection is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

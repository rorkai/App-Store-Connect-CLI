package asc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"
)

func TestGetGameCenterAppVersions_WithLimitAndFilter(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAppVersions" {
			t.Fatalf("expected path /v1/gameCenterAppVersions, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", values.Get("limit"))
		}
		if values.Get("filter[enabled]") != "true" {
			t.Fatalf("expected filter[enabled]=true, got %q", values.Get("filter[enabled]"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAppVersions(context.Background(), WithGCAppVersionsLimit(10), WithGCAppVersionsEnabled(true)); err != nil {
		t.Fatalf("GetGameCenterAppVersions() error: %v", err)
	}
}

func TestGetGameCenterAppVersions_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterAppVersions?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAppVersions(context.Background(), WithGCAppVersionsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterAppVersions() error: %v", err)
	}
}

func TestGetGameCenterAppVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAppVersions","id":"gcav-1","attributes":{"enabled":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAppVersions/gcav-1" {
			t.Fatalf("expected path /v1/gameCenterAppVersions/gcav-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAppVersion(context.Background(), "gcav-1"); err != nil {
		t.Fatalf("GetGameCenterAppVersion() error: %v", err)
	}
}

func TestGetGameCenterDetailGameCenterAppVersions_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails/detail-1/gameCenterAppVersions" {
			t.Fatalf("expected path /v1/gameCenterDetails/detail-1/gameCenterAppVersions, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "20" {
			t.Fatalf("expected limit=20, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterDetailGameCenterAppVersions(context.Background(), "detail-1", WithGCAppVersionsLimit(20)); err != nil {
		t.Fatalf("GetGameCenterDetailGameCenterAppVersions() error: %v", err)
	}
}

func TestGetGameCenterDetailGameCenterAppVersions_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-1/gameCenterAppVersions?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterDetailGameCenterAppVersions(context.Background(), "", WithGCAppVersionsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterDetailGameCenterAppVersions() error: %v", err)
	}
}

func TestGetGameCenterAppVersionCompatibilityVersions_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAppVersions/gcav-1/compatibilityVersions" {
			t.Fatalf("expected path /v1/gameCenterAppVersions/gcav-1/compatibilityVersions, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "15" {
			t.Fatalf("expected limit=15, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAppVersionCompatibilityVersions(context.Background(), "gcav-1", WithGCAppVersionsLimit(15)); err != nil {
		t.Fatalf("GetGameCenterAppVersionCompatibilityVersions() error: %v", err)
	}
}

func TestGetGameCenterAppVersionCompatibilityVersions_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterAppVersions/gcav-1/compatibilityVersions?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAppVersionCompatibilityVersions(context.Background(), "", WithGCAppVersionsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterAppVersionCompatibilityVersions() error: %v", err)
	}
}

func TestGetGameCenterAppVersionAppStoreVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAppVersions/gcav-1/appStoreVersion" {
			t.Fatalf("expected path /v1/gameCenterAppVersions/gcav-1/appStoreVersion, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAppVersionAppStoreVersion(context.Background(), "gcav-1"); err != nil {
		t.Fatalf("GetGameCenterAppVersionAppStoreVersion() error: %v", err)
	}
}

func TestGetAppStoreVersionGameCenterAppVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAppVersions","id":"gcav-1","attributes":{"enabled":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/appStoreVersions/version-1/gameCenterAppVersion" {
			t.Fatalf("expected path /v1/appStoreVersions/version-1/gameCenterAppVersion, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppStoreVersionGameCenterAppVersion(context.Background(), "version-1"); err != nil {
		t.Fatalf("GetAppStoreVersionGameCenterAppVersion() error: %v", err)
	}
}

func TestCreateGameCenterAppVersion(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterAppVersions","id":"gcav-new","attributes":{"enabled":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAppVersions" {
			t.Fatalf("expected path /v1/gameCenterAppVersions, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload GameCenterAppVersionCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeGameCenterAppVersions {
			t.Fatalf("expected type gameCenterAppVersions, got %s", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.AppStoreVersion == nil {
			t.Fatalf("expected appStoreVersion relationship to be set")
		}
		if payload.Data.Relationships.AppStoreVersion.Data.ID != "version-123" {
			t.Fatalf("expected appStoreVersion ID version-123, got %s", payload.Data.Relationships.AppStoreVersion.Data.ID)
		}
		if payload.Data.Relationships.AppStoreVersion.Data.Type != ResourceTypeAppStoreVersions {
			t.Fatalf("expected appStoreVersion type appStoreVersions, got %s", payload.Data.Relationships.AppStoreVersion.Data.Type)
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.CreateGameCenterAppVersion(context.Background(), "version-123")
	if err != nil {
		t.Fatalf("CreateGameCenterAppVersion() error: %v", err)
	}

	if resp.Data.ID != "gcav-new" {
		t.Fatalf("expected ID gcav-new, got %s", resp.Data.ID)
	}
}

func TestUpdateGameCenterAppVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAppVersions","id":"gcav-1","attributes":{"enabled":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAppVersions/gcav-1" {
			t.Fatalf("expected path /v1/gameCenterAppVersions/gcav-1, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload GameCenterAppVersionUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeGameCenterAppVersions {
			t.Fatalf("expected type gameCenterAppVersions, got %s", payload.Data.Type)
		}
		if payload.Data.ID != "gcav-1" {
			t.Fatalf("expected id gcav-1, got %s", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Enabled == nil {
			t.Fatalf("expected enabled attribute to be set")
		}
		if *payload.Data.Attributes.Enabled != true {
			t.Fatalf("expected enabled true, got %v", *payload.Data.Attributes.Enabled)
		}
		assertAuthorized(t, req)
	}, response)

	enabled := true
	attrs := GameCenterAppVersionUpdateAttributes{Enabled: &enabled}
	resp, err := client.UpdateGameCenterAppVersion(context.Background(), "gcav-1", attrs)
	if err != nil {
		t.Fatalf("UpdateGameCenterAppVersion() error: %v", err)
	}

	if resp.Data.ID != "gcav-1" {
		t.Fatalf("expected ID gcav-1, got %s", resp.Data.ID)
	}
	if !resp.Data.Attributes.Enabled {
		t.Fatalf("expected enabled true, got false")
	}
}

func TestCreateGameCenterAppVersion_RequiresID(t *testing.T) {
	client := newTestClient(t, nil, nil)

	_, err := client.CreateGameCenterAppVersion(context.Background(), " ")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateGameCenterAppVersion_ReturnsAPIError(t *testing.T) {
	response := jsonResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)
	client := newTestClient(t, nil, response)

	_, err := client.CreateGameCenterAppVersion(context.Background(), "version-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status code %d, got %d", http.StatusForbidden, apiErr.StatusCode)
	}
}

func TestUpdateGameCenterAppVersion_ValidationErrors(t *testing.T) {
	client := newTestClient(t, nil, nil)

	enabled := true
	tests := []struct {
		name string
		id   string
		attr GameCenterAppVersionUpdateAttributes
	}{
		{
			name: "missing id",
			id:   " ",
			attr: GameCenterAppVersionUpdateAttributes{Enabled: &enabled},
		},
		{
			name: "missing attributes",
			id:   "gcav-1",
			attr: GameCenterAppVersionUpdateAttributes{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.UpdateGameCenterAppVersion(context.Background(), test.id, test.attr)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestUpdateGameCenterAppVersion_ReturnsAPIError(t *testing.T) {
	response := jsonResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)
	client := newTestClient(t, nil, response)

	enabled := true
	_, err := client.UpdateGameCenterAppVersion(context.Background(), "gcav-1", GameCenterAppVersionUpdateAttributes{Enabled: &enabled})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status code %d, got %d", http.StatusForbidden, apiErr.StatusCode)
	}
}

func TestGCAppVersionsOptions(t *testing.T) {
	query := &gcAppVersionsQuery{}
	WithGCAppVersionsLimit(12)(query)
	if query.limit != 12 {
		t.Fatalf("expected limit 12, got %d", query.limit)
	}
	WithGCAppVersionsNextURL("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	WithGCAppVersionsEnabled(true)(query)
	values, err := url.ParseQuery(buildGCAppVersionsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("filter[enabled]") != "true" {
		t.Fatalf("expected filter[enabled]=true, got %q", values.Get("filter[enabled]"))
	}
	if values.Get("limit") != "12" {
		t.Fatalf("expected limit=12, got %q", values.Get("limit"))
	}
}

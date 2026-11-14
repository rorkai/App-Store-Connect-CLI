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
	Limit[GCAppVersionsOption](12)(query)
	if query.limit != 12 {
		t.Fatalf("expected limit 12, got %d", query.limit)
	}
	NextURL[GCAppVersionsOption]("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCAppVersionsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "12" {
		t.Fatalf("expected limit=12, got %q", values.Get("limit"))
	}
}

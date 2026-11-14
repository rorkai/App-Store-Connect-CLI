package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateGameCenterGroup(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterGroups","id":"group-1","attributes":{"referenceName":"Group"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups" {
			t.Fatalf("expected path /v1/gameCenterGroups, got %s", req.URL.Path)
		}
		var payload GameCenterGroupCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterGroups {
			t.Fatalf("expected type gameCenterGroups, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.ReferenceName == nil || *payload.Data.Attributes.ReferenceName != "Group" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Group"
	if _, err := client.CreateGameCenterGroup(context.Background(), &name); err != nil {
		t.Fatalf("CreateGameCenterGroup() error: %v", err)
	}
}

func TestUpdateGameCenterGroup(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterGroups","id":"group-1","attributes":{"referenceName":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups/group-1" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1, got %s", req.URL.Path)
		}
		var payload GameCenterGroupUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "group-1" || payload.Data.Type != ResourceTypeGameCenterGroups {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.ReferenceName == nil || *payload.Data.Attributes.ReferenceName != "Updated" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	if _, err := client.UpdateGameCenterGroup(context.Background(), "group-1", &name); err != nil {
		t.Fatalf("UpdateGameCenterGroup() error: %v", err)
	}
}

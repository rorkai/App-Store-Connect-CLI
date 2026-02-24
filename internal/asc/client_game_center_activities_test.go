package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestGetGameCenterActivities_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails/gc-detail-1/gameCenterActivities" {
			t.Fatalf("expected path /v1/gameCenterDetails/gc-detail-1/gameCenterActivities, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "25" {
			t.Fatalf("expected limit=25, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivities(context.Background(), "gc-detail-1", WithGCActivitiesLimit(25)); err != nil {
		t.Fatalf("GetGameCenterActivities() error: %v", err)
	}
}

func TestGetGameCenterActivities_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/gameCenterActivities?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivities(context.Background(), "gc-detail-1", WithGCActivitiesNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterActivities() error: %v", err)
	}
}

func TestGetGameCenterActivity(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterActivities","id":"act-1","attributes":{"referenceName":"Seasonal"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivities/act-1" {
			t.Fatalf("expected path /v1/gameCenterActivities/act-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivity(context.Background(), "act-1"); err != nil {
		t.Fatalf("GetGameCenterActivity() error: %v", err)
	}
}

func TestCreateGameCenterActivity_WithDetail(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterActivities","id":"act-1","attributes":{"referenceName":"Seasonal"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivities" {
			t.Fatalf("expected path /v1/gameCenterActivities, got %s", req.URL.Path)
		}
		var payload GameCenterActivityCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterActivities {
			t.Fatalf("expected type gameCenterActivities, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.ReferenceName != "Seasonal" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.GameCenterDetail == nil {
			t.Fatalf("expected gameCenterDetail relationship")
		}
		if payload.Data.Relationships.GameCenterDetail.Data.ID != "gc-detail-1" {
			t.Fatalf("expected gc-detail-1, got %s", payload.Data.Relationships.GameCenterDetail.Data.ID)
		}
		if payload.Data.Relationships.GameCenterGroup != nil {
			t.Fatalf("expected no group relationship, got %+v", payload.Data.Relationships.GameCenterGroup)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterActivityCreateAttributes{
		ReferenceName: "Seasonal",
	}
	if _, err := client.CreateGameCenterActivity(context.Background(), "gc-detail-1", attrs, ""); err != nil {
		t.Fatalf("CreateGameCenterActivity() error: %v", err)
	}
}

func TestCreateGameCenterActivity_WithGroup(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterActivities","id":"act-1","attributes":{"referenceName":"Seasonal"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivities" {
			t.Fatalf("expected path /v1/gameCenterActivities, got %s", req.URL.Path)
		}
		var payload GameCenterActivityCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterActivities {
			t.Fatalf("expected type gameCenterActivities, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.GameCenterGroup == nil {
			t.Fatalf("expected gameCenterGroup relationship")
		}
		if payload.Data.Relationships.GameCenterGroup.Data.ID != "group-1" {
			t.Fatalf("expected group-1, got %s", payload.Data.Relationships.GameCenterGroup.Data.ID)
		}
		if payload.Data.Relationships.GameCenterDetail != nil {
			t.Fatalf("expected no gameCenterDetail relationship, got %+v", payload.Data.Relationships.GameCenterDetail)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterActivityCreateAttributes{
		ReferenceName: "Seasonal",
	}
	if _, err := client.CreateGameCenterActivity(context.Background(), "", attrs, "group-1"); err != nil {
		t.Fatalf("CreateGameCenterActivity() error: %v", err)
	}
}

func TestUpdateGameCenterActivity(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterActivities","id":"act-1","attributes":{"referenceName":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivities/act-1" {
			t.Fatalf("expected path /v1/gameCenterActivities/act-1, got %s", req.URL.Path)
		}
		var payload GameCenterActivityUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "act-1" || payload.Data.Type != ResourceTypeGameCenterActivities {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.ReferenceName == nil || *payload.Data.Attributes.ReferenceName != "Updated" {
			t.Fatalf("expected referenceName update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	attrs := GameCenterActivityUpdateAttributes{ReferenceName: &name}
	if _, err := client.UpdateGameCenterActivity(context.Background(), "act-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterActivity() error: %v", err)
	}
}

func TestDeleteGameCenterActivity(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivities/act-1" {
			t.Fatalf("expected path /v1/gameCenterActivities/act-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterActivity(context.Background(), "act-1"); err != nil {
		t.Fatalf("DeleteGameCenterActivity() error: %v", err)
	}
}

func TestAddGameCenterActivityAchievements(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivities/act-1/relationships/achievements" {
			t.Fatalf("expected path /v1/gameCenterActivities/act-1/relationships/achievements, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 relationships, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeGameCenterAchievements {
			t.Fatalf("expected type gameCenterAchievements, got %q", payload.Data[0].Type)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.AddGameCenterActivityAchievements(context.Background(), "act-1", []string{"ach-1", "ach-2"}); err != nil {
		t.Fatalf("AddGameCenterActivityAchievements() error: %v", err)
	}
}

func TestRemoveGameCenterActivityAchievements(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivities/act-1/relationships/achievements" {
			t.Fatalf("expected path /v1/gameCenterActivities/act-1/relationships/achievements, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(payload.Data) != 1 || payload.Data[0].ID != "ach-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.RemoveGameCenterActivityAchievements(context.Background(), "act-1", []string{"ach-1"}); err != nil {
		t.Fatalf("RemoveGameCenterActivityAchievements() error: %v", err)
	}
}

func TestAddGameCenterActivityLeaderboards(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivities/act-1/relationships/leaderboards" {
			t.Fatalf("expected path /v1/gameCenterActivities/act-1/relationships/leaderboards, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 relationships, got %d", len(payload.Data))
		}
		if payload.Data[0].Type != ResourceTypeGameCenterLeaderboards {
			t.Fatalf("expected type gameCenterLeaderboards, got %q", payload.Data[0].Type)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.AddGameCenterActivityLeaderboards(context.Background(), "act-1", []string{"lb-1", "lb-2"}); err != nil {
		t.Fatalf("AddGameCenterActivityLeaderboards() error: %v", err)
	}
}

func TestRemoveGameCenterActivityLeaderboards(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivities/act-1/relationships/leaderboards" {
			t.Fatalf("expected path /v1/gameCenterActivities/act-1/relationships/leaderboards, got %s", req.URL.Path)
		}
		var payload RelationshipRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(payload.Data) != 1 || payload.Data[0].ID != "lb-1" {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.RemoveGameCenterActivityLeaderboards(context.Background(), "act-1", []string{"lb-1"}); err != nil {
		t.Fatalf("RemoveGameCenterActivityLeaderboards() error: %v", err)
	}
}

func TestGetGameCenterActivityVersions_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivities/act-1/versions" {
			t.Fatalf("expected path /v1/gameCenterActivities/act-1/versions, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "5" {
			t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivityVersions(context.Background(), "act-1", WithGCActivityVersionsLimit(5)); err != nil {
		t.Fatalf("GetGameCenterActivityVersions() error: %v", err)
	}
}

func TestGetGameCenterActivityVersions_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterActivities/act-1/versions?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivityVersions(context.Background(), "act-1", WithGCActivityVersionsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterActivityVersions() error: %v", err)
	}
}

func TestGetGameCenterActivityVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterActivityVersions","id":"ver-1","attributes":{"version":1}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityVersions/ver-1" {
			t.Fatalf("expected path /v1/gameCenterActivityVersions/ver-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivityVersion(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetGameCenterActivityVersion() error: %v", err)
	}
}

func TestCreateGameCenterActivityVersion(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterActivityVersions","id":"ver-2","attributes":{"version":2}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityVersions" {
			t.Fatalf("expected path /v1/gameCenterActivityVersions, got %s", req.URL.Path)
		}
		var payload GameCenterActivityVersionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterActivityVersions {
			t.Fatalf("expected type gameCenterActivityVersions, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Activity == nil || payload.Data.Relationships.Activity.Data.ID != "act-1" {
			t.Fatalf("expected activity relationship act-1")
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateGameCenterActivityVersion(context.Background(), "act-1", ""); err != nil {
		t.Fatalf("CreateGameCenterActivityVersion() error: %v", err)
	}
}

func TestGetGameCenterActivityLocalizations_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityVersions/ver-1/localizations" {
			t.Fatalf("expected path /v1/gameCenterActivityVersions/ver-1/localizations, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivityLocalizations(context.Background(), "ver-1", WithGCActivityLocalizationsLimit(10)); err != nil {
		t.Fatalf("GetGameCenterActivityLocalizations() error: %v", err)
	}
}

func TestGetGameCenterActivityLocalizations_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterActivityVersions/ver-1/localizations?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivityLocalizations(context.Background(), "ver-1", WithGCActivityLocalizationsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterActivityLocalizations() error: %v", err)
	}
}

func TestGetGameCenterActivityLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterActivityLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"Weekly","description":"Desc"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityLocalizations/loc-1" {
			t.Fatalf("expected path /v1/gameCenterActivityLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivityLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterActivityLocalization() error: %v", err)
	}
}

func TestCreateGameCenterActivityLocalization(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterActivityLocalizations","id":"loc-2","attributes":{"locale":"en-US","name":"Weekly","description":"Desc"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityLocalizations" {
			t.Fatalf("expected path /v1/gameCenterActivityLocalizations, got %s", req.URL.Path)
		}
		var payload GameCenterActivityLocalizationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterActivityLocalizations {
			t.Fatalf("expected type gameCenterActivityLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Version == nil || payload.Data.Relationships.Version.Data.ID != "ver-1" {
			t.Fatalf("expected version relationship ver-1")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterActivityLocalizationCreateAttributes{
		Locale:      "en-US",
		Name:        "Weekly",
		Description: "Desc",
	}
	if _, err := client.CreateGameCenterActivityLocalization(context.Background(), "ver-1", attrs); err != nil {
		t.Fatalf("CreateGameCenterActivityLocalization() error: %v", err)
	}
}

func TestUpdateGameCenterActivityLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterActivityLocalizations","id":"loc-1","attributes":{"name":"Updated","description":"Desc"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityLocalizations/loc-1" {
			t.Fatalf("expected path /v1/gameCenterActivityLocalizations/loc-1, got %s", req.URL.Path)
		}
		var payload GameCenterActivityLocalizationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "loc-1" || payload.Data.Type != ResourceTypeGameCenterActivityLocalizations {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name == nil || *payload.Data.Attributes.Name != "Updated" {
			t.Fatalf("expected name update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	attrs := GameCenterActivityLocalizationUpdateAttributes{Name: &name}
	if _, err := client.UpdateGameCenterActivityLocalization(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterActivityLocalization() error: %v", err)
	}
}

func TestDeleteGameCenterActivityLocalization(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityLocalizations/loc-1" {
			t.Fatalf("expected path /v1/gameCenterActivityLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterActivityLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteGameCenterActivityLocalization() error: %v", err)
	}
}

func TestGetGameCenterActivityImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterActivityImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityImages/img-1" {
			t.Fatalf("expected path /v1/gameCenterActivityImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivityImage(context.Background(), "img-1"); err != nil {
		t.Fatalf("GetGameCenterActivityImage() error: %v", err)
	}
}

func TestCreateGameCenterActivityImage(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterActivityImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityImages" {
			t.Fatalf("expected path /v1/gameCenterActivityImages, got %s", req.URL.Path)
		}
		var payload GameCenterActivityImageCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterActivityImages {
			t.Fatalf("expected type gameCenterActivityImages, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.FileName != "image.png" || payload.Data.Attributes.FileSize != 12 {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Localization == nil || payload.Data.Relationships.Localization.Data.ID != "loc-1" {
			t.Fatalf("expected localization relationship loc-1")
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateGameCenterActivityImage(context.Background(), "loc-1", "image.png", 12); err != nil {
		t.Fatalf("CreateGameCenterActivityImage() error: %v", err)
	}
}

func TestUpdateGameCenterActivityImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterActivityImages","id":"img-1","attributes":{"uploaded":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityImages/img-1" {
			t.Fatalf("expected path /v1/gameCenterActivityImages/img-1, got %s", req.URL.Path)
		}
		var payload GameCenterActivityImageUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "img-1" || payload.Data.Type != ResourceTypeGameCenterActivityImages {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Uploaded == nil || !*payload.Data.Attributes.Uploaded {
			t.Fatalf("expected uploaded true, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.UpdateGameCenterActivityImage(context.Background(), "img-1", true); err != nil {
		t.Fatalf("UpdateGameCenterActivityImage() error: %v", err)
	}
}

func TestDeleteGameCenterActivityImage(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityImages/img-1" {
			t.Fatalf("expected path /v1/gameCenterActivityImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterActivityImage(context.Background(), "img-1"); err != nil {
		t.Fatalf("DeleteGameCenterActivityImage() error: %v", err)
	}
}

func TestUploadGameCenterActivityImage(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "activity.png")
	if err := os.WriteFile(filePath, []byte("activity-image"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	uploadCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadCalled = true
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT upload, got %s", r.Method)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { server.Close() }()

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	client := newUploadTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/gameCenterActivityImages":
			body := fmt.Sprintf(`{"data":{"type":"gameCenterActivityImages","id":"img-1","attributes":{"fileName":"%s","fileSize":%d,"uploadOperations":[{"method":"PUT","url":"%s","length":%d,"offset":0}]}}}`, fileInfo.Name(), fileInfo.Size(), server.URL, fileInfo.Size())
			return jsonResponse(http.StatusCreated, body), nil
		case "/v1/gameCenterActivityImages/img-1":
			body := fmt.Sprintf(`{"data":{"type":"gameCenterActivityImages","id":"img-1","attributes":{"fileName":"%s","fileSize":%d,"uploaded":true,"assetDeliveryState":{"state":"COMPLETE"}}}}`, fileInfo.Name(), fileInfo.Size())
			return jsonResponse(http.StatusOK, body), nil
		default:
			return jsonResponse(http.StatusNotFound, `{"errors":[{"title":"not found"}]}`), nil
		}
	})

	result, err := client.UploadGameCenterActivityImage(context.Background(), "loc-1", filePath)
	if err != nil {
		t.Fatalf("UploadGameCenterActivityImage() error: %v", err)
	}
	if !uploadCalled {
		t.Fatalf("expected upload operation to be called")
	}
	if result.ID != "img-1" {
		t.Fatalf("expected result id img-1, got %s", result.ID)
	}
	if result.LocalizationID != "loc-1" {
		t.Fatalf("expected localization id loc-1, got %s", result.LocalizationID)
	}
	if result.AssetDeliveryState != "COMPLETE" {
		t.Fatalf("expected state COMPLETE, got %s", result.AssetDeliveryState)
	}
}

func TestGetGameCenterActivityVersionRelease(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterActivityVersionReleases","id":"rel-1","attributes":{"live":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityVersionReleases/rel-1" {
			t.Fatalf("expected path /v1/gameCenterActivityVersionReleases/rel-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivityVersionRelease(context.Background(), "rel-1"); err != nil {
		t.Fatalf("GetGameCenterActivityVersionRelease() error: %v", err)
	}
}

func TestGetGameCenterActivityVersionReleases_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails/gc-detail-1/activityReleases" {
			t.Fatalf("expected path /v1/gameCenterDetails/gc-detail-1/activityReleases, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "30" {
			t.Fatalf("expected limit=30, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivityVersionReleases(context.Background(), "gc-detail-1", WithGCActivityVersionReleasesLimit(30)); err != nil {
		t.Fatalf("GetGameCenterActivityVersionReleases() error: %v", err)
	}
}

func TestGetGameCenterActivityVersionReleases_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/activityReleases?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivityVersionReleases(context.Background(), "gc-detail-1", WithGCActivityVersionReleasesNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterActivityVersionReleases() error: %v", err)
	}
}

func TestCreateGameCenterActivityVersionRelease(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterActivityVersionReleases","id":"rel-2","attributes":{"live":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityVersionReleases" {
			t.Fatalf("expected path /v1/gameCenterActivityVersionReleases, got %s", req.URL.Path)
		}
		var payload GameCenterActivityVersionReleaseCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterActivityVersionReleases {
			t.Fatalf("expected type gameCenterActivityVersionReleases, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Version == nil || payload.Data.Relationships.Version.Data.ID != "ver-1" {
			t.Fatalf("expected version relationship ver-1")
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateGameCenterActivityVersionRelease(context.Background(), "ver-1"); err != nil {
		t.Fatalf("CreateGameCenterActivityVersionRelease() error: %v", err)
	}
}

func TestDeleteGameCenterActivityVersionRelease(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityVersionReleases/rel-1" {
			t.Fatalf("expected path /v1/gameCenterActivityVersionReleases/rel-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterActivityVersionRelease(context.Background(), "rel-1"); err != nil {
		t.Fatalf("DeleteGameCenterActivityVersionRelease() error: %v", err)
	}
}

func TestGetGameCenterActivityLocalizationImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterActivityImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityLocalizations/loc-1/image" {
			t.Fatalf("expected path /v1/gameCenterActivityLocalizations/loc-1/image, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivityLocalizationImage(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterActivityLocalizationImage() error: %v", err)
	}
}

func TestGetGameCenterActivityVersionDefaultImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterActivityImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterActivityVersions/ver-1/defaultImage" {
			t.Fatalf("expected path /v1/gameCenterActivityVersions/ver-1/defaultImage, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterActivityVersionDefaultImage(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetGameCenterActivityVersionDefaultImage() error: %v", err)
	}
}

func TestBuildRelationshipData(t *testing.T) {
	data := buildRelationshipData(ResourceTypeGameCenterAchievements, []string{" ach-1 ", "", "ach-2"})
	if len(data) != 2 {
		t.Fatalf("expected 2 relationships, got %d", len(data))
	}
	if data[0].ID != "ach-1" || data[1].ID != "ach-2" {
		t.Fatalf("unexpected relationship IDs: %+v", data)
	}
}

func TestGCActivitiesOptions(t *testing.T) {
	query := &gcActivitiesQuery{}
	WithGCActivitiesLimit(20)(query)
	if query.limit != 20 {
		t.Fatalf("expected limit 20, got %d", query.limit)
	}
	WithGCActivitiesNextURL("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCActivitiesQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "20" {
		t.Fatalf("expected limit=20, got %q", values.Get("limit"))
	}
}

func TestGCActivityVersionsOptions(t *testing.T) {
	query := &gcActivityVersionsQuery{}
	WithGCActivityVersionsLimit(10)(query)
	if query.limit != 10 {
		t.Fatalf("expected limit 10, got %d", query.limit)
	}
	WithGCActivityVersionsNextURL("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCActivityVersionsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "10" {
		t.Fatalf("expected limit=10, got %q", values.Get("limit"))
	}
}

func TestGCActivityLocalizationsOptions(t *testing.T) {
	query := &gcActivityLocalizationsQuery{}
	WithGCActivityLocalizationsLimit(6)(query)
	if query.limit != 6 {
		t.Fatalf("expected limit 6, got %d", query.limit)
	}
	WithGCActivityLocalizationsNextURL("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCActivityLocalizationsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "6" {
		t.Fatalf("expected limit=6, got %q", values.Get("limit"))
	}
}

func TestGCActivityVersionReleasesOptions(t *testing.T) {
	query := &gcActivityVersionReleasesQuery{}
	WithGCActivityVersionReleasesLimit(14)(query)
	if query.limit != 14 {
		t.Fatalf("expected limit 14, got %d", query.limit)
	}
	WithGCActivityVersionReleasesNextURL("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCActivityVersionReleasesQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "14" {
		t.Fatalf("expected limit=14, got %q", values.Get("limit"))
	}
}

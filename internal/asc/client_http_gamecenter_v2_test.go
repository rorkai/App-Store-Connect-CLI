package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetGameCenterAchievementsV2_WithDetailLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails/gc-detail-1/gameCenterAchievementsV2" {
			t.Fatalf("expected path /v1/gameCenterDetails/gc-detail-1/gameCenterAchievementsV2, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementsV2(context.Background(), "gc-detail-1", "", WithGCAchievementsLimit(50)); err != nil {
		t.Fatalf("GetGameCenterAchievementsV2() error: %v", err)
	}
}

func TestGetGameCenterAchievementsV2_WithGroup(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups/group-1/gameCenterAchievementsV2" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/gameCenterAchievementsV2, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementsV2(context.Background(), "", "group-1"); err != nil {
		t.Fatalf("GetGameCenterAchievementsV2() error: %v", err)
	}
}

func TestGetGameCenterAchievementVersions_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievements/ach-1/versions" {
			t.Fatalf("expected path /v2/gameCenterAchievements/ach-1/versions, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementVersions(context.Background(), "ach-1", WithGCAchievementVersionsLimit(10)); err != nil {
		t.Fatalf("GetGameCenterAchievementVersions() error: %v", err)
	}
}

func TestGetGameCenterAchievementVersions_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v2/gameCenterAchievements/ach-1/versions?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementVersions(context.Background(), "ach-1", WithGCAchievementVersionsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterAchievementVersions() error: %v", err)
	}
}

func TestGetGameCenterAchievementVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievementVersions","id":"ver-1","attributes":{"version":1,"state":"READY_FOR_REVIEW"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementVersions/ver-1" {
			t.Fatalf("expected path /v2/gameCenterAchievementVersions/ver-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementVersion(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetGameCenterAchievementVersion() error: %v", err)
	}
}

func TestCreateGameCenterAchievementVersion(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterAchievementVersions","id":"ver-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementVersions" {
			t.Fatalf("expected path /v2/gameCenterAchievementVersions, got %s", req.URL.Path)
		}
		var payload GameCenterAchievementVersionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterAchievementVersions {
			t.Fatalf("expected type gameCenterAchievementVersions, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Achievement == nil {
			t.Fatalf("expected achievement relationship")
		}
		if payload.Data.Relationships.Achievement.Data.ID != "ach-1" {
			t.Fatalf("expected achievement ID ach-1, got %s", payload.Data.Relationships.Achievement.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateGameCenterAchievementVersion(context.Background(), "ach-1"); err != nil {
		t.Fatalf("CreateGameCenterAchievementVersion() error: %v", err)
	}
}

func TestGetGameCenterAchievementVersionLocalizations(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementVersions/ver-1/localizations" {
			t.Fatalf("expected path /v2/gameCenterAchievementVersions/ver-1/localizations, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementVersionLocalizations(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetGameCenterAchievementVersionLocalizations() error: %v", err)
	}
}

func TestGetGameCenterAchievementVersionLocalizationsRelationships(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementVersions/ver-1/relationships/localizations" {
			t.Fatalf("expected path /v2/gameCenterAchievementVersions/ver-1/relationships/localizations, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "9" {
			t.Fatalf("expected limit=9, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementVersionLocalizationsRelationships(context.Background(), "ver-1", WithLinkagesLimit(9)); err != nil {
		t.Fatalf("GetGameCenterAchievementVersionLocalizationsRelationships() error: %v", err)
	}
}

func TestCreateGameCenterAchievementLocalizationV2(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterAchievementLocalizations","id":"loc-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementLocalizations" {
			t.Fatalf("expected path /v2/gameCenterAchievementLocalizations, got %s", req.URL.Path)
		}
		var payload GameCenterAchievementLocalizationV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterAchievementLocalizations {
			t.Fatalf("expected type gameCenterAchievementLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Version == nil {
			t.Fatalf("expected version relationship")
		}
		if payload.Data.Relationships.Version.Data.ID != "ver-1" {
			t.Fatalf("expected version ID ver-1, got %s", payload.Data.Relationships.Version.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterAchievementLocalizationCreateAttributes{
		Locale:                  "en-US",
		Name:                    "First Win",
		BeforeEarnedDescription: "Before",
		AfterEarnedDescription:  "After",
	}
	if _, err := client.CreateGameCenterAchievementLocalizationV2(context.Background(), "ver-1", attrs); err != nil {
		t.Fatalf("CreateGameCenterAchievementLocalizationV2() error: %v", err)
	}
}

func TestGetGameCenterAchievementLocalizationV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievementLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementLocalizations/loc-1" {
			t.Fatalf("expected path /v2/gameCenterAchievementLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementLocalizationV2(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterAchievementLocalizationV2() error: %v", err)
	}
}

func TestUpdateGameCenterAchievementLocalizationV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievementLocalizations","id":"loc-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementLocalizations/loc-1" {
			t.Fatalf("expected path /v2/gameCenterAchievementLocalizations/loc-1, got %s", req.URL.Path)
		}
		var payload GameCenterAchievementLocalizationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "loc-1" || payload.Data.Type != ResourceTypeGameCenterAchievementLocalizations {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	attrs := GameCenterAchievementLocalizationUpdateAttributes{Name: &name}
	if _, err := client.UpdateGameCenterAchievementLocalizationV2(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterAchievementLocalizationV2() error: %v", err)
	}
}

func TestDeleteGameCenterAchievementLocalizationV2(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementLocalizations/loc-1" {
			t.Fatalf("expected path /v2/gameCenterAchievementLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterAchievementLocalizationV2(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteGameCenterAchievementLocalizationV2() error: %v", err)
	}
}

func TestGetGameCenterAchievementLocalizationImageV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievementImages","id":"img-1","attributes":{"fileName":"img.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementLocalizations/loc-1/image" {
			t.Fatalf("expected path /v2/gameCenterAchievementLocalizations/loc-1/image, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementLocalizationImageV2(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterAchievementLocalizationImageV2() error: %v", err)
	}
}

func TestGetGameCenterAchievementLocalizationImageRelationshipV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievementImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementLocalizations/loc-1/relationships/image" {
			t.Fatalf("expected path /v2/gameCenterAchievementLocalizations/loc-1/relationships/image, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementLocalizationImageRelationshipV2(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterAchievementLocalizationImageRelationshipV2() error: %v", err)
	}
}

func TestCreateGameCenterAchievementImageV2(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterAchievementImages","id":"img-1","attributes":{"fileName":"img.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementImages" {
			t.Fatalf("expected path /v2/gameCenterAchievementImages, got %s", req.URL.Path)
		}
		var payload GameCenterAchievementImageV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterAchievementImages {
			t.Fatalf("expected type gameCenterAchievementImages, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Localization == nil {
			t.Fatalf("expected localization relationship")
		}
		if payload.Data.Relationships.Localization.Data.ID != "loc-1" {
			t.Fatalf("expected localization ID loc-1, got %s", payload.Data.Relationships.Localization.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateGameCenterAchievementImageV2(context.Background(), "loc-1", "img.png", 12); err != nil {
		t.Fatalf("CreateGameCenterAchievementImageV2() error: %v", err)
	}
}

func TestGetGameCenterAchievementImageV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievementImages","id":"img-1","attributes":{"fileName":"img.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementImages/img-1" {
			t.Fatalf("expected path /v2/gameCenterAchievementImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementImageV2(context.Background(), "img-1"); err != nil {
		t.Fatalf("GetGameCenterAchievementImageV2() error: %v", err)
	}
}

func TestUpdateGameCenterAchievementImageV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievementImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementImages/img-1" {
			t.Fatalf("expected path /v2/gameCenterAchievementImages/img-1, got %s", req.URL.Path)
		}
		var payload GameCenterAchievementImageUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "img-1" || payload.Data.Type != ResourceTypeGameCenterAchievementImages {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.UpdateGameCenterAchievementImageV2(context.Background(), "img-1", true); err != nil {
		t.Fatalf("UpdateGameCenterAchievementImageV2() error: %v", err)
	}
}

func TestDeleteGameCenterAchievementImageV2(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievementImages/img-1" {
			t.Fatalf("expected path /v2/gameCenterAchievementImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterAchievementImageV2(context.Background(), "img-1"); err != nil {
		t.Fatalf("DeleteGameCenterAchievementImageV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardsV2_WithGroup(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterGroups/group-1/gameCenterLeaderboardsV2" {
			t.Fatalf("expected path /v1/gameCenterGroups/group-1/gameCenterLeaderboardsV2, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardsV2(context.Background(), "", "group-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardsV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardVersions_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboards/lb-1/versions" {
			t.Fatalf("expected path /v2/gameCenterLeaderboards/lb-1/versions, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardVersions(context.Background(), "lb-1", WithGCLeaderboardVersionsLimit(10)); err != nil {
		t.Fatalf("GetGameCenterLeaderboardVersions() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardVersions_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v2/gameCenterLeaderboards/lb-1/versions?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardVersions(context.Background(), "lb-1", WithGCLeaderboardVersionsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterLeaderboardVersions() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardVersions","id":"ver-1","attributes":{"version":1}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardVersions/ver-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardVersions/ver-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardVersion(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardVersion() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboardVersion(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterLeaderboardVersions","id":"ver-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardVersions" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardVersions, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardVersionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterLeaderboardVersions {
			t.Fatalf("expected type gameCenterLeaderboardVersions, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Leaderboard == nil {
			t.Fatalf("expected leaderboard relationship")
		}
		if payload.Data.Relationships.Leaderboard.Data.ID != "lb-1" {
			t.Fatalf("expected leaderboard ID lb-1, got %s", payload.Data.Relationships.Leaderboard.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateGameCenterLeaderboardVersion(context.Background(), "lb-1"); err != nil {
		t.Fatalf("CreateGameCenterLeaderboardVersion() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardVersionLocalizations(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardVersions/ver-1/localizations" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardVersions/ver-1/localizations, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardVersionLocalizations(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardVersionLocalizations() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardVersionLocalizationsRelationships(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardVersions/ver-1/relationships/localizations" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardVersions/ver-1/relationships/localizations, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "9" {
			t.Fatalf("expected limit=9, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardVersionLocalizationsRelationships(context.Background(), "ver-1", WithLinkagesLimit(9)); err != nil {
		t.Fatalf("GetGameCenterLeaderboardVersionLocalizationsRelationships() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboardLocalizationV2(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterLeaderboardLocalizations","id":"loc-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardLocalizations" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardLocalizations, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardLocalizationV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Version == nil {
			t.Fatalf("expected version relationship")
		}
		if payload.Data.Relationships.Version.Data.ID != "ver-1" {
			t.Fatalf("expected version ID ver-1, got %s", payload.Data.Relationships.Version.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterLeaderboardLocalizationCreateAttributes{
		Locale: "en-US",
		Name:   "High Score",
	}
	if _, err := client.CreateGameCenterLeaderboardLocalizationV2(context.Background(), "ver-1", attrs); err != nil {
		t.Fatalf("CreateGameCenterLeaderboardLocalizationV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardLocalizationV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardLocalizations/loc-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardLocalizationV2(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardLocalizationV2() error: %v", err)
	}
}

func TestUpdateGameCenterLeaderboardLocalizationV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardLocalizations","id":"loc-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardLocalizations/loc-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardLocalizations/loc-1, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardLocalizationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "loc-1" || payload.Data.Type != ResourceTypeGameCenterLeaderboardLocalizations {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	attrs := GameCenterLeaderboardLocalizationUpdateAttributes{Name: &name}
	if _, err := client.UpdateGameCenterLeaderboardLocalizationV2(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterLeaderboardLocalizationV2() error: %v", err)
	}
}

func TestDeleteGameCenterLeaderboardLocalizationV2(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardLocalizations/loc-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterLeaderboardLocalizationV2(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteGameCenterLeaderboardLocalizationV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardLocalizationImageV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardImages","id":"img-1","attributes":{"fileName":"img.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardLocalizations/loc-1/image" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardLocalizations/loc-1/image, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardLocalizationImageV2(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardLocalizationImageV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardLocalizationImageRelationshipV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardLocalizations/loc-1/relationships/image" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardLocalizations/loc-1/relationships/image, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardLocalizationImageRelationshipV2(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardLocalizationImageRelationshipV2() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboardImageV2(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterLeaderboardImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardImages" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardImages, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardImageV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Localization == nil {
			t.Fatalf("expected localization relationship")
		}
		if payload.Data.Relationships.Localization.Data.ID != "loc-1" {
			t.Fatalf("expected localization ID loc-1, got %s", payload.Data.Relationships.Localization.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateGameCenterLeaderboardImageV2(context.Background(), "loc-1", "img.png", 12); err != nil {
		t.Fatalf("CreateGameCenterLeaderboardImageV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardImageV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardImages/img-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardImageV2(context.Background(), "img-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardImageV2() error: %v", err)
	}
}

func TestUpdateGameCenterLeaderboardImageV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardImages/img-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardImages/img-1, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardImageUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "img-1" || payload.Data.Type != ResourceTypeGameCenterLeaderboardImages {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.UpdateGameCenterLeaderboardImageV2(context.Background(), "img-1", true); err != nil {
		t.Fatalf("UpdateGameCenterLeaderboardImageV2() error: %v", err)
	}
}

func TestDeleteGameCenterLeaderboardImageV2(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardImages/img-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterLeaderboardImageV2(context.Background(), "img-1"); err != nil {
		t.Fatalf("DeleteGameCenterLeaderboardImageV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetsV2_WithDetailLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails/gc-detail-1/gameCenterLeaderboardSetsV2" {
			t.Fatalf("expected path /v1/gameCenterDetails/gc-detail-1/gameCenterLeaderboardSetsV2, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "25" {
			t.Fatalf("expected limit=25, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetsV2(context.Background(), "gc-detail-1", "", WithGCLeaderboardSetsLimit(25)); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetsV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSets","id":"set-1","attributes":{"referenceName":"Season","vendorIdentifier":"com.example.season"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSets/set-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSets/set-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetV2(context.Background(), "set-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetV2() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboardSetV2_WithDetail(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterLeaderboardSets","id":"set-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSets" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSets, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardSetV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.GameCenterDetail == nil {
			t.Fatalf("expected gameCenterDetail relationship")
		}
		if payload.Data.Relationships.GameCenterDetail.Data.ID != "gc-detail-1" {
			t.Fatalf("expected gc-detail-1, got %s", payload.Data.Relationships.GameCenterDetail.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterLeaderboardSetCreateAttributes{
		ReferenceName:    "Season",
		VendorIdentifier: "com.example.season",
	}
	if _, err := client.CreateGameCenterLeaderboardSetV2(context.Background(), "gc-detail-1", "", attrs); err != nil {
		t.Fatalf("CreateGameCenterLeaderboardSetV2() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboardSetV2_WithGroup(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterLeaderboardSets","id":"set-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSets" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSets, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardSetV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.GameCenterGroup == nil {
			t.Fatalf("expected gameCenterGroup relationship")
		}
		if payload.Data.Relationships.GameCenterGroup.Data.ID != "group-1" {
			t.Fatalf("expected group-1, got %s", payload.Data.Relationships.GameCenterGroup.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterLeaderboardSetCreateAttributes{
		ReferenceName:    "Season",
		VendorIdentifier: "grp.com.example.season",
	}
	if _, err := client.CreateGameCenterLeaderboardSetV2(context.Background(), "", "group-1", attrs); err != nil {
		t.Fatalf("CreateGameCenterLeaderboardSetV2() error: %v", err)
	}
}

func TestUpdateGameCenterLeaderboardSetV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSets","id":"set-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSets/set-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSets/set-1, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardSetUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "set-1" || payload.Data.Type != ResourceTypeGameCenterLeaderboardSets {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	attrs := GameCenterLeaderboardSetUpdateAttributes{ReferenceName: &name}
	if _, err := client.UpdateGameCenterLeaderboardSetV2(context.Background(), "set-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterLeaderboardSetV2() error: %v", err)
	}
}

func TestDeleteGameCenterLeaderboardSetV2(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSets/set-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSets/set-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterLeaderboardSetV2(context.Background(), "set-1"); err != nil {
		t.Fatalf("DeleteGameCenterLeaderboardSetV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetMembersV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSets/set-1/gameCenterLeaderboards" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSets/set-1/gameCenterLeaderboards, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetMembersV2(context.Background(), "set-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetMembersV2() error: %v", err)
	}
}

func TestUpdateGameCenterLeaderboardSetMembersV2(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardSetMembersUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(payload.Data) != 2 {
			t.Fatalf("expected 2 relationship entries, got %d", len(payload.Data))
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.UpdateGameCenterLeaderboardSetMembersV2(context.Background(), "set-1", []string{"lb-1", "lb-2"}); err != nil {
		t.Fatalf("UpdateGameCenterLeaderboardSetMembersV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetVersions(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSets/set-1/versions" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSets/set-1/versions, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetVersions(context.Background(), "set-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetVersions() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSetVersions","id":"ver-1","attributes":{"version":1}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetVersions/ver-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetVersions/ver-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetVersion(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetVersion() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboardSetVersion(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterLeaderboardSetVersions","id":"ver-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetVersions" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetVersions, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardSetVersionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.LeaderboardSet == nil {
			t.Fatalf("expected leaderboardSet relationship")
		}
		if payload.Data.Relationships.LeaderboardSet.Data.ID != "set-1" {
			t.Fatalf("expected set-1, got %s", payload.Data.Relationships.LeaderboardSet.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateGameCenterLeaderboardSetVersion(context.Background(), "set-1"); err != nil {
		t.Fatalf("CreateGameCenterLeaderboardSetVersion() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetVersionLocalizations(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetVersions/ver-1/localizations" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetVersions/ver-1/localizations, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetVersionLocalizations(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetVersionLocalizations() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetVersionLocalizationsRelationships(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetVersions/ver-1/relationships/localizations" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetVersions/ver-1/relationships/localizations, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "9" {
			t.Fatalf("expected limit=9, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetVersionLocalizationsRelationships(context.Background(), "ver-1", WithLinkagesLimit(9)); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetVersionLocalizationsRelationships() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboardSetLocalizationV2(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterLeaderboardSetLocalizations","id":"loc-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetLocalizations" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetLocalizations, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardSetLocalizationV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Version == nil {
			t.Fatalf("expected version relationship")
		}
		if payload.Data.Relationships.Version.Data.ID != "ver-1" {
			t.Fatalf("expected version ID ver-1, got %s", payload.Data.Relationships.Version.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterLeaderboardSetLocalizationCreateAttributes{
		Locale: "en-US",
		Name:   "Season 1",
	}
	if _, err := client.CreateGameCenterLeaderboardSetLocalizationV2(context.Background(), "ver-1", attrs); err != nil {
		t.Fatalf("CreateGameCenterLeaderboardSetLocalizationV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetLocalizationV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSetLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetLocalizations/loc-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetLocalizationV2(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetLocalizationV2() error: %v", err)
	}
}

func TestUpdateGameCenterLeaderboardSetLocalizationV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSetLocalizations","id":"loc-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetLocalizations/loc-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetLocalizations/loc-1, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardSetLocalizationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "loc-1" || payload.Data.Type != ResourceTypeGameCenterLeaderboardSetLocalizations {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	attrs := GameCenterLeaderboardSetLocalizationUpdateAttributes{Name: &name}
	if _, err := client.UpdateGameCenterLeaderboardSetLocalizationV2(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterLeaderboardSetLocalizationV2() error: %v", err)
	}
}

func TestDeleteGameCenterLeaderboardSetLocalizationV2(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetLocalizations/loc-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterLeaderboardSetLocalizationV2(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteGameCenterLeaderboardSetLocalizationV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetLocalizationImageV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSetImages","id":"img-1","attributes":{"fileName":"img.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetLocalizations/loc-1/image" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetLocalizations/loc-1/image, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetLocalizationImageV2(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetLocalizationImageV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetLocalizationImageRelationshipV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSetImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetLocalizations/loc-1/relationships/image" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetLocalizations/loc-1/relationships/image, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetLocalizationImageRelationshipV2(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetLocalizationImageRelationshipV2() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboardSetImageV2(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterLeaderboardSetImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetImages" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetImages, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardSetImageV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Localization == nil {
			t.Fatalf("expected localization relationship")
		}
		if payload.Data.Relationships.Localization.Data.ID != "loc-1" {
			t.Fatalf("expected localization ID loc-1, got %s", payload.Data.Relationships.Localization.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateGameCenterLeaderboardSetImageV2(context.Background(), "loc-1", "img.png", 12); err != nil {
		t.Fatalf("CreateGameCenterLeaderboardSetImageV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetImageV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSetImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetImages/img-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetImageV2(context.Background(), "img-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetImageV2() error: %v", err)
	}
}

func TestUpdateGameCenterLeaderboardSetImageV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSetImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetImages/img-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetImages/img-1, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardSetImageUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "img-1" || payload.Data.Type != ResourceTypeGameCenterLeaderboardSetImages {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.UpdateGameCenterLeaderboardSetImageV2(context.Background(), "img-1", true); err != nil {
		t.Fatalf("UpdateGameCenterLeaderboardSetImageV2() error: %v", err)
	}
}

func TestDeleteGameCenterLeaderboardSetImageV2(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboardSetImages/img-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboardSetImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterLeaderboardSetImageV2(context.Background(), "img-1"); err != nil {
		t.Fatalf("DeleteGameCenterLeaderboardSetImageV2() error: %v", err)
	}
}

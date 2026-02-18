package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetGameCenterDetailID(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterDetails","id":"gc-detail-1","attributes":{"achievementEnabled":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/gameCenterDetail" {
			t.Fatalf("expected path /v1/apps/app-1/gameCenterDetail, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	id, err := client.GetGameCenterDetailID(context.Background(), "app-1")
	if err != nil {
		t.Fatalf("GetGameCenterDetailID() error: %v", err)
	}
	if id != "gc-detail-1" {
		t.Fatalf("expected gc-detail-1, got %s", id)
	}
}

func TestGetGameCenterAchievements_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"gameCenterAchievements","id":"ach-1","attributes":{"referenceName":"First Win","vendorIdentifier":"com.example.firstwin","points":10,"showBeforeEarned":true,"repeatable":false}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails/gc-detail-1/gameCenterAchievements" {
			t.Fatalf("expected path /v1/gameCenterDetails/gc-detail-1/gameCenterAchievements, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievements(context.Background(), "gc-detail-1", WithGCAchievementsLimit(50)); err != nil {
		t.Fatalf("GetGameCenterAchievements() error: %v", err)
	}
}

func TestGetGameCenterAchievements_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/gameCenterAchievements?cursor=abc"
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

	if _, err := client.GetGameCenterAchievements(context.Background(), "gc-detail-1", WithGCAchievementsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterAchievements() error: %v", err)
	}
}

func TestGetGameCenterAchievement(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievements","id":"ach-1","attributes":{"referenceName":"First Win","vendorIdentifier":"com.example.firstwin","points":10,"showBeforeEarned":true,"repeatable":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievements/ach-1" {
			t.Fatalf("expected path /v1/gameCenterAchievements/ach-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievement(context.Background(), "ach-1"); err != nil {
		t.Fatalf("GetGameCenterAchievement() error: %v", err)
	}
}

func TestCreateGameCenterAchievement(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterAchievements","id":"ach-1","attributes":{"referenceName":"First Win","vendorIdentifier":"com.example.firstwin","points":10,"showBeforeEarned":true,"repeatable":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievements" {
			t.Fatalf("expected path /v1/gameCenterAchievements, got %s", req.URL.Path)
		}
		var payload GameCenterAchievementCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterAchievements {
			t.Fatalf("expected type gameCenterAchievements, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.ReferenceName != "First Win" || payload.Data.Attributes.VendorIdentifier != "com.example.firstwin" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.GameCenterDetail == nil {
			t.Fatalf("expected gameCenterDetail relationship")
		}
		if payload.Data.Relationships.GameCenterDetail.Data.Type != ResourceTypeGameCenterDetails || payload.Data.Relationships.GameCenterDetail.Data.ID != "gc-detail-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data.Relationships.GameCenterDetail.Data)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterAchievementCreateAttributes{
		ReferenceName:    "First Win",
		VendorIdentifier: "com.example.firstwin",
		Points:           10,
		ShowBeforeEarned: true,
		Repeatable:       false,
	}
	if _, err := client.CreateGameCenterAchievement(context.Background(), "gc-detail-1", attrs); err != nil {
		t.Fatalf("CreateGameCenterAchievement() error: %v", err)
	}
}

func TestUpdateGameCenterAchievement(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievements","id":"ach-1","attributes":{"referenceName":"Updated Name","vendorIdentifier":"com.example.firstwin","points":20,"showBeforeEarned":true,"repeatable":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievements/ach-1" {
			t.Fatalf("expected path /v1/gameCenterAchievements/ach-1, got %s", req.URL.Path)
		}
		var payload GameCenterAchievementUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "ach-1" || payload.Data.Type != ResourceTypeGameCenterAchievements {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Points == nil || *payload.Data.Attributes.Points != 20 {
			t.Fatalf("expected points update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	points := 20
	attrs := GameCenterAchievementUpdateAttributes{Points: &points}
	if _, err := client.UpdateGameCenterAchievement(context.Background(), "ach-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterAchievement() error: %v", err)
	}
}

func TestDeleteGameCenterAchievement(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievements/ach-1" {
			t.Fatalf("expected path /v1/gameCenterAchievements/ach-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterAchievement(context.Background(), "ach-1"); err != nil {
		t.Fatalf("DeleteGameCenterAchievement() error: %v", err)
	}
}

func TestGetGameCenterAchievementV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievements","id":"ach-1","attributes":{"referenceName":"First Win","vendorIdentifier":"com.example.firstwin","points":10,"showBeforeEarned":true,"repeatable":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievements/ach-1" {
			t.Fatalf("expected path /v2/gameCenterAchievements/ach-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementV2(context.Background(), "ach-1"); err != nil {
		t.Fatalf("GetGameCenterAchievementV2() error: %v", err)
	}
}

func TestCreateGameCenterAchievementV2_WithDetail(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterAchievements","id":"ach-1","attributes":{"referenceName":"First Win","vendorIdentifier":"com.example.firstwin","points":10,"showBeforeEarned":true,"repeatable":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievements" {
			t.Fatalf("expected path /v2/gameCenterAchievements, got %s", req.URL.Path)
		}
		var payload GameCenterAchievementV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
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

	attrs := GameCenterAchievementCreateAttributes{
		ReferenceName:    "First Win",
		VendorIdentifier: "com.example.firstwin",
		Points:           10,
		ShowBeforeEarned: true,
		Repeatable:       false,
	}
	if _, err := client.CreateGameCenterAchievementV2(context.Background(), "gc-detail-1", "", attrs); err != nil {
		t.Fatalf("CreateGameCenterAchievementV2() error: %v", err)
	}
}

func TestCreateGameCenterAchievementV2_WithGroup(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterAchievements","id":"ach-1","attributes":{"referenceName":"Group Win","vendorIdentifier":"grp.example.win","points":10,"showBeforeEarned":true,"repeatable":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievements" {
			t.Fatalf("expected path /v2/gameCenterAchievements, got %s", req.URL.Path)
		}
		var payload GameCenterAchievementV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
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

	attrs := GameCenterAchievementCreateAttributes{
		ReferenceName:    "Group Win",
		VendorIdentifier: "grp.example.win",
		Points:           10,
		ShowBeforeEarned: true,
		Repeatable:       false,
	}
	if _, err := client.CreateGameCenterAchievementV2(context.Background(), "", "group-1", attrs); err != nil {
		t.Fatalf("CreateGameCenterAchievementV2() error: %v", err)
	}
}

func TestUpdateGameCenterAchievementV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievements","id":"ach-1","attributes":{"referenceName":"Updated Name","vendorIdentifier":"com.example.firstwin","points":20,"showBeforeEarned":true,"repeatable":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievements/ach-1" {
			t.Fatalf("expected path /v2/gameCenterAchievements/ach-1, got %s", req.URL.Path)
		}
		var payload GameCenterAchievementUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "ach-1" || payload.Data.Type != ResourceTypeGameCenterAchievements {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	points := 20
	attrs := GameCenterAchievementUpdateAttributes{Points: &points}
	if _, err := client.UpdateGameCenterAchievementV2(context.Background(), "ach-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterAchievementV2() error: %v", err)
	}
}

func TestDeleteGameCenterAchievementV2(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterAchievements/ach-1" {
			t.Fatalf("expected path /v2/gameCenterAchievements/ach-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterAchievementV2(context.Background(), "ach-1"); err != nil {
		t.Fatalf("DeleteGameCenterAchievementV2() error: %v", err)
	}
}

func TestGetGameCenterLeaderboards_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{"referenceName":"High Score","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails/gc-detail-1/gameCenterLeaderboards" {
			t.Fatalf("expected path /v1/gameCenterDetails/gc-detail-1/gameCenterLeaderboards, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboards(context.Background(), "gc-detail-1", WithGCLeaderboardsLimit(50)); err != nil {
		t.Fatalf("GetGameCenterLeaderboards() error: %v", err)
	}
}

func TestGetGameCenterLeaderboards_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/gameCenterLeaderboards?cursor=abc"
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

	if _, err := client.GetGameCenterLeaderboards(context.Background(), "gc-detail-1", WithGCLeaderboardsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterLeaderboards() error: %v", err)
	}
}

func TestGetGameCenterLeaderboard(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{"referenceName":"High Score","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboards/lb-1" {
			t.Fatalf("expected path /v1/gameCenterLeaderboards/lb-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboard(context.Background(), "lb-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboard() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboard(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{"referenceName":"High Score","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboards" {
			t.Fatalf("expected path /v1/gameCenterLeaderboards, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterLeaderboards {
			t.Fatalf("expected type gameCenterLeaderboards, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.ReferenceName != "High Score" || payload.Data.Attributes.VendorIdentifier != "com.example.highscore" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.GameCenterDetail == nil {
			t.Fatalf("expected gameCenterDetail relationship")
		}
		if payload.Data.Relationships.GameCenterDetail.Data.Type != ResourceTypeGameCenterDetails || payload.Data.Relationships.GameCenterDetail.Data.ID != "gc-detail-1" {
			t.Fatalf("unexpected relationship: %+v", payload.Data.Relationships.GameCenterDetail.Data)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterLeaderboardCreateAttributes{
		ReferenceName:    "High Score",
		VendorIdentifier: "com.example.highscore",
		DefaultFormatter: "INTEGER",
		ScoreSortType:    "DESC",
		SubmissionType:   "BEST_SCORE",
	}
	if _, err := client.CreateGameCenterLeaderboard(context.Background(), "gc-detail-1", attrs); err != nil {
		t.Fatalf("CreateGameCenterLeaderboard() error: %v", err)
	}
}

func TestUpdateGameCenterLeaderboard(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{"referenceName":"Updated Name","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboards/lb-1" {
			t.Fatalf("expected path /v1/gameCenterLeaderboards/lb-1, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "lb-1" || payload.Data.Type != ResourceTypeGameCenterLeaderboards {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.ReferenceName == nil || *payload.Data.Attributes.ReferenceName != "Updated Name" {
			t.Fatalf("expected name update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated Name"
	attrs := GameCenterLeaderboardUpdateAttributes{ReferenceName: &name}
	if _, err := client.UpdateGameCenterLeaderboard(context.Background(), "lb-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterLeaderboard() error: %v", err)
	}
}

func TestDeleteGameCenterLeaderboard(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboards/lb-1" {
			t.Fatalf("expected path /v1/gameCenterLeaderboards/lb-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterLeaderboard(context.Background(), "lb-1"); err != nil {
		t.Fatalf("DeleteGameCenterLeaderboard() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{"referenceName":"High Score","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboards/lb-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboards/lb-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardV2(context.Background(), "lb-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardV2() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboardV2_WithDetail(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{"referenceName":"High Score","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboards" {
			t.Fatalf("expected path /v2/gameCenterLeaderboards, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.GameCenterDetail == nil {
			t.Fatalf("expected gameCenterDetail relationship")
		}
		if payload.Data.Relationships.GameCenterDetail.Data.ID != "gc-detail-1" {
			t.Fatalf("expected gc-detail-1, got %s", payload.Data.Relationships.GameCenterDetail.Data.ID)
		}
		if payload.Data.Relationships.GameCenterGroup != nil {
			t.Fatalf("expected no gameCenterGroup relationship, got %+v", payload.Data.Relationships.GameCenterGroup)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterLeaderboardCreateAttributes{
		ReferenceName:    "High Score",
		VendorIdentifier: "com.example.highscore",
		DefaultFormatter: "INTEGER",
		ScoreSortType:    "DESC",
		SubmissionType:   "BEST_SCORE",
	}
	if _, err := client.CreateGameCenterLeaderboardV2(context.Background(), "gc-detail-1", "", attrs); err != nil {
		t.Fatalf("CreateGameCenterLeaderboardV2() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboardV2_WithGroup(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{"referenceName":"Group Score","vendorIdentifier":"grp.example.groupscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboards" {
			t.Fatalf("expected path /v2/gameCenterLeaderboards, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardV2CreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
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

	attrs := GameCenterLeaderboardCreateAttributes{
		ReferenceName:    "Group Score",
		VendorIdentifier: "grp.example.groupscore",
		DefaultFormatter: "INTEGER",
		ScoreSortType:    "DESC",
		SubmissionType:   "BEST_SCORE",
	}
	if _, err := client.CreateGameCenterLeaderboardV2(context.Background(), "", "group-1", attrs); err != nil {
		t.Fatalf("CreateGameCenterLeaderboardV2() error: %v", err)
	}
}

func TestUpdateGameCenterLeaderboardV2(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{"referenceName":"Updated Name","vendorIdentifier":"com.example.highscore","defaultFormatter":"INTEGER","scoreSortType":"DESC","submissionType":"BEST_SCORE"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboards/lb-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboards/lb-1, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "lb-1" || payload.Data.Type != ResourceTypeGameCenterLeaderboards {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated Name"
	attrs := GameCenterLeaderboardUpdateAttributes{ReferenceName: &name}
	if _, err := client.UpdateGameCenterLeaderboardV2(context.Background(), "lb-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterLeaderboardV2() error: %v", err)
	}
}

func TestDeleteGameCenterLeaderboardV2(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v2/gameCenterLeaderboards/lb-1" {
			t.Fatalf("expected path /v2/gameCenterLeaderboards/lb-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterLeaderboardV2(context.Background(), "lb-1"); err != nil {
		t.Fatalf("DeleteGameCenterLeaderboardV2() error: %v", err)
	}
}

func TestGetGameCenterAchievementLocalizations(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[{"type":"gameCenterAchievementLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"First Win","beforeEarnedDescription":"Win your first game","afterEarnedDescription":"You won!"}}]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievements/ach-1/localizations" {
			t.Fatalf("expected path /v1/gameCenterAchievements/ach-1/localizations, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetGameCenterAchievementLocalizations(context.Background(), "ach-1")
	if err != nil {
		t.Fatalf("GetGameCenterAchievementLocalizations() error: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "loc-1" {
		t.Fatalf("unexpected response: %+v", resp.Data)
	}
}

func TestGetGameCenterAchievementLocalizations_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		values := req.URL.Query()
		if values.Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementLocalizations(context.Background(), "ach-1", WithGCAchievementLocalizationsLimit(50)); err != nil {
		t.Fatalf("GetGameCenterAchievementLocalizations() error: %v", err)
	}
}

func TestGetGameCenterAchievementLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievementLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"First Win","beforeEarnedDescription":"Win your first game","afterEarnedDescription":"You won!"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievementLocalizations/loc-1" {
			t.Fatalf("expected path /v1/gameCenterAchievementLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetGameCenterAchievementLocalization(context.Background(), "loc-1")
	if err != nil {
		t.Fatalf("GetGameCenterAchievementLocalization() error: %v", err)
	}
	if resp.Data.ID != "loc-1" || resp.Data.Attributes.Locale != "en-US" {
		t.Fatalf("unexpected response: %+v", resp.Data)
	}
}

func TestCreateGameCenterAchievementLocalization(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterAchievementLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"First Win","beforeEarnedDescription":"Win your first game","afterEarnedDescription":"You won!"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievementLocalizations" {
			t.Fatalf("expected path /v1/gameCenterAchievementLocalizations, got %s", req.URL.Path)
		}
		var payload GameCenterAchievementLocalizationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterAchievementLocalizations {
			t.Fatalf("unexpected type: %s", payload.Data.Type)
		}
		if payload.Data.Attributes.Locale != "en-US" {
			t.Fatalf("unexpected locale: %s", payload.Data.Attributes.Locale)
		}
		if payload.Data.Attributes.Name != "First Win" {
			t.Fatalf("unexpected name: %s", payload.Data.Attributes.Name)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.GameCenterAchievement.Data.ID != "ach-1" {
			t.Fatalf("unexpected relationships: %+v", payload.Data.Relationships)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterAchievementLocalizationCreateAttributes{
		Locale:                  "en-US",
		Name:                    "First Win",
		BeforeEarnedDescription: "Win your first game",
		AfterEarnedDescription:  "You won!",
	}
	if _, err := client.CreateGameCenterAchievementLocalization(context.Background(), "ach-1", attrs); err != nil {
		t.Fatalf("CreateGameCenterAchievementLocalization() error: %v", err)
	}
}

func TestUpdateGameCenterAchievementLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievementLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"Updated Name","beforeEarnedDescription":"Win a game","afterEarnedDescription":"Winner!"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievementLocalizations/loc-1" {
			t.Fatalf("expected path /v1/gameCenterAchievementLocalizations/loc-1, got %s", req.URL.Path)
		}
		var payload GameCenterAchievementLocalizationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "loc-1" || payload.Data.Type != ResourceTypeGameCenterAchievementLocalizations {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name == nil || *payload.Data.Attributes.Name != "Updated Name" {
			t.Fatalf("expected name update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated Name"
	attrs := GameCenterAchievementLocalizationUpdateAttributes{Name: &name}
	if _, err := client.UpdateGameCenterAchievementLocalization(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterAchievementLocalization() error: %v", err)
	}
}

func TestDeleteGameCenterAchievementLocalization(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievementLocalizations/loc-1" {
			t.Fatalf("expected path /v1/gameCenterAchievementLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterAchievementLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteGameCenterAchievementLocalization() error: %v", err)
	}
}

func TestCreateGameCenterLeaderboardEntrySubmission(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterLeaderboardEntrySubmissions","id":"sub-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardEntrySubmissions" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardEntrySubmissions, got %s", req.URL.Path)
		}
		var payload GameCenterLeaderboardEntrySubmissionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterLeaderboardEntrySubmissions {
			t.Fatalf("expected type gameCenterLeaderboardEntrySubmissions, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.VendorIdentifier != "com.example.leaderboard" || payload.Data.Attributes.Score != "100" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterLeaderboardEntrySubmissionAttributes{
		VendorIdentifier: "com.example.leaderboard",
		Score:            "100",
		BundleID:         "com.example.app",
		ScopedPlayerID:   "player-1",
	}
	if _, err := client.CreateGameCenterLeaderboardEntrySubmission(context.Background(), attrs); err != nil {
		t.Fatalf("CreateGameCenterLeaderboardEntrySubmission() error: %v", err)
	}
}

func TestCreateGameCenterPlayerAchievementSubmission(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterPlayerAchievementSubmissions","id":"sub-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterPlayerAchievementSubmissions" {
			t.Fatalf("expected path /v1/gameCenterPlayerAchievementSubmissions, got %s", req.URL.Path)
		}
		var payload GameCenterPlayerAchievementSubmissionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterPlayerAchievementSubmissions {
			t.Fatalf("expected type gameCenterPlayerAchievementSubmissions, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.VendorIdentifier != "com.example.achievement" || payload.Data.Attributes.PercentageAchieved != 100 {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterPlayerAchievementSubmissionAttributes{
		VendorIdentifier:   "com.example.achievement",
		PercentageAchieved: 100,
		BundleID:           "com.example.app",
		ScopedPlayerID:     "player-1",
	}
	if _, err := client.CreateGameCenterPlayerAchievementSubmission(context.Background(), attrs); err != nil {
		t.Fatalf("CreateGameCenterPlayerAchievementSubmission() error: %v", err)
	}
}

func TestGetGameCenterAchievementGroupAchievement(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievements","id":"ach-1","attributes":{"referenceName":"Group Achievement"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievements/ach-1/groupAchievement" {
			t.Fatalf("expected path /v1/gameCenterAchievements/ach-1/groupAchievement, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementGroupAchievement(context.Background(), "ach-1"); err != nil {
		t.Fatalf("GetGameCenterAchievementGroupAchievement() error: %v", err)
	}
}

func TestGetGameCenterAchievementLocalizationAchievement(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievements","id":"ach-1","attributes":{"referenceName":"Achievement"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievementLocalizations/loc-1/gameCenterAchievement" {
			t.Fatalf("expected path /v1/gameCenterAchievementLocalizations/loc-1/gameCenterAchievement, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementLocalizationAchievement(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterAchievementLocalizationAchievement() error: %v", err)
	}
}

func TestGetGameCenterAchievementLocalizationImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievementImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievementLocalizations/loc-1/gameCenterAchievementImage" {
			t.Fatalf("expected path /v1/gameCenterAchievementLocalizations/loc-1/gameCenterAchievementImage, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterAchievementLocalizationImage(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterAchievementLocalizationImage() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardGroupLeaderboard(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboards","id":"lb-1","attributes":{"referenceName":"Group Leaderboard"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboards/lb-1/groupLeaderboard" {
			t.Fatalf("expected path /v1/gameCenterLeaderboards/lb-1/groupLeaderboard, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardGroupLeaderboard(context.Background(), "lb-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardGroupLeaderboard() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardLocalizationImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardLocalizations/loc-1/gameCenterLeaderboardImage" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardLocalizations/loc-1/gameCenterLeaderboardImage, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardLocalizationImage(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardLocalizationImage() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetGroupLeaderboardSet(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSets","id":"set-1","attributes":{"referenceName":"Group Set"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSets/set-1/groupLeaderboardSet" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSets/set-1/groupLeaderboardSet, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetGroupLeaderboardSet(context.Background(), "set-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetGroupLeaderboardSet() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetLocalizationImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSetImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSetLocalizations/loc-1/gameCenterLeaderboardSetImage" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSetLocalizations/loc-1/gameCenterLeaderboardSetImage, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetLocalizationImage(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetLocalizationImage() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSetLocalizationImageRelationship(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSetImages","id":"img-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSetLocalizations/loc-1/relationships/gameCenterLeaderboardSetImage" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSetLocalizations/loc-1/relationships/gameCenterLeaderboardSetImage, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterLeaderboardSetLocalizationImageRelationship(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetLocalizationImageRelationship() error: %v", err)
	}
}

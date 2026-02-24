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

func TestGetGameCenterChallenges_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails/gc-detail-1/gameCenterChallenges" {
			t.Fatalf("expected path /v1/gameCenterDetails/gc-detail-1/gameCenterChallenges, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallenges(context.Background(), "gc-detail-1", WithGCChallengesLimit(50)); err != nil {
		t.Fatalf("GetGameCenterChallenges() error: %v", err)
	}
}

func TestGetGameCenterChallenges_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/gameCenterChallenges?cursor=abc"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallenges(context.Background(), "gc-detail-1", WithGCChallengesNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterChallenges() error: %v", err)
	}
}

func TestGetGameCenterChallenge(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterChallenges","id":"ch-1","attributes":{"referenceName":"Weekly","vendorIdentifier":"com.test.weekly","challengeType":"LEADERBOARD"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallenges/ch-1" {
			t.Fatalf("expected path /v1/gameCenterChallenges/ch-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallenge(context.Background(), "ch-1"); err != nil {
		t.Fatalf("GetGameCenterChallenge() error: %v", err)
	}
}

func TestCreateGameCenterChallenge_WithDetail(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterChallenges","id":"ch-1","attributes":{"referenceName":"Weekly","vendorIdentifier":"com.test.weekly","challengeType":"LEADERBOARD"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallenges" {
			t.Fatalf("expected path /v1/gameCenterChallenges, got %s", req.URL.Path)
		}
		var payload GameCenterChallengeCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterChallenges {
			t.Fatalf("expected type gameCenterChallenges, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.ReferenceName != "Weekly" || payload.Data.Attributes.VendorIdentifier != "com.test.weekly" {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.GameCenterDetail == nil {
			t.Fatalf("expected gameCenterDetail relationship")
		}
		if payload.Data.Relationships.GameCenterDetail.Data.ID != "gc-detail-1" {
			t.Fatalf("expected gc-detail-1, got %s", payload.Data.Relationships.GameCenterDetail.Data.ID)
		}
		if payload.Data.Relationships.Leaderboard == nil || payload.Data.Relationships.Leaderboard.Data.ID != "lb-1" {
			t.Fatalf("expected leaderboard relationship lb-1")
		}
		if payload.Data.Relationships.GameCenterGroup != nil {
			t.Fatalf("expected no group relationship, got %+v", payload.Data.Relationships.GameCenterGroup)
		}
		assertAuthorized(t, req)
	}, response)

	repeatable := true
	attrs := GameCenterChallengeCreateAttributes{
		ReferenceName:    "Weekly",
		VendorIdentifier: "com.test.weekly",
		ChallengeType:    "LEADERBOARD",
		Repeatable:       &repeatable,
	}
	if _, err := client.CreateGameCenterChallenge(context.Background(), "gc-detail-1", attrs, "lb-1", ""); err != nil {
		t.Fatalf("CreateGameCenterChallenge() error: %v", err)
	}
}

func TestCreateGameCenterChallenge_WithGroup(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterChallenges","id":"ch-1","attributes":{"referenceName":"Weekly","vendorIdentifier":"com.test.weekly","challengeType":"LEADERBOARD"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallenges" {
			t.Fatalf("expected path /v1/gameCenterChallenges, got %s", req.URL.Path)
		}
		var payload GameCenterChallengeCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterChallenges {
			t.Fatalf("expected type gameCenterChallenges, got %q", payload.Data.Type)
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
		if payload.Data.Relationships.Leaderboard == nil || payload.Data.Relationships.Leaderboard.Data.ID != "lb-1" {
			t.Fatalf("expected leaderboard relationship lb-1")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterChallengeCreateAttributes{
		ReferenceName:    "Weekly",
		VendorIdentifier: "com.test.weekly",
		ChallengeType:    "LEADERBOARD",
	}
	if _, err := client.CreateGameCenterChallenge(context.Background(), "", attrs, "lb-1", "group-1"); err != nil {
		t.Fatalf("CreateGameCenterChallenge() error: %v", err)
	}
}

func TestUpdateGameCenterChallenge(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterChallenges","id":"ch-1","attributes":{"referenceName":"Updated"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallenges/ch-1" {
			t.Fatalf("expected path /v1/gameCenterChallenges/ch-1, got %s", req.URL.Path)
		}
		var payload GameCenterChallengeUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "ch-1" || payload.Data.Type != ResourceTypeGameCenterChallenges {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.ReferenceName == nil || *payload.Data.Attributes.ReferenceName != "Updated" {
			t.Fatalf("expected referenceName update, got %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Leaderboard == nil || payload.Data.Relationships.Leaderboard.Data.ID != "lb-2" {
			t.Fatalf("expected leaderboard relationship lb-2")
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	attrs := GameCenterChallengeUpdateAttributes{ReferenceName: &name}
	if _, err := client.UpdateGameCenterChallenge(context.Background(), "ch-1", attrs, "lb-2"); err != nil {
		t.Fatalf("UpdateGameCenterChallenge() error: %v", err)
	}
}

func TestDeleteGameCenterChallenge(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallenges/ch-1" {
			t.Fatalf("expected path /v1/gameCenterChallenges/ch-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterChallenge(context.Background(), "ch-1"); err != nil {
		t.Fatalf("DeleteGameCenterChallenge() error: %v", err)
	}
}

func TestGetGameCenterChallengeVersions_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallenges/ch-1/versions" {
			t.Fatalf("expected path /v1/gameCenterChallenges/ch-1/versions, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "20" {
			t.Fatalf("expected limit=20, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallengeVersions(context.Background(), "ch-1", WithGCChallengeVersionsLimit(20)); err != nil {
		t.Fatalf("GetGameCenterChallengeVersions() error: %v", err)
	}
}

func TestGetGameCenterChallengeVersions_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterChallenges/ch-1/versions?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallengeVersions(context.Background(), "ch-1", WithGCChallengeVersionsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterChallengeVersions() error: %v", err)
	}
}

func TestGetGameCenterChallengeVersion(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterChallengeVersions","id":"ver-1","attributes":{"version":1}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeVersions/ver-1" {
			t.Fatalf("expected path /v1/gameCenterChallengeVersions/ver-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallengeVersion(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetGameCenterChallengeVersion() error: %v", err)
	}
}

func TestCreateGameCenterChallengeVersion(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterChallengeVersions","id":"ver-2","attributes":{"version":2}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeVersions" {
			t.Fatalf("expected path /v1/gameCenterChallengeVersions, got %s", req.URL.Path)
		}
		var payload GameCenterChallengeVersionCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterChallengeVersions {
			t.Fatalf("expected type gameCenterChallengeVersions, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Challenge == nil || payload.Data.Relationships.Challenge.Data.ID != "ch-1" {
			t.Fatalf("expected challenge relationship ch-1")
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateGameCenterChallengeVersion(context.Background(), "ch-1"); err != nil {
		t.Fatalf("CreateGameCenterChallengeVersion() error: %v", err)
	}
}

func TestGetGameCenterChallengeLocalizations_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeVersions/ver-1/localizations" {
			t.Fatalf("expected path /v1/gameCenterChallengeVersions/ver-1/localizations, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallengeLocalizations(context.Background(), "ver-1", WithGCChallengeLocalizationsLimit(10)); err != nil {
		t.Fatalf("GetGameCenterChallengeLocalizations() error: %v", err)
	}
}

func TestGetGameCenterChallengeLocalizations_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterChallengeVersions/ver-1/localizations?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallengeLocalizations(context.Background(), "ver-1", WithGCChallengeLocalizationsNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterChallengeLocalizations() error: %v", err)
	}
}

func TestGetGameCenterChallengeLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterChallengeLocalizations","id":"loc-1","attributes":{"locale":"en-US","name":"Weekly","description":"Desc"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeLocalizations/loc-1" {
			t.Fatalf("expected path /v1/gameCenterChallengeLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallengeLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterChallengeLocalization() error: %v", err)
	}
}

func TestCreateGameCenterChallengeLocalization(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterChallengeLocalizations","id":"loc-2","attributes":{"locale":"en-US","name":"Weekly","description":"Desc"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeLocalizations" {
			t.Fatalf("expected path /v1/gameCenterChallengeLocalizations, got %s", req.URL.Path)
		}
		var payload GameCenterChallengeLocalizationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterChallengeLocalizations {
			t.Fatalf("expected type gameCenterChallengeLocalizations, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Version == nil || payload.Data.Relationships.Version.Data.ID != "ver-1" {
			t.Fatalf("expected version relationship ver-1")
		}
		assertAuthorized(t, req)
	}, response)

	attrs := GameCenterChallengeLocalizationCreateAttributes{
		Locale:      "en-US",
		Name:        "Weekly",
		Description: "Desc",
	}
	if _, err := client.CreateGameCenterChallengeLocalization(context.Background(), "ver-1", attrs); err != nil {
		t.Fatalf("CreateGameCenterChallengeLocalization() error: %v", err)
	}
}

func TestUpdateGameCenterChallengeLocalization(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterChallengeLocalizations","id":"loc-1","attributes":{"name":"Updated","description":"Desc"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeLocalizations/loc-1" {
			t.Fatalf("expected path /v1/gameCenterChallengeLocalizations/loc-1, got %s", req.URL.Path)
		}
		var payload GameCenterChallengeLocalizationUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "loc-1" || payload.Data.Type != ResourceTypeGameCenterChallengeLocalizations {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name == nil || *payload.Data.Attributes.Name != "Updated" {
			t.Fatalf("expected name update, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	name := "Updated"
	attrs := GameCenterChallengeLocalizationUpdateAttributes{Name: &name}
	if _, err := client.UpdateGameCenterChallengeLocalization(context.Background(), "loc-1", attrs); err != nil {
		t.Fatalf("UpdateGameCenterChallengeLocalization() error: %v", err)
	}
}

func TestDeleteGameCenterChallengeLocalization(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeLocalizations/loc-1" {
			t.Fatalf("expected path /v1/gameCenterChallengeLocalizations/loc-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterChallengeLocalization(context.Background(), "loc-1"); err != nil {
		t.Fatalf("DeleteGameCenterChallengeLocalization() error: %v", err)
	}
}

func TestGetGameCenterChallengeImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterChallengeImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeImages/img-1" {
			t.Fatalf("expected path /v1/gameCenterChallengeImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallengeImage(context.Background(), "img-1"); err != nil {
		t.Fatalf("GetGameCenterChallengeImage() error: %v", err)
	}
}

func TestCreateGameCenterChallengeImage(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterChallengeImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeImages" {
			t.Fatalf("expected path /v1/gameCenterChallengeImages, got %s", req.URL.Path)
		}
		var payload GameCenterChallengeImageCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterChallengeImages {
			t.Fatalf("expected type gameCenterChallengeImages, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.FileName != "image.png" || payload.Data.Attributes.FileSize != 12 {
			t.Fatalf("unexpected attributes: %+v", payload.Data.Attributes)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Localization == nil || payload.Data.Relationships.Localization.Data.ID != "loc-1" {
			t.Fatalf("expected localization relationship loc-1")
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateGameCenterChallengeImage(context.Background(), "loc-1", "image.png", 12); err != nil {
		t.Fatalf("CreateGameCenterChallengeImage() error: %v", err)
	}
}

func TestUpdateGameCenterChallengeImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterChallengeImages","id":"img-1","attributes":{"uploaded":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeImages/img-1" {
			t.Fatalf("expected path /v1/gameCenterChallengeImages/img-1, got %s", req.URL.Path)
		}
		var payload GameCenterChallengeImageUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.ID != "img-1" || payload.Data.Type != ResourceTypeGameCenterChallengeImages {
			t.Fatalf("unexpected payload: %+v", payload.Data)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Uploaded == nil || !*payload.Data.Attributes.Uploaded {
			t.Fatalf("expected uploaded true, got %+v", payload.Data.Attributes)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.UpdateGameCenterChallengeImage(context.Background(), "img-1", true); err != nil {
		t.Fatalf("UpdateGameCenterChallengeImage() error: %v", err)
	}
}

func TestDeleteGameCenterChallengeImage(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeImages/img-1" {
			t.Fatalf("expected path /v1/gameCenterChallengeImages/img-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterChallengeImage(context.Background(), "img-1"); err != nil {
		t.Fatalf("DeleteGameCenterChallengeImage() error: %v", err)
	}
}

func TestUploadGameCenterChallengeImage(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "challenge.png")
	if err := os.WriteFile(filePath, []byte("challenge-image"), 0o600); err != nil {
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
		case "/v1/gameCenterChallengeImages":
			body := fmt.Sprintf(`{"data":{"type":"gameCenterChallengeImages","id":"img-1","attributes":{"fileName":"%s","fileSize":%d,"uploadOperations":[{"method":"PUT","url":"%s","length":%d,"offset":0}]}}}`, fileInfo.Name(), fileInfo.Size(), server.URL, fileInfo.Size())
			return jsonResponse(http.StatusCreated, body), nil
		case "/v1/gameCenterChallengeImages/img-1":
			body := fmt.Sprintf(`{"data":{"type":"gameCenterChallengeImages","id":"img-1","attributes":{"fileName":"%s","fileSize":%d,"uploaded":true,"assetDeliveryState":{"state":"COMPLETE"}}}}`, fileInfo.Name(), fileInfo.Size())
			return jsonResponse(http.StatusOK, body), nil
		default:
			return jsonResponse(http.StatusNotFound, `{"errors":[{"title":"not found"}]}`), nil
		}
	})

	result, err := client.UploadGameCenterChallengeImage(context.Background(), "loc-1", filePath)
	if err != nil {
		t.Fatalf("UploadGameCenterChallengeImage() error: %v", err)
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

func TestGetGameCenterChallengeVersionRelease(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterChallengeVersionReleases","id":"rel-1","attributes":{"live":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeVersionReleases/rel-1" {
			t.Fatalf("expected path /v1/gameCenterChallengeVersionReleases/rel-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallengeVersionRelease(context.Background(), "rel-1"); err != nil {
		t.Fatalf("GetGameCenterChallengeVersionRelease() error: %v", err)
	}
}

func TestGetGameCenterChallengeVersionReleases_WithLimit(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails/gc-detail-1/challengeReleases" {
			t.Fatalf("expected path /v1/gameCenterDetails/gc-detail-1/challengeReleases, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "15" {
			t.Fatalf("expected limit=15, got %q", req.URL.Query().Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallengeVersionReleases(context.Background(), "gc-detail-1", WithGCChallengeVersionReleasesLimit(15)); err != nil {
		t.Fatalf("GetGameCenterChallengeVersionReleases() error: %v", err)
	}
}

func TestGetGameCenterChallengeVersionReleases_UsesNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-1/challengeReleases?cursor=next"
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.String() != next {
			t.Fatalf("expected URL %q, got %q", next, req.URL.String())
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallengeVersionReleases(context.Background(), "gc-detail-1", WithGCChallengeVersionReleasesNextURL(next)); err != nil {
		t.Fatalf("GetGameCenterChallengeVersionReleases() error: %v", err)
	}
}

func TestCreateGameCenterChallengeVersionRelease(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterChallengeVersionReleases","id":"rel-2","attributes":{"live":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeVersionReleases" {
			t.Fatalf("expected path /v1/gameCenterChallengeVersionReleases, got %s", req.URL.Path)
		}
		var payload GameCenterChallengeVersionReleaseCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeGameCenterChallengeVersionReleases {
			t.Fatalf("expected type gameCenterChallengeVersionReleases, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.Version == nil || payload.Data.Relationships.Version.Data.ID != "ver-1" {
			t.Fatalf("expected version relationship ver-1")
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateGameCenterChallengeVersionRelease(context.Background(), "ver-1"); err != nil {
		t.Fatalf("CreateGameCenterChallengeVersionRelease() error: %v", err)
	}
}

func TestDeleteGameCenterChallengeVersionRelease(t *testing.T) {
	response := jsonResponse(http.StatusNoContent, "")
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeVersionReleases/rel-1" {
			t.Fatalf("expected path /v1/gameCenterChallengeVersionReleases/rel-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if err := client.DeleteGameCenterChallengeVersionRelease(context.Background(), "rel-1"); err != nil {
		t.Fatalf("DeleteGameCenterChallengeVersionRelease() error: %v", err)
	}
}

func TestGCChallengesOptions(t *testing.T) {
	query := &gcChallengesQuery{}
	if got := buildGCChallengesQuery(query); got != "" {
		t.Fatalf("expected empty query, got %q", got)
	}
	WithGCChallengesLimit(25)(query)
	if query.limit != 25 {
		t.Fatalf("expected limit 25, got %d", query.limit)
	}
	WithGCChallengesNextURL(" https://example.com/next ")(query)
	if query.nextURL != "https://example.com/next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCChallengesQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "25" {
		t.Fatalf("expected limit=25, got %q", values.Get("limit"))
	}
}

func TestGCChallengeVersionsOptions(t *testing.T) {
	query := &gcChallengeVersionsQuery{}
	WithGCChallengeVersionsLimit(10)(query)
	if query.limit != 10 {
		t.Fatalf("expected limit 10, got %d", query.limit)
	}
	WithGCChallengeVersionsNextURL("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCChallengeVersionsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "10" {
		t.Fatalf("expected limit=10, got %q", values.Get("limit"))
	}
}

func TestGCChallengeLocalizationsOptions(t *testing.T) {
	query := &gcChallengeLocalizationsQuery{}
	WithGCChallengeLocalizationsLimit(5)(query)
	if query.limit != 5 {
		t.Fatalf("expected limit 5, got %d", query.limit)
	}
	WithGCChallengeLocalizationsNextURL("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCChallengeLocalizationsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "5" {
		t.Fatalf("expected limit=5, got %q", values.Get("limit"))
	}
}

func TestGetGameCenterChallengeLocalizationImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterChallengeImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeLocalizations/loc-1/image" {
			t.Fatalf("expected path /v1/gameCenterChallengeLocalizations/loc-1/image, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallengeLocalizationImage(context.Background(), "loc-1"); err != nil {
		t.Fatalf("GetGameCenterChallengeLocalizationImage() error: %v", err)
	}
}

func TestGetGameCenterChallengeVersionDefaultImage(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterChallengeImages","id":"img-1","attributes":{"fileName":"image.png","fileSize":12}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterChallengeVersions/ver-1/defaultImage" {
			t.Fatalf("expected path /v1/gameCenterChallengeVersions/ver-1/defaultImage, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterChallengeVersionDefaultImage(context.Background(), "ver-1"); err != nil {
		t.Fatalf("GetGameCenterChallengeVersionDefaultImage() error: %v", err)
	}
}

func TestGCChallengeVersionReleasesOptions(t *testing.T) {
	query := &gcChallengeVersionReleasesQuery{}
	WithGCChallengeVersionReleasesLimit(12)(query)
	if query.limit != 12 {
		t.Fatalf("expected limit 12, got %d", query.limit)
	}
	WithGCChallengeVersionReleasesNextURL("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCChallengeVersionReleasesQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "12" {
		t.Fatalf("expected limit=12, got %q", values.Get("limit"))
	}
}

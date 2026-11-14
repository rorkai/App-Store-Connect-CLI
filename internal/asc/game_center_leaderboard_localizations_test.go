package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func newGCLeaderboardLocalizationTestClient(t *testing.T, check func(*http.Request), response *http.Response) *Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if check != nil {
			check(req)
		}
		return response, nil
	})

	return &Client{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}
}

func gcLeaderboardLocalizationJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGetGameCenterLeaderboardLocalizations(t *testing.T) {
	response := gcLeaderboardLocalizationJSONResponse(http.StatusOK, `{
		"data": [{
			"type": "gameCenterLeaderboardLocalizations",
			"id": "loc-123",
			"attributes": {
				"locale": "en-US",
				"name": "High Score",
				"formatterOverride": "",
				"formatterSuffix": " pts",
				"formatterSuffixSingular": " pt"
			}
		}],
		"links": {
			"self": "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboards/lb-123/localizations"
		}
	}`)

	client := newGCLeaderboardLocalizationTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboards/lb-123/localizations" {
			t.Fatalf("expected path /v1/gameCenterLeaderboards/lb-123/localizations, got %s", req.URL.Path)
		}
	}, response)

	resp, err := client.GetGameCenterLeaderboardLocalizations(context.Background(), "lb-123")
	if err != nil {
		t.Fatalf("GetGameCenterLeaderboardLocalizations() error: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 localization, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != "loc-123" {
		t.Fatalf("expected ID loc-123, got %s", resp.Data[0].ID)
	}
	if resp.Data[0].Attributes.Locale != "en-US" {
		t.Fatalf("expected locale en-US, got %s", resp.Data[0].Attributes.Locale)
	}
	if resp.Data[0].Attributes.Name != "High Score" {
		t.Fatalf("expected name 'High Score', got %s", resp.Data[0].Attributes.Name)
	}
	if resp.Data[0].Attributes.FormatterSuffix == nil {
		t.Fatalf("expected formatterSuffix to be set")
	}
	if *resp.Data[0].Attributes.FormatterSuffix != " pts" {
		t.Fatalf("expected formatterSuffix ' pts', got %s", *resp.Data[0].Attributes.FormatterSuffix)
	}
}

func TestGetGameCenterLeaderboardLocalization(t *testing.T) {
	response := gcLeaderboardLocalizationJSONResponse(http.StatusOK, `{
		"data": {
			"type": "gameCenterLeaderboardLocalizations",
			"id": "loc-456",
			"attributes": {
				"locale": "de-DE",
				"name": "Highscore",
				"formatterOverride": "",
				"formatterSuffix": " Punkte",
				"formatterSuffixSingular": " Punkt"
			}
		}
	}`)

	client := newGCLeaderboardLocalizationTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardLocalizations/loc-456" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardLocalizations/loc-456, got %s", req.URL.Path)
		}
	}, response)

	resp, err := client.GetGameCenterLeaderboardLocalization(context.Background(), "loc-456")
	if err != nil {
		t.Fatalf("GetGameCenterLeaderboardLocalization() error: %v", err)
	}

	if resp.Data.ID != "loc-456" {
		t.Fatalf("expected ID loc-456, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.Locale != "de-DE" {
		t.Fatalf("expected locale de-DE, got %s", resp.Data.Attributes.Locale)
	}
	if resp.Data.Attributes.Name != "Highscore" {
		t.Fatalf("expected name 'Highscore', got %s", resp.Data.Attributes.Name)
	}
}

func TestCreateGameCenterLeaderboardLocalization(t *testing.T) {
	response := gcLeaderboardLocalizationJSONResponse(http.StatusCreated, `{
		"data": {
			"type": "gameCenterLeaderboardLocalizations",
			"id": "loc-789",
			"attributes": {
				"locale": "fr-FR",
				"name": "Meilleur Score",
				"formatterOverride": "",
				"formatterSuffix": " points",
				"formatterSuffixSingular": " point"
			}
		}
	}`)

	client := newGCLeaderboardLocalizationTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardLocalizations" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardLocalizations, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload GameCenterLeaderboardLocalizationCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeGameCenterLeaderboardLocalizations {
			t.Fatalf("expected type gameCenterLeaderboardLocalizations, got %s", payload.Data.Type)
		}
		if payload.Data.Attributes.Locale != "fr-FR" {
			t.Fatalf("expected locale fr-FR, got %s", payload.Data.Attributes.Locale)
		}
		if payload.Data.Attributes.Name != "Meilleur Score" {
			t.Fatalf("expected name 'Meilleur Score', got %s", payload.Data.Attributes.Name)
		}
		if payload.Data.Relationships.GameCenterLeaderboard.Data.ID != "lb-123" {
			t.Fatalf("expected leaderboard ID lb-123, got %s", payload.Data.Relationships.GameCenterLeaderboard.Data.ID)
		}
	}, response)

	formatterSuffix := " points"
	formatterSuffixSingular := " point"
	attrs := GameCenterLeaderboardLocalizationCreateAttributes{
		Locale:                  "fr-FR",
		Name:                    "Meilleur Score",
		FormatterSuffix:         &formatterSuffix,
		FormatterSuffixSingular: &formatterSuffixSingular,
	}

	resp, err := client.CreateGameCenterLeaderboardLocalization(context.Background(), "lb-123", attrs)
	if err != nil {
		t.Fatalf("CreateGameCenterLeaderboardLocalization() error: %v", err)
	}

	if resp.Data.ID != "loc-789" {
		t.Fatalf("expected ID loc-789, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.Locale != "fr-FR" {
		t.Fatalf("expected locale fr-FR, got %s", resp.Data.Attributes.Locale)
	}
}

func TestUpdateGameCenterLeaderboardLocalization(t *testing.T) {
	response := gcLeaderboardLocalizationJSONResponse(http.StatusOK, `{
		"data": {
			"type": "gameCenterLeaderboardLocalizations",
			"id": "loc-999",
			"attributes": {
				"locale": "en-US",
				"name": "Top Score",
				"formatterOverride": "",
				"formatterSuffix": " pts",
				"formatterSuffixSingular": " pt"
			}
		}
	}`)

	client := newGCLeaderboardLocalizationTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardLocalizations/loc-999" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardLocalizations/loc-999, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload GameCenterLeaderboardLocalizationUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.ID != "loc-999" {
			t.Fatalf("expected id loc-999, got %s", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.Name == nil {
			t.Fatalf("expected name to be set")
		}
		if *payload.Data.Attributes.Name != "Top Score" {
			t.Fatalf("expected name 'Top Score', got %s", *payload.Data.Attributes.Name)
		}
	}, response)

	newName := "Top Score"
	attrs := GameCenterLeaderboardLocalizationUpdateAttributes{
		Name: &newName,
	}

	resp, err := client.UpdateGameCenterLeaderboardLocalization(context.Background(), "loc-999", attrs)
	if err != nil {
		t.Fatalf("UpdateGameCenterLeaderboardLocalization() error: %v", err)
	}

	if resp.Data.ID != "loc-999" {
		t.Fatalf("expected ID loc-999, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.Name != "Top Score" {
		t.Fatalf("expected name 'Top Score', got %s", resp.Data.Attributes.Name)
	}
}

func TestDeleteGameCenterLeaderboardLocalization(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusNoContent,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader("")),
	}

	client := newGCLeaderboardLocalizationTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardLocalizations/loc-123" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardLocalizations/loc-123, got %s", req.URL.Path)
		}
	}, response)

	err := client.DeleteGameCenterLeaderboardLocalization(context.Background(), "loc-123")
	if err != nil {
		t.Fatalf("DeleteGameCenterLeaderboardLocalization() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardLocalizations_WithLimit(t *testing.T) {
	response := gcLeaderboardLocalizationJSONResponse(http.StatusOK, `{
		"data": [],
		"links": {
			"self": "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboards/lb-123/localizations?limit=50"
		}
	}`)

	client := newGCLeaderboardLocalizationTestClient(t, func(req *http.Request) {
		if req.URL.Query().Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %s", req.URL.Query().Get("limit"))
		}
	}, response)

	_, err := client.GetGameCenterLeaderboardLocalizations(context.Background(), "lb-123", WithGCLeaderboardLocalizationsLimit(50))
	if err != nil {
		t.Fatalf("GetGameCenterLeaderboardLocalizations() error: %v", err)
	}
}

func TestGetAllGameCenterLeaderboardLocalizations_FollowsPagination(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboards/lb-123/localizations?cursor=abc"
	client := newTestClient(
		t, func(req *http.Request) {
			assertAuthorized(t, req)
		},
		jsonResponse(http.StatusOK, `{"data":[{"type":"gameCenterLeaderboardLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}],"links":{"next":"`+next+`"}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"gameCenterLeaderboardLocalizations","id":"loc-2","attributes":{"locale":"ja"}}]}`),
	)

	resp, err := client.GetAllGameCenterLeaderboardLocalizations(context.Background(), "lb-123")
	if err != nil {
		t.Fatalf("GetAllGameCenterLeaderboardLocalizations() error: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 localizations, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != "loc-1" || resp.Data[1].ID != "loc-2" {
		t.Fatalf("expected aggregated localizations loc-1 and loc-2, got %q and %q", resp.Data[0].ID, resp.Data[1].ID)
	}
}

func TestGetAllGameCenterLeaderboardLocalizations_RejectsRepeatedNextURL(t *testing.T) {
	next := "https://api.appstoreconnect.apple.com/v1/gameCenterLeaderboards/lb-123/localizations?cursor=repeat"
	requests := 0
	client := newTestClient(
		t, func(req *http.Request) {
			requests++
			assertAuthorized(t, req)
		},
		jsonResponse(http.StatusOK, `{"data":[{"type":"gameCenterLeaderboardLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}],"links":{"next":"`+next+`"}}`),
		jsonResponse(http.StatusOK, `{"data":[{"type":"gameCenterLeaderboardLocalizations","id":"loc-2","attributes":{"locale":"ja"}}],"links":{"next":"`+next+`"}}`),
	)

	_, err := client.GetAllGameCenterLeaderboardLocalizations(context.Background(), "lb-123")
	if !errors.Is(err, ErrRepeatedPaginationURL) {
		t.Fatalf("expected ErrRepeatedPaginationURL, got %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected repeated next URL to stop after two requests, got %d", requests)
	}
}

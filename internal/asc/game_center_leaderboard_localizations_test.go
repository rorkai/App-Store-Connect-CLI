package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
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

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

func newGCLeaderboardSetTestClient(t *testing.T, check func(*http.Request), response *http.Response) *Client {
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

func gcLeaderboardSetJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestCreateGameCenterLeaderboardSet(t *testing.T) {
	response := gcLeaderboardSetJSONResponse(http.StatusCreated, `{
		"data": {
			"type": "gameCenterLeaderboardSets",
			"id": "set-789",
			"attributes": {
				"referenceName": "Monthly Tournament",
				"vendorIdentifier": "com.example.monthly"
			}
		}
	}`)

	client := newGCLeaderboardSetTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSets" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSets, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload GameCenterLeaderboardSetCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeGameCenterLeaderboardSets {
			t.Fatalf("expected type gameCenterLeaderboardSets, got %s", payload.Data.Type)
		}
		if payload.Data.Attributes.ReferenceName != "Monthly Tournament" {
			t.Fatalf("expected referenceName 'Monthly Tournament', got %s", payload.Data.Attributes.ReferenceName)
		}
		if payload.Data.Attributes.VendorIdentifier != "com.example.monthly" {
			t.Fatalf("expected vendorIdentifier 'com.example.monthly', got %s", payload.Data.Attributes.VendorIdentifier)
		}
		if payload.Data.Relationships.GameCenterDetail.Data.ID != "gc-detail-123" {
			t.Fatalf("expected gcDetailID gc-detail-123, got %s", payload.Data.Relationships.GameCenterDetail.Data.ID)
		}
	}, response)

	attrs := GameCenterLeaderboardSetCreateAttributes{
		ReferenceName:    "Monthly Tournament",
		VendorIdentifier: "com.example.monthly",
	}

	resp, err := client.CreateGameCenterLeaderboardSet(context.Background(), "gc-detail-123", attrs, nil)
	if err != nil {
		t.Fatalf("CreateGameCenterLeaderboardSet() error: %v", err)
	}

	if resp.Data.ID != "set-789" {
		t.Fatalf("expected ID set-789, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.ReferenceName != "Monthly Tournament" {
		t.Fatalf("expected referenceName 'Monthly Tournament', got %s", resp.Data.Attributes.ReferenceName)
	}
}

func TestGetGameCenterLeaderboardSets_UsesNextURL(t *testing.T) {
	response := gcLeaderboardSetJSONResponse(http.StatusOK, `{
		"data": [],
		"links": {
			"self": "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-123/gameCenterLeaderboardSets?cursor=abc"
		}
	}`)

	nextURL := "/v1/gameCenterDetails/gc-detail-123/gameCenterLeaderboardSets?cursor=abc"
	client := newGCLeaderboardSetTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/gameCenterDetails/gc-detail-123/gameCenterLeaderboardSets" {
			t.Fatalf("expected path /v1/gameCenterDetails/gc-detail-123/gameCenterLeaderboardSets, got %s", req.URL.Path)
		}
		if req.URL.RawQuery != "cursor=abc" {
			t.Fatalf("expected query cursor=abc, got %s", req.URL.RawQuery)
		}
	}, response)

	_, err := client.GetGameCenterLeaderboardSets(context.Background(), "gc-detail-123", NextURL[GCLeaderboardSetsOption](nextURL))
	if err != nil {
		t.Fatalf("GetGameCenterLeaderboardSets() error: %v", err)
	}
}

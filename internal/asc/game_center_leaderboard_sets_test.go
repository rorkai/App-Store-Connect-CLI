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

func TestGetGameCenterLeaderboardSets(t *testing.T) {
	response := gcLeaderboardSetJSONResponse(http.StatusOK, `{
		"data": [{
			"type": "gameCenterLeaderboardSets",
			"id": "set-123",
			"attributes": {
				"referenceName": "Season 1",
				"vendorIdentifier": "com.example.season1"
			}
		}],
		"links": {
			"self": "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-123/gameCenterLeaderboardSets"
		}
	}`)

	client := newGCLeaderboardSetTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails/gc-detail-123/gameCenterLeaderboardSets" {
			t.Fatalf("expected path /v1/gameCenterDetails/gc-detail-123/gameCenterLeaderboardSets, got %s", req.URL.Path)
		}
	}, response)

	resp, err := client.GetGameCenterLeaderboardSets(context.Background(), "gc-detail-123")
	if err != nil {
		t.Fatalf("GetGameCenterLeaderboardSets() error: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 leaderboard set, got %d", len(resp.Data))
	}
	if resp.Data[0].ID != "set-123" {
		t.Fatalf("expected ID set-123, got %s", resp.Data[0].ID)
	}
	if resp.Data[0].Attributes.ReferenceName != "Season 1" {
		t.Fatalf("expected referenceName 'Season 1', got %s", resp.Data[0].Attributes.ReferenceName)
	}
	if resp.Data[0].Attributes.VendorIdentifier != "com.example.season1" {
		t.Fatalf("expected vendorIdentifier 'com.example.season1', got %s", resp.Data[0].Attributes.VendorIdentifier)
	}
}

func TestGetGameCenterLeaderboardSet(t *testing.T) {
	response := gcLeaderboardSetJSONResponse(http.StatusOK, `{
		"data": {
			"type": "gameCenterLeaderboardSets",
			"id": "set-456",
			"attributes": {
				"referenceName": "Weekly Challenge",
				"vendorIdentifier": "com.example.weekly"
			}
		}
	}`)

	client := newGCLeaderboardSetTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSets/set-456" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSets/set-456, got %s", req.URL.Path)
		}
	}, response)

	resp, err := client.GetGameCenterLeaderboardSet(context.Background(), "set-456")
	if err != nil {
		t.Fatalf("GetGameCenterLeaderboardSet() error: %v", err)
	}

	if resp.Data.ID != "set-456" {
		t.Fatalf("expected ID set-456, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.ReferenceName != "Weekly Challenge" {
		t.Fatalf("expected referenceName 'Weekly Challenge', got %s", resp.Data.Attributes.ReferenceName)
	}
	if resp.Data.Attributes.VendorIdentifier != "com.example.weekly" {
		t.Fatalf("expected vendorIdentifier 'com.example.weekly', got %s", resp.Data.Attributes.VendorIdentifier)
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

func TestUpdateGameCenterLeaderboardSet(t *testing.T) {
	response := gcLeaderboardSetJSONResponse(http.StatusOK, `{
		"data": {
			"type": "gameCenterLeaderboardSets",
			"id": "set-999",
			"attributes": {
				"referenceName": "Updated Season Name",
				"vendorIdentifier": "com.example.season1"
			}
		}
	}`)

	client := newGCLeaderboardSetTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSets/set-999" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSets/set-999, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload GameCenterLeaderboardSetUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.ID != "set-999" {
			t.Fatalf("expected id set-999, got %s", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.ReferenceName == nil {
			t.Fatalf("expected referenceName to be set")
		}
		if *payload.Data.Attributes.ReferenceName != "Updated Season Name" {
			t.Fatalf("expected referenceName 'Updated Season Name', got %s", *payload.Data.Attributes.ReferenceName)
		}
	}, response)

	newName := "Updated Season Name"
	attrs := GameCenterLeaderboardSetUpdateAttributes{
		ReferenceName: &newName,
	}

	resp, err := client.UpdateGameCenterLeaderboardSet(context.Background(), "set-999", attrs)
	if err != nil {
		t.Fatalf("UpdateGameCenterLeaderboardSet() error: %v", err)
	}

	if resp.Data.ID != "set-999" {
		t.Fatalf("expected ID set-999, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.ReferenceName != "Updated Season Name" {
		t.Fatalf("expected referenceName 'Updated Season Name', got %s", resp.Data.Attributes.ReferenceName)
	}
}

func TestDeleteGameCenterLeaderboardSet(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusNoContent,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader("")),
	}

	client := newGCLeaderboardSetTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSets/set-123" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSets/set-123, got %s", req.URL.Path)
		}
	}, response)

	err := client.DeleteGameCenterLeaderboardSet(context.Background(), "set-123")
	if err != nil {
		t.Fatalf("DeleteGameCenterLeaderboardSet() error: %v", err)
	}
}

func TestGetGameCenterLeaderboardSets_WithLimit(t *testing.T) {
	response := gcLeaderboardSetJSONResponse(http.StatusOK, `{
		"data": [],
		"links": {
			"self": "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/gc-detail-123/gameCenterLeaderboardSets?limit=50"
		}
	}`)

	client := newGCLeaderboardSetTestClient(t, func(req *http.Request) {
		if req.URL.Query().Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %s", req.URL.Query().Get("limit"))
		}
	}, response)

	_, err := client.GetGameCenterLeaderboardSets(context.Background(), "gc-detail-123", WithGCLeaderboardSetsLimit(50))
	if err != nil {
		t.Fatalf("GetGameCenterLeaderboardSets() error: %v", err)
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

	_, err := client.GetGameCenterLeaderboardSets(context.Background(), "gc-detail-123", WithGCLeaderboardSetsNextURL(nextURL))
	if err != nil {
		t.Fatalf("GetGameCenterLeaderboardSets() error: %v", err)
	}
}

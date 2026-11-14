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

func newGCAchievementReleaseTestClient(t *testing.T, check func(*http.Request), response *http.Response) *Client {
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

func gcAchievementReleaseJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestCreateGameCenterAchievementRelease(t *testing.T) {
	response := gcAchievementReleaseJSONResponse(http.StatusCreated, `{
		"data": {
			"type": "gameCenterAchievementReleases",
			"id": "release-789",
			"attributes": {
				"live": false
			}
		}
	}`)

	client := newGCAchievementReleaseTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievementReleases" {
			t.Fatalf("expected path /v1/gameCenterAchievementReleases, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload GameCenterAchievementReleaseCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeGameCenterAchievementReleases {
			t.Fatalf("expected type gameCenterAchievementReleases, got %s", payload.Data.Type)
		}
		if payload.Data.Relationships.GameCenterDetail.Data.ID != "gc-detail-123" {
			t.Fatalf("expected gc detail ID gc-detail-123, got %s", payload.Data.Relationships.GameCenterDetail.Data.ID)
		}
		if payload.Data.Relationships.GameCenterAchievement.Data.ID != "ach-456" {
			t.Fatalf("expected achievement ID ach-456, got %s", payload.Data.Relationships.GameCenterAchievement.Data.ID)
		}
	}, response)

	resp, err := client.CreateGameCenterAchievementRelease(context.Background(), "gc-detail-123", "ach-456")
	if err != nil {
		t.Fatalf("CreateGameCenterAchievementRelease() error: %v", err)
	}

	if resp.Data.ID != "release-789" {
		t.Fatalf("expected ID release-789, got %s", resp.Data.ID)
	}
	if resp.Data.Attributes.Live {
		t.Fatalf("expected live to be false")
	}
}

func TestBuildGCAchievementReleasesQuery(t *testing.T) {
	query := &gcAchievementReleasesQuery{}

	// Test empty query
	result := buildGCAchievementReleasesQuery(query)
	if result != "" {
		t.Fatalf("expected empty query string, got %s", result)
	}

	// Test with limit
	query.limit = 25
	result = buildGCAchievementReleasesQuery(query)
	if result != "limit=25" {
		t.Fatalf("expected limit=25, got %s", result)
	}
}

func TestWithGCAchievementReleasesLimit(t *testing.T) {
	query := &gcAchievementReleasesQuery{}

	// Valid limit
	Limit[GCAchievementReleasesOption](100)(query)
	if query.limit != 100 {
		t.Fatalf("expected limit 100, got %d", query.limit)
	}

	// Invalid limit (0) should not change
	query2 := &gcAchievementReleasesQuery{}
	Limit[GCAchievementReleasesOption](0)(query2)
	if query2.limit != 0 {
		t.Fatalf("expected limit 0 (unchanged), got %d", query2.limit)
	}

	// Negative limit should not change
	query3 := &gcAchievementReleasesQuery{}
	Limit[GCAchievementReleasesOption](-5)(query3)
	if query3.limit != 0 {
		t.Fatalf("expected limit 0 (unchanged), got %d", query3.limit)
	}
}

func TestWithGCAchievementReleasesNextURL(t *testing.T) {
	query := &gcAchievementReleasesQuery{}

	// Valid URL
	url := "https://api.appstoreconnect.apple.com/v1/gameCenterAchievements/ach-123/releases?cursor=xyz"
	NextURL[GCAchievementReleasesOption](url)(query)
	if query.nextURL != url {
		t.Fatalf("expected nextURL %s, got %s", url, query.nextURL)
	}

	// Empty URL should not set
	query2 := &gcAchievementReleasesQuery{}
	NextURL[GCAchievementReleasesOption]("")(query2)
	if query2.nextURL != "" {
		t.Fatalf("expected empty nextURL, got %s", query2.nextURL)
	}

	// Whitespace URL should not set
	query3 := &gcAchievementReleasesQuery{}
	NextURL[GCAchievementReleasesOption]("   ")(query3)
	if query3.nextURL != "" {
		t.Fatalf("expected empty nextURL, got %s", query3.nextURL)
	}
}

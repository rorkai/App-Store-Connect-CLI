package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestGetGameCenterAchievementRelease_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterAchievementReleases","id":"rel-1","attributes":{"live":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterAchievementReleases/rel-1" {
			t.Fatalf("expected path /v1/gameCenterAchievementReleases/rel-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetGameCenterAchievementRelease(context.Background(), "rel-1")
	if err != nil {
		t.Fatalf("GetGameCenterAchievementRelease() error: %v", err)
	}
	if resp.Data.ID != "rel-1" {
		t.Fatalf("expected id rel-1, got %q", resp.Data.ID)
	}
	if !resp.Data.Attributes.Live {
		t.Fatalf("expected live to be true")
	}
}

func TestGetGameCenterLeaderboardRelease_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardReleases","id":"rel-2","attributes":{"live":false}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardReleases/rel-2" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardReleases/rel-2, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetGameCenterLeaderboardRelease(context.Background(), "rel-2")
	if err != nil {
		t.Fatalf("GetGameCenterLeaderboardRelease() error: %v", err)
	}
	if resp.Data.ID != "rel-2" {
		t.Fatalf("expected id rel-2, got %q", resp.Data.ID)
	}
	if resp.Data.Attributes.Live {
		t.Fatalf("expected live to be false")
	}
}

func TestGetGameCenterLeaderboardSetRelease_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterLeaderboardSetReleases","id":"rel-3","attributes":{"live":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterLeaderboardSetReleases/rel-3" {
			t.Fatalf("expected path /v1/gameCenterLeaderboardSetReleases/rel-3, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.GetGameCenterLeaderboardSetRelease(context.Background(), "rel-3")
	if err != nil {
		t.Fatalf("GetGameCenterLeaderboardSetRelease() error: %v", err)
	}
	if resp.Data.ID != "rel-3" {
		t.Fatalf("expected id rel-3, got %q", resp.Data.ID)
	}
}

package asc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"
)

func TestGameCenterDetailListEndpoints_WithLimit(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		path     string
		limit    string
		response string
		call     func(*testing.T, *Client)
	}{
		{
			name:     "GetGameCenterGroupGameCenterDetails",
			path:     "/v1/gameCenterGroups/group-1/gameCenterDetails",
			limit:    "30",
			response: `{"data":[{"type":"gameCenterDetails","id":"detail-1","attributes":{"arcadeEnabled":true}}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetGameCenterGroupGameCenterDetails(ctx, "group-1", WithGCDetailsLimit(30))
				if err != nil {
					t.Fatalf("GetGameCenterGroupGameCenterDetails() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "detail-1" {
					t.Fatalf("expected decoded group game center detail, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetGameCenterDetailsAchievementReleases",
			path:     "/v1/gameCenterDetails/detail-1/achievementReleases",
			limit:    "12",
			response: `{"data":[{"type":"gameCenterAchievementReleases","id":"rel-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetGameCenterDetailsAchievementReleases(ctx, "detail-1", WithGCAchievementReleasesLimit(12))
				if err != nil {
					t.Fatalf("GetGameCenterDetailsAchievementReleases() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "rel-1" {
					t.Fatalf("expected decoded achievement release, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetGameCenterDetailsLeaderboardReleases",
			path:     "/v1/gameCenterDetails/detail-1/leaderboardReleases",
			limit:    "15",
			response: `{"data":[{"type":"gameCenterLeaderboardReleases","id":"rel-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetGameCenterDetailsLeaderboardReleases(ctx, "detail-1", WithGCLeaderboardReleasesLimit(15))
				if err != nil {
					t.Fatalf("GetGameCenterDetailsLeaderboardReleases() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "rel-1" {
					t.Fatalf("expected decoded leaderboard release, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetGameCenterDetailsLeaderboardSetReleases",
			path:     "/v1/gameCenterDetails/detail-1/leaderboardSetReleases",
			limit:    "18",
			response: `{"data":[{"type":"gameCenterLeaderboardSetReleases","id":"rel-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetGameCenterDetailsLeaderboardSetReleases(ctx, "detail-1", WithGCLeaderboardSetReleasesLimit(18))
				if err != nil {
					t.Fatalf("GetGameCenterDetailsLeaderboardSetReleases() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "rel-1" {
					t.Fatalf("expected decoded leaderboard set release, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetGameCenterDetailsAchievementsV2",
			path:     "/v1/gameCenterDetails/detail-1/gameCenterAchievementsV2",
			limit:    "20",
			response: `{"data":[{"type":"gameCenterAchievements","id":"ach-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetGameCenterDetailsAchievementsV2(ctx, "detail-1", WithGCAchievementsLimit(20))
				if err != nil {
					t.Fatalf("GetGameCenterDetailsAchievementsV2() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "ach-1" {
					t.Fatalf("expected decoded achievement, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetGameCenterDetailsLeaderboardsV2",
			path:     "/v1/gameCenterDetails/detail-1/gameCenterLeaderboardsV2",
			limit:    "25",
			response: `{"data":[{"type":"gameCenterLeaderboards","id":"lb-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetGameCenterDetailsLeaderboardsV2(ctx, "detail-1", WithGCLeaderboardsLimit(25))
				if err != nil {
					t.Fatalf("GetGameCenterDetailsLeaderboardsV2() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "lb-1" {
					t.Fatalf("expected decoded leaderboard, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetGameCenterDetailsLeaderboardSetsV2",
			path:     "/v1/gameCenterDetails/detail-1/gameCenterLeaderboardSetsV2",
			limit:    "30",
			response: `{"data":[{"type":"gameCenterLeaderboardSets","id":"set-1"}]}`,
			call: func(t *testing.T, c *Client) {
				resp, err := c.GetGameCenterDetailsLeaderboardSetsV2(ctx, "detail-1", WithGCLeaderboardSetsLimit(30))
				if err != nil {
					t.Fatalf("GetGameCenterDetailsLeaderboardSetsV2() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "set-1" {
					t.Fatalf("expected decoded leaderboard set, got %+v", resp.Data)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", req.Method)
				}
				if req.URL.Path != tt.path {
					t.Fatalf("expected path %s, got %s", tt.path, req.URL.Path)
				}
				if req.URL.Query().Get("limit") != tt.limit {
					t.Fatalf("expected limit=%s, got %q", tt.limit, req.URL.Query().Get("limit"))
				}
				assertAuthorized(t, req)
			}, jsonResponse(http.StatusOK, tt.response))

			tt.call(t, client)
		})
	}
}

func TestGameCenterDetailListEndpoints_UseNextURL(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name     string
		next     string
		response string
		call     func(*testing.T, *Client, string)
	}{
		{
			name:     "GetGameCenterGroupGameCenterDetails",
			next:     "https://api.appstoreconnect.apple.com/v1/gameCenterGroups/group-1/gameCenterDetails?cursor=next",
			response: `{"data":[{"type":"gameCenterDetails","id":"detail-1","attributes":{"arcadeEnabled":true}}]}`,
			call: func(t *testing.T, c *Client, next string) {
				resp, err := c.GetGameCenterGroupGameCenterDetails(ctx, "", WithGCDetailsNextURL(next))
				if err != nil {
					t.Fatalf("GetGameCenterGroupGameCenterDetails() error: %v", err)
				}
				if len(resp.Data) != 1 || resp.Data[0].ID != "detail-1" {
					t.Fatalf("expected decoded next-url group detail, got %+v", resp.Data)
				}
			},
		},
		{
			name:     "GetGameCenterDetailsRuleBasedMatchmakingRequests",
			next:     "https://api.appstoreconnect.apple.com/v1/gameCenterDetails/detail-1/metrics/ruleBasedMatchmakingRequests?cursor=next",
			response: `{"data":[{"dataPoints":[{"start":"2026-01-01","end":"2026-01-02","values":{"count":1}}],"dimensions":{"result":{"data":{"id":"MATCHED"}}}}]}`,
			call: func(t *testing.T, c *Client, next string) {
				resp, err := c.GetGameCenterDetailsRuleBasedMatchmakingRequests(ctx, "detail-1", WithGCMatchmakingMetricsNextURL(next))
				if err != nil {
					t.Fatalf("GetGameCenterDetailsRuleBasedMatchmakingRequests() error: %v", err)
				}
				if len(resp.Data) != 1 || len(resp.Data[0].DataPoints) != 1 {
					t.Fatalf("expected decoded matchmaking metrics row, got %+v", resp.Data)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.URL.String() != tt.next {
					t.Fatalf("expected URL %q, got %q", tt.next, req.URL.String())
				}
				assertAuthorized(t, req)
			}, jsonResponse(http.StatusOK, tt.response))

			tt.call(t, client, tt.next)
		})
	}
}

func TestGetGameCenterDetail(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterDetails","id":"detail-1","attributes":{"arcadeEnabled":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails/detail-1" {
			t.Fatalf("expected path /v1/gameCenterDetails/detail-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterDetail(context.Background(), "detail-1"); err != nil {
		t.Fatalf("GetGameCenterDetail() error: %v", err)
	}
}

func TestGetGameCenterDetailGameCenterGroup(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterGroups","id":"group-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails/detail-1/gameCenterGroup" {
			t.Fatalf("expected path /v1/gameCenterDetails/detail-1/gameCenterGroup, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetGameCenterDetailGameCenterGroup(context.Background(), "detail-1"); err != nil {
		t.Fatalf("GetGameCenterDetailGameCenterGroup() error: %v", err)
	}
}

func TestGCDetailsOptions(t *testing.T) {
	query := &gcDetailsQuery{}
	WithGCDetailsLimit(8)(query)
	if query.limit != 8 {
		t.Fatalf("expected limit 8, got %d", query.limit)
	}
	WithGCDetailsNextURL("next")(query)
	if query.nextURL != "next" {
		t.Fatalf("expected nextURL set, got %q", query.nextURL)
	}
	values, err := url.ParseQuery(buildGCDetailsQuery(query))
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if values.Get("limit") != "8" {
		t.Fatalf("expected limit=8, got %q", values.Get("limit"))
	}
}

func TestGetGameCenterDetailsClassicMatchmakingRequests_WithQuery(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.URL.Path != "/v1/gameCenterDetails/detail-1/metrics/classicMatchmakingRequests" {
			t.Fatalf("expected path /v1/gameCenterDetails/detail-1/metrics/classicMatchmakingRequests, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("granularity") != "P1D" {
			t.Fatalf("expected granularity=P1D, got %q", values.Get("granularity"))
		}
		if values.Get("groupBy") != "result" {
			t.Fatalf("expected groupBy=result, got %q", values.Get("groupBy"))
		}
		if values.Get("filter[result]") != "MATCHED" {
			t.Fatalf("expected filter[result]=MATCHED, got %q", values.Get("filter[result]"))
		}
		if values.Get("sort") != "-count" {
			t.Fatalf("expected sort=-count, got %q", values.Get("sort"))
		}
		if values.Get("limit") != "50" {
			t.Fatalf("expected limit=50, got %q", values.Get("limit"))
		}
		assertAuthorized(t, req)
	}, response)

	opts := []GCMatchmakingMetricsOption{
		WithGCMatchmakingMetricsGranularity("P1D"),
		WithGCMatchmakingMetricsGroupBy([]string{"result"}),
		WithGCMatchmakingMetricsFilterResult("MATCHED"),
		WithGCMatchmakingMetricsSort([]string{"-count"}),
		WithGCMatchmakingMetricsLimit(50),
	}

	if _, err := client.GetGameCenterDetailsClassicMatchmakingRequests(context.Background(), "detail-1", opts...); err != nil {
		t.Fatalf("GetGameCenterDetailsClassicMatchmakingRequests() error: %v", err)
	}
}

func TestCreateGameCenterDetail(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"gameCenterDetails","id":"detail-new","attributes":{"challengeEnabled":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails" {
			t.Fatalf("expected path /v1/gameCenterDetails, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload GameCenterDetailCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeGameCenterDetails {
			t.Fatalf("expected type gameCenterDetails, got %s", payload.Data.Type)
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.App == nil {
			t.Fatalf("expected app relationship to be set")
		}
		if payload.Data.Relationships.App.Data.ID != "app-123" {
			t.Fatalf("expected app ID app-123, got %s", payload.Data.Relationships.App.Data.ID)
		}
		if payload.Data.Relationships.App.Data.Type != ResourceTypeApps {
			t.Fatalf("expected app type apps, got %s", payload.Data.Relationships.App.Data.Type)
		}
		assertAuthorized(t, req)
	}, response)

	resp, err := client.CreateGameCenterDetail(context.Background(), "app-123", nil)
	if err != nil {
		t.Fatalf("CreateGameCenterDetail() error: %v", err)
	}

	if resp.Data.ID != "detail-new" {
		t.Fatalf("expected ID detail-new, got %s", resp.Data.ID)
	}
}

func TestUpdateGameCenterDetail(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"gameCenterDetails","id":"detail-1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/gameCenterDetails/detail-1" {
			t.Fatalf("expected path /v1/gameCenterDetails/detail-1, got %s", req.URL.Path)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		var payload GameCenterDetailUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		if payload.Data.Type != ResourceTypeGameCenterDetails {
			t.Fatalf("expected type gameCenterDetails, got %s", payload.Data.Type)
		}
		if payload.Data.ID != "detail-1" {
			t.Fatalf("expected id detail-1, got %s", payload.Data.ID)
		}
		if payload.Data.Attributes != nil {
			t.Fatalf("expected attributes to be omitted")
		}
		if payload.Data.Relationships == nil || payload.Data.Relationships.DefaultLeaderboard == nil {
			t.Fatalf("expected defaultLeaderboard relationship to be set")
		}
		if payload.Data.Relationships.DefaultLeaderboard.Data.ID != "lb-1" {
			t.Fatalf("expected defaultLeaderboard id lb-1, got %s", payload.Data.Relationships.DefaultLeaderboard.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	rels := &GameCenterDetailUpdateRelationships{
		DefaultLeaderboard: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterLeaderboards,
				ID:   "lb-1",
			},
		},
	}
	resp, err := client.UpdateGameCenterDetail(context.Background(), "detail-1", nil, rels)
	if err != nil {
		t.Fatalf("UpdateGameCenterDetail() error: %v", err)
	}

	if resp.Data.ID != "detail-1" {
		t.Fatalf("expected ID detail-1, got %s", resp.Data.ID)
	}
}

func TestCreateGameCenterDetail_RequiresAppID(t *testing.T) {
	client := newTestClient(t, nil, nil)

	_, err := client.CreateGameCenterDetail(context.Background(), " ", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateGameCenterDetail_ReturnsAPIError(t *testing.T) {
	response := jsonResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)
	client := newTestClient(t, nil, response)

	_, err := client.CreateGameCenterDetail(context.Background(), "app-123", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status code %d, got %d", http.StatusForbidden, apiErr.StatusCode)
	}
}

func TestUpdateGameCenterDetail_ValidationErrors(t *testing.T) {
	client := newTestClient(t, nil, nil)

	tests := []struct {
		name  string
		id    string
		attrs *GameCenterDetailUpdateAttributes
		rels  *GameCenterDetailUpdateRelationships
	}{
		{
			name: "missing detail ID",
			id:   " ",
			rels: &GameCenterDetailUpdateRelationships{
				DefaultLeaderboard: &Relationship{Data: ResourceData{Type: ResourceTypeGameCenterLeaderboards, ID: "lb-1"}},
			},
		},
		{
			name: "no update fields",
			id:   "detail-1",
		},
		{
			name: "empty gameCenterGroup relationship ID",
			id:   "detail-1",
			rels: &GameCenterDetailUpdateRelationships{
				GameCenterGroup:    &Relationship{Data: ResourceData{Type: ResourceTypeGameCenterGroups, ID: " "}},
				DefaultLeaderboard: &Relationship{Data: ResourceData{Type: ResourceTypeGameCenterLeaderboards, ID: "lb-1"}},
			},
		},
		{
			name: "empty defaultLeaderboard relationship ID",
			id:   "detail-1",
			rels: &GameCenterDetailUpdateRelationships{
				GameCenterGroup:    &Relationship{Data: ResourceData{Type: ResourceTypeGameCenterGroups, ID: "group-1"}},
				DefaultLeaderboard: &Relationship{Data: ResourceData{Type: ResourceTypeGameCenterLeaderboards, ID: " "}},
			},
		},
		{
			name: "empty defaultLeaderboardV2 relationship ID",
			id:   "detail-1",
			rels: &GameCenterDetailUpdateRelationships{
				DefaultLeaderboardV2: &Relationship{Data: ResourceData{Type: ResourceTypeGameCenterLeaderboards, ID: " "}},
			},
		},
		{
			name: "empty defaultGroupLeaderboard relationship ID",
			id:   "detail-1",
			rels: &GameCenterDetailUpdateRelationships{
				DefaultGroupLeaderboard: &Relationship{Data: ResourceData{Type: ResourceTypeGameCenterLeaderboards, ID: " "}},
			},
		},
		{
			name: "empty defaultGroupLeaderboardV2 relationship ID",
			id:   "detail-1",
			rels: &GameCenterDetailUpdateRelationships{
				DefaultGroupLeaderboardV2: &Relationship{Data: ResourceData{Type: ResourceTypeGameCenterLeaderboards, ID: " "}},
			},
		},
		{
			name: "deprecated challengeEnabled attribute",
			id:   "detail-1",
			attrs: &GameCenterDetailUpdateAttributes{
				ChallengeEnabled: func() *bool { b := true; return &b }(),
			},
			rels: &GameCenterDetailUpdateRelationships{
				DefaultLeaderboard: &Relationship{Data: ResourceData{Type: ResourceTypeGameCenterLeaderboards, ID: "lb-1"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.UpdateGameCenterDetail(context.Background(), test.id, test.attrs, test.rels)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestUpdateGameCenterDetail_ReturnsAPIError(t *testing.T) {
	response := jsonResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"not allowed"}]}`)
	client := newTestClient(t, nil, response)

	rels := &GameCenterDetailUpdateRelationships{
		DefaultLeaderboard: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterLeaderboards,
				ID:   "lb-1",
			},
		},
	}
	_, err := client.UpdateGameCenterDetail(context.Background(), "detail-1", nil, rels)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status code %d, got %d", http.StatusForbidden, apiErr.StatusCode)
	}
}

func TestCreateGameCenterDetail_RejectsDeprecatedChallengeEnabled(t *testing.T) {
	client := newTestClient(t, nil, nil)
	value := true

	_, err := client.CreateGameCenterDetail(context.Background(), "app-123", &GameCenterDetailCreateAttributes{
		ChallengeEnabled: &value,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

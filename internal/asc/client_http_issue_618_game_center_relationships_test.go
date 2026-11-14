package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestIssue618_GameCenterRelationshipEndpoints_Mutations(t *testing.T) {
	ctx := context.Background()

	type toOneRequest struct {
		Data ResourceData `json:"data"`
	}

	tests := []struct {
		name       string
		wantMethod string
		wantPath   string
		wantBodyFn func(*testing.T, []byte)
		call       func(*Client) error
	}{
		{
			name:       "AddGameCenterActivityAchievementsV2",
			wantMethod: http.MethodPost,
			wantPath:   "/v1/gameCenterActivities/act-1/relationships/achievementsV2",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 1 || got.Data[0].Type != ResourceTypeGameCenterAchievements || got.Data[0].ID != "ach-1" {
					t.Fatalf("unexpected body: %#v", got)
				}
			},
			call: func(client *Client) error {
				return client.AddGameCenterActivityAchievementsV2(ctx, "act-1", []string{"ach-1"})
			},
		},
		{
			name:       "RemoveGameCenterActivityAchievementsV2",
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/gameCenterActivities/act-1/relationships/achievementsV2",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 1 || got.Data[0].Type != ResourceTypeGameCenterAchievements || got.Data[0].ID != "ach-1" {
					t.Fatalf("unexpected body: %#v", got)
				}
			},
			call: func(client *Client) error {
				return client.RemoveGameCenterActivityAchievementsV2(ctx, "act-1", []string{"ach-1"})
			},
		},
		{
			name:       "AddGameCenterActivityLeaderboardsV2",
			wantMethod: http.MethodPost,
			wantPath:   "/v1/gameCenterActivities/act-1/relationships/leaderboardsV2",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 1 || got.Data[0].Type != ResourceTypeGameCenterLeaderboards || got.Data[0].ID != "lb-1" {
					t.Fatalf("unexpected body: %#v", got)
				}
			},
			call: func(client *Client) error {
				return client.AddGameCenterActivityLeaderboardsV2(ctx, "act-1", []string{"lb-1"})
			},
		},
		{
			name:       "RemoveGameCenterActivityLeaderboardsV2",
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/gameCenterActivities/act-1/relationships/leaderboardsV2",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 1 || got.Data[0].Type != ResourceTypeGameCenterLeaderboards || got.Data[0].ID != "lb-1" {
					t.Fatalf("unexpected body: %#v", got)
				}
			},
			call: func(client *Client) error {
				return client.RemoveGameCenterActivityLeaderboardsV2(ctx, "act-1", []string{"lb-1"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != tt.wantMethod {
					t.Fatalf("expected %s, got %s", tt.wantMethod, req.Method)
				}
				if req.URL.Path != tt.wantPath {
					t.Fatalf("expected path %s, got %s", tt.wantPath, req.URL.Path)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				tt.wantBodyFn(t, body)
				assertAuthorized(t, req)
			}, jsonResponse(http.StatusNoContent, ""))

			if err := tt.call(client); err != nil {
				t.Fatalf("request error: %v", err)
			}
		})
	}
}

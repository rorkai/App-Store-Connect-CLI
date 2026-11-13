package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestIssue617_GameCenterLeaderboardRelationshipEndpoints_Mutations(t *testing.T) {
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
			name:       "AddGameCenterLeaderboardSetMembers (POST v1)",
			wantMethod: http.MethodPost,
			wantPath:   "/v1/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 2 {
					t.Fatalf("expected 2 relationship items, got %d", len(got.Data))
				}
				if got.Data[0].Type != ResourceTypeGameCenterLeaderboards || got.Data[0].ID != "lb-1" {
					t.Fatalf("unexpected first item: %#v", got.Data[0])
				}
				if got.Data[1].Type != ResourceTypeGameCenterLeaderboards || got.Data[1].ID != "lb-2" {
					t.Fatalf("unexpected second item: %#v", got.Data[1])
				}
			},
			call: func(client *Client) error {
				return client.AddGameCenterLeaderboardSetMembers(ctx, "set-1", []string{"lb-1", "lb-2"})
			},
		},
		{
			name:       "RemoveGameCenterLeaderboardSetMembers (DELETE v1)",
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 1 {
					t.Fatalf("expected 1 relationship item, got %d", len(got.Data))
				}
				if got.Data[0].Type != ResourceTypeGameCenterLeaderboards || got.Data[0].ID != "lb-1" {
					t.Fatalf("unexpected item: %#v", got.Data[0])
				}
			},
			call: func(client *Client) error {
				return client.RemoveGameCenterLeaderboardSetMembers(ctx, "set-1", []string{"lb-1"})
			},
		},
		{
			name:       "AddGameCenterLeaderboardSetMembersV2 (POST v2)",
			wantMethod: http.MethodPost,
			wantPath:   "/v2/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 2 {
					t.Fatalf("expected 2 relationship items, got %d", len(got.Data))
				}
				if got.Data[0].Type != ResourceTypeGameCenterLeaderboards || got.Data[0].ID != "lb-1" {
					t.Fatalf("unexpected first item: %#v", got.Data[0])
				}
				if got.Data[1].Type != ResourceTypeGameCenterLeaderboards || got.Data[1].ID != "lb-2" {
					t.Fatalf("unexpected second item: %#v", got.Data[1])
				}
			},
			call: func(client *Client) error {
				return client.AddGameCenterLeaderboardSetMembersV2(ctx, "set-1", []string{"lb-1", "lb-2"})
			},
		},
		{
			name:       "RemoveGameCenterLeaderboardSetMembersV2 (DELETE v2)",
			wantMethod: http.MethodDelete,
			wantPath:   "/v2/gameCenterLeaderboardSets/set-1/relationships/gameCenterLeaderboards",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 1 {
					t.Fatalf("expected 1 relationship item, got %d", len(got.Data))
				}
				if got.Data[0].Type != ResourceTypeGameCenterLeaderboards || got.Data[0].ID != "lb-1" {
					t.Fatalf("unexpected item: %#v", got.Data[0])
				}
			},
			call: func(client *Client) error {
				return client.RemoveGameCenterLeaderboardSetMembersV2(ctx, "set-1", []string{"lb-1"})
			},
		},
		{
			name:       "UpdateGameCenterLeaderboardSetGroupLeaderboardSetRelationship (PATCH v1)",
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/gameCenterLeaderboardSets/set-1/relationships/groupLeaderboardSet",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got toOneRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if got.Data.Type != ResourceTypeGameCenterLeaderboardSets || got.Data.ID != "set-2" {
					t.Fatalf("unexpected data: %#v", got.Data)
				}
			},
			call: func(client *Client) error {
				return client.UpdateGameCenterLeaderboardSetGroupLeaderboardSetRelationship(ctx, "set-1", "set-2")
			},
		},
		{
			name:       "UpdateGameCenterLeaderboardActivityRelationship (PATCH v1)",
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/gameCenterLeaderboards/lb-1/relationships/activity",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got toOneRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if got.Data.Type != ResourceTypeGameCenterActivities || got.Data.ID != "act-1" {
					t.Fatalf("unexpected data: %#v", got.Data)
				}
			},
			call: func(client *Client) error {
				return client.UpdateGameCenterLeaderboardActivityRelationship(ctx, "lb-1", "act-1")
			},
		},
		{
			name:       "UpdateGameCenterLeaderboardChallengeRelationship (PATCH v1)",
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/gameCenterLeaderboards/lb-1/relationships/challenge",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got toOneRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if got.Data.Type != ResourceTypeGameCenterChallenges || got.Data.ID != "chal-1" {
					t.Fatalf("unexpected data: %#v", got.Data)
				}
			},
			call: func(client *Client) error {
				return client.UpdateGameCenterLeaderboardChallengeRelationship(ctx, "lb-1", "chal-1")
			},
		},
		{
			name:       "UpdateGameCenterLeaderboardGroupLeaderboardRelationship (PATCH v1)",
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/gameCenterLeaderboards/lb-1/relationships/groupLeaderboard",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got toOneRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if got.Data.Type != ResourceTypeGameCenterLeaderboards || got.Data.ID != "lb-2" {
					t.Fatalf("unexpected data: %#v", got.Data)
				}
			},
			call: func(client *Client) error {
				return client.UpdateGameCenterLeaderboardGroupLeaderboardRelationship(ctx, "lb-1", "lb-2")
			},
		},
		{
			name:       "UpdateGameCenterLeaderboardActivityRelationshipV2 (PATCH v2)",
			wantMethod: http.MethodPatch,
			wantPath:   "/v2/gameCenterLeaderboards/lb-1/relationships/activity",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got toOneRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if got.Data.Type != ResourceTypeGameCenterActivities || got.Data.ID != "act-1" {
					t.Fatalf("unexpected data: %#v", got.Data)
				}
			},
			call: func(client *Client) error {
				return client.UpdateGameCenterLeaderboardActivityRelationshipV2(ctx, "lb-1", "act-1")
			},
		},
		{
			name:       "UpdateGameCenterLeaderboardChallengeRelationshipV2 (PATCH v2)",
			wantMethod: http.MethodPatch,
			wantPath:   "/v2/gameCenterLeaderboards/lb-1/relationships/challenge",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got toOneRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if got.Data.Type != ResourceTypeGameCenterChallenges || got.Data.ID != "chal-1" {
					t.Fatalf("unexpected data: %#v", got.Data)
				}
			},
			call: func(client *Client) error {
				return client.UpdateGameCenterLeaderboardChallengeRelationshipV2(ctx, "lb-1", "chal-1")
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

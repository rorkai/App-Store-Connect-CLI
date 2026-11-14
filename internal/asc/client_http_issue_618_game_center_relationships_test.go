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
		{
			name:       "AddGameCenterAppVersionCompatibilityVersions",
			wantMethod: http.MethodPost,
			wantPath:   "/v1/gameCenterAppVersions/appver-1/relationships/compatibilityVersions",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 1 || got.Data[0].Type != ResourceTypeGameCenterAppVersions || got.Data[0].ID != "appver-2" {
					t.Fatalf("unexpected body: %#v", got)
				}
			},
			call: func(client *Client) error {
				return client.AddGameCenterAppVersionCompatibilityVersions(ctx, "appver-1", []string{"appver-2"})
			},
		},
		{
			name:       "RemoveGameCenterAppVersionCompatibilityVersions",
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/gameCenterAppVersions/appver-1/relationships/compatibilityVersions",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 1 || got.Data[0].Type != ResourceTypeGameCenterAppVersions || got.Data[0].ID != "appver-2" {
					t.Fatalf("unexpected body: %#v", got)
				}
			},
			call: func(client *Client) error {
				return client.RemoveGameCenterAppVersionCompatibilityVersions(ctx, "appver-1", []string{"appver-2"})
			},
		},
		{
			name:       "AddGameCenterEnabledVersionCompatibleVersions",
			wantMethod: http.MethodPost,
			wantPath:   "/v1/gameCenterEnabledVersions/env-1/relationships/compatibleVersions",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 1 || got.Data[0].Type != ResourceTypeGameCenterEnabledVersions || got.Data[0].ID != "env-2" {
					t.Fatalf("unexpected body: %#v", got)
				}
			},
			call: func(client *Client) error {
				return client.AddGameCenterEnabledVersionCompatibleVersions(ctx, "env-1", []string{"env-2"})
			},
		},
		{
			name:       "RemoveGameCenterEnabledVersionCompatibleVersions",
			wantMethod: http.MethodDelete,
			wantPath:   "/v1/gameCenterEnabledVersions/env-1/relationships/compatibleVersions",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 1 || got.Data[0].Type != ResourceTypeGameCenterEnabledVersions || got.Data[0].ID != "env-2" {
					t.Fatalf("unexpected body: %#v", got)
				}
			},
			call: func(client *Client) error {
				return client.RemoveGameCenterEnabledVersionCompatibleVersions(ctx, "env-1", []string{"env-2"})
			},
		},
		{
			name:       "UpdateGameCenterEnabledVersionCompatibleVersionsRelationship",
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/gameCenterEnabledVersions/env-1/relationships/compatibleVersions",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if len(got.Data) != 2 {
					t.Fatalf("expected 2 items, got %d", len(got.Data))
				}
				if got.Data[0].Type != ResourceTypeGameCenterEnabledVersions || got.Data[0].ID != "env-2" {
					t.Fatalf("unexpected first item: %#v", got.Data[0])
				}
				if got.Data[1].Type != ResourceTypeGameCenterEnabledVersions || got.Data[1].ID != "env-3" {
					t.Fatalf("unexpected second item: %#v", got.Data[1])
				}
			},
			call: func(client *Client) error {
				return client.UpdateGameCenterEnabledVersionCompatibleVersionsRelationship(ctx, "env-1", []string{"env-2", "env-3"})
			},
		},
		{
			name:       "UpdateGameCenterAchievementActivityRelationship",
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/gameCenterAchievements/ach-1/relationships/activity",
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
				return client.UpdateGameCenterAchievementActivityRelationship(ctx, "ach-1", "act-1")
			},
		},
		{
			name:       "UpdateGameCenterAchievementGroupAchievementRelationship",
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/gameCenterAchievements/ach-1/relationships/groupAchievement",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got toOneRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if got.Data.Type != ResourceTypeGameCenterAchievements || got.Data.ID != "ach-2" {
					t.Fatalf("unexpected data: %#v", got.Data)
				}
			},
			call: func(client *Client) error {
				return client.UpdateGameCenterAchievementGroupAchievementRelationship(ctx, "ach-1", "ach-2")
			},
		},
		{
			name:       "UpdateGameCenterChallengeLeaderboardRelationship",
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/gameCenterChallenges/chal-1/relationships/leaderboard",
			wantBodyFn: func(t *testing.T, body []byte) {
				t.Helper()
				var got toOneRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				if got.Data.Type != ResourceTypeGameCenterLeaderboards || got.Data.ID != "lb-1" {
					t.Fatalf("unexpected data: %#v", got.Data)
				}
			},
			call: func(client *Client) error {
				return client.UpdateGameCenterChallengeLeaderboardRelationship(ctx, "chal-1", "lb-1")
			},
		},
		{
			name:       "UpdateGameCenterChallengeLeaderboardV2Relationship",
			wantMethod: http.MethodPatch,
			wantPath:   "/v1/gameCenterChallenges/chal-1/relationships/leaderboardV2",
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
				return client.UpdateGameCenterChallengeLeaderboardV2Relationship(ctx, "chal-1", "lb-2")
			},
		},
		{
			name:       "UpdateGameCenterAchievementActivityRelationshipV2",
			wantMethod: http.MethodPatch,
			wantPath:   "/v2/gameCenterAchievements/ach-1/relationships/activity",
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
				return client.UpdateGameCenterAchievementActivityRelationshipV2(ctx, "ach-1", "act-1")
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

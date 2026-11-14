package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestIssue616_GameCenterDetailAndGroupRelationshipEndpoints_PATCH(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		wantPath string
		wantType ResourceType
		wantIDs  []string
		call     func(*Client) error
	}{
		{
			name:     "UpdateGameCenterDetailChallengesMinimumPlatformVersionsRelationship",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/challengesMinimumPlatformVersions",
			wantType: ResourceTypeAppStoreVersions,
			wantIDs:  []string{"ver-1", "ver-2"},
			call: func(client *Client) error {
				return client.UpdateGameCenterDetailChallengesMinimumPlatformVersionsRelationship(ctx, "detail-1", []string{"ver-1", "ver-2"})
			},
		},
		{
			name:     "UpdateGameCenterDetailGameCenterAchievementsRelationship",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterAchievements",
			wantType: ResourceTypeGameCenterAchievements,
			wantIDs:  []string{"ach-1"},
			call: func(client *Client) error {
				return client.UpdateGameCenterDetailGameCenterAchievementsRelationship(ctx, "detail-1", []string{"ach-1"})
			},
		},
		{
			name:     "UpdateGameCenterDetailGameCenterAchievementsV2Relationship",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterAchievementsV2",
			wantType: ResourceTypeGameCenterAchievements,
			wantIDs:  []string{"ach-1"},
			call: func(client *Client) error {
				return client.UpdateGameCenterDetailGameCenterAchievementsV2Relationship(ctx, "detail-1", []string{"ach-1"})
			},
		},
		{
			name:     "UpdateGameCenterDetailGameCenterLeaderboardSetsRelationship",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterLeaderboardSets",
			wantType: ResourceTypeGameCenterLeaderboardSets,
			wantIDs:  []string{"set-1"},
			call: func(client *Client) error {
				return client.UpdateGameCenterDetailGameCenterLeaderboardSetsRelationship(ctx, "detail-1", []string{"set-1"})
			},
		},
		{
			name:     "UpdateGameCenterDetailGameCenterLeaderboardSetsV2Relationship",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterLeaderboardSetsV2",
			wantType: ResourceTypeGameCenterLeaderboardSets,
			wantIDs:  []string{"set-1"},
			call: func(client *Client) error {
				return client.UpdateGameCenterDetailGameCenterLeaderboardSetsV2Relationship(ctx, "detail-1", []string{"set-1"})
			},
		},
		{
			name:     "UpdateGameCenterDetailGameCenterLeaderboardsRelationship",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterLeaderboards",
			wantType: ResourceTypeGameCenterLeaderboards,
			wantIDs:  []string{"lb-1"},
			call: func(client *Client) error {
				return client.UpdateGameCenterDetailGameCenterLeaderboardsRelationship(ctx, "detail-1", []string{"lb-1"})
			},
		},
		{
			name:     "UpdateGameCenterDetailGameCenterLeaderboardsRelationship empty clears relationship",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterLeaderboards",
			wantType: ResourceTypeGameCenterLeaderboards,
			wantIDs:  []string{},
			call: func(client *Client) error {
				return client.UpdateGameCenterDetailGameCenterLeaderboardsRelationship(ctx, "detail-1", nil)
			},
		},
		{
			name:     "UpdateGameCenterDetailGameCenterLeaderboardsV2Relationship",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterLeaderboardsV2",
			wantType: ResourceTypeGameCenterLeaderboards,
			wantIDs:  []string{"lb-1"},
			call: func(client *Client) error {
				return client.UpdateGameCenterDetailGameCenterLeaderboardsV2Relationship(ctx, "detail-1", []string{"lb-1"})
			},
		},
		{
			name:     "UpdateGameCenterGroupGameCenterLeaderboardSetsRelationship",
			wantPath: "/v1/gameCenterGroups/group-1/relationships/gameCenterLeaderboardSets",
			wantType: ResourceTypeGameCenterLeaderboardSets,
			wantIDs:  []string{"set-1"},
			call: func(client *Client) error {
				return client.UpdateGameCenterGroupGameCenterLeaderboardSetsRelationship(ctx, "group-1", []string{"set-1"})
			},
		},
		{
			name:     "UpdateGameCenterGroupGameCenterLeaderboardSetsV2Relationship",
			wantPath: "/v1/gameCenterGroups/group-1/relationships/gameCenterLeaderboardSetsV2",
			wantType: ResourceTypeGameCenterLeaderboardSets,
			wantIDs:  []string{"set-1"},
			call: func(client *Client) error {
				return client.UpdateGameCenterGroupGameCenterLeaderboardSetsV2Relationship(ctx, "group-1", []string{"set-1"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodPatch {
					t.Fatalf("expected PATCH, got %s", req.Method)
				}
				if req.URL.Path != tt.wantPath {
					t.Fatalf("expected path %s, got %s", tt.wantPath, req.URL.Path)
				}

				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}

				var got RelationshipRequest
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("unmarshal body: %v", err)
				}
				// For to-many relationship replace requests, "data" must be an array.
				// Ensure we don't send {"data":null} when clearing a relationship.
				if len(tt.wantIDs) == 0 && got.Data == nil {
					t.Fatalf("expected empty relationship data array, got null")
				}
				if len(got.Data) != len(tt.wantIDs) {
					t.Fatalf("expected %d relationship items, got %d", len(tt.wantIDs), len(got.Data))
				}
				for i := range tt.wantIDs {
					if got.Data[i].Type != tt.wantType {
						t.Fatalf("expected type %q, got %q", tt.wantType, got.Data[i].Type)
					}
					if got.Data[i].ID != tt.wantIDs[i] {
						t.Fatalf("expected id %q, got %q", tt.wantIDs[i], got.Data[i].ID)
					}
				}

				assertAuthorized(t, req)
			}, jsonResponse(http.StatusNoContent, ""))

			if err := tt.call(client); err != nil {
				t.Fatalf("request error: %v", err)
			}
		})
	}
}

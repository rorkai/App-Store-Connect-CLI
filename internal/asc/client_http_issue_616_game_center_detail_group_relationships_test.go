package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestIssue616_GameCenterDetailAndGroupRelationshipEndpoints_GET(t *testing.T) {
	ctx := context.Background()

	const (
		linkagesOK = `{"data":[{"type":"apps","id":"1"}],"links":{}}`
		toOneOK    = `{"data":{"type":"apps","id":"1"},"links":{}}`
	)

	tests := []struct {
		name     string
		wantPath string
		body     string
		call     func(*Client) error
	}{
		{
			name:     "GetGameCenterDetailAchievementReleasesRelationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/achievementReleases",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailAchievementReleasesRelationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailActivityReleasesRelationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/activityReleases",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailActivityReleasesRelationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailChallengeReleasesRelationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/challengeReleases",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailChallengeReleasesRelationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailGameCenterAchievementsRelationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterAchievements",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailGameCenterAchievementsRelationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailGameCenterAchievementsV2Relationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterAchievementsV2",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailGameCenterAchievementsV2Relationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailGameCenterActivitiesRelationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterActivities",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailGameCenterActivitiesRelationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailGameCenterAppVersionsRelationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterAppVersions",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailGameCenterAppVersionsRelationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailGameCenterChallengesRelationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterChallenges",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailGameCenterChallengesRelationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailGameCenterGroupRelationship",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterGroup",
			body:     toOneOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailGameCenterGroupRelationship(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailGameCenterLeaderboardSetsRelationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterLeaderboardSets",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailGameCenterLeaderboardSetsRelationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailGameCenterLeaderboardSetsV2Relationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterLeaderboardSetsV2",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailGameCenterLeaderboardSetsV2Relationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailGameCenterLeaderboardsRelationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterLeaderboards",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailGameCenterLeaderboardsRelationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailGameCenterLeaderboardsV2Relationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/gameCenterLeaderboardsV2",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailGameCenterLeaderboardsV2Relationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailLeaderboardReleasesRelationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/leaderboardReleases",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailLeaderboardReleasesRelationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterDetailLeaderboardSetReleasesRelationships",
			wantPath: "/v1/gameCenterDetails/detail-1/relationships/leaderboardSetReleases",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterDetailLeaderboardSetReleasesRelationships(ctx, "detail-1")
				return err
			},
		},
		{
			name:     "GetGameCenterGroupGameCenterAchievementsRelationships",
			wantPath: "/v1/gameCenterGroups/group-1/relationships/gameCenterAchievements",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterGroupGameCenterAchievementsRelationships(ctx, "group-1")
				return err
			},
		},
		{
			name:     "GetGameCenterGroupGameCenterAchievementsV2Relationships",
			wantPath: "/v1/gameCenterGroups/group-1/relationships/gameCenterAchievementsV2",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterGroupGameCenterAchievementsV2Relationships(ctx, "group-1")
				return err
			},
		},
		{
			name:     "GetGameCenterGroupGameCenterActivitiesRelationships",
			wantPath: "/v1/gameCenterGroups/group-1/relationships/gameCenterActivities",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterGroupGameCenterActivitiesRelationships(ctx, "group-1")
				return err
			},
		},
		{
			name:     "GetGameCenterGroupGameCenterChallengesRelationships",
			wantPath: "/v1/gameCenterGroups/group-1/relationships/gameCenterChallenges",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterGroupGameCenterChallengesRelationships(ctx, "group-1")
				return err
			},
		},
		{
			name:     "GetGameCenterGroupGameCenterDetailsRelationships",
			wantPath: "/v1/gameCenterGroups/group-1/relationships/gameCenterDetails",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterGroupGameCenterDetailsRelationships(ctx, "group-1")
				return err
			},
		},
		{
			name:     "GetGameCenterGroupGameCenterLeaderboardSetsRelationships",
			wantPath: "/v1/gameCenterGroups/group-1/relationships/gameCenterLeaderboardSets",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterGroupGameCenterLeaderboardSetsRelationships(ctx, "group-1")
				return err
			},
		},
		{
			name:     "GetGameCenterGroupGameCenterLeaderboardSetsV2Relationships",
			wantPath: "/v1/gameCenterGroups/group-1/relationships/gameCenterLeaderboardSetsV2",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterGroupGameCenterLeaderboardSetsV2Relationships(ctx, "group-1")
				return err
			},
		},
		{
			name:     "GetGameCenterGroupGameCenterLeaderboardsRelationships",
			wantPath: "/v1/gameCenterGroups/group-1/relationships/gameCenterLeaderboards",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterGroupGameCenterLeaderboardsRelationships(ctx, "group-1")
				return err
			},
		},
		{
			name:     "GetGameCenterGroupGameCenterLeaderboardsV2Relationships",
			wantPath: "/v1/gameCenterGroups/group-1/relationships/gameCenterLeaderboardsV2",
			body:     linkagesOK,
			call: func(client *Client) error {
				_, err := client.GetGameCenterGroupGameCenterLeaderboardsV2Relationships(ctx, "group-1")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", req.Method)
				}
				if req.URL.Path != tt.wantPath {
					t.Fatalf("expected path %s, got %s", tt.wantPath, req.URL.Path)
				}
				assertAuthorized(t, req)
			}, jsonResponse(http.StatusOK, tt.body))

			if err := tt.call(client); err != nil {
				t.Fatalf("request error: %v", err)
			}
		})
	}
}

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

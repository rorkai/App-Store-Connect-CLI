package asc

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestGameCenterAddRemoveRelationshipValidationErrors(t *testing.T) {
	ctx := context.Background()
	response := jsonResponse(http.StatusOK, `{"data":[]}`)

	tests := []struct {
		name    string
		wantErr string
		call    func(*Client) error
	}{
		{
			name:    "AddGameCenterLeaderboardSetMembers missing setID",
			wantErr: "setID is required",
			call: func(client *Client) error {
				return client.AddGameCenterLeaderboardSetMembers(ctx, "", []string{"lb-1"})
			},
		},
		{
			name:    "AddGameCenterLeaderboardSetMembers missing leaderboardIDs",
			wantErr: "leaderboardIDs are required",
			call: func(client *Client) error {
				return client.AddGameCenterLeaderboardSetMembers(ctx, "set-1", []string{" ", ""})
			},
		},
		{
			name:    "RemoveGameCenterLeaderboardSetMembers missing setID",
			wantErr: "setID is required",
			call: func(client *Client) error {
				return client.RemoveGameCenterLeaderboardSetMembers(ctx, " ", []string{"lb-1"})
			},
		},
		{
			name:    "RemoveGameCenterLeaderboardSetMembers missing leaderboardIDs",
			wantErr: "leaderboardIDs are required",
			call: func(client *Client) error {
				return client.RemoveGameCenterLeaderboardSetMembers(ctx, "set-1", nil)
			},
		},
		{
			name:    "AddGameCenterEnabledVersionCompatibleVersions missing enabledVersionID",
			wantErr: "enabledVersionID is required",
			call: func(client *Client) error {
				return client.AddGameCenterEnabledVersionCompatibleVersions(ctx, "", []string{"ver-1"})
			},
		},
		{
			name:    "AddGameCenterEnabledVersionCompatibleVersions missing compatibleIDs",
			wantErr: "compatibleIDs are required",
			call: func(client *Client) error {
				return client.AddGameCenterEnabledVersionCompatibleVersions(ctx, "ver-1", []string{})
			},
		},
		{
			name:    "RemoveGameCenterEnabledVersionCompatibleVersions missing enabledVersionID",
			wantErr: "enabledVersionID is required",
			call: func(client *Client) error {
				return client.RemoveGameCenterEnabledVersionCompatibleVersions(ctx, " ", []string{"ver-1"})
			},
		},
		{
			name:    "RemoveGameCenterEnabledVersionCompatibleVersions missing compatibleIDs",
			wantErr: "compatibleIDs are required",
			call: func(client *Client) error {
				return client.RemoveGameCenterEnabledVersionCompatibleVersions(ctx, "ver-1", []string{" "})
			},
		},
		{
			name:    "AddGameCenterAppVersionCompatibilityVersions missing appVersionID",
			wantErr: "appVersionID is required",
			call: func(client *Client) error {
				return client.AddGameCenterAppVersionCompatibilityVersions(ctx, "", []string{"ver-1"})
			},
		},
		{
			name:    "AddGameCenterAppVersionCompatibilityVersions missing compatibleIDs",
			wantErr: "compatibleIDs are required",
			call: func(client *Client) error {
				return client.AddGameCenterAppVersionCompatibilityVersions(ctx, "ver-1", []string{})
			},
		},
		{
			name:    "RemoveGameCenterAppVersionCompatibilityVersions missing appVersionID",
			wantErr: "appVersionID is required",
			call: func(client *Client) error {
				return client.RemoveGameCenterAppVersionCompatibilityVersions(ctx, " ", []string{"ver-1"})
			},
		},
		{
			name:    "RemoveGameCenterAppVersionCompatibilityVersions missing compatibleIDs",
			wantErr: "compatibleIDs are required",
			call: func(client *Client) error {
				return client.RemoveGameCenterAppVersionCompatibilityVersions(ctx, "ver-1", nil)
			},
		},
		{
			name:    "AddGameCenterActivityAchievementsV2 missing activityID",
			wantErr: "activityID is required",
			call: func(client *Client) error {
				return client.AddGameCenterActivityAchievementsV2(ctx, "", []string{"ach-1"})
			},
		},
		{
			name:    "AddGameCenterActivityAchievementsV2 missing achievementIDs",
			wantErr: "achievementIDs are required",
			call: func(client *Client) error {
				return client.AddGameCenterActivityAchievementsV2(ctx, "act-1", []string{" "})
			},
		},
		{
			name:    "RemoveGameCenterActivityAchievementsV2 missing activityID",
			wantErr: "activityID is required",
			call: func(client *Client) error {
				return client.RemoveGameCenterActivityAchievementsV2(ctx, " ", []string{"ach-1"})
			},
		},
		{
			name:    "RemoveGameCenterActivityAchievementsV2 missing achievementIDs",
			wantErr: "achievementIDs are required",
			call: func(client *Client) error {
				return client.RemoveGameCenterActivityAchievementsV2(ctx, "act-1", nil)
			},
		},
		{
			name:    "AddGameCenterActivityLeaderboardsV2 missing activityID",
			wantErr: "activityID is required",
			call: func(client *Client) error {
				return client.AddGameCenterActivityLeaderboardsV2(ctx, "", []string{"lb-1"})
			},
		},
		{
			name:    "AddGameCenterActivityLeaderboardsV2 missing leaderboardIDs",
			wantErr: "leaderboardIDs are required",
			call: func(client *Client) error {
				return client.AddGameCenterActivityLeaderboardsV2(ctx, "act-1", []string{})
			},
		},
		{
			name:    "RemoveGameCenterActivityLeaderboardsV2 missing activityID",
			wantErr: "activityID is required",
			call: func(client *Client) error {
				return client.RemoveGameCenterActivityLeaderboardsV2(ctx, " ", []string{"lb-1"})
			},
		},
		{
			name:    "RemoveGameCenterActivityLeaderboardsV2 missing leaderboardIDs",
			wantErr: "leaderboardIDs are required",
			call: func(client *Client) error {
				return client.RemoveGameCenterActivityLeaderboardsV2(ctx, "act-1", []string{" "})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			}, response)

			err := test.call(client)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected error to contain %q, got %v", test.wantErr, err)
			}
		})
	}
}

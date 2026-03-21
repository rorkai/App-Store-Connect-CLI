package asc

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestRelationshipMutationValidationErrors(t *testing.T) {
	ctx := context.Background()
	response := jsonResponse(http.StatusOK, `{"data":[]}`)

	tests := []struct {
		name    string
		wantErr string
		call    func(*Client) error
	}{
		{
			name:    "AddBetaGroupsToBuildWithNotify missing buildID",
			wantErr: "buildID is required",
			call: func(client *Client) error {
				_, err := client.AddBetaGroupsToBuildWithNotify(ctx, "", []string{"group-1"}, false)
				return err
			},
		},
		{
			name:    "AddBetaGroupsToBuildWithNotify missing groupIDs",
			wantErr: "groupIDs are required",
			call: func(client *Client) error {
				_, err := client.AddBetaGroupsToBuildWithNotify(ctx, "build-1", []string{" ", ""}, false)
				return err
			},
		},
		{
			name:    "RemoveBetaGroupsFromBuild missing buildID",
			wantErr: "buildID is required",
			call: func(client *Client) error {
				return client.RemoveBetaGroupsFromBuild(ctx, " ", []string{"group-1"})
			},
		},
		{
			name:    "RemoveBetaGroupsFromBuild missing groupIDs",
			wantErr: "groupIDs are required",
			call: func(client *Client) error {
				return client.RemoveBetaGroupsFromBuild(ctx, "build-1", nil)
			},
		},
		{
			name:    "AddBetaTestersToGroup missing groupID",
			wantErr: "groupID is required",
			call: func(client *Client) error {
				return client.AddBetaTestersToGroup(ctx, "", []string{"tester-1"})
			},
		},
		{
			name:    "AddBetaTestersToGroup missing testerIDs",
			wantErr: "testerIDs are required",
			call: func(client *Client) error {
				return client.AddBetaTestersToGroup(ctx, "group-1", []string{" ", ""})
			},
		},
		{
			name:    "RemoveBetaTestersFromGroup missing groupID",
			wantErr: "groupID is required",
			call: func(client *Client) error {
				return client.RemoveBetaTestersFromGroup(ctx, " ", []string{"tester-1"})
			},
		},
		{
			name:    "RemoveBetaTestersFromGroup missing testerIDs",
			wantErr: "testerIDs are required",
			call: func(client *Client) error {
				return client.RemoveBetaTestersFromGroup(ctx, "group-1", nil)
			},
		},
		{
			name:    "GetUserVisibleAppsRelationships missing userID",
			wantErr: "userID is required",
			call: func(client *Client) error {
				_, err := client.GetUserVisibleAppsRelationships(ctx, "")
				return err
			},
		},
		{
			name:    "AddUserVisibleApps missing userID",
			wantErr: "userID is required",
			call: func(client *Client) error {
				return client.AddUserVisibleApps(ctx, "", []string{"app-1"})
			},
		},
		{
			name:    "AddUserVisibleApps missing appIDs",
			wantErr: "appIDs are required",
			call: func(client *Client) error {
				return client.AddUserVisibleApps(ctx, "user-1", []string{" "})
			},
		},
		{
			name:    "RemoveUserVisibleApps missing userID",
			wantErr: "userID is required",
			call: func(client *Client) error {
				return client.RemoveUserVisibleApps(ctx, " ", []string{"app-1"})
			},
		},
		{
			name:    "RemoveUserVisibleApps missing appIDs",
			wantErr: "appIDs are required",
			call: func(client *Client) error {
				return client.RemoveUserVisibleApps(ctx, "user-1", nil)
			},
		},
		{
			name:    "SetUserVisibleApps missing userID",
			wantErr: "userID is required",
			call: func(client *Client) error {
				return client.SetUserVisibleApps(ctx, "", []string{"app-1"})
			},
		},
		{
			name:    "AddBuildsToAppEncryptionDeclaration missing buildIDs",
			wantErr: "buildIDs are required",
			call: func(client *Client) error {
				return client.AddBuildsToAppEncryptionDeclaration(ctx, "dec-1", []string{" "})
			},
		},
		{
			name:    "AddGameCenterActivityAchievements missing activityID",
			wantErr: "activityID is required",
			call: func(client *Client) error {
				return client.AddGameCenterActivityAchievements(ctx, "", []string{"ach-1"})
			},
		},
		{
			name:    "AddGameCenterActivityAchievements missing achievementIDs",
			wantErr: "achievementIDs are required",
			call: func(client *Client) error {
				return client.AddGameCenterActivityAchievements(ctx, "act-1", []string{" "})
			},
		},
		{
			name:    "RemoveGameCenterActivityAchievements missing activityID",
			wantErr: "activityID is required",
			call: func(client *Client) error {
				return client.RemoveGameCenterActivityAchievements(ctx, " ", []string{"ach-1"})
			},
		},
		{
			name:    "RemoveGameCenterActivityAchievements missing achievementIDs",
			wantErr: "achievementIDs are required",
			call: func(client *Client) error {
				return client.RemoveGameCenterActivityAchievements(ctx, "act-1", nil)
			},
		},
		{
			name:    "AddGameCenterActivityLeaderboards missing activityID",
			wantErr: "activityID is required",
			call: func(client *Client) error {
				return client.AddGameCenterActivityLeaderboards(ctx, "", []string{"lb-1"})
			},
		},
		{
			name:    "AddGameCenterActivityLeaderboards missing leaderboardIDs",
			wantErr: "leaderboardIDs are required",
			call: func(client *Client) error {
				return client.AddGameCenterActivityLeaderboards(ctx, "act-1", []string{" "})
			},
		},
		{
			name:    "RemoveGameCenterActivityLeaderboards missing activityID",
			wantErr: "activityID is required",
			call: func(client *Client) error {
				return client.RemoveGameCenterActivityLeaderboards(ctx, " ", []string{"lb-1"})
			},
		},
		{
			name:    "RemoveGameCenterActivityLeaderboards missing leaderboardIDs",
			wantErr: "leaderboardIDs are required",
			call: func(client *Client) error {
				return client.RemoveGameCenterActivityLeaderboards(ctx, "act-1", nil)
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

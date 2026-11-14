package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// UpdateGameCenterLeaderboardActivityRelationshipV2 updates the activity relationship on a v2 leaderboard.
func (c *Client) UpdateGameCenterLeaderboardActivityRelationshipV2(ctx context.Context, leaderboardID, activityID string) error {
	leaderboardID = strings.TrimSpace(leaderboardID)
	activityID = strings.TrimSpace(activityID)
	if leaderboardID == "" {
		return fmt.Errorf("leaderboardID is required")
	}
	if activityID == "" {
		return fmt.Errorf("activityID is required")
	}

	payload := struct {
		Data ResourceData `json:"data"`
	}{
		Data: ResourceData{
			Type: ResourceTypeGameCenterActivities,
			ID:   activityID,
		},
	}
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v2/gameCenterLeaderboards/%s/relationships/activity", leaderboardID), payload)
}

// UpdateGameCenterLeaderboardChallengeRelationshipV2 updates the challenge relationship on a v2 leaderboard.
func (c *Client) UpdateGameCenterLeaderboardChallengeRelationshipV2(ctx context.Context, leaderboardID, challengeID string) error {
	leaderboardID = strings.TrimSpace(leaderboardID)
	challengeID = strings.TrimSpace(challengeID)
	if leaderboardID == "" {
		return fmt.Errorf("leaderboardID is required")
	}
	if challengeID == "" {
		return fmt.Errorf("challengeID is required")
	}

	payload := struct {
		Data ResourceData `json:"data"`
	}{
		Data: ResourceData{
			Type: ResourceTypeGameCenterChallenges,
			ID:   challengeID,
		},
	}
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v2/gameCenterLeaderboards/%s/relationships/challenge", leaderboardID), payload)
}

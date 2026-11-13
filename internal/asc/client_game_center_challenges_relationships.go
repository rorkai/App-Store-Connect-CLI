package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// UpdateGameCenterChallengeLeaderboardRelationship updates the leaderboard relationship on a challenge.
func (c *Client) UpdateGameCenterChallengeLeaderboardRelationship(ctx context.Context, challengeID, leaderboardID string) error {
	challengeID = strings.TrimSpace(challengeID)
	leaderboardID = strings.TrimSpace(leaderboardID)
	if challengeID == "" {
		return fmt.Errorf("challengeID is required")
	}
	if leaderboardID == "" {
		return fmt.Errorf("leaderboardID is required")
	}

	payload := struct {
		Data ResourceData `json:"data"`
	}{
		Data: ResourceData{
			Type: ResourceTypeGameCenterLeaderboards,
			ID:   leaderboardID,
		},
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterChallenges/%s/relationships/leaderboard", challengeID)
	_, err = c.do(ctx, http.MethodPatch, path, body)
	return err
}

// UpdateGameCenterChallengeLeaderboardV2Relationship updates the leaderboardV2 relationship on a challenge.
func (c *Client) UpdateGameCenterChallengeLeaderboardV2Relationship(ctx context.Context, challengeID, leaderboardID string) error {
	challengeID = strings.TrimSpace(challengeID)
	leaderboardID = strings.TrimSpace(leaderboardID)
	if challengeID == "" {
		return fmt.Errorf("challengeID is required")
	}
	if leaderboardID == "" {
		return fmt.Errorf("leaderboardID is required")
	}

	payload := struct {
		Data ResourceData `json:"data"`
	}{
		Data: ResourceData{
			Type: ResourceTypeGameCenterLeaderboards,
			ID:   leaderboardID,
		},
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterChallenges/%s/relationships/leaderboardV2", challengeID)
	_, err = c.do(ctx, http.MethodPatch, path, body)
	return err
}

package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// AddGameCenterLeaderboardSetMembers adds leaderboards to a leaderboard set.
func (c *Client) AddGameCenterLeaderboardSetMembers(ctx context.Context, setID string, leaderboardIDs []string) error {
	setID = strings.TrimSpace(setID)
	leaderboardIDs = normalizeList(leaderboardIDs)
	if setID == "" {
		return fmt.Errorf("setID is required")
	}
	if len(leaderboardIDs) == 0 {
		return fmt.Errorf("leaderboardIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
	}
	return sendNoContent(ctx, c, http.MethodPost, fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/relationships/gameCenterLeaderboards", setID), payload)
}

// RemoveGameCenterLeaderboardSetMembers removes leaderboards from a leaderboard set.
func (c *Client) RemoveGameCenterLeaderboardSetMembers(ctx context.Context, setID string, leaderboardIDs []string) error {
	setID = strings.TrimSpace(setID)
	leaderboardIDs = normalizeList(leaderboardIDs)
	if setID == "" {
		return fmt.Errorf("setID is required")
	}
	if len(leaderboardIDs) == 0 {
		return fmt.Errorf("leaderboardIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
	}
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/relationships/gameCenterLeaderboards", setID), payload)
}

// UpdateGameCenterLeaderboardSetGroupLeaderboardSetRelationship updates the groupLeaderboardSet relationship on a leaderboard set.
func (c *Client) UpdateGameCenterLeaderboardSetGroupLeaderboardSetRelationship(ctx context.Context, setID, groupSetID string) error {
	setID = strings.TrimSpace(setID)
	groupSetID = strings.TrimSpace(groupSetID)
	if setID == "" {
		return fmt.Errorf("setID is required")
	}
	if groupSetID == "" {
		return fmt.Errorf("groupSetID is required")
	}

	payload := struct {
		Data ResourceData `json:"data"`
	}{
		Data: ResourceData{
			Type: ResourceTypeGameCenterLeaderboardSets,
			ID:   groupSetID,
		},
	}
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/relationships/groupLeaderboardSet", setID), payload)
}

// UpdateGameCenterLeaderboardActivityRelationship updates the activity relationship on a leaderboard.
func (c *Client) UpdateGameCenterLeaderboardActivityRelationship(ctx context.Context, leaderboardID, activityID string) error {
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
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/gameCenterLeaderboards/%s/relationships/activity", leaderboardID), payload)
}

// UpdateGameCenterLeaderboardChallengeRelationship updates the challenge relationship on a leaderboard.
func (c *Client) UpdateGameCenterLeaderboardChallengeRelationship(ctx context.Context, leaderboardID, challengeID string) error {
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
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/gameCenterLeaderboards/%s/relationships/challenge", leaderboardID), payload)
}

// UpdateGameCenterLeaderboardGroupLeaderboardRelationship updates the groupLeaderboard relationship on a leaderboard.
func (c *Client) UpdateGameCenterLeaderboardGroupLeaderboardRelationship(ctx context.Context, leaderboardID, groupLeaderboardID string) error {
	leaderboardID = strings.TrimSpace(leaderboardID)
	groupLeaderboardID = strings.TrimSpace(groupLeaderboardID)
	if leaderboardID == "" {
		return fmt.Errorf("leaderboardID is required")
	}
	if groupLeaderboardID == "" {
		return fmt.Errorf("groupLeaderboardID is required")
	}

	payload := struct {
		Data ResourceData `json:"data"`
	}{
		Data: ResourceData{
			Type: ResourceTypeGameCenterLeaderboards,
			ID:   groupLeaderboardID,
		},
	}
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/gameCenterLeaderboards/%s/relationships/groupLeaderboard", leaderboardID), payload)
}

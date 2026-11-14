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

package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// AddGameCenterLeaderboardSetMembersV2 adds leaderboards to a v2 leaderboard set.
func (c *Client) AddGameCenterLeaderboardSetMembersV2(ctx context.Context, setID string, leaderboardIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
	}
	return sendNoContent(ctx, c, http.MethodPost, fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s/relationships/gameCenterLeaderboards", strings.TrimSpace(setID)), payload)
}

// RemoveGameCenterLeaderboardSetMembersV2 removes leaderboards from a v2 leaderboard set.
func (c *Client) RemoveGameCenterLeaderboardSetMembersV2(ctx context.Context, setID string, leaderboardIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
	}
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s/relationships/gameCenterLeaderboards", strings.TrimSpace(setID)), payload)
}

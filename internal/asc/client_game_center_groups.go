package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// GetGameCenterGroups retrieves the list of Game Center groups.
func (c *Client) GetGameCenterGroups(ctx context.Context, opts ...GCGroupsOption) (*GameCenterGroupsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterGroupsResponse](ctx, c, readRequest{
		Path:        "/v1/gameCenterGroups",
		Query:       buildGCGroupsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-groups",
	})
}

// GetGameCenterGroup retrieves a Game Center group by ID.
func (c *Client) GetGameCenterGroup(ctx context.Context, groupID string) (*GameCenterGroupResponse, error) {
	return getJSON[GameCenterGroupResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterGroups/%s", strings.TrimSpace(groupID))})
}

// CreateGameCenterGroup creates a new Game Center group.
func (c *Client) CreateGameCenterGroup(ctx context.Context, referenceName *string) (*GameCenterGroupResponse, error) {
	var attrs *GameCenterGroupCreateAttributes
	if referenceName != nil {
		value := strings.TrimSpace(*referenceName)
		attrs = &GameCenterGroupCreateAttributes{
			ReferenceName: &value,
		}
	}

	payload := GameCenterGroupCreateRequest{
		Data: GameCenterGroupCreateData{
			Type:       ResourceTypeGameCenterGroups,
			Attributes: attrs,
		},
	}
	return sendJSON[GameCenterGroupResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterGroups", Payload: payload})
}

// UpdateGameCenterGroup updates an existing Game Center group.
func (c *Client) UpdateGameCenterGroup(ctx context.Context, groupID string, referenceName *string) (*GameCenterGroupResponse, error) {
	var attrs *GameCenterGroupUpdateAttributes
	if referenceName != nil {
		value := strings.TrimSpace(*referenceName)
		attrs = &GameCenterGroupUpdateAttributes{
			ReferenceName: &value,
		}
	}

	payload := GameCenterGroupUpdateRequest{
		Data: GameCenterGroupUpdateData{
			Type:       ResourceTypeGameCenterGroups,
			ID:         strings.TrimSpace(groupID),
			Attributes: attrs,
		},
	}
	return sendJSON[GameCenterGroupResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterGroups/%s", strings.TrimSpace(groupID)), Payload: payload})
}

// DeleteGameCenterGroup deletes a Game Center group.
func (c *Client) DeleteGameCenterGroup(ctx context.Context, groupID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterGroups/%s", strings.TrimSpace(groupID)), nil)
}

// GetGameCenterGroupAchievements retrieves the group's achievements.
func (c *Client) GetGameCenterGroupAchievements(ctx context.Context, groupID string, opts ...GCAchievementsOption) (*GameCenterAchievementsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterAchievementsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterAchievements", strings.TrimSpace(groupID)),
		Query:       buildGCAchievementsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-group-achievements",
	})
}

// GetGameCenterGroupAchievementsV2 retrieves the group's v2 achievements.
func (c *Client) GetGameCenterGroupAchievementsV2(ctx context.Context, groupID string, opts ...GCAchievementsOption) (*GameCenterAchievementsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterAchievementsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterAchievementsV2", strings.TrimSpace(groupID)),
		Query:       buildGCAchievementsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-group-achievements-v2",
	})
}

// GetGameCenterGroupActivities retrieves the group's activities.
func (c *Client) GetGameCenterGroupActivities(ctx context.Context, groupID string, opts ...GCActivitiesOption) (*GameCenterActivitiesResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterActivitiesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterActivities", strings.TrimSpace(groupID)),
		Query:       buildGCActivitiesQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-group-activities",
	})
}

// GetGameCenterGroupChallenges retrieves the group's challenges.
func (c *Client) GetGameCenterGroupChallenges(ctx context.Context, groupID string, opts ...GCChallengesOption) (*GameCenterChallengesResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterChallengesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterChallenges", strings.TrimSpace(groupID)),
		Query:       buildGCChallengesQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-group-challenges",
	})
}

// GetGameCenterGroupLeaderboards retrieves the group's leaderboards.
func (c *Client) GetGameCenterGroupLeaderboards(ctx context.Context, groupID string, opts ...GCLeaderboardsOption) (*GameCenterLeaderboardsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterLeaderboards", strings.TrimSpace(groupID)),
		Query:       buildGCLeaderboardsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-group-leaderboards",
	})
}

// GetGameCenterGroupLeaderboardsV2 retrieves the group's v2 leaderboards.
func (c *Client) GetGameCenterGroupLeaderboardsV2(ctx context.Context, groupID string, opts ...GCLeaderboardsOption) (*GameCenterLeaderboardsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterLeaderboardsV2", strings.TrimSpace(groupID)),
		Query:       buildGCLeaderboardsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-group-leaderboards-v2",
	})
}

// GetGameCenterGroupLeaderboardSets retrieves the group's leaderboard sets.
func (c *Client) GetGameCenterGroupLeaderboardSets(ctx context.Context, groupID string, opts ...GCLeaderboardSetsOption) (*GameCenterLeaderboardSetsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardSetsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterLeaderboardSets", strings.TrimSpace(groupID)),
		Query:       buildGCLeaderboardSetsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-group-leaderboard-sets",
	})
}

// GetGameCenterGroupLeaderboardSetsV2 retrieves the group's v2 leaderboard sets.
func (c *Client) GetGameCenterGroupLeaderboardSetsV2(ctx context.Context, groupID string, opts ...GCLeaderboardSetsOption) (*GameCenterLeaderboardSetsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardSetsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterLeaderboardSetsV2", strings.TrimSpace(groupID)),
		Query:       buildGCLeaderboardSetsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-group-leaderboard-sets-v2",
	})
}

// UpdateGameCenterGroupAchievements replaces the group's achievements.
func (c *Client) UpdateGameCenterGroupAchievements(ctx context.Context, groupID string, achievementIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterAchievements, achievementIDs),
	}
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/gameCenterGroups/%s/relationships/gameCenterAchievements", strings.TrimSpace(groupID)), payload)
}

// UpdateGameCenterGroupAchievementsV2 replaces the group's v2 achievements.
func (c *Client) UpdateGameCenterGroupAchievementsV2(ctx context.Context, groupID string, achievementIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterAchievements, achievementIDs),
	}
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/gameCenterGroups/%s/relationships/gameCenterAchievementsV2", strings.TrimSpace(groupID)), payload)
}

// UpdateGameCenterGroupLeaderboards replaces the group's leaderboards.
func (c *Client) UpdateGameCenterGroupLeaderboards(ctx context.Context, groupID string, leaderboardIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
	}
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/gameCenterGroups/%s/relationships/gameCenterLeaderboards", strings.TrimSpace(groupID)), payload)
}

// UpdateGameCenterGroupLeaderboardsV2 replaces the group's v2 leaderboards.
func (c *Client) UpdateGameCenterGroupLeaderboardsV2(ctx context.Context, groupID string, leaderboardIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
	}
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/gameCenterGroups/%s/relationships/gameCenterLeaderboardsV2", strings.TrimSpace(groupID)), payload)
}

package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// GetGameCenterDetail retrieves a Game Center detail by ID.
func (c *Client) GetGameCenterDetail(ctx context.Context, detailID string) (*GameCenterDetailResponse, error) {
	return getJSON[GameCenterDetailResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterDetails/%s", strings.TrimSpace(detailID))})
}

// GetGameCenterDetailGameCenterGroup retrieves the related Game Center group.
func (c *Client) GetGameCenterDetailGameCenterGroup(ctx context.Context, detailID string) (*GameCenterGroupResponse, error) {
	return getJSON[GameCenterGroupResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterGroup", strings.TrimSpace(detailID))})
}

// CreateGameCenterDetail creates a new Game Center detail.
func (c *Client) CreateGameCenterDetail(ctx context.Context, appID string, attrs *GameCenterDetailCreateAttributes) (*GameCenterDetailResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	if attrs != nil && attrs.ChallengeEnabled != nil {
		return nil, fmt.Errorf("challengeEnabled is no longer supported by App Store Connect")
	}

	payload := GameCenterDetailCreateRequest{
		Data: GameCenterDetailCreateData{
			Type:       ResourceTypeGameCenterDetails,
			Attributes: attrs,
			Relationships: &GameCenterDetailCreateRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
			},
		},
	}
	return sendJSON[GameCenterDetailResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterDetails", Payload: payload})
}

// UpdateGameCenterDetail updates an existing Game Center detail.
func (c *Client) UpdateGameCenterDetail(ctx context.Context, detailID string, attrs *GameCenterDetailUpdateAttributes, rels *GameCenterDetailUpdateRelationships) (*GameCenterDetailResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}
	if attrs != nil && attrs.ChallengeEnabled != nil {
		return nil, fmt.Errorf("challengeEnabled is no longer supported by App Store Connect")
	}

	hasAttributeUpdate := false
	hasRelationshipUpdate := false
	if rels != nil {
		for _, relationship := range []struct {
			name  string
			value *Relationship
		}{
			{"gameCenterGroup", rels.GameCenterGroup},
			{"defaultLeaderboard", rels.DefaultLeaderboard},
			{"defaultLeaderboardV2", rels.DefaultLeaderboardV2},
			{"defaultGroupLeaderboard", rels.DefaultGroupLeaderboard},
			{"defaultGroupLeaderboardV2", rels.DefaultGroupLeaderboardV2},
		} {
			if relationship.value == nil {
				continue
			}
			id := strings.TrimSpace(relationship.value.Data.ID)
			if id == "" {
				return nil, fmt.Errorf("%s relationship ID is required", relationship.name)
			}
			relationship.value.Data.ID = id
			hasRelationshipUpdate = true
		}
	}
	if !hasAttributeUpdate && !hasRelationshipUpdate {
		return nil, fmt.Errorf("at least one update field is required")
	}

	payload := GameCenterDetailUpdateRequest{
		Data: GameCenterDetailUpdateData{
			Type:          ResourceTypeGameCenterDetails,
			ID:            detailID,
			Attributes:    attrs,
			Relationships: rels,
		},
	}
	return sendJSON[GameCenterDetailResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterDetails/%s", detailID), Payload: payload})
}

// GetGameCenterDetailsAchievementReleases retrieves achievement releases for a Game Center detail.
func (c *Client) GetGameCenterDetailsAchievementReleases(ctx context.Context, gcDetailID string, opts ...GCAchievementReleasesOption) (*GameCenterAchievementReleasesResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterAchievementReleasesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/achievementReleases", strings.TrimSpace(gcDetailID)),
		Query:       buildGCAchievementReleasesQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-details-achievement-releases",
	})
}

// GetGameCenterDetailsLeaderboardReleases retrieves leaderboard releases for a Game Center detail.
func (c *Client) GetGameCenterDetailsLeaderboardReleases(ctx context.Context, gcDetailID string, opts ...GCLeaderboardReleasesOption) (*GameCenterLeaderboardReleasesResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardReleasesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/leaderboardReleases", strings.TrimSpace(gcDetailID)),
		Query:       buildGCLeaderboardReleasesQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-details-leaderboard-releases",
	})
}

// GetGameCenterDetailsLeaderboardSetReleases retrieves leaderboard set releases for a Game Center detail.
func (c *Client) GetGameCenterDetailsLeaderboardSetReleases(ctx context.Context, gcDetailID string, opts ...GCLeaderboardSetReleasesOption) (*GameCenterLeaderboardSetReleasesResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardSetReleasesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/leaderboardSetReleases", strings.TrimSpace(gcDetailID)),
		Query:       buildGCLeaderboardSetReleasesQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-details-leaderboard-set-releases",
	})
}

// GetGameCenterDetailsAchievementsV2 retrieves v2 achievements for a Game Center detail.
func (c *Client) GetGameCenterDetailsAchievementsV2(ctx context.Context, gcDetailID string, opts ...GCAchievementsOption) (*GameCenterAchievementsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterAchievementsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterAchievementsV2", strings.TrimSpace(gcDetailID)),
		Query:       buildGCAchievementsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-details-achievements-v2",
	})
}

// GetGameCenterDetailsLeaderboardsV2 retrieves v2 leaderboards for a Game Center detail.
func (c *Client) GetGameCenterDetailsLeaderboardsV2(ctx context.Context, gcDetailID string, opts ...GCLeaderboardsOption) (*GameCenterLeaderboardsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterLeaderboardsV2", strings.TrimSpace(gcDetailID)),
		Query:       buildGCLeaderboardsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-details-leaderboards-v2",
	})
}

// GetGameCenterDetailsLeaderboardSetsV2 retrieves v2 leaderboard sets for a Game Center detail.
func (c *Client) GetGameCenterDetailsLeaderboardSetsV2(ctx context.Context, gcDetailID string, opts ...GCLeaderboardSetsOption) (*GameCenterLeaderboardSetsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardSetsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterLeaderboardSetsV2", strings.TrimSpace(gcDetailID)),
		Query:       buildGCLeaderboardSetsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-details-leaderboard-sets-v2",
	})
}

// GetGameCenterDetailsClassicMatchmakingRequests retrieves classic matchmaking request metrics.
func (c *Client) GetGameCenterDetailsClassicMatchmakingRequests(ctx context.Context, gcDetailID string, opts ...GCMatchmakingMetricsOption) (*GameCenterMetricsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterMetricsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/metrics/classicMatchmakingRequests", strings.TrimSpace(gcDetailID)),
		Query:       buildGCMatchmakingQueueRequestsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-details-classic-matchmaking-requests",
	})
}

// GetGameCenterDetailsRuleBasedMatchmakingRequests retrieves rule-based matchmaking request metrics.
func (c *Client) GetGameCenterDetailsRuleBasedMatchmakingRequests(ctx context.Context, gcDetailID string, opts ...GCMatchmakingMetricsOption) (*GameCenterMetricsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterMetricsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/metrics/ruleBasedMatchmakingRequests", strings.TrimSpace(gcDetailID)),
		Query:       buildGCMatchmakingQueueRequestsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-details-rule-based-matchmaking-requests",
	})
}

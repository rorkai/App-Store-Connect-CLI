package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GetGameCenterDetailID retrieves the Game Center detail ID for an app.
func (c *Client) GetGameCenterDetailID(ctx context.Context, appID string) (string, error) {
	path := fmt.Sprintf("/v1/apps/%s/gameCenterDetail", strings.TrimSpace(appID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}

	var response GameCenterDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data.ID, nil
}

// GetGameCenterAchievements retrieves the list of Game Center achievements for a Game Center detail.
func (c *Client) GetGameCenterAchievements(ctx context.Context, gcDetailID string, opts ...GCAchievementsOption) (*GameCenterAchievementsResponse, error) {
	query := &gcAchievementsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterAchievements", strings.TrimSpace(gcDetailID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-achievements: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCAchievementsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterAchievement retrieves a Game Center achievement by ID.
func (c *Client) GetGameCenterAchievement(ctx context.Context, achievementID string) (*GameCenterAchievementResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterAchievements/%s", strings.TrimSpace(achievementID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterAchievementGroupAchievement retrieves the group achievement for an achievement.
func (c *Client) GetGameCenterAchievementGroupAchievement(ctx context.Context, achievementID string) (*GameCenterAchievementResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterAchievements/%s/groupAchievement", strings.TrimSpace(achievementID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterAchievement creates a new Game Center achievement.
func (c *Client) CreateGameCenterAchievement(ctx context.Context, gcDetailID string, attrs GameCenterAchievementCreateAttributes) (*GameCenterAchievementResponse, error) {
	payload := GameCenterAchievementCreateRequest{
		Data: GameCenterAchievementCreateData{
			Type:       ResourceTypeGameCenterAchievements,
			Attributes: attrs,
			Relationships: &GameCenterAchievementRelationships{
				GameCenterDetail: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterDetails,
						ID:   strings.TrimSpace(gcDetailID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterAchievements", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterAchievement updates an existing Game Center achievement.
func (c *Client) UpdateGameCenterAchievement(ctx context.Context, achievementID string, attrs GameCenterAchievementUpdateAttributes) (*GameCenterAchievementResponse, error) {
	payload := GameCenterAchievementUpdateRequest{
		Data: GameCenterAchievementUpdateData{
			Type:       ResourceTypeGameCenterAchievements,
			ID:         strings.TrimSpace(achievementID),
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterAchievements/%s", strings.TrimSpace(achievementID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterAchievement deletes a Game Center achievement.
func (c *Client) DeleteGameCenterAchievement(ctx context.Context, achievementID string) error {
	path := fmt.Sprintf("/v1/gameCenterAchievements/%s", strings.TrimSpace(achievementID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterLeaderboards retrieves the list of Game Center leaderboards for a Game Center detail.
func (c *Client) GetGameCenterLeaderboards(ctx context.Context, gcDetailID string, opts ...GCLeaderboardsOption) (*GameCenterLeaderboardsResponse, error) {
	query := &gcLeaderboardsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterLeaderboards", strings.TrimSpace(gcDetailID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboards: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCLeaderboardsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboard retrieves a Game Center leaderboard by ID.
func (c *Client) GetGameCenterLeaderboard(ctx context.Context, leaderboardID string) (*GameCenterLeaderboardResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterLeaderboards/%s", strings.TrimSpace(leaderboardID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardGroupLeaderboard retrieves the group leaderboard for a leaderboard.
func (c *Client) GetGameCenterLeaderboardGroupLeaderboard(ctx context.Context, leaderboardID string) (*GameCenterLeaderboardResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterLeaderboards/%s/groupLeaderboard", strings.TrimSpace(leaderboardID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterLeaderboard creates a new Game Center leaderboard.
func (c *Client) CreateGameCenterLeaderboard(ctx context.Context, gcDetailID string, attrs GameCenterLeaderboardCreateAttributes) (*GameCenterLeaderboardResponse, error) {
	payload := GameCenterLeaderboardCreateRequest{
		Data: GameCenterLeaderboardCreateData{
			Type:       ResourceTypeGameCenterLeaderboards,
			Attributes: attrs,
			Relationships: &GameCenterLeaderboardRelationships{
				GameCenterDetail: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterDetails,
						ID:   strings.TrimSpace(gcDetailID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterLeaderboards", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterLeaderboard updates an existing Game Center leaderboard.
func (c *Client) UpdateGameCenterLeaderboard(ctx context.Context, leaderboardID string, attrs GameCenterLeaderboardUpdateAttributes) (*GameCenterLeaderboardResponse, error) {
	payload := GameCenterLeaderboardUpdateRequest{
		Data: GameCenterLeaderboardUpdateData{
			Type:       ResourceTypeGameCenterLeaderboards,
			ID:         strings.TrimSpace(leaderboardID),
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterLeaderboards/%s", strings.TrimSpace(leaderboardID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterLeaderboard deletes a Game Center leaderboard.
func (c *Client) DeleteGameCenterLeaderboard(ctx context.Context, leaderboardID string) error {
	path := fmt.Sprintf("/v1/gameCenterLeaderboards/%s", strings.TrimSpace(leaderboardID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterLeaderboardSets retrieves the list of Game Center leaderboard sets for a Game Center detail.
func (c *Client) GetGameCenterLeaderboardSets(ctx context.Context, gcDetailID string, opts ...GCLeaderboardSetsOption) (*GameCenterLeaderboardSetsResponse, error) {
	query := &gcLeaderboardSetsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterLeaderboardSets", strings.TrimSpace(gcDetailID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-sets: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCLeaderboardSetsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardSet retrieves a Game Center leaderboard set by ID.
func (c *Client) GetGameCenterLeaderboardSet(ctx context.Context, setID string) (*GameCenterLeaderboardSetResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s", strings.TrimSpace(setID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardSetGroupLeaderboardSet retrieves the group leaderboard set for a leaderboard set.
func (c *Client) GetGameCenterLeaderboardSetGroupLeaderboardSet(ctx context.Context, setID string) (*GameCenterLeaderboardSetResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/groupLeaderboardSet", strings.TrimSpace(setID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterLeaderboardSet creates a new Game Center leaderboard set.
func (c *Client) CreateGameCenterLeaderboardSet(ctx context.Context, gcDetailID string, attrs GameCenterLeaderboardSetCreateAttributes) (*GameCenterLeaderboardSetResponse, error) {
	payload := GameCenterLeaderboardSetCreateRequest{
		Data: GameCenterLeaderboardSetCreateData{
			Type:       ResourceTypeGameCenterLeaderboardSets,
			Attributes: attrs,
			Relationships: &GameCenterLeaderboardSetRelationships{
				GameCenterDetail: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterDetails,
						ID:   strings.TrimSpace(gcDetailID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterLeaderboardSets", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterLeaderboardSet updates an existing Game Center leaderboard set.
func (c *Client) UpdateGameCenterLeaderboardSet(ctx context.Context, setID string, attrs GameCenterLeaderboardSetUpdateAttributes) (*GameCenterLeaderboardSetResponse, error) {
	payload := GameCenterLeaderboardSetUpdateRequest{
		Data: GameCenterLeaderboardSetUpdateData{
			Type:       ResourceTypeGameCenterLeaderboardSets,
			ID:         strings.TrimSpace(setID),
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s", strings.TrimSpace(setID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterLeaderboardSet deletes a Game Center leaderboard set.
func (c *Client) DeleteGameCenterLeaderboardSet(ctx context.Context, setID string) error {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s", strings.TrimSpace(setID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterLeaderboardLocalizations retrieves the list of localizations for a Game Center leaderboard.
func (c *Client) GetGameCenterLeaderboardLocalizations(ctx context.Context, leaderboardID string, opts ...GCLeaderboardLocalizationsOption) (*GameCenterLeaderboardLocalizationsResponse, error) {
	query := &gcLeaderboardLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterLeaderboards/%s/localizations", strings.TrimSpace(leaderboardID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-localizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCLeaderboardLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardLocalization retrieves a Game Center leaderboard localization by ID.
func (c *Client) GetGameCenterLeaderboardLocalization(ctx context.Context, localizationID string) (*GameCenterLeaderboardLocalizationResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardLocalizationImage retrieves the image for a leaderboard localization.
func (c *Client) GetGameCenterLeaderboardLocalizationImage(ctx context.Context, localizationID string) (*GameCenterLeaderboardImageResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardLocalizations/%s/gameCenterLeaderboardImage", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterLeaderboardLocalization creates a new Game Center leaderboard localization.
func (c *Client) CreateGameCenterLeaderboardLocalization(ctx context.Context, leaderboardID string, attrs GameCenterLeaderboardLocalizationCreateAttributes) (*GameCenterLeaderboardLocalizationResponse, error) {
	payload := GameCenterLeaderboardLocalizationCreateRequest{
		Data: GameCenterLeaderboardLocalizationCreateData{
			Type:       ResourceTypeGameCenterLeaderboardLocalizations,
			Attributes: attrs,
			Relationships: &GameCenterLeaderboardLocalizationRelationships{
				GameCenterLeaderboard: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboards,
						ID:   strings.TrimSpace(leaderboardID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterLeaderboardLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterLeaderboardLocalization updates an existing Game Center leaderboard localization.
func (c *Client) UpdateGameCenterLeaderboardLocalization(ctx context.Context, localizationID string, attrs GameCenterLeaderboardLocalizationUpdateAttributes) (*GameCenterLeaderboardLocalizationResponse, error) {
	payload := GameCenterLeaderboardLocalizationUpdateRequest{
		Data: GameCenterLeaderboardLocalizationUpdateData{
			Type:       ResourceTypeGameCenterLeaderboardLocalizations,
			ID:         strings.TrimSpace(localizationID),
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterLeaderboardLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterLeaderboardLocalization deletes a Game Center leaderboard localization.
func (c *Client) DeleteGameCenterLeaderboardLocalization(ctx context.Context, localizationID string) error {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardLocalizations/%s", strings.TrimSpace(localizationID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetAllGameCenterLeaderboardLocalizations retrieves all leaderboard localizations using automatic pagination.
func (c *Client) GetAllGameCenterLeaderboardLocalizations(ctx context.Context, leaderboardID string, opts ...GCLeaderboardLocalizationsOption) (*GameCenterLeaderboardLocalizationsResponse, error) {
	var allData []Resource[GameCenterLeaderboardLocalizationAttributes]

	for {
		resp, err := c.GetGameCenterLeaderboardLocalizations(ctx, leaderboardID, opts...)
		if err != nil {
			return nil, err
		}
		allData = append(allData, resp.Data...)

		if resp.Links.Next == "" {
			break
		}
		opts = []GCLeaderboardLocalizationsOption{
			WithGCLeaderboardLocalizationsNextURL(resp.Links.Next),
		}
	}

	return &GameCenterLeaderboardLocalizationsResponse{
		Data:  allData,
		Links: Links{Self: ""},
	}, nil
}

// GetGameCenterLeaderboardReleases retrieves the list of releases for a Game Center leaderboard.
func (c *Client) GetGameCenterLeaderboardReleases(ctx context.Context, leaderboardID string, opts ...GCLeaderboardReleasesOption) (*GameCenterLeaderboardReleasesResponse, error) {
	query := &gcLeaderboardReleasesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterLeaderboards/%s/releases", strings.TrimSpace(leaderboardID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-releases: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCLeaderboardReleasesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardReleasesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardRelease retrieves a leaderboard release by ID.
func (c *Client) GetGameCenterLeaderboardRelease(ctx context.Context, releaseID string) (*GameCenterLeaderboardReleaseResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardReleases/%s", strings.TrimSpace(releaseID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardReleaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterLeaderboardRelease creates a new Game Center leaderboard release.
func (c *Client) CreateGameCenterLeaderboardRelease(ctx context.Context, gcDetailID, leaderboardID string) (*GameCenterLeaderboardReleaseResponse, error) {
	payload := GameCenterLeaderboardReleaseCreateRequest{
		Data: GameCenterLeaderboardReleaseCreateData{
			Type: ResourceTypeGameCenterLeaderboardReleases,
			Relationships: &GameCenterLeaderboardReleaseRelationships{
				GameCenterDetail: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterDetails,
						ID:   strings.TrimSpace(gcDetailID),
					},
				},
				GameCenterLeaderboard: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboards,
						ID:   strings.TrimSpace(leaderboardID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterLeaderboardReleases", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardReleaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterLeaderboardRelease deletes a Game Center leaderboard release.
func (c *Client) DeleteGameCenterLeaderboardRelease(ctx context.Context, releaseID string) error {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardReleases/%s", strings.TrimSpace(releaseID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterAchievementReleases retrieves the list of releases for a Game Center achievement.
func (c *Client) GetGameCenterAchievementReleases(ctx context.Context, achievementID string, opts ...GCAchievementReleasesOption) (*GameCenterAchievementReleasesResponse, error) {
	query := &gcAchievementReleasesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterAchievements/%s/releases", strings.TrimSpace(achievementID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-achievement-releases: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCAchievementReleasesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementReleasesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterAchievementRelease retrieves an achievement release by ID.
func (c *Client) GetGameCenterAchievementRelease(ctx context.Context, releaseID string) (*GameCenterAchievementReleaseResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterAchievementReleases/%s", strings.TrimSpace(releaseID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementReleaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterAchievementRelease creates a new Game Center achievement release.
func (c *Client) CreateGameCenterAchievementRelease(ctx context.Context, gcDetailID, achievementID string) (*GameCenterAchievementReleaseResponse, error) {
	payload := GameCenterAchievementReleaseCreateRequest{
		Data: GameCenterAchievementReleaseCreateData{
			Type: ResourceTypeGameCenterAchievementReleases,
			Relationships: &GameCenterAchievementReleaseRelationships{
				GameCenterDetail: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterDetails,
						ID:   strings.TrimSpace(gcDetailID),
					},
				},
				GameCenterAchievement: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterAchievements,
						ID:   strings.TrimSpace(achievementID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterAchievementReleases", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementReleaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterAchievementRelease deletes a Game Center achievement release.
func (c *Client) DeleteGameCenterAchievementRelease(ctx context.Context, releaseID string) error {
	path := fmt.Sprintf("/v1/gameCenterAchievementReleases/%s", strings.TrimSpace(releaseID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterLeaderboardSetMembers retrieves the leaderboards in a leaderboard set.
func (c *Client) GetGameCenterLeaderboardSetMembers(ctx context.Context, setID string, opts ...GCLeaderboardSetMembersOption) (*GameCenterLeaderboardsResponse, error) {
	query := &gcLeaderboardSetMembersQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/gameCenterLeaderboards", strings.TrimSpace(setID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-set-members: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCLeaderboardSetMembersQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterLeaderboardSetMembers replaces all leaderboard members in a leaderboard set.
func (c *Client) UpdateGameCenterLeaderboardSetMembers(ctx context.Context, setID string, leaderboardIDs []string) error {
	leaderboardIDs = normalizeList(leaderboardIDs)

	payload := GameCenterLeaderboardSetMembersUpdateRequest{
		Data: make([]RelationshipData, 0, len(leaderboardIDs)),
	}
	for _, leaderboardID := range leaderboardIDs {
		payload.Data = append(payload.Data, RelationshipData{
			Type: ResourceTypeGameCenterLeaderboards,
			ID:   leaderboardID,
		})
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/relationships/gameCenterLeaderboards", strings.TrimSpace(setID))
	_, err = c.do(ctx, http.MethodPatch, path, body)
	return err
}

// GetGameCenterLeaderboardSetReleases retrieves the list of releases for a Game Center leaderboard set.
func (c *Client) GetGameCenterLeaderboardSetReleases(ctx context.Context, setID string, opts ...GCLeaderboardSetReleasesOption) (*GameCenterLeaderboardSetReleasesResponse, error) {
	query := &gcLeaderboardSetReleasesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/releases", strings.TrimSpace(setID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-set-releases: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCLeaderboardSetReleasesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetReleasesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardSetRelease retrieves a leaderboard set release by ID.
func (c *Client) GetGameCenterLeaderboardSetRelease(ctx context.Context, releaseID string) (*GameCenterLeaderboardSetReleaseResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardSetReleases/%s", strings.TrimSpace(releaseID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetReleaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterLeaderboardSetRelease creates a new Game Center leaderboard set release.
func (c *Client) CreateGameCenterLeaderboardSetRelease(ctx context.Context, gcDetailID, setID string) (*GameCenterLeaderboardSetReleaseResponse, error) {
	payload := GameCenterLeaderboardSetReleaseCreateRequest{
		Data: GameCenterLeaderboardSetReleaseCreateData{
			Type: ResourceTypeGameCenterLeaderboardSetReleases,
			Relationships: &GameCenterLeaderboardSetReleaseRelationships{
				GameCenterDetail: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterDetails,
						ID:   strings.TrimSpace(gcDetailID),
					},
				},
				GameCenterLeaderboardSet: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboardSets,
						ID:   strings.TrimSpace(setID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterLeaderboardSetReleases", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetReleaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterLeaderboardSetRelease deletes a Game Center leaderboard set release.
func (c *Client) DeleteGameCenterLeaderboardSetRelease(ctx context.Context, releaseID string) error {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardSetReleases/%s", strings.TrimSpace(releaseID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterLeaderboardSetLocalizations retrieves the list of localizations for a Game Center leaderboard set.
func (c *Client) GetGameCenterLeaderboardSetLocalizations(ctx context.Context, setID string, opts ...GCLeaderboardSetLocalizationsOption) (*GameCenterLeaderboardSetLocalizationsResponse, error) {
	query := &gcLeaderboardSetLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/localizations", strings.TrimSpace(setID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-set-localizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCLeaderboardSetLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardSetLocalization retrieves a Game Center leaderboard set localization by ID.
func (c *Client) GetGameCenterLeaderboardSetLocalization(ctx context.Context, localizationID string) (*GameCenterLeaderboardSetLocalizationResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardSetLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardSetLocalizationImage retrieves the image for a leaderboard set localization.
func (c *Client) GetGameCenterLeaderboardSetLocalizationImage(ctx context.Context, localizationID string) (*GameCenterLeaderboardSetImageResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardSetLocalizations/%s/gameCenterLeaderboardSetImage", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterLeaderboardSetLocalization creates a new Game Center leaderboard set localization.
func (c *Client) CreateGameCenterLeaderboardSetLocalization(ctx context.Context, setID string, attrs GameCenterLeaderboardSetLocalizationCreateAttributes) (*GameCenterLeaderboardSetLocalizationResponse, error) {
	payload := GameCenterLeaderboardSetLocalizationCreateRequest{
		Data: GameCenterLeaderboardSetLocalizationCreateData{
			Type:       ResourceTypeGameCenterLeaderboardSetLocalizations,
			Attributes: attrs,
			Relationships: &GameCenterLeaderboardSetLocalizationRelationships{
				GameCenterLeaderboardSet: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboardSets,
						ID:   strings.TrimSpace(setID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterLeaderboardSetLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterLeaderboardSetLocalization updates an existing Game Center leaderboard set localization.
func (c *Client) UpdateGameCenterLeaderboardSetLocalization(ctx context.Context, localizationID string, attrs GameCenterLeaderboardSetLocalizationUpdateAttributes) (*GameCenterLeaderboardSetLocalizationResponse, error) {
	payload := GameCenterLeaderboardSetLocalizationUpdateRequest{
		Data: GameCenterLeaderboardSetLocalizationUpdateData{
			Type:       ResourceTypeGameCenterLeaderboardSetLocalizations,
			ID:         strings.TrimSpace(localizationID),
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterLeaderboardSetLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterLeaderboardSetLocalization deletes a Game Center leaderboard set localization.
func (c *Client) DeleteGameCenterLeaderboardSetLocalization(ctx context.Context, localizationID string) error {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardSetLocalizations/%s", strings.TrimSpace(localizationID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterAchievementLocalizations retrieves the list of localizations for a Game Center achievement.
func (c *Client) GetGameCenterAchievementLocalizations(ctx context.Context, achievementID string, opts ...GCAchievementLocalizationsOption) (*GameCenterAchievementLocalizationsResponse, error) {
	query := &gcAchievementLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterAchievements/%s/localizations", strings.TrimSpace(achievementID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-achievement-localizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCAchievementLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterAchievementLocalization retrieves a Game Center achievement localization by ID.
func (c *Client) GetGameCenterAchievementLocalization(ctx context.Context, localizationID string) (*GameCenterAchievementLocalizationResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterAchievementLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterAchievementLocalizationAchievement retrieves the achievement for a localization.
func (c *Client) GetGameCenterAchievementLocalizationAchievement(ctx context.Context, localizationID string) (*GameCenterAchievementResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterAchievementLocalizations/%s/gameCenterAchievement", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterAchievementLocalizationImage retrieves the image for a localization.
func (c *Client) GetGameCenterAchievementLocalizationImage(ctx context.Context, localizationID string) (*GameCenterAchievementImageResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterAchievementLocalizations/%s/gameCenterAchievementImage", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterAchievementLocalization creates a new Game Center achievement localization.
func (c *Client) CreateGameCenterAchievementLocalization(ctx context.Context, achievementID string, attrs GameCenterAchievementLocalizationCreateAttributes) (*GameCenterAchievementLocalizationResponse, error) {
	payload := GameCenterAchievementLocalizationCreateRequest{
		Data: GameCenterAchievementLocalizationCreateData{
			Type:       ResourceTypeGameCenterAchievementLocalizations,
			Attributes: attrs,
			Relationships: &GameCenterAchievementLocalizationRelationships{
				GameCenterAchievement: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterAchievements,
						ID:   strings.TrimSpace(achievementID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterAchievementLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterAchievementLocalization updates an existing Game Center achievement localization.
func (c *Client) UpdateGameCenterAchievementLocalization(ctx context.Context, localizationID string, attrs GameCenterAchievementLocalizationUpdateAttributes) (*GameCenterAchievementLocalizationResponse, error) {
	payload := GameCenterAchievementLocalizationUpdateRequest{
		Data: GameCenterAchievementLocalizationUpdateData{
			Type:       ResourceTypeGameCenterAchievementLocalizations,
			ID:         strings.TrimSpace(localizationID),
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterAchievementLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterAchievementLocalization deletes a Game Center achievement localization.
func (c *Client) DeleteGameCenterAchievementLocalization(ctx context.Context, localizationID string) error {
	path := fmt.Sprintf("/v1/gameCenterAchievementLocalizations/%s", strings.TrimSpace(localizationID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterLeaderboardImage retrieves a Game Center leaderboard image by ID.
func (c *Client) GetGameCenterLeaderboardImage(ctx context.Context, imageID string) (*GameCenterLeaderboardImageResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardImages/%s", strings.TrimSpace(imageID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterLeaderboardImage reserves an image upload for a leaderboard localization.
func (c *Client) CreateGameCenterLeaderboardImage(ctx context.Context, localizationID string, fileName string, fileSize int64) (*GameCenterLeaderboardImageResponse, error) {
	payload := GameCenterLeaderboardImageCreateRequest{
		Data: GameCenterLeaderboardImageCreateData{
			Type: ResourceTypeGameCenterLeaderboardImages,
			Attributes: GameCenterLeaderboardImageCreateAttributes{
				FileSize: fileSize,
				FileName: fileName,
			},
			Relationships: &GameCenterLeaderboardImageRelationships{
				GameCenterLeaderboardLocalization: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboardLocalizations,
						ID:   strings.TrimSpace(localizationID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterLeaderboardImages", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterLeaderboardImage commits an image upload.
func (c *Client) UpdateGameCenterLeaderboardImage(ctx context.Context, imageID string, uploaded bool) (*GameCenterLeaderboardImageResponse, error) {
	payload := GameCenterLeaderboardImageUpdateRequest{
		Data: GameCenterLeaderboardImageUpdateData{
			Type: ResourceTypeGameCenterLeaderboardImages,
			ID:   strings.TrimSpace(imageID),
			Attributes: &GameCenterLeaderboardImageUpdateAttributes{
				Uploaded: &uploaded,
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterLeaderboardImages/%s", strings.TrimSpace(imageID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterLeaderboardImage deletes a Game Center leaderboard image.
func (c *Client) DeleteGameCenterLeaderboardImage(ctx context.Context, imageID string) error {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardImages/%s", strings.TrimSpace(imageID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// UploadGameCenterLeaderboardImage performs the full upload flow for a leaderboard image.
// It reserves the upload, uploads the file, and commits the upload.
func (c *Client) UploadGameCenterLeaderboardImage(ctx context.Context, localizationID string, filePath string) (*GameCenterLeaderboardImageUploadResult, error) {
	// Validate the file
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	// Step 1: Reserve the upload
	reservation, err := c.CreateGameCenterLeaderboardImage(ctx, localizationID, fileName, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve image upload: %w", err)
	}

	imageID := reservation.Data.ID
	operations := reservation.Data.Attributes.UploadOperations

	if len(operations) == 0 {
		return nil, fmt.Errorf("no upload operations returned from API")
	}

	// Step 2: Upload the file
	if err := UploadAsset(ctx, filePath, operations); err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	// Step 3: Commit the upload
	committed, err := c.UpdateGameCenterLeaderboardImage(ctx, imageID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit image upload: %w", err)
	}

	state := ""
	if committed.Data.Attributes.AssetDeliveryState != nil {
		state = committed.Data.Attributes.AssetDeliveryState.State
	}

	return &GameCenterLeaderboardImageUploadResult{
		ID:                 committed.Data.ID,
		LocalizationID:     localizationID,
		FileName:           fileName,
		FileSize:           fileSize,
		AssetDeliveryState: state,
		Uploaded:           true,
	}, nil
}

// GetGameCenterAchievementImage retrieves a Game Center achievement image by ID.
func (c *Client) GetGameCenterAchievementImage(ctx context.Context, imageID string) (*GameCenterAchievementImageResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterAchievementImages/%s", strings.TrimSpace(imageID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterAchievementImage reserves a new Game Center achievement image upload.
func (c *Client) CreateGameCenterAchievementImage(ctx context.Context, localizationID string, fileName string, fileSize int64) (*GameCenterAchievementImageResponse, error) {
	payload := GameCenterAchievementImageCreateRequest{
		Data: GameCenterAchievementImageCreateData{
			Type: ResourceTypeGameCenterAchievementImages,
			Attributes: GameCenterAchievementImageCreateAttributes{
				FileSize: fileSize,
				FileName: fileName,
			},
			Relationships: &GameCenterAchievementImageRelationships{
				GameCenterAchievementLocalization: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterAchievementLocalizations,
						ID:   strings.TrimSpace(localizationID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterAchievementImages", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterAchievementImage updates a Game Center achievement image (used to commit upload).
func (c *Client) UpdateGameCenterAchievementImage(ctx context.Context, imageID string, uploaded bool) (*GameCenterAchievementImageResponse, error) {
	payload := GameCenterAchievementImageUpdateRequest{
		Data: GameCenterAchievementImageUpdateData{
			Type: ResourceTypeGameCenterAchievementImages,
			ID:   strings.TrimSpace(imageID),
			Attributes: &GameCenterAchievementImageUpdateAttributes{
				Uploaded: &uploaded,
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterAchievementImages/%s", strings.TrimSpace(imageID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterAchievementImage deletes a Game Center achievement image.
func (c *Client) DeleteGameCenterAchievementImage(ctx context.Context, imageID string) error {
	path := fmt.Sprintf("/v1/gameCenterAchievementImages/%s", strings.TrimSpace(imageID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// UploadGameCenterAchievementImage performs the complete upload flow: reserve, upload chunks, commit.
func (c *Client) UploadGameCenterAchievementImage(ctx context.Context, localizationID string, filePath string) (*GameCenterAchievementImageUploadResult, error) {
	// Validate the file
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	// Step 1: Reserve the upload
	reservation, err := c.CreateGameCenterAchievementImage(ctx, localizationID, fileName, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve image upload: %w", err)
	}

	imageID := reservation.Data.ID
	operations := reservation.Data.Attributes.UploadOperations

	if len(operations) == 0 {
		return nil, fmt.Errorf("no upload operations returned from API")
	}

	// Step 2: Upload the file
	if err := UploadAsset(ctx, filePath, operations); err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	// Step 3: Commit the upload
	committed, err := c.UpdateGameCenterAchievementImage(ctx, imageID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit image upload: %w", err)
	}

	state := ""
	if committed.Data.Attributes.AssetDeliveryState != nil {
		state = committed.Data.Attributes.AssetDeliveryState.State
	}

	return &GameCenterAchievementImageUploadResult{
		ID:                 committed.Data.ID,
		LocalizationID:     localizationID,
		FileName:           fileName,
		FileSize:           fileSize,
		AssetDeliveryState: state,
		Uploaded:           true,
	}, nil
}

// GetGameCenterLeaderboardSetImage retrieves a Game Center leaderboard set image by ID.
func (c *Client) GetGameCenterLeaderboardSetImage(ctx context.Context, imageID string) (*GameCenterLeaderboardSetImageResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardSetImages/%s", strings.TrimSpace(imageID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterLeaderboardSetImage reserves an image upload slot for a leaderboard set localization.
func (c *Client) CreateGameCenterLeaderboardSetImage(ctx context.Context, localizationID string, fileName string, fileSize int64) (*GameCenterLeaderboardSetImageResponse, error) {
	payload := GameCenterLeaderboardSetImageCreateRequest{
		Data: GameCenterLeaderboardSetImageCreateData{
			Type: ResourceTypeGameCenterLeaderboardSetImages,
			Attributes: GameCenterLeaderboardSetImageCreateAttributes{
				FileSize: fileSize,
				FileName: fileName,
			},
			Relationships: &GameCenterLeaderboardSetImageRelationships{
				GameCenterLeaderboardSetLocalization: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboardSetLocalizations,
						ID:   strings.TrimSpace(localizationID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterLeaderboardSetImages", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterLeaderboardSetImage commits an image upload (sets uploaded=true).
func (c *Client) UpdateGameCenterLeaderboardSetImage(ctx context.Context, imageID string, uploaded bool) (*GameCenterLeaderboardSetImageResponse, error) {
	payload := GameCenterLeaderboardSetImageUpdateRequest{
		Data: GameCenterLeaderboardSetImageUpdateData{
			Type: ResourceTypeGameCenterLeaderboardSetImages,
			ID:   strings.TrimSpace(imageID),
			Attributes: &GameCenterLeaderboardSetImageUpdateAttributes{
				Uploaded: &uploaded,
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterLeaderboardSetImages/%s", strings.TrimSpace(imageID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterLeaderboardSetImage deletes a Game Center leaderboard set image.
func (c *Client) DeleteGameCenterLeaderboardSetImage(ctx context.Context, imageID string) error {
	path := fmt.Sprintf("/v1/gameCenterLeaderboardSetImages/%s", strings.TrimSpace(imageID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// UploadGameCenterLeaderboardSetImage performs the full upload flow for a leaderboard set image.
// It reserves an upload slot, uploads the file, and commits the upload.
func (c *Client) UploadGameCenterLeaderboardSetImage(ctx context.Context, localizationID, filePath string) (*GameCenterLeaderboardSetImageUploadResult, error) {
	// Validate the image file
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	// Get file info
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	// Step 1: Reserve upload slot
	reservation, err := c.CreateGameCenterLeaderboardSetImage(ctx, localizationID, fileName, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve upload: %w", err)
	}

	imageID := reservation.Data.ID
	operations := reservation.Data.Attributes.UploadOperations

	if len(operations) == 0 {
		return nil, fmt.Errorf("no upload operations returned from reservation")
	}

	// Step 2: Upload the file
	if err := UploadAsset(ctx, filePath, operations); err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	// Step 3: Commit the upload
	committed, err := c.UpdateGameCenterLeaderboardSetImage(ctx, imageID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit upload: %w", err)
	}

	result := &GameCenterLeaderboardSetImageUploadResult{
		ID:             committed.Data.ID,
		LocalizationID: localizationID,
		FileName:       committed.Data.Attributes.FileName,
		FileSize:       committed.Data.Attributes.FileSize,
		Uploaded:       true,
	}

	if committed.Data.Attributes.AssetDeliveryState != nil {
		result.AssetDeliveryState = committed.Data.Attributes.AssetDeliveryState.State
	}

	return result, nil
}

// GetGameCenterAchievementV2 retrieves a v2 Game Center achievement by ID.
func (c *Client) GetGameCenterAchievementV2(ctx context.Context, achievementID string) (*GameCenterAchievementResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterAchievements/%s", strings.TrimSpace(achievementID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterAchievementV2 creates a new v2 Game Center achievement.
func (c *Client) CreateGameCenterAchievementV2(ctx context.Context, gcDetailID string, groupID string, attrs GameCenterAchievementCreateAttributes) (*GameCenterAchievementResponse, error) {
	relationships := &GameCenterAchievementV2Relationships{}
	hasRelationship := false

	if strings.TrimSpace(gcDetailID) != "" {
		relationships.GameCenterDetail = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterDetails,
				ID:   strings.TrimSpace(gcDetailID),
			},
		}
		hasRelationship = true
	}
	if strings.TrimSpace(groupID) != "" {
		relationships.GameCenterGroup = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterGroups,
				ID:   strings.TrimSpace(groupID),
			},
		}
		hasRelationship = true
	}
	if !hasRelationship {
		relationships = nil
	}

	payload := GameCenterAchievementV2CreateRequest{
		Data: GameCenterAchievementV2CreateData{
			Type:          ResourceTypeGameCenterAchievements,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v2/gameCenterAchievements", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterAchievementV2 updates an existing v2 Game Center achievement.
func (c *Client) UpdateGameCenterAchievementV2(ctx context.Context, achievementID string, attrs GameCenterAchievementUpdateAttributes) (*GameCenterAchievementResponse, error) {
	payload := GameCenterAchievementUpdateRequest{
		Data: GameCenterAchievementUpdateData{
			Type:       ResourceTypeGameCenterAchievements,
			ID:         strings.TrimSpace(achievementID),
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v2/gameCenterAchievements/%s", strings.TrimSpace(achievementID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterAchievementV2 deletes a v2 Game Center achievement.
func (c *Client) DeleteGameCenterAchievementV2(ctx context.Context, achievementID string) error {
	path := fmt.Sprintf("/v2/gameCenterAchievements/%s", strings.TrimSpace(achievementID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterLeaderboardV2 retrieves a v2 Game Center leaderboard by ID.
func (c *Client) GetGameCenterLeaderboardV2(ctx context.Context, leaderboardID string) (*GameCenterLeaderboardResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterLeaderboards/%s", strings.TrimSpace(leaderboardID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterLeaderboardV2 creates a new v2 Game Center leaderboard.
func (c *Client) CreateGameCenterLeaderboardV2(ctx context.Context, gcDetailID string, groupID string, attrs GameCenterLeaderboardCreateAttributes) (*GameCenterLeaderboardResponse, error) {
	relationships := &GameCenterLeaderboardV2Relationships{}
	hasRelationship := false

	if strings.TrimSpace(gcDetailID) != "" {
		relationships.GameCenterDetail = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterDetails,
				ID:   strings.TrimSpace(gcDetailID),
			},
		}
		hasRelationship = true
	}
	if strings.TrimSpace(groupID) != "" {
		relationships.GameCenterGroup = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterGroups,
				ID:   strings.TrimSpace(groupID),
			},
		}
		hasRelationship = true
	}
	if !hasRelationship {
		relationships = nil
	}

	payload := GameCenterLeaderboardV2CreateRequest{
		Data: GameCenterLeaderboardV2CreateData{
			Type:          ResourceTypeGameCenterLeaderboards,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v2/gameCenterLeaderboards", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterLeaderboardV2 updates an existing v2 Game Center leaderboard.
func (c *Client) UpdateGameCenterLeaderboardV2(ctx context.Context, leaderboardID string, attrs GameCenterLeaderboardUpdateAttributes) (*GameCenterLeaderboardResponse, error) {
	payload := GameCenterLeaderboardUpdateRequest{
		Data: GameCenterLeaderboardUpdateData{
			Type:       ResourceTypeGameCenterLeaderboards,
			ID:         strings.TrimSpace(leaderboardID),
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v2/gameCenterLeaderboards/%s", strings.TrimSpace(leaderboardID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterLeaderboardV2 deletes a v2 Game Center leaderboard.
func (c *Client) DeleteGameCenterLeaderboardV2(ctx context.Context, leaderboardID string) error {
	path := fmt.Sprintf("/v2/gameCenterLeaderboards/%s", strings.TrimSpace(leaderboardID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

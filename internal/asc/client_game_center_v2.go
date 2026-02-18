package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GameCenterAchievementLocalizationV2ImageLinkageResponse is the response for v2 achievement localization image relationship.
type GameCenterAchievementLocalizationV2ImageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GameCenterLeaderboardLocalizationV2ImageLinkageResponse is the response for v2 leaderboard localization image relationship.
type GameCenterLeaderboardLocalizationV2ImageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GameCenterLeaderboardSetLocalizationV2ImageLinkageResponse is the response for v2 leaderboard set localization image relationship.
type GameCenterLeaderboardSetLocalizationV2ImageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GetGameCenterAchievementsV2 retrieves v2 achievements for a Game Center detail or group.
func (c *Client) GetGameCenterAchievementsV2(ctx context.Context, gcDetailID, groupID string, opts ...GCAchievementsOption) (*GameCenterAchievementsResponse, error) {
	query := &gcAchievementsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterAchievementsV2", strings.TrimSpace(gcDetailID))
	if strings.TrimSpace(groupID) != "" {
		path = fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterAchievementsV2", strings.TrimSpace(groupID))
	}
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-achievements-v2: %w", err)
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

// GetGameCenterAchievementVersions retrieves versions for a v2 achievement.
func (c *Client) GetGameCenterAchievementVersions(ctx context.Context, achievementID string, opts ...GCAchievementVersionsOption) (*GameCenterAchievementVersionsResponse, error) {
	query := &gcAchievementVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v2/gameCenterAchievements/%s/versions", strings.TrimSpace(achievementID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-achievement-versions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCAchievementVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterAchievementVersion retrieves a v2 achievement version by ID.
func (c *Client) GetGameCenterAchievementVersion(ctx context.Context, versionID string) (*GameCenterAchievementVersionResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterAchievementVersions/%s", strings.TrimSpace(versionID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterAchievementVersion creates a new v2 achievement version.
func (c *Client) CreateGameCenterAchievementVersion(ctx context.Context, achievementID string) (*GameCenterAchievementVersionResponse, error) {
	payload := GameCenterAchievementVersionCreateRequest{
		Data: GameCenterAchievementVersionCreateData{
			Type: ResourceTypeGameCenterAchievementVersions,
			Relationships: &GameCenterAchievementVersionRelationships{
				Achievement: &Relationship{
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

	data, err := c.do(ctx, http.MethodPost, "/v2/gameCenterAchievementVersions", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterAchievementVersionLocalizations retrieves localizations for a v2 achievement version.
func (c *Client) GetGameCenterAchievementVersionLocalizations(ctx context.Context, versionID string, opts ...GCAchievementLocalizationsOption) (*GameCenterAchievementLocalizationsResponse, error) {
	query := &gcAchievementLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v2/gameCenterAchievementVersions/%s/localizations", strings.TrimSpace(versionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-achievement-version-localizations: %w", err)
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

// GetGameCenterAchievementVersionLocalizationsRelationships retrieves localization linkages for a v2 achievement version.
func (c *Client) GetGameCenterAchievementVersionLocalizationsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v2/gameCenterAchievementVersions/%s/relationships/localizations", strings.TrimSpace(versionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-achievement-version-localizations-relationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterAchievementLocalizationV2 retrieves a v2 achievement localization by ID.
func (c *Client) GetGameCenterAchievementLocalizationV2(ctx context.Context, localizationID string) (*GameCenterAchievementLocalizationResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterAchievementLocalizations/%s", strings.TrimSpace(localizationID))
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

// CreateGameCenterAchievementLocalizationV2 creates a new v2 achievement localization.
func (c *Client) CreateGameCenterAchievementLocalizationV2(ctx context.Context, versionID string, attrs GameCenterAchievementLocalizationCreateAttributes) (*GameCenterAchievementLocalizationResponse, error) {
	payload := GameCenterAchievementLocalizationV2CreateRequest{
		Data: GameCenterAchievementLocalizationV2CreateData{
			Type:       ResourceTypeGameCenterAchievementLocalizations,
			Attributes: attrs,
			Relationships: &GameCenterAchievementLocalizationV2Relationships{
				Version: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterAchievementVersions,
						ID:   strings.TrimSpace(versionID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v2/gameCenterAchievementLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterAchievementLocalizationV2 updates a v2 achievement localization.
func (c *Client) UpdateGameCenterAchievementLocalizationV2(ctx context.Context, localizationID string, attrs GameCenterAchievementLocalizationUpdateAttributes) (*GameCenterAchievementLocalizationResponse, error) {
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

	path := fmt.Sprintf("/v2/gameCenterAchievementLocalizations/%s", strings.TrimSpace(localizationID))
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

// DeleteGameCenterAchievementLocalizationV2 deletes a v2 achievement localization.
func (c *Client) DeleteGameCenterAchievementLocalizationV2(ctx context.Context, localizationID string) error {
	path := fmt.Sprintf("/v2/gameCenterAchievementLocalizations/%s", strings.TrimSpace(localizationID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterAchievementLocalizationImageV2 retrieves the image for a v2 achievement localization.
func (c *Client) GetGameCenterAchievementLocalizationImageV2(ctx context.Context, localizationID string) (*GameCenterAchievementImageResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterAchievementLocalizations/%s/image", strings.TrimSpace(localizationID))
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

// GetGameCenterAchievementLocalizationImageRelationshipV2 retrieves the image linkage for a v2 achievement localization.
func (c *Client) GetGameCenterAchievementLocalizationImageRelationshipV2(ctx context.Context, localizationID string) (*GameCenterAchievementLocalizationV2ImageLinkageResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterAchievementLocalizations/%s/relationships/image", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementLocalizationV2ImageLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterAchievementImageV2 retrieves a v2 achievement image by ID.
func (c *Client) GetGameCenterAchievementImageV2(ctx context.Context, imageID string) (*GameCenterAchievementImageResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterAchievementImages/%s", strings.TrimSpace(imageID))
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

// CreateGameCenterAchievementImageV2 reserves a new v2 achievement image upload.
func (c *Client) CreateGameCenterAchievementImageV2(ctx context.Context, localizationID, fileName string, fileSize int64) (*GameCenterAchievementImageResponse, error) {
	payload := GameCenterAchievementImageV2CreateRequest{
		Data: GameCenterAchievementImageV2CreateData{
			Type: ResourceTypeGameCenterAchievementImages,
			Attributes: GameCenterAchievementImageCreateAttributes{
				FileSize: fileSize,
				FileName: fileName,
			},
			Relationships: &GameCenterAchievementImageV2Relationships{
				Localization: &Relationship{
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

	data, err := c.do(ctx, http.MethodPost, "/v2/gameCenterAchievementImages", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterAchievementImageV2 updates a v2 achievement image (used to commit upload).
func (c *Client) UpdateGameCenterAchievementImageV2(ctx context.Context, imageID string, uploaded bool) (*GameCenterAchievementImageResponse, error) {
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

	path := fmt.Sprintf("/v2/gameCenterAchievementImages/%s", strings.TrimSpace(imageID))
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

// DeleteGameCenterAchievementImageV2 deletes a v2 achievement image.
func (c *Client) DeleteGameCenterAchievementImageV2(ctx context.Context, imageID string) error {
	path := fmt.Sprintf("/v2/gameCenterAchievementImages/%s", strings.TrimSpace(imageID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// UploadGameCenterAchievementImageV2 performs the full upload flow for a v2 achievement image.
func (c *Client) UploadGameCenterAchievementImageV2(ctx context.Context, localizationID, filePath string) (*GameCenterAchievementImageUploadResult, error) {
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	reservation, err := c.CreateGameCenterAchievementImageV2(ctx, localizationID, fileName, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve upload: %w", err)
	}

	imageID := reservation.Data.ID
	operations := reservation.Data.Attributes.UploadOperations
	if len(operations) == 0 {
		return nil, fmt.Errorf("no upload operations returned from reservation")
	}

	if err := UploadAsset(ctx, filePath, operations); err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	committed, err := c.UpdateGameCenterAchievementImageV2(ctx, imageID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit upload: %w", err)
	}

	result := &GameCenterAchievementImageUploadResult{
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

// GetGameCenterLeaderboardsV2 retrieves v2 leaderboards for a Game Center detail or group.
func (c *Client) GetGameCenterLeaderboardsV2(ctx context.Context, gcDetailID, groupID string, opts ...GCLeaderboardsOption) (*GameCenterLeaderboardsResponse, error) {
	query := &gcLeaderboardsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterLeaderboardsV2", strings.TrimSpace(gcDetailID))
	if strings.TrimSpace(groupID) != "" {
		path = fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterLeaderboardsV2", strings.TrimSpace(groupID))
	}
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboards-v2: %w", err)
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

// GetGameCenterLeaderboardVersions retrieves versions for a v2 leaderboard.
func (c *Client) GetGameCenterLeaderboardVersions(ctx context.Context, leaderboardID string, opts ...GCLeaderboardVersionsOption) (*GameCenterLeaderboardVersionsResponse, error) {
	query := &gcLeaderboardVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v2/gameCenterLeaderboards/%s/versions", strings.TrimSpace(leaderboardID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-versions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCLeaderboardVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardVersion retrieves a v2 leaderboard version by ID.
func (c *Client) GetGameCenterLeaderboardVersion(ctx context.Context, versionID string) (*GameCenterLeaderboardVersionResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardVersions/%s", strings.TrimSpace(versionID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterLeaderboardVersion creates a new v2 leaderboard version.
func (c *Client) CreateGameCenterLeaderboardVersion(ctx context.Context, leaderboardID string) (*GameCenterLeaderboardVersionResponse, error) {
	payload := GameCenterLeaderboardVersionCreateRequest{
		Data: GameCenterLeaderboardVersionCreateData{
			Type: ResourceTypeGameCenterLeaderboardVersions,
			Relationships: &GameCenterLeaderboardVersionRelationships{
				Leaderboard: &Relationship{
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

	data, err := c.do(ctx, http.MethodPost, "/v2/gameCenterLeaderboardVersions", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardVersionLocalizations retrieves localizations for a v2 leaderboard version.
func (c *Client) GetGameCenterLeaderboardVersionLocalizations(ctx context.Context, versionID string, opts ...GCLeaderboardLocalizationsOption) (*GameCenterLeaderboardLocalizationsResponse, error) {
	query := &gcLeaderboardLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v2/gameCenterLeaderboardVersions/%s/localizations", strings.TrimSpace(versionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-version-localizations: %w", err)
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

// GetGameCenterLeaderboardVersionLocalizationsRelationships retrieves localization linkages for a v2 leaderboard version.
func (c *Client) GetGameCenterLeaderboardVersionLocalizationsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v2/gameCenterLeaderboardVersions/%s/relationships/localizations", strings.TrimSpace(versionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-version-localizations-relationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardLocalizationV2 retrieves a v2 leaderboard localization by ID.
func (c *Client) GetGameCenterLeaderboardLocalizationV2(ctx context.Context, localizationID string) (*GameCenterLeaderboardLocalizationResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardLocalizations/%s", strings.TrimSpace(localizationID))
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

// CreateGameCenterLeaderboardLocalizationV2 creates a new v2 leaderboard localization.
func (c *Client) CreateGameCenterLeaderboardLocalizationV2(ctx context.Context, versionID string, attrs GameCenterLeaderboardLocalizationCreateAttributes) (*GameCenterLeaderboardLocalizationResponse, error) {
	payload := GameCenterLeaderboardLocalizationV2CreateRequest{
		Data: GameCenterLeaderboardLocalizationV2CreateData{
			Type:       ResourceTypeGameCenterLeaderboardLocalizations,
			Attributes: attrs,
			Relationships: &GameCenterLeaderboardLocalizationV2Relationships{
				Version: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboardVersions,
						ID:   strings.TrimSpace(versionID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v2/gameCenterLeaderboardLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterLeaderboardLocalizationV2 updates a v2 leaderboard localization.
func (c *Client) UpdateGameCenterLeaderboardLocalizationV2(ctx context.Context, localizationID string, attrs GameCenterLeaderboardLocalizationUpdateAttributes) (*GameCenterLeaderboardLocalizationResponse, error) {
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

	path := fmt.Sprintf("/v2/gameCenterLeaderboardLocalizations/%s", strings.TrimSpace(localizationID))
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

// DeleteGameCenterLeaderboardLocalizationV2 deletes a v2 leaderboard localization.
func (c *Client) DeleteGameCenterLeaderboardLocalizationV2(ctx context.Context, localizationID string) error {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardLocalizations/%s", strings.TrimSpace(localizationID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterLeaderboardLocalizationImageV2 retrieves the image for a v2 leaderboard localization.
func (c *Client) GetGameCenterLeaderboardLocalizationImageV2(ctx context.Context, localizationID string) (*GameCenterLeaderboardImageResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardLocalizations/%s/image", strings.TrimSpace(localizationID))
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

// GetGameCenterLeaderboardLocalizationImageRelationshipV2 retrieves the image linkage for a v2 leaderboard localization.
func (c *Client) GetGameCenterLeaderboardLocalizationImageRelationshipV2(ctx context.Context, localizationID string) (*GameCenterLeaderboardLocalizationV2ImageLinkageResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardLocalizations/%s/relationships/image", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardLocalizationV2ImageLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardImageV2 retrieves a v2 leaderboard image by ID.
func (c *Client) GetGameCenterLeaderboardImageV2(ctx context.Context, imageID string) (*GameCenterLeaderboardImageResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardImages/%s", strings.TrimSpace(imageID))
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

// CreateGameCenterLeaderboardImageV2 reserves a new v2 leaderboard image upload.
func (c *Client) CreateGameCenterLeaderboardImageV2(ctx context.Context, localizationID, fileName string, fileSize int64) (*GameCenterLeaderboardImageResponse, error) {
	payload := GameCenterLeaderboardImageV2CreateRequest{
		Data: GameCenterLeaderboardImageV2CreateData{
			Type: ResourceTypeGameCenterLeaderboardImages,
			Attributes: GameCenterLeaderboardImageCreateAttributes{
				FileSize: fileSize,
				FileName: fileName,
			},
			Relationships: &GameCenterLeaderboardImageV2Relationships{
				Localization: &Relationship{
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

	data, err := c.do(ctx, http.MethodPost, "/v2/gameCenterLeaderboardImages", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterLeaderboardImageV2 updates a v2 leaderboard image (used to commit upload).
func (c *Client) UpdateGameCenterLeaderboardImageV2(ctx context.Context, imageID string, uploaded bool) (*GameCenterLeaderboardImageResponse, error) {
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

	path := fmt.Sprintf("/v2/gameCenterLeaderboardImages/%s", strings.TrimSpace(imageID))
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

// DeleteGameCenterLeaderboardImageV2 deletes a v2 leaderboard image.
func (c *Client) DeleteGameCenterLeaderboardImageV2(ctx context.Context, imageID string) error {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardImages/%s", strings.TrimSpace(imageID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// UploadGameCenterLeaderboardImageV2 performs the full upload flow for a v2 leaderboard image.
func (c *Client) UploadGameCenterLeaderboardImageV2(ctx context.Context, localizationID, filePath string) (*GameCenterLeaderboardImageUploadResult, error) {
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	reservation, err := c.CreateGameCenterLeaderboardImageV2(ctx, localizationID, fileName, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve upload: %w", err)
	}

	imageID := reservation.Data.ID
	operations := reservation.Data.Attributes.UploadOperations
	if len(operations) == 0 {
		return nil, fmt.Errorf("no upload operations returned from reservation")
	}

	if err := UploadAsset(ctx, filePath, operations); err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	committed, err := c.UpdateGameCenterLeaderboardImageV2(ctx, imageID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit upload: %w", err)
	}

	result := &GameCenterLeaderboardImageUploadResult{
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

// GetGameCenterLeaderboardSetsV2 retrieves v2 leaderboard sets for a Game Center detail or group.
func (c *Client) GetGameCenterLeaderboardSetsV2(ctx context.Context, gcDetailID, groupID string, opts ...GCLeaderboardSetsOption) (*GameCenterLeaderboardSetsResponse, error) {
	query := &gcLeaderboardSetsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterLeaderboardSetsV2", strings.TrimSpace(gcDetailID))
	if strings.TrimSpace(groupID) != "" {
		path = fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterLeaderboardSetsV2", strings.TrimSpace(groupID))
	}
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-sets-v2: %w", err)
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

// GetGameCenterLeaderboardSetV2 retrieves a v2 leaderboard set by ID.
func (c *Client) GetGameCenterLeaderboardSetV2(ctx context.Context, setID string) (*GameCenterLeaderboardSetResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s", strings.TrimSpace(setID))
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

// CreateGameCenterLeaderboardSetV2 creates a new v2 leaderboard set.
func (c *Client) CreateGameCenterLeaderboardSetV2(ctx context.Context, gcDetailID, groupID string, attrs GameCenterLeaderboardSetCreateAttributes) (*GameCenterLeaderboardSetResponse, error) {
	relationships := &GameCenterLeaderboardSetV2Relationships{}
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

	payload := GameCenterLeaderboardSetV2CreateRequest{
		Data: GameCenterLeaderboardSetV2CreateData{
			Type:          ResourceTypeGameCenterLeaderboardSets,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v2/gameCenterLeaderboardSets", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterLeaderboardSetV2 updates an existing v2 leaderboard set.
func (c *Client) UpdateGameCenterLeaderboardSetV2(ctx context.Context, setID string, attrs GameCenterLeaderboardSetUpdateAttributes) (*GameCenterLeaderboardSetResponse, error) {
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

	path := fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s", strings.TrimSpace(setID))
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

// DeleteGameCenterLeaderboardSetV2 deletes a v2 leaderboard set.
func (c *Client) DeleteGameCenterLeaderboardSetV2(ctx context.Context, setID string) error {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s", strings.TrimSpace(setID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterLeaderboardSetMembersV2 retrieves the leaderboards in a v2 leaderboard set.
func (c *Client) GetGameCenterLeaderboardSetMembersV2(ctx context.Context, setID string, opts ...GCLeaderboardSetMembersOption) (*GameCenterLeaderboardsResponse, error) {
	query := &gcLeaderboardSetMembersQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s/gameCenterLeaderboards", strings.TrimSpace(setID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-set-members-v2: %w", err)
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

// UpdateGameCenterLeaderboardSetMembersV2 replaces all leaderboard members in a v2 leaderboard set.
func (c *Client) UpdateGameCenterLeaderboardSetMembersV2(ctx context.Context, setID string, leaderboardIDs []string) error {
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

	path := fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s/relationships/gameCenterLeaderboards", strings.TrimSpace(setID))
	_, err = c.do(ctx, http.MethodPatch, path, body)
	return err
}

// GetGameCenterLeaderboardSetVersions retrieves versions for a v2 leaderboard set.
func (c *Client) GetGameCenterLeaderboardSetVersions(ctx context.Context, setID string, opts ...GCLeaderboardSetVersionsOption) (*GameCenterLeaderboardSetVersionsResponse, error) {
	query := &gcLeaderboardSetVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s/versions", strings.TrimSpace(setID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-set-versions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCLeaderboardSetVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardSetVersion retrieves a v2 leaderboard set version by ID.
func (c *Client) GetGameCenterLeaderboardSetVersion(ctx context.Context, versionID string) (*GameCenterLeaderboardSetVersionResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardSetVersions/%s", strings.TrimSpace(versionID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterLeaderboardSetVersion creates a new v2 leaderboard set version.
func (c *Client) CreateGameCenterLeaderboardSetVersion(ctx context.Context, setID string) (*GameCenterLeaderboardSetVersionResponse, error) {
	payload := GameCenterLeaderboardSetVersionCreateRequest{
		Data: GameCenterLeaderboardSetVersionCreateData{
			Type: ResourceTypeGameCenterLeaderboardSetVersions,
			Relationships: &GameCenterLeaderboardSetVersionRelationships{
				LeaderboardSet: &Relationship{
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

	data, err := c.do(ctx, http.MethodPost, "/v2/gameCenterLeaderboardSetVersions", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardSetVersionLocalizations retrieves localizations for a v2 leaderboard set version.
func (c *Client) GetGameCenterLeaderboardSetVersionLocalizations(ctx context.Context, versionID string, opts ...GCLeaderboardSetLocalizationsOption) (*GameCenterLeaderboardSetLocalizationsResponse, error) {
	query := &gcLeaderboardSetLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v2/gameCenterLeaderboardSetVersions/%s/localizations", strings.TrimSpace(versionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-set-version-localizations: %w", err)
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

// GetGameCenterLeaderboardSetVersionLocalizationsRelationships retrieves localization linkages for a v2 leaderboard set version.
func (c *Client) GetGameCenterLeaderboardSetVersionLocalizationsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v2/gameCenterLeaderboardSetVersions/%s/relationships/localizations", strings.TrimSpace(versionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-leaderboard-set-version-localizations-relationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardSetLocalizationV2 retrieves a v2 leaderboard set localization by ID.
func (c *Client) GetGameCenterLeaderboardSetLocalizationV2(ctx context.Context, localizationID string) (*GameCenterLeaderboardSetLocalizationResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardSetLocalizations/%s", strings.TrimSpace(localizationID))
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

// CreateGameCenterLeaderboardSetLocalizationV2 creates a new v2 leaderboard set localization.
func (c *Client) CreateGameCenterLeaderboardSetLocalizationV2(ctx context.Context, versionID string, attrs GameCenterLeaderboardSetLocalizationCreateAttributes) (*GameCenterLeaderboardSetLocalizationResponse, error) {
	payload := GameCenterLeaderboardSetLocalizationV2CreateRequest{
		Data: GameCenterLeaderboardSetLocalizationV2CreateData{
			Type:       ResourceTypeGameCenterLeaderboardSetLocalizations,
			Attributes: attrs,
			Relationships: &GameCenterLeaderboardSetLocalizationV2Relationships{
				Version: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterLeaderboardSetVersions,
						ID:   strings.TrimSpace(versionID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v2/gameCenterLeaderboardSetLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterLeaderboardSetLocalizationV2 updates a v2 leaderboard set localization.
func (c *Client) UpdateGameCenterLeaderboardSetLocalizationV2(ctx context.Context, localizationID string, attrs GameCenterLeaderboardSetLocalizationUpdateAttributes) (*GameCenterLeaderboardSetLocalizationResponse, error) {
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

	path := fmt.Sprintf("/v2/gameCenterLeaderboardSetLocalizations/%s", strings.TrimSpace(localizationID))
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

// DeleteGameCenterLeaderboardSetLocalizationV2 deletes a v2 leaderboard set localization.
func (c *Client) DeleteGameCenterLeaderboardSetLocalizationV2(ctx context.Context, localizationID string) error {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardSetLocalizations/%s", strings.TrimSpace(localizationID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterLeaderboardSetLocalizationImageV2 retrieves the image for a v2 leaderboard set localization.
func (c *Client) GetGameCenterLeaderboardSetLocalizationImageV2(ctx context.Context, localizationID string) (*GameCenterLeaderboardSetImageResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardSetLocalizations/%s/image", strings.TrimSpace(localizationID))
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

// GetGameCenterLeaderboardSetLocalizationImageRelationshipV2 retrieves the image linkage for a v2 leaderboard set localization.
func (c *Client) GetGameCenterLeaderboardSetLocalizationImageRelationshipV2(ctx context.Context, localizationID string) (*GameCenterLeaderboardSetLocalizationV2ImageLinkageResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardSetLocalizations/%s/relationships/image", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetLocalizationV2ImageLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterLeaderboardSetImageV2 retrieves a v2 leaderboard set image by ID.
func (c *Client) GetGameCenterLeaderboardSetImageV2(ctx context.Context, imageID string) (*GameCenterLeaderboardSetImageResponse, error) {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardSetImages/%s", strings.TrimSpace(imageID))
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

// CreateGameCenterLeaderboardSetImageV2 reserves a new v2 leaderboard set image upload.
func (c *Client) CreateGameCenterLeaderboardSetImageV2(ctx context.Context, localizationID, fileName string, fileSize int64) (*GameCenterLeaderboardSetImageResponse, error) {
	payload := GameCenterLeaderboardSetImageV2CreateRequest{
		Data: GameCenterLeaderboardSetImageV2CreateData{
			Type: ResourceTypeGameCenterLeaderboardSetImages,
			Attributes: GameCenterLeaderboardSetImageCreateAttributes{
				FileSize: fileSize,
				FileName: fileName,
			},
			Relationships: &GameCenterLeaderboardSetImageV2Relationships{
				Localization: &Relationship{
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

	data, err := c.do(ctx, http.MethodPost, "/v2/gameCenterLeaderboardSetImages", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterLeaderboardSetImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterLeaderboardSetImageV2 updates a v2 leaderboard set image (used to commit upload).
func (c *Client) UpdateGameCenterLeaderboardSetImageV2(ctx context.Context, imageID string, uploaded bool) (*GameCenterLeaderboardSetImageResponse, error) {
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

	path := fmt.Sprintf("/v2/gameCenterLeaderboardSetImages/%s", strings.TrimSpace(imageID))
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

// DeleteGameCenterLeaderboardSetImageV2 deletes a v2 leaderboard set image.
func (c *Client) DeleteGameCenterLeaderboardSetImageV2(ctx context.Context, imageID string) error {
	path := fmt.Sprintf("/v2/gameCenterLeaderboardSetImages/%s", strings.TrimSpace(imageID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// UploadGameCenterLeaderboardSetImageV2 performs the full upload flow for a v2 leaderboard set image.
func (c *Client) UploadGameCenterLeaderboardSetImageV2(ctx context.Context, localizationID, filePath string) (*GameCenterLeaderboardSetImageUploadResult, error) {
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	reservation, err := c.CreateGameCenterLeaderboardSetImageV2(ctx, localizationID, fileName, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve upload: %w", err)
	}

	imageID := reservation.Data.ID
	operations := reservation.Data.Attributes.UploadOperations
	if len(operations) == 0 {
		return nil, fmt.Errorf("no upload operations returned from reservation")
	}

	if err := UploadAsset(ctx, filePath, operations); err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	committed, err := c.UpdateGameCenterLeaderboardSetImageV2(ctx, imageID, true)
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

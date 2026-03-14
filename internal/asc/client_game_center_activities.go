package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GetGameCenterActivities retrieves the list of Game Center activities for a Game Center detail.
func (c *Client) GetGameCenterActivities(ctx context.Context, gcDetailID string, opts ...GCActivitiesOption) (*GameCenterActivitiesResponse, error) {
	query := &gcActivitiesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterActivities", strings.TrimSpace(gcDetailID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-activities: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCActivitiesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivitiesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterActivity retrieves a Game Center activity by ID.
func (c *Client) GetGameCenterActivity(ctx context.Context, activityID string) (*GameCenterActivityResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterActivities/%s", strings.TrimSpace(activityID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterActivity creates a new Game Center activity.
func (c *Client) CreateGameCenterActivity(ctx context.Context, gcDetailID string, attrs GameCenterActivityCreateAttributes, groupID string, initialVersion *GameCenterActivityVersionCreateAttributes) (*GameCenterActivityResponse, error) {
	const initialVersionID = "initial-version"

	relationships := &GameCenterActivityRelationships{}
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
	if initialVersion != nil {
		relationships.Versions = &RelationshipList{
			Data: []ResourceData{{
				Type: ResourceTypeGameCenterActivityVersions,
				ID:   initialVersionID,
			}},
		}
		hasRelationship = true
	}
	if !hasRelationship {
		relationships = nil
	}

	payload := GameCenterActivityCreateRequest{
		Data: GameCenterActivityCreateData{
			Type:          ResourceTypeGameCenterActivities,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}
	if initialVersion != nil {
		inline := GameCenterActivityVersionInlineCreate{
			Type: ResourceTypeGameCenterActivityVersions,
			ID:   initialVersionID,
		}
		if initialVersion.FallbackURL != nil {
			inline.Attributes = initialVersion
		}
		payload.Included = []GameCenterActivityVersionInlineCreate{inline}
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterActivities", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterActivity updates an existing Game Center activity.
func (c *Client) UpdateGameCenterActivity(ctx context.Context, activityID string, attrs GameCenterActivityUpdateAttributes) (*GameCenterActivityResponse, error) {
	payload := GameCenterActivityUpdateRequest{
		Data: GameCenterActivityUpdateData{
			Type:       ResourceTypeGameCenterActivities,
			ID:         strings.TrimSpace(activityID),
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterActivities/%s", strings.TrimSpace(activityID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterActivity deletes a Game Center activity.
func (c *Client) DeleteGameCenterActivity(ctx context.Context, activityID string) error {
	path := fmt.Sprintf("/v1/gameCenterActivities/%s", strings.TrimSpace(activityID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// AddGameCenterActivityAchievements adds achievements to an activity.
func (c *Client) AddGameCenterActivityAchievements(ctx context.Context, activityID string, achievementIDs []string) error {
	activityID = strings.TrimSpace(activityID)
	achievementIDs = normalizeList(achievementIDs)
	if activityID == "" {
		return fmt.Errorf("activityID is required")
	}
	if len(achievementIDs) == 0 {
		return fmt.Errorf("achievementIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterAchievements, achievementIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterActivities/%s/relationships/achievements", activityID)
	_, err = c.do(ctx, http.MethodPost, path, body)
	return err
}

// RemoveGameCenterActivityAchievements removes achievements from an activity.
func (c *Client) RemoveGameCenterActivityAchievements(ctx context.Context, activityID string, achievementIDs []string) error {
	activityID = strings.TrimSpace(activityID)
	achievementIDs = normalizeList(achievementIDs)
	if activityID == "" {
		return fmt.Errorf("activityID is required")
	}
	if len(achievementIDs) == 0 {
		return fmt.Errorf("achievementIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterAchievements, achievementIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterActivities/%s/relationships/achievements", activityID)
	_, err = c.do(ctx, http.MethodDelete, path, body)
	return err
}

// AddGameCenterActivityLeaderboards adds leaderboards to an activity.
func (c *Client) AddGameCenterActivityLeaderboards(ctx context.Context, activityID string, leaderboardIDs []string) error {
	activityID = strings.TrimSpace(activityID)
	leaderboardIDs = normalizeList(leaderboardIDs)
	if activityID == "" {
		return fmt.Errorf("activityID is required")
	}
	if len(leaderboardIDs) == 0 {
		return fmt.Errorf("leaderboardIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterActivities/%s/relationships/leaderboards", activityID)
	_, err = c.do(ctx, http.MethodPost, path, body)
	return err
}

// RemoveGameCenterActivityLeaderboards removes leaderboards from an activity.
func (c *Client) RemoveGameCenterActivityLeaderboards(ctx context.Context, activityID string, leaderboardIDs []string) error {
	activityID = strings.TrimSpace(activityID)
	leaderboardIDs = normalizeList(leaderboardIDs)
	if activityID == "" {
		return fmt.Errorf("activityID is required")
	}
	if len(leaderboardIDs) == 0 {
		return fmt.Errorf("leaderboardIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterActivities/%s/relationships/leaderboards", activityID)
	_, err = c.do(ctx, http.MethodDelete, path, body)
	return err
}

// GetGameCenterActivityVersions retrieves the list of activity versions for an activity.
func (c *Client) GetGameCenterActivityVersions(ctx context.Context, activityID string, opts ...GCActivityVersionsOption) (*GameCenterActivityVersionsResponse, error) {
	query := &gcActivityVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterActivities/%s/versions", strings.TrimSpace(activityID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-activity-versions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCActivityVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterActivityVersion retrieves an activity version by ID.
func (c *Client) GetGameCenterActivityVersion(ctx context.Context, versionID string) (*GameCenterActivityVersionResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterActivityVersions/%s", strings.TrimSpace(versionID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterActivityVersion creates a new activity version.
func (c *Client) CreateGameCenterActivityVersion(ctx context.Context, activityID string, fallbackURL string) (*GameCenterActivityVersionResponse, error) {
	var attrs *GameCenterActivityVersionCreateAttributes
	if strings.TrimSpace(fallbackURL) != "" {
		value := strings.TrimSpace(fallbackURL)
		attrs = &GameCenterActivityVersionCreateAttributes{
			FallbackURL: &value,
		}
	}

	payload := GameCenterActivityVersionCreateRequest{
		Data: GameCenterActivityVersionCreateData{
			Type:       ResourceTypeGameCenterActivityVersions,
			Attributes: attrs,
			Relationships: &GameCenterActivityVersionRelationships{
				Activity: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterActivities,
						ID:   strings.TrimSpace(activityID),
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterActivityVersions", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterActivityVersion updates an activity version.
func (c *Client) UpdateGameCenterActivityVersion(ctx context.Context, versionID string, fallbackURL *string) (*GameCenterActivityVersionResponse, error) {
	var attrs *GameCenterActivityVersionUpdateAttributes
	if fallbackURL != nil {
		attrs = &GameCenterActivityVersionUpdateAttributes{
			FallbackURL: fallbackURL,
		}
	}

	payload := GameCenterActivityVersionUpdateRequest{
		Data: GameCenterActivityVersionUpdateData{
			Type:       ResourceTypeGameCenterActivityVersions,
			ID:         strings.TrimSpace(versionID),
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterActivityVersions/%s", strings.TrimSpace(versionID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterActivityLocalizations retrieves the list of localizations for an activity version.
func (c *Client) GetGameCenterActivityLocalizations(ctx context.Context, versionID string, opts ...GCActivityLocalizationsOption) (*GameCenterActivityLocalizationsResponse, error) {
	query := &gcActivityLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterActivityVersions/%s/localizations", strings.TrimSpace(versionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-activity-localizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCActivityLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterActivityLocalization retrieves an activity localization by ID.
func (c *Client) GetGameCenterActivityLocalization(ctx context.Context, localizationID string) (*GameCenterActivityLocalizationResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterActivityLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterActivityLocalizationImage retrieves the image for an activity localization.
func (c *Client) GetGameCenterActivityLocalizationImage(ctx context.Context, localizationID string) (*GameCenterActivityImageResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterActivityLocalizations/%s/image", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterActivityVersionDefaultImage retrieves the default image for an activity version.
func (c *Client) GetGameCenterActivityVersionDefaultImage(ctx context.Context, versionID string) (*GameCenterActivityImageResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterActivityVersions/%s/defaultImage", strings.TrimSpace(versionID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterActivityLocalization creates a new activity localization.
func (c *Client) CreateGameCenterActivityLocalization(ctx context.Context, versionID string, attrs GameCenterActivityLocalizationCreateAttributes) (*GameCenterActivityLocalizationResponse, error) {
	payload := GameCenterActivityLocalizationCreateRequest{
		Data: GameCenterActivityLocalizationCreateData{
			Type:       ResourceTypeGameCenterActivityLocalizations,
			Attributes: attrs,
			Relationships: &GameCenterActivityLocalizationRelationships{
				Version: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterActivityVersions,
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

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterActivityLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterActivityLocalization updates an activity localization.
func (c *Client) UpdateGameCenterActivityLocalization(ctx context.Context, localizationID string, attrs GameCenterActivityLocalizationUpdateAttributes) (*GameCenterActivityLocalizationResponse, error) {
	payload := GameCenterActivityLocalizationUpdateRequest{
		Data: GameCenterActivityLocalizationUpdateData{
			Type:       ResourceTypeGameCenterActivityLocalizations,
			ID:         strings.TrimSpace(localizationID),
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterActivityLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterActivityLocalization deletes an activity localization.
func (c *Client) DeleteGameCenterActivityLocalization(ctx context.Context, localizationID string) error {
	path := fmt.Sprintf("/v1/gameCenterActivityLocalizations/%s", strings.TrimSpace(localizationID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterActivityImage retrieves an activity image by ID.
func (c *Client) GetGameCenterActivityImage(ctx context.Context, imageID string) (*GameCenterActivityImageResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterActivityImages/%s", strings.TrimSpace(imageID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterActivityImage reserves an activity image upload.
func (c *Client) CreateGameCenterActivityImage(ctx context.Context, localizationID, fileName string, fileSize int64) (*GameCenterActivityImageResponse, error) {
	payload := GameCenterActivityImageCreateRequest{
		Data: GameCenterActivityImageCreateData{
			Type: ResourceTypeGameCenterActivityImages,
			Attributes: GameCenterActivityImageCreateAttributes{
				FileName: fileName,
				FileSize: fileSize,
			},
			Relationships: &GameCenterActivityImageRelationships{
				Localization: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterActivityLocalizations,
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

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterActivityImages", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateGameCenterActivityImage commits an activity image upload.
func (c *Client) UpdateGameCenterActivityImage(ctx context.Context, imageID string, uploaded bool) (*GameCenterActivityImageResponse, error) {
	payload := GameCenterActivityImageUpdateRequest{
		Data: GameCenterActivityImageUpdateData{
			Type: ResourceTypeGameCenterActivityImages,
			ID:   strings.TrimSpace(imageID),
			Attributes: &GameCenterActivityImageUpdateAttributes{
				Uploaded: &uploaded,
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterActivityImages/%s", strings.TrimSpace(imageID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterActivityImage deletes an activity image.
func (c *Client) DeleteGameCenterActivityImage(ctx context.Context, imageID string) error {
	path := fmt.Sprintf("/v1/gameCenterActivityImages/%s", strings.TrimSpace(imageID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// UploadGameCenterActivityImage performs the full upload flow for an activity image.
func (c *Client) UploadGameCenterActivityImage(ctx context.Context, localizationID, filePath string) (*GameCenterActivityImageUploadResult, error) {
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	reservation, err := c.CreateGameCenterActivityImage(ctx, localizationID, fileName, fileSize)
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

	committed, err := c.UpdateGameCenterActivityImage(ctx, imageID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit upload: %w", err)
	}

	result := &GameCenterActivityImageUploadResult{
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

// GetGameCenterActivityVersionRelease retrieves an activity version release by ID.
func (c *Client) GetGameCenterActivityVersionRelease(ctx context.Context, releaseID string) (*GameCenterActivityVersionReleaseResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterActivityVersionReleases/%s", strings.TrimSpace(releaseID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityVersionReleaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterActivityVersionReleases retrieves activity releases for a Game Center detail.
func (c *Client) GetGameCenterActivityVersionReleases(ctx context.Context, gcDetailID string, opts ...GCActivityVersionReleasesOption) (*GameCenterActivityVersionReleasesResponse, error) {
	query := &gcActivityVersionReleasesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterDetails/%s/activityReleases", strings.TrimSpace(gcDetailID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-activity-releases: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCActivityVersionReleasesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityVersionReleasesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterActivityVersionRelease creates a new activity version release.
func (c *Client) CreateGameCenterActivityVersionRelease(ctx context.Context, versionID string) (*GameCenterActivityVersionReleaseResponse, error) {
	payload := GameCenterActivityVersionReleaseCreateRequest{
		Data: GameCenterActivityVersionReleaseCreateData{
			Type: ResourceTypeGameCenterActivityVersionReleases,
			Relationships: &GameCenterActivityVersionReleaseRelationships{
				Version: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterActivityVersions,
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

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterActivityVersionReleases", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityVersionReleaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterActivityVersionRelease deletes an activity version release.
func (c *Client) DeleteGameCenterActivityVersionRelease(ctx context.Context, releaseID string) error {
	path := fmt.Sprintf("/v1/gameCenterActivityVersionReleases/%s", strings.TrimSpace(releaseID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

func buildRelationshipData(resourceType ResourceType, ids []string) []RelationshipData {
	ids = normalizeList(ids)
	if len(ids) == 0 {
		return nil
	}

	data := make([]RelationshipData, 0, len(ids))
	for _, id := range ids {
		data = append(data, RelationshipData{
			Type: resourceType,
			ID:   id,
		})
	}
	return data
}

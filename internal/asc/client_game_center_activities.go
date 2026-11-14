package asc

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// CreateGameCenterActivity creates a new Game Center activity.
func (c *Client) CreateGameCenterActivity(ctx context.Context, gcDetailID string, attrs GameCenterActivityCreateAttributes, groupID string, initialVersion *GameCenterActivityVersionCreateAttributes) (*GameCenterActivityResponse, error) {
	const initialVersionID = "${activityVer1}"

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

	return sendJSON[GameCenterActivityResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterActivities", Payload: payload})
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

	path, err := resourcePath("/v1/gameCenterActivities/%s/relationships/achievements", activityID)
	if err != nil {
		return err
	}

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

	path, err := resourcePath("/v1/gameCenterActivities/%s/relationships/achievements", activityID)
	if err != nil {
		return err
	}

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

	path, err := resourcePath("/v1/gameCenterActivities/%s/relationships/leaderboards", activityID)
	if err != nil {
		return err
	}

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

	path, err := resourcePath("/v1/gameCenterActivities/%s/relationships/leaderboards", activityID)
	if err != nil {
		return err
	}

	_, err = c.do(ctx, http.MethodDelete, path, body)
	return err
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

	return sendJSON[GameCenterActivityVersionResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterActivityVersions", Payload: payload})
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

	return sendJSON[GameCenterActivityVersionResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterActivityVersions/%s", strings.TrimSpace(versionID)), Payload: payload})
}

// CreateGameCenterActivityImage reserves an activity image upload.
// Pass exactly one of localizationID or versionID; a version target sets the version's default image.
func (c *Client) CreateGameCenterActivityImage(ctx context.Context, localizationID, versionID, fileName string, fileSize int64) (*GameCenterActivityImageResponse, error) {
	relationships := &GameCenterActivityImageRelationships{}
	if id := strings.TrimSpace(localizationID); id != "" {
		relationships.Localization = &Relationship{
			Data: ResourceData{Type: ResourceTypeGameCenterActivityLocalizations, ID: id},
		}
	}
	if id := strings.TrimSpace(versionID); id != "" {
		relationships.Version = &Relationship{
			Data: ResourceData{Type: ResourceTypeGameCenterActivityVersions, ID: id},
		}
	}

	payload := GameCenterActivityImageCreateRequest{
		Data: GameCenterActivityImageCreateData{
			Type: ResourceTypeGameCenterActivityImages,
			Attributes: GameCenterActivityImageCreateAttributes{
				FileName: fileName,
				FileSize: fileSize,
			},
			Relationships: relationships,
		},
	}

	return sendJSON[GameCenterActivityImageResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterActivityImages", Payload: payload})
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

	return sendJSON[GameCenterActivityImageResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterActivityImages/%s", strings.TrimSpace(imageID)), Payload: payload})
}

// UploadGameCenterActivityImage performs the full upload flow for an activity image.
func (c *Client) UploadGameCenterActivityImage(ctx context.Context, localizationID, versionID, filePath string) (*GameCenterActivityImageUploadResult, error) {
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	reservation, err := c.CreateGameCenterActivityImage(ctx, localizationID, versionID, fileName, fileSize)
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
		VersionID:      versionID,
		FileName:       committed.Data.Attributes.FileName,
		FileSize:       committed.Data.Attributes.FileSize,
		Uploaded:       true,
	}
	if committed.Data.Attributes.AssetDeliveryState != nil {
		result.AssetDeliveryState = committed.Data.Attributes.AssetDeliveryState.State
	}

	return result, nil
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

	return sendJSON[GameCenterActivityVersionReleaseResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterActivityVersionReleases", Payload: payload})
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

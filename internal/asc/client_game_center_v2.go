package asc

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GetGameCenterAchievementsV2 retrieves v2 achievements for a Game Center detail or group.
func (c *Client) GetGameCenterAchievementsV2(ctx context.Context, gcDetailID, groupID string, opts ...GCAchievementsOption) (*GameCenterAchievementsResponse, error) {
	query := applyOptions(opts)
	path := fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterAchievementsV2", strings.TrimSpace(gcDetailID))
	if strings.TrimSpace(groupID) != "" {
		path = fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterAchievementsV2", strings.TrimSpace(groupID))
	}
	return getJSON[GameCenterAchievementsResponse](ctx, c, readRequest{
		Path:        path,
		Query:       buildGCAchievementsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-achievements-v2",
	})
}

// CreateGameCenterAchievementVersion creates a new v2 achievement version.
func (c *Client) CreateGameCenterAchievementVersion(ctx context.Context, achievementID string) (*GameCenterAchievementVersionResponse, error) {
	payload := GameCenterAchievementVersionV2CreateRequest{
		Data: GameCenterAchievementVersionV2CreateData{
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

	return sendJSON[GameCenterAchievementVersionResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v2/gameCenterAchievementVersions",
		Payload: payload,
	})
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

	return sendJSON[GameCenterAchievementImageResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v2/gameCenterAchievementImages",
		Payload: payload,
	})
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
	query := applyOptions(opts)
	path := fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterLeaderboardsV2", strings.TrimSpace(gcDetailID))
	if strings.TrimSpace(groupID) != "" {
		path = fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterLeaderboardsV2", strings.TrimSpace(groupID))
	}
	return getJSON[GameCenterLeaderboardsResponse](ctx, c, readRequest{
		Path:        path,
		Query:       buildGCLeaderboardsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboards-v2",
	})
}

// CreateGameCenterLeaderboardVersion creates a new v2 leaderboard version.
func (c *Client) CreateGameCenterLeaderboardVersion(ctx context.Context, leaderboardID string) (*GameCenterLeaderboardVersionResponse, error) {
	payload := GameCenterLeaderboardVersionV2CreateRequest{
		Data: GameCenterLeaderboardVersionV2CreateData{
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

	return sendJSON[GameCenterLeaderboardVersionResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v2/gameCenterLeaderboardVersions",
		Payload: payload,
	})
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

	return sendJSON[GameCenterLeaderboardImageResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v2/gameCenterLeaderboardImages",
		Payload: payload,
	})
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
	query := applyOptions(opts)
	path := fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterLeaderboardSetsV2", strings.TrimSpace(gcDetailID))
	if strings.TrimSpace(groupID) != "" {
		path = fmt.Sprintf("/v1/gameCenterGroups/%s/gameCenterLeaderboardSetsV2", strings.TrimSpace(groupID))
	}
	return getJSON[GameCenterLeaderboardSetsResponse](ctx, c, readRequest{
		Path:        path,
		Query:       buildGCLeaderboardSetsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboard-sets-v2",
	})
}

// CreateGameCenterLeaderboardSetV2 creates a new v2 leaderboard set.
func (c *Client) CreateGameCenterLeaderboardSetV2(ctx context.Context, gcDetailID, groupID string, attrs GameCenterLeaderboardSetCreateAttributes, leaderboardIDs []string) (*GameCenterLeaderboardSetResponse, error) {
	const initialVersionID = "${lbsetVer1}"

	relationships := &GameCenterLeaderboardSetV2Relationships{
		GameCenterLeaderboards: buildRelationshipList(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
		Versions: &RelationshipList{Data: []ResourceData{{
			Type: ResourceTypeGameCenterLeaderboardSetVersions,
			ID:   initialVersionID,
		}}},
	}

	if strings.TrimSpace(gcDetailID) != "" {
		relationships.GameCenterDetail = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterDetails,
				ID:   strings.TrimSpace(gcDetailID),
			},
		}
	}
	if strings.TrimSpace(groupID) != "" {
		relationships.GameCenterGroup = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterGroups,
				ID:   strings.TrimSpace(groupID),
			},
		}
	}

	payload := GameCenterLeaderboardSetV2CreateRequest{
		Data: GameCenterLeaderboardSetV2CreateData{
			Type:          ResourceTypeGameCenterLeaderboardSets,
			Attributes:    attrs,
			Relationships: relationships,
		},
		Included: []GameCenterLeaderboardSetVersionInlineCreate{{
			Type: ResourceTypeGameCenterLeaderboardSetVersions,
			ID:   initialVersionID,
		}},
	}

	return sendJSON[GameCenterLeaderboardSetResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v2/gameCenterLeaderboardSets",
		Payload: payload,
	})
}

// SetGameCenterLeaderboardSetMembersV2 sets leaderboard members for a v2 leaderboard set.
func (c *Client) SetGameCenterLeaderboardSetMembersV2(ctx context.Context, setID string, leaderboardIDs []string) error {
	return setGameCenterLeaderboardSetMembers(ctx, setID, leaderboardIDs, leaderboardSetMembersOperations{
		list:    c.GetGameCenterLeaderboardSetMembersV2,
		add:     c.AddGameCenterLeaderboardSetMembersV2,
		remove:  c.RemoveGameCenterLeaderboardSetMembersV2,
		replace: c.UpdateGameCenterLeaderboardSetMembersV2,
	})
}

// CreateGameCenterLeaderboardSetVersion creates a new v2 leaderboard set version.
func (c *Client) CreateGameCenterLeaderboardSetVersion(ctx context.Context, setID string) (*GameCenterLeaderboardSetVersionResponse, error) {
	payload := GameCenterLeaderboardSetVersionV2CreateRequest{
		Data: GameCenterLeaderboardSetVersionV2CreateData{
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

	return sendJSON[GameCenterLeaderboardSetVersionResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v2/gameCenterLeaderboardSetVersions",
		Payload: payload,
	})
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

	return sendJSON[GameCenterLeaderboardSetImageResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v2/gameCenterLeaderboardSetImages",
		Payload: payload,
	})
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

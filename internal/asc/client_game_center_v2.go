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

// GetGameCenterAchievementVersions retrieves versions for a v2 achievement.
func (c *Client) GetGameCenterAchievementVersions(ctx context.Context, achievementID string, opts ...GCAchievementVersionsOption) (*GameCenterAchievementVersionsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterAchievementVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v2/gameCenterAchievements/%s/versions", strings.TrimSpace(achievementID)),
		Query:       buildGCAchievementVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-achievement-versions",
	})
}

// GetGameCenterAchievementVersion retrieves a v2 achievement version by ID.
func (c *Client) GetGameCenterAchievementVersion(ctx context.Context, versionID string) (*GameCenterAchievementVersionResponse, error) {
	return getJSON[GameCenterAchievementVersionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterAchievementVersions/%s", strings.TrimSpace(versionID))})
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

	return sendJSON[GameCenterAchievementVersionResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v2/gameCenterAchievementVersions",
		Payload: payload,
	})
}

// GetGameCenterAchievementVersionLocalizations retrieves localizations for a v2 achievement version.
func (c *Client) GetGameCenterAchievementVersionLocalizations(ctx context.Context, versionID string, opts ...GCAchievementLocalizationsOption) (*GameCenterAchievementLocalizationsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterAchievementLocalizationsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v2/gameCenterAchievementVersions/%s/localizations", strings.TrimSpace(versionID)),
		Query:       buildGCAchievementLocalizationsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-achievement-version-localizations",
	})
}

// GetGameCenterAchievementLocalizationV2 retrieves a v2 achievement localization by ID.
func (c *Client) GetGameCenterAchievementLocalizationV2(ctx context.Context, localizationID string) (*GameCenterAchievementLocalizationResponse, error) {
	return getJSON[GameCenterAchievementLocalizationResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterAchievementLocalizations/%s", strings.TrimSpace(localizationID))})
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

	return sendJSON[GameCenterAchievementLocalizationResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v2/gameCenterAchievementLocalizations",
		Payload: payload,
	})
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

	return sendJSON[GameCenterAchievementLocalizationResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v2/gameCenterAchievementLocalizations/%s", strings.TrimSpace(localizationID)),
		Payload: payload,
	})
}

// DeleteGameCenterAchievementLocalizationV2 deletes a v2 achievement localization.
func (c *Client) DeleteGameCenterAchievementLocalizationV2(ctx context.Context, localizationID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/gameCenterAchievementLocalizations/%s", strings.TrimSpace(localizationID)), nil)
}

// GetGameCenterAchievementLocalizationImageV2 retrieves the image for a v2 achievement localization.
func (c *Client) GetGameCenterAchievementLocalizationImageV2(ctx context.Context, localizationID string) (*GameCenterAchievementImageResponse, error) {
	return getJSON[GameCenterAchievementImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterAchievementLocalizations/%s/image", strings.TrimSpace(localizationID))})
}

// GetGameCenterAchievementImageV2 retrieves a v2 achievement image by ID.
func (c *Client) GetGameCenterAchievementImageV2(ctx context.Context, imageID string) (*GameCenterAchievementImageResponse, error) {
	return getJSON[GameCenterAchievementImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterAchievementImages/%s", strings.TrimSpace(imageID))})
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

	return sendJSON[GameCenterAchievementImageResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v2/gameCenterAchievementImages/%s", strings.TrimSpace(imageID)),
		Payload: payload,
	})
}

// DeleteGameCenterAchievementImageV2 deletes a v2 achievement image.
func (c *Client) DeleteGameCenterAchievementImageV2(ctx context.Context, imageID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/gameCenterAchievementImages/%s", strings.TrimSpace(imageID)), nil)
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

// GetGameCenterLeaderboardVersions retrieves versions for a v2 leaderboard.
func (c *Client) GetGameCenterLeaderboardVersions(ctx context.Context, leaderboardID string, opts ...GCLeaderboardVersionsOption) (*GameCenterLeaderboardVersionsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v2/gameCenterLeaderboards/%s/versions", strings.TrimSpace(leaderboardID)),
		Query:       buildGCLeaderboardVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboard-versions",
	})
}

// GetGameCenterLeaderboardVersion retrieves a v2 leaderboard version by ID.
func (c *Client) GetGameCenterLeaderboardVersion(ctx context.Context, versionID string) (*GameCenterLeaderboardVersionResponse, error) {
	return getJSON[GameCenterLeaderboardVersionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterLeaderboardVersions/%s", strings.TrimSpace(versionID))})
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

	return sendJSON[GameCenterLeaderboardVersionResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v2/gameCenterLeaderboardVersions",
		Payload: payload,
	})
}

// GetGameCenterLeaderboardVersionLocalizations retrieves localizations for a v2 leaderboard version.
func (c *Client) GetGameCenterLeaderboardVersionLocalizations(ctx context.Context, versionID string, opts ...GCLeaderboardLocalizationsOption) (*GameCenterLeaderboardLocalizationsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardLocalizationsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v2/gameCenterLeaderboardVersions/%s/localizations", strings.TrimSpace(versionID)),
		Query:       buildGCLeaderboardLocalizationsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboard-version-localizations",
	})
}

// GetGameCenterLeaderboardLocalizationV2 retrieves a v2 leaderboard localization by ID.
func (c *Client) GetGameCenterLeaderboardLocalizationV2(ctx context.Context, localizationID string) (*GameCenterLeaderboardLocalizationResponse, error) {
	return getJSON[GameCenterLeaderboardLocalizationResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterLeaderboardLocalizations/%s", strings.TrimSpace(localizationID))})
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

	return sendJSON[GameCenterLeaderboardLocalizationResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v2/gameCenterLeaderboardLocalizations",
		Payload: payload,
	})
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

	return sendJSON[GameCenterLeaderboardLocalizationResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v2/gameCenterLeaderboardLocalizations/%s", strings.TrimSpace(localizationID)),
		Payload: payload,
	})
}

// DeleteGameCenterLeaderboardLocalizationV2 deletes a v2 leaderboard localization.
func (c *Client) DeleteGameCenterLeaderboardLocalizationV2(ctx context.Context, localizationID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/gameCenterLeaderboardLocalizations/%s", strings.TrimSpace(localizationID)), nil)
}

// GetGameCenterLeaderboardLocalizationImageV2 retrieves the image for a v2 leaderboard localization.
func (c *Client) GetGameCenterLeaderboardLocalizationImageV2(ctx context.Context, localizationID string) (*GameCenterLeaderboardImageResponse, error) {
	return getJSON[GameCenterLeaderboardImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterLeaderboardLocalizations/%s/image", strings.TrimSpace(localizationID))})
}

// GetGameCenterLeaderboardImageV2 retrieves a v2 leaderboard image by ID.
func (c *Client) GetGameCenterLeaderboardImageV2(ctx context.Context, imageID string) (*GameCenterLeaderboardImageResponse, error) {
	return getJSON[GameCenterLeaderboardImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterLeaderboardImages/%s", strings.TrimSpace(imageID))})
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

	return sendJSON[GameCenterLeaderboardImageResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v2/gameCenterLeaderboardImages/%s", strings.TrimSpace(imageID)),
		Payload: payload,
	})
}

// DeleteGameCenterLeaderboardImageV2 deletes a v2 leaderboard image.
func (c *Client) DeleteGameCenterLeaderboardImageV2(ctx context.Context, imageID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/gameCenterLeaderboardImages/%s", strings.TrimSpace(imageID)), nil)
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

// GetGameCenterLeaderboardSetV2 retrieves a v2 leaderboard set by ID.
func (c *Client) GetGameCenterLeaderboardSetV2(ctx context.Context, setID string) (*GameCenterLeaderboardSetResponse, error) {
	return getJSON[GameCenterLeaderboardSetResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s", strings.TrimSpace(setID))})
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

// UpdateGameCenterLeaderboardSetV2 updates an existing v2 leaderboard set.
func (c *Client) UpdateGameCenterLeaderboardSetV2(ctx context.Context, setID string, attrs GameCenterLeaderboardSetUpdateAttributes) (*GameCenterLeaderboardSetResponse, error) {
	payload := GameCenterLeaderboardSetUpdateRequest{
		Data: GameCenterLeaderboardSetUpdateData{
			Type:       ResourceTypeGameCenterLeaderboardSets,
			ID:         strings.TrimSpace(setID),
			Attributes: &attrs,
		},
	}

	return sendJSON[GameCenterLeaderboardSetResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s", strings.TrimSpace(setID)),
		Payload: payload,
	})
}

// DeleteGameCenterLeaderboardSetV2 deletes a v2 leaderboard set.
func (c *Client) DeleteGameCenterLeaderboardSetV2(ctx context.Context, setID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s", strings.TrimSpace(setID)), nil)
}

// GetGameCenterLeaderboardSetMembersV2 retrieves the leaderboards in a v2 leaderboard set.
func (c *Client) GetGameCenterLeaderboardSetMembersV2(ctx context.Context, setID string, opts ...GCLeaderboardSetMembersOption) (*GameCenterLeaderboardsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s/gameCenterLeaderboards", strings.TrimSpace(setID)),
		Query:       buildGCLeaderboardSetMembersQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboard-set-members-v2",
	})
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

	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s/relationships/gameCenterLeaderboards", strings.TrimSpace(setID)), payload)
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

// GetGameCenterLeaderboardSetVersions retrieves versions for a v2 leaderboard set.
func (c *Client) GetGameCenterLeaderboardSetVersions(ctx context.Context, setID string, opts ...GCLeaderboardSetVersionsOption) (*GameCenterLeaderboardSetVersionsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardSetVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v2/gameCenterLeaderboardSets/%s/versions", strings.TrimSpace(setID)),
		Query:       buildGCLeaderboardSetVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboard-set-versions",
	})
}

// GetGameCenterLeaderboardSetVersion retrieves a v2 leaderboard set version by ID.
func (c *Client) GetGameCenterLeaderboardSetVersion(ctx context.Context, versionID string) (*GameCenterLeaderboardSetVersionResponse, error) {
	return getJSON[GameCenterLeaderboardSetVersionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterLeaderboardSetVersions/%s", strings.TrimSpace(versionID))})
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

	return sendJSON[GameCenterLeaderboardSetVersionResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v2/gameCenterLeaderboardSetVersions",
		Payload: payload,
	})
}

// GetGameCenterLeaderboardSetVersionLocalizations retrieves localizations for a v2 leaderboard set version.
func (c *Client) GetGameCenterLeaderboardSetVersionLocalizations(ctx context.Context, versionID string, opts ...GCLeaderboardSetLocalizationsOption) (*GameCenterLeaderboardSetLocalizationsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterLeaderboardSetLocalizationsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v2/gameCenterLeaderboardSetVersions/%s/localizations", strings.TrimSpace(versionID)),
		Query:       buildGCLeaderboardSetLocalizationsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-leaderboard-set-version-localizations",
	})
}

// GetGameCenterLeaderboardSetLocalizationV2 retrieves a v2 leaderboard set localization by ID.
func (c *Client) GetGameCenterLeaderboardSetLocalizationV2(ctx context.Context, localizationID string) (*GameCenterLeaderboardSetLocalizationResponse, error) {
	return getJSON[GameCenterLeaderboardSetLocalizationResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterLeaderboardSetLocalizations/%s", strings.TrimSpace(localizationID))})
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

	return sendJSON[GameCenterLeaderboardSetLocalizationResponse](ctx, c, writeRequest{
		Method:  http.MethodPost,
		Path:    "/v2/gameCenterLeaderboardSetLocalizations",
		Payload: payload,
	})
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

	return sendJSON[GameCenterLeaderboardSetLocalizationResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v2/gameCenterLeaderboardSetLocalizations/%s", strings.TrimSpace(localizationID)),
		Payload: payload,
	})
}

// DeleteGameCenterLeaderboardSetLocalizationV2 deletes a v2 leaderboard set localization.
func (c *Client) DeleteGameCenterLeaderboardSetLocalizationV2(ctx context.Context, localizationID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/gameCenterLeaderboardSetLocalizations/%s", strings.TrimSpace(localizationID)), nil)
}

// GetGameCenterLeaderboardSetLocalizationImageV2 retrieves the image for a v2 leaderboard set localization.
func (c *Client) GetGameCenterLeaderboardSetLocalizationImageV2(ctx context.Context, localizationID string) (*GameCenterLeaderboardSetImageResponse, error) {
	return getJSON[GameCenterLeaderboardSetImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterLeaderboardSetLocalizations/%s/image", strings.TrimSpace(localizationID))})
}

// GetGameCenterLeaderboardSetImageV2 retrieves a v2 leaderboard set image by ID.
func (c *Client) GetGameCenterLeaderboardSetImageV2(ctx context.Context, imageID string) (*GameCenterLeaderboardSetImageResponse, error) {
	return getJSON[GameCenterLeaderboardSetImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/gameCenterLeaderboardSetImages/%s", strings.TrimSpace(imageID))})
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

	return sendJSON[GameCenterLeaderboardSetImageResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v2/gameCenterLeaderboardSetImages/%s", strings.TrimSpace(imageID)),
		Payload: payload,
	})
}

// DeleteGameCenterLeaderboardSetImageV2 deletes a v2 leaderboard set image.
func (c *Client) DeleteGameCenterLeaderboardSetImageV2(ctx context.Context, imageID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/gameCenterLeaderboardSetImages/%s", strings.TrimSpace(imageID)), nil)
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

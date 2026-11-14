package asc

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GetGameCenterDetailID retrieves the Game Center detail ID for an app.
func (c *Client) GetGameCenterDetailID(ctx context.Context, appID string) (string, error) {
	resp, err := getJSON[GameCenterDetailResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/apps/%s/gameCenterDetail", strings.TrimSpace(appID))})
	if err != nil {
		return "", err
	}
	return resp.Data.ID, nil
}

// gameCenterDetailRelationship returns nil for an empty ID so a create that
// inherits its parent from its members sends no gameCenterDetail.
func gameCenterDetailRelationship(gcDetailID string) *Relationship {
	gcDetailID = strings.TrimSpace(gcDetailID)
	if gcDetailID == "" {
		return nil
	}
	return &Relationship{Data: ResourceData{Type: ResourceTypeGameCenterDetails, ID: gcDetailID}}
}

// CreateGameCenterLeaderboard creates a new Game Center leaderboard.
func (c *Client) CreateGameCenterLeaderboard(ctx context.Context, gcDetailID string, attrs GameCenterLeaderboardCreateAttributes, setIDs []string) (*GameCenterLeaderboardResponse, error) {
	payload := GameCenterLeaderboardCreateRequest{
		Data: GameCenterLeaderboardCreateData{
			Type:       ResourceTypeGameCenterLeaderboards,
			Attributes: attrs,
			Relationships: &GameCenterLeaderboardRelationships{
				GameCenterDetail:          gameCenterDetailRelationship(gcDetailID),
				GameCenterLeaderboardSets: buildRelationshipList(ResourceTypeGameCenterLeaderboardSets, setIDs),
			},
		},
	}

	return sendJSON[GameCenterLeaderboardResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterLeaderboards", Payload: payload})
}

// CreateGameCenterLeaderboardSet creates a new Game Center leaderboard set.
func (c *Client) CreateGameCenterLeaderboardSet(ctx context.Context, gcDetailID string, attrs GameCenterLeaderboardSetCreateAttributes, leaderboardIDs []string) (*GameCenterLeaderboardSetResponse, error) {
	payload := GameCenterLeaderboardSetCreateRequest{
		Data: GameCenterLeaderboardSetCreateData{
			Type:       ResourceTypeGameCenterLeaderboardSets,
			Attributes: attrs,
			Relationships: &GameCenterLeaderboardSetRelationships{
				GameCenterDetail:       gameCenterDetailRelationship(gcDetailID),
				GameCenterLeaderboards: buildRelationshipList(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
			},
		},
	}

	return sendJSON[GameCenterLeaderboardSetResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterLeaderboardSets", Payload: payload})
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

	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/gameCenterLeaderboardSets/%s/relationships/gameCenterLeaderboards", strings.TrimSpace(setID)), payload)
}

// SetGameCenterLeaderboardSetMembers sets leaderboard members for a leaderboard set.
func (c *Client) SetGameCenterLeaderboardSetMembers(ctx context.Context, setID string, leaderboardIDs []string) error {
	return setGameCenterLeaderboardSetMembers(ctx, setID, leaderboardIDs, leaderboardSetMembersOperations{
		list:    c.GetGameCenterLeaderboardSetMembers,
		add:     c.AddGameCenterLeaderboardSetMembers,
		remove:  c.RemoveGameCenterLeaderboardSetMembers,
		replace: c.UpdateGameCenterLeaderboardSetMembers,
	})
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

	return sendJSON[GameCenterLeaderboardImageResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterLeaderboardImages", Payload: payload})
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

	return sendJSON[GameCenterLeaderboardImageResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterLeaderboardImages/%s", strings.TrimSpace(imageID)), Payload: payload})
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

	return sendJSON[GameCenterAchievementImageResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterAchievementImages", Payload: payload})
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

	return sendJSON[GameCenterAchievementImageResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterAchievementImages/%s", strings.TrimSpace(imageID)), Payload: payload})
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

	return sendJSON[GameCenterLeaderboardSetImageResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterLeaderboardSetImages", Payload: payload})
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

	return sendJSON[GameCenterLeaderboardSetImageResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterLeaderboardSetImages/%s", strings.TrimSpace(imageID)), Payload: payload})
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

// CreateGameCenterAchievementV2 creates a new v2 Game Center achievement.
func (c *Client) CreateGameCenterAchievementV2(ctx context.Context, gcDetailID string, groupID string, attrs GameCenterAchievementCreateAttributes) (*GameCenterAchievementResponse, error) {
	const initialVersionID = "${achVer1}"

	relationships := &GameCenterAchievementV2Relationships{
		Versions: &RelationshipList{Data: []ResourceData{{
			Type: ResourceTypeGameCenterAchievementVersions,
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

	payload := GameCenterAchievementV2CreateRequest{
		Data: GameCenterAchievementV2CreateData{
			Type:          ResourceTypeGameCenterAchievements,
			Attributes:    attrs,
			Relationships: relationships,
		},
		Included: []GameCenterAchievementVersionInlineCreate{{
			Type: ResourceTypeGameCenterAchievementVersions,
			ID:   initialVersionID,
		}},
	}

	return sendJSON[GameCenterAchievementResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v2/gameCenterAchievements", Payload: payload})
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

	return sendJSON[GameCenterAchievementResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v2/gameCenterAchievements/%s", strings.TrimSpace(achievementID)), Payload: payload})
}

// CreateGameCenterLeaderboardV2 creates a new v2 Game Center leaderboard.
func (c *Client) CreateGameCenterLeaderboardV2(ctx context.Context, gcDetailID string, groupID string, attrs GameCenterLeaderboardCreateAttributes, setIDs []string) (*GameCenterLeaderboardResponse, error) {
	const initialVersionID = "${lbVer1}"

	relationships := &GameCenterLeaderboardV2Relationships{
		GameCenterLeaderboardSets: buildRelationshipList(ResourceTypeGameCenterLeaderboardSets, setIDs),
		Versions: &RelationshipList{Data: []ResourceData{{
			Type: ResourceTypeGameCenterLeaderboardVersions,
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

	payload := GameCenterLeaderboardV2CreateRequest{
		Data: GameCenterLeaderboardV2CreateData{
			Type:          ResourceTypeGameCenterLeaderboards,
			Attributes:    attrs,
			Relationships: relationships,
		},
		Included: []GameCenterLeaderboardVersionInlineCreate{{
			Type: ResourceTypeGameCenterLeaderboardVersions,
			ID:   initialVersionID,
		}},
	}

	return sendJSON[GameCenterLeaderboardResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v2/gameCenterLeaderboards", Payload: payload})
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

	return sendJSON[GameCenterLeaderboardResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v2/gameCenterLeaderboards/%s", strings.TrimSpace(leaderboardID)), Payload: payload})
}

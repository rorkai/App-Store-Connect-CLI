package asc

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// CreateGameCenterChallenge creates a new Game Center challenge.
func (c *Client) CreateGameCenterChallenge(ctx context.Context, gcDetailID string, attrs GameCenterChallengeCreateAttributes, leaderboardID string, groupID string, createInitialVersion bool) (*GameCenterChallengeResponse, error) {
	const initialVersionID = "${challengeVer1}"

	relationships := &GameCenterChallengeRelationships{}
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
	if strings.TrimSpace(leaderboardID) != "" {
		relationships.Leaderboard = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeGameCenterLeaderboards,
				ID:   strings.TrimSpace(leaderboardID),
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
	if createInitialVersion {
		relationships.Versions = &RelationshipList{
			Data: []ResourceData{{
				Type: ResourceTypeGameCenterChallengeVersions,
				ID:   initialVersionID,
			}},
		}
		hasRelationship = true
	}
	if !hasRelationship {
		relationships = nil
	}

	payload := GameCenterChallengeCreateRequest{
		Data: GameCenterChallengeCreateData{
			Type:          ResourceTypeGameCenterChallenges,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}
	if createInitialVersion {
		payload.Included = []GameCenterChallengeVersionInlineCreate{{
			Type: ResourceTypeGameCenterChallengeVersions,
			ID:   initialVersionID,
		}}
	}

	return sendJSON[GameCenterChallengeResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterChallenges", Payload: payload})
}

// UpdateGameCenterChallenge updates an existing Game Center challenge.
func (c *Client) UpdateGameCenterChallenge(ctx context.Context, challengeID string, attrs GameCenterChallengeUpdateAttributes, leaderboardID string) (*GameCenterChallengeResponse, error) {
	var relationships *GameCenterChallengeUpdateRelationships
	if strings.TrimSpace(leaderboardID) != "" {
		relationships = &GameCenterChallengeUpdateRelationships{
			Leaderboard: &Relationship{
				Data: ResourceData{
					Type: ResourceTypeGameCenterLeaderboards,
					ID:   strings.TrimSpace(leaderboardID),
				},
			},
		}
	}

	payload := GameCenterChallengeUpdateRequest{
		Data: GameCenterChallengeUpdateData{
			Type:          ResourceTypeGameCenterChallenges,
			ID:            strings.TrimSpace(challengeID),
			Attributes:    &attrs,
			Relationships: relationships,
		},
	}

	return sendJSON[GameCenterChallengeResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterChallenges/%s", strings.TrimSpace(challengeID)), Payload: payload})
}

// CreateGameCenterChallengeVersion creates a new challenge version.
func (c *Client) CreateGameCenterChallengeVersion(ctx context.Context, challengeID string) (*GameCenterChallengeVersionResponse, error) {
	payload := GameCenterChallengeVersionCreateRequest{
		Data: GameCenterChallengeVersionCreateData{
			Type: ResourceTypeGameCenterChallengeVersions,
			Relationships: &GameCenterChallengeVersionRelationships{
				Challenge: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterChallenges,
						ID:   strings.TrimSpace(challengeID),
					},
				},
			},
		},
	}

	return sendJSON[GameCenterChallengeVersionResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterChallengeVersions", Payload: payload})
}

// CreateGameCenterChallengeLocalization creates a new challenge localization.
func (c *Client) CreateGameCenterChallengeLocalization(ctx context.Context, versionID string, attrs GameCenterChallengeLocalizationCreateAttributes) (*GameCenterChallengeLocalizationResponse, error) {
	payload := GameCenterChallengeLocalizationCreateRequest{
		Data: GameCenterChallengeLocalizationCreateData{
			Type:       ResourceTypeGameCenterChallengeLocalizations,
			Attributes: attrs,
			Relationships: &GameCenterChallengeLocalizationRelationships{
				Version: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterChallengeVersions,
						ID:   strings.TrimSpace(versionID),
					},
				},
			},
		},
	}

	return sendJSON[GameCenterChallengeLocalizationResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterChallengeLocalizations", Payload: payload})
}

// CreateGameCenterChallengeImage reserves a challenge image upload.
// Pass exactly one of localizationID or versionID; a version target sets the version's default image.
func (c *Client) CreateGameCenterChallengeImage(ctx context.Context, localizationID, versionID, fileName string, fileSize int64) (*GameCenterChallengeImageResponse, error) {
	relationships := &GameCenterChallengeImageRelationships{}
	if id := strings.TrimSpace(localizationID); id != "" {
		relationships.Localization = &Relationship{
			Data: ResourceData{Type: ResourceTypeGameCenterChallengeLocalizations, ID: id},
		}
	}
	if id := strings.TrimSpace(versionID); id != "" {
		relationships.Version = &Relationship{
			Data: ResourceData{Type: ResourceTypeGameCenterChallengeVersions, ID: id},
		}
	}

	payload := GameCenterChallengeImageCreateRequest{
		Data: GameCenterChallengeImageCreateData{
			Type: ResourceTypeGameCenterChallengeImages,
			Attributes: GameCenterChallengeImageCreateAttributes{
				FileName: fileName,
				FileSize: fileSize,
			},
			Relationships: relationships,
		},
	}

	return sendJSON[GameCenterChallengeImageResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterChallengeImages", Payload: payload})
}

// UpdateGameCenterChallengeImage commits a challenge image upload.
func (c *Client) UpdateGameCenterChallengeImage(ctx context.Context, imageID string, uploaded bool) (*GameCenterChallengeImageResponse, error) {
	payload := GameCenterChallengeImageUpdateRequest{
		Data: GameCenterChallengeImageUpdateData{
			Type: ResourceTypeGameCenterChallengeImages,
			ID:   strings.TrimSpace(imageID),
			Attributes: &GameCenterChallengeImageUpdateAttributes{
				Uploaded: &uploaded,
			},
		},
	}

	return sendJSON[GameCenterChallengeImageResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterChallengeImages/%s", strings.TrimSpace(imageID)), Payload: payload})
}

// UploadGameCenterChallengeImage performs the full upload flow for a challenge image.
func (c *Client) UploadGameCenterChallengeImage(ctx context.Context, localizationID, versionID, filePath string) (*GameCenterChallengeImageUploadResult, error) {
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	reservation, err := c.CreateGameCenterChallengeImage(ctx, localizationID, versionID, fileName, fileSize)
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

	committed, err := c.UpdateGameCenterChallengeImage(ctx, imageID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit upload: %w", err)
	}

	result := &GameCenterChallengeImageUploadResult{
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

// CreateGameCenterChallengeVersionRelease creates a new challenge version release.
func (c *Client) CreateGameCenterChallengeVersionRelease(ctx context.Context, versionID string) (*GameCenterChallengeVersionReleaseResponse, error) {
	payload := GameCenterChallengeVersionReleaseCreateRequest{
		Data: GameCenterChallengeVersionReleaseCreateData{
			Type: ResourceTypeGameCenterChallengeVersionReleases,
			Relationships: &GameCenterChallengeVersionReleaseRelationships{
				Version: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterChallengeVersions,
						ID:   strings.TrimSpace(versionID),
					},
				},
			},
		},
	}

	return sendJSON[GameCenterChallengeVersionReleaseResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterChallengeVersionReleases", Payload: payload})
}

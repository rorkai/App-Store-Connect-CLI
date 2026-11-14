package asc

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GetGameCenterChallenges retrieves the list of Game Center challenges for a Game Center detail.
func (c *Client) GetGameCenterChallenges(ctx context.Context, gcDetailID string, opts ...GCChallengesOption) (*GameCenterChallengesResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterChallengesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterChallenges", strings.TrimSpace(gcDetailID)),
		Query:       buildGCChallengesQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-challenges",
	})
}

// GetGameCenterChallenge retrieves a Game Center challenge by ID.
func (c *Client) GetGameCenterChallenge(ctx context.Context, challengeID string) (*GameCenterChallengeResponse, error) {
	return getJSON[GameCenterChallengeResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterChallenges/%s", strings.TrimSpace(challengeID))})
}

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

// DeleteGameCenterChallenge deletes a Game Center challenge.
func (c *Client) DeleteGameCenterChallenge(ctx context.Context, challengeID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterChallenges/%s", strings.TrimSpace(challengeID)), nil)
}

// GetGameCenterChallengeVersions retrieves the list of challenge versions for a challenge.
func (c *Client) GetGameCenterChallengeVersions(ctx context.Context, challengeID string, opts ...GCChallengeVersionsOption) (*GameCenterChallengeVersionsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterChallengeVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterChallenges/%s/versions", strings.TrimSpace(challengeID)),
		Query:       buildGCChallengeVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-challenge-versions",
	})
}

// GetGameCenterChallengeVersion retrieves a challenge version by ID.
func (c *Client) GetGameCenterChallengeVersion(ctx context.Context, versionID string) (*GameCenterChallengeVersionResponse, error) {
	return getJSON[GameCenterChallengeVersionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterChallengeVersions/%s", strings.TrimSpace(versionID))})
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

// GetGameCenterChallengeLocalizations retrieves the list of localizations for a challenge version.
func (c *Client) GetGameCenterChallengeLocalizations(ctx context.Context, versionID string, opts ...GCChallengeLocalizationsOption) (*GameCenterChallengeLocalizationsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterChallengeLocalizationsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterChallengeVersions/%s/localizations", strings.TrimSpace(versionID)),
		Query:       buildGCChallengeLocalizationsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-challenge-localizations",
	})
}

// GetGameCenterChallengeLocalization retrieves a challenge localization by ID.
func (c *Client) GetGameCenterChallengeLocalization(ctx context.Context, localizationID string) (*GameCenterChallengeLocalizationResponse, error) {
	return getJSON[GameCenterChallengeLocalizationResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterChallengeLocalizations/%s", strings.TrimSpace(localizationID))})
}

// GetGameCenterChallengeLocalizationImage retrieves the image for a challenge localization.
func (c *Client) GetGameCenterChallengeLocalizationImage(ctx context.Context, localizationID string) (*GameCenterChallengeImageResponse, error) {
	return getJSON[GameCenterChallengeImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterChallengeLocalizations/%s/image", strings.TrimSpace(localizationID))})
}

// GetGameCenterChallengeVersionDefaultImage retrieves the default image for a challenge version.
func (c *Client) GetGameCenterChallengeVersionDefaultImage(ctx context.Context, versionID string) (*GameCenterChallengeVersionDefaultImageResponse, error) {
	return getJSON[GameCenterChallengeVersionDefaultImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterChallengeVersions/%s/defaultImage", strings.TrimSpace(versionID))})
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

// UpdateGameCenterChallengeLocalization updates a challenge localization.
func (c *Client) UpdateGameCenterChallengeLocalization(ctx context.Context, localizationID string, attrs GameCenterChallengeLocalizationUpdateAttributes) (*GameCenterChallengeLocalizationResponse, error) {
	payload := GameCenterChallengeLocalizationUpdateRequest{
		Data: GameCenterChallengeLocalizationUpdateData{
			Type:       ResourceTypeGameCenterChallengeLocalizations,
			ID:         strings.TrimSpace(localizationID),
			Attributes: &attrs,
		},
	}

	return sendJSON[GameCenterChallengeLocalizationResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterChallengeLocalizations/%s", strings.TrimSpace(localizationID)), Payload: payload})
}

// DeleteGameCenterChallengeLocalization deletes a challenge localization.
func (c *Client) DeleteGameCenterChallengeLocalization(ctx context.Context, localizationID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterChallengeLocalizations/%s", strings.TrimSpace(localizationID)), nil)
}

// GetGameCenterChallengeImage retrieves a challenge image by ID.
func (c *Client) GetGameCenterChallengeImage(ctx context.Context, imageID string) (*GameCenterChallengeImageResponse, error) {
	return getJSON[GameCenterChallengeImageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterChallengeImages/%s", strings.TrimSpace(imageID))})
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

// DeleteGameCenterChallengeImage deletes a challenge image.
func (c *Client) DeleteGameCenterChallengeImage(ctx context.Context, imageID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterChallengeImages/%s", strings.TrimSpace(imageID)), nil)
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

// GetGameCenterChallengeVersionReleases retrieves challenge releases for a Game Center detail.
func (c *Client) GetGameCenterChallengeVersionReleases(ctx context.Context, gcDetailID string, opts ...GCChallengeVersionReleasesOption) (*GameCenterChallengeVersionReleasesResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterChallengeVersionReleasesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/challengeReleases", strings.TrimSpace(gcDetailID)),
		Query:       buildGCChallengeVersionReleasesQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-challenge-releases",
	})
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

// DeleteGameCenterChallengeVersionRelease deletes a challenge version release.
func (c *Client) DeleteGameCenterChallengeVersionRelease(ctx context.Context, releaseID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterChallengeVersionReleases/%s", strings.TrimSpace(releaseID)), nil)
}

package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// GetGameCenterChallenges retrieves the list of Game Center challenges for a Game Center detail.
func (c *Client) GetGameCenterChallenges(ctx context.Context, gcDetailID string, opts ...GCChallengesOption) (*GameCenterChallengesResponse, error) {
	query := &gcChallengesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterChallenges", strings.TrimSpace(gcDetailID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-challenges: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCChallengesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterChallenge retrieves a Game Center challenge by ID.
func (c *Client) GetGameCenterChallenge(ctx context.Context, challengeID string) (*GameCenterChallengeResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterChallenges/%s", strings.TrimSpace(challengeID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterChallenge creates a new Game Center challenge.
func (c *Client) CreateGameCenterChallenge(ctx context.Context, gcDetailID string, attrs GameCenterChallengeCreateAttributes, leaderboardID string, groupID string, createInitialVersion bool) (*GameCenterChallengeResponse, error) {
	const initialVersionID = "initial-version"

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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterChallenges", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterChallenges/%s", strings.TrimSpace(challengeID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterChallenge deletes a Game Center challenge.
func (c *Client) DeleteGameCenterChallenge(ctx context.Context, challengeID string) error {
	path := fmt.Sprintf("/v1/gameCenterChallenges/%s", strings.TrimSpace(challengeID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterChallengeVersions retrieves the list of challenge versions for a challenge.
func (c *Client) GetGameCenterChallengeVersions(ctx context.Context, challengeID string, opts ...GCChallengeVersionsOption) (*GameCenterChallengeVersionsResponse, error) {
	query := &gcChallengeVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterChallenges/%s/versions", strings.TrimSpace(challengeID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-challenge-versions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCChallengeVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterChallengeVersion retrieves a challenge version by ID.
func (c *Client) GetGameCenterChallengeVersion(ctx context.Context, versionID string) (*GameCenterChallengeVersionResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterChallengeVersions/%s", strings.TrimSpace(versionID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterChallengeVersions", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterChallengeLocalizations retrieves the list of localizations for a challenge version.
func (c *Client) GetGameCenterChallengeLocalizations(ctx context.Context, versionID string, opts ...GCChallengeLocalizationsOption) (*GameCenterChallengeLocalizationsResponse, error) {
	query := &gcChallengeLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterChallengeVersions/%s/localizations", strings.TrimSpace(versionID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-challenge-localizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCChallengeLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterChallengeLocalization retrieves a challenge localization by ID.
func (c *Client) GetGameCenterChallengeLocalization(ctx context.Context, localizationID string) (*GameCenterChallengeLocalizationResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterChallengeLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterChallengeLocalizationImage retrieves the image for a challenge localization.
func (c *Client) GetGameCenterChallengeLocalizationImage(ctx context.Context, localizationID string) (*GameCenterChallengeImageResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterChallengeLocalizations/%s/image", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterChallengeVersionDefaultImage retrieves the default image for a challenge version.
func (c *Client) GetGameCenterChallengeVersionDefaultImage(ctx context.Context, versionID string) (*GameCenterChallengeImageResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterChallengeVersions/%s/defaultImage", strings.TrimSpace(versionID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterChallengeLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterChallengeLocalizations/%s", strings.TrimSpace(localizationID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterChallengeLocalization deletes a challenge localization.
func (c *Client) DeleteGameCenterChallengeLocalization(ctx context.Context, localizationID string) error {
	path := fmt.Sprintf("/v1/gameCenterChallengeLocalizations/%s", strings.TrimSpace(localizationID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetGameCenterChallengeImage retrieves a challenge image by ID.
func (c *Client) GetGameCenterChallengeImage(ctx context.Context, imageID string) (*GameCenterChallengeImageResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterChallengeImages/%s", strings.TrimSpace(imageID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateGameCenterChallengeImage reserves a challenge image upload.
func (c *Client) CreateGameCenterChallengeImage(ctx context.Context, localizationID, fileName string, fileSize int64) (*GameCenterChallengeImageResponse, error) {
	payload := GameCenterChallengeImageCreateRequest{
		Data: GameCenterChallengeImageCreateData{
			Type: ResourceTypeGameCenterChallengeImages,
			Attributes: GameCenterChallengeImageCreateAttributes{
				FileName: fileName,
				FileSize: fileSize,
			},
			Relationships: &GameCenterChallengeImageRelationships{
				Localization: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeGameCenterChallengeLocalizations,
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

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterChallengeImages", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/gameCenterChallengeImages/%s", strings.TrimSpace(imageID))
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterChallengeImage deletes a challenge image.
func (c *Client) DeleteGameCenterChallengeImage(ctx context.Context, imageID string) error {
	path := fmt.Sprintf("/v1/gameCenterChallengeImages/%s", strings.TrimSpace(imageID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// UploadGameCenterChallengeImage performs the full upload flow for a challenge image.
func (c *Client) UploadGameCenterChallengeImage(ctx context.Context, localizationID, filePath string) (*GameCenterChallengeImageUploadResult, error) {
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	reservation, err := c.CreateGameCenterChallengeImage(ctx, localizationID, fileName, fileSize)
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
		FileName:       committed.Data.Attributes.FileName,
		FileSize:       committed.Data.Attributes.FileSize,
		Uploaded:       true,
	}
	if committed.Data.Attributes.AssetDeliveryState != nil {
		result.AssetDeliveryState = committed.Data.Attributes.AssetDeliveryState.State
	}

	return result, nil
}

// GetGameCenterChallengeVersionRelease retrieves a challenge version release by ID.
func (c *Client) GetGameCenterChallengeVersionRelease(ctx context.Context, releaseID string) (*GameCenterChallengeVersionReleaseResponse, error) {
	path := fmt.Sprintf("/v1/gameCenterChallengeVersionReleases/%s", strings.TrimSpace(releaseID))
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeVersionReleaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterChallengeVersionReleases retrieves challenge releases for a Game Center detail.
func (c *Client) GetGameCenterChallengeVersionReleases(ctx context.Context, gcDetailID string, opts ...GCChallengeVersionReleasesOption) (*GameCenterChallengeVersionReleasesResponse, error) {
	query := &gcChallengeVersionReleasesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/gameCenterDetails/%s/challengeReleases", strings.TrimSpace(gcDetailID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("game-center-challenge-releases: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildGCChallengeVersionReleasesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeVersionReleasesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/gameCenterChallengeVersionReleases", body)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeVersionReleaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteGameCenterChallengeVersionRelease deletes a challenge version release.
func (c *Client) DeleteGameCenterChallengeVersionRelease(ctx context.Context, releaseID string) error {
	path := fmt.Sprintf("/v1/gameCenterChallengeVersionReleases/%s", strings.TrimSpace(releaseID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

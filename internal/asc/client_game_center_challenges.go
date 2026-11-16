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

	reservation, err := c.CreateGameCenterChallengeImage(ctx, fileName, fileSize, localizationID, versionID)
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

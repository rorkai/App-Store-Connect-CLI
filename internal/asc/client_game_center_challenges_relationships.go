package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GameCenterChallengeLocalizationImageLinkageResponse is the response for challenge localization image relationships.
type GameCenterChallengeLocalizationImageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GameCenterChallengeVersionDefaultImageLinkageResponse is the response for challenge version defaultImage relationships.
type GameCenterChallengeVersionDefaultImageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GetGameCenterChallengeVersionsRelationships retrieves version linkages for a challenge.
func (c *Client) GetGameCenterChallengeVersionsRelationships(ctx context.Context, challengeID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getGameCenterChallengeLinkages(ctx, challengeID, "versions", opts...)
}

// GetGameCenterChallengeLocalizationImageRelationship retrieves the image linkage for a challenge localization.
func (c *Client) GetGameCenterChallengeLocalizationImageRelationship(ctx context.Context, localizationID string) (*GameCenterChallengeLocalizationImageLinkageResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/gameCenterChallengeLocalizations/%s/relationships/image", localizationID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeLocalizationImageLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterChallengeVersionDefaultImageRelationship retrieves the default image linkage for a challenge version.
func (c *Client) GetGameCenterChallengeVersionDefaultImageRelationship(ctx context.Context, versionID string) (*GameCenterChallengeVersionDefaultImageLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/gameCenterChallengeVersions/%s/relationships/defaultImage", versionID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterChallengeVersionDefaultImageLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterChallengeVersionLocalizationsRelationships retrieves localization linkages for a challenge version.
func (c *Client) GetGameCenterChallengeVersionLocalizationsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getGameCenterChallengeVersionLinkages(ctx, versionID, "localizations", opts...)
}

// UpdateGameCenterChallengeLeaderboardRelationship updates the leaderboard relationship on a challenge.
func (c *Client) UpdateGameCenterChallengeLeaderboardRelationship(ctx context.Context, challengeID, leaderboardID string) error {
	challengeID = strings.TrimSpace(challengeID)
	leaderboardID = strings.TrimSpace(leaderboardID)
	if challengeID == "" {
		return fmt.Errorf("challengeID is required")
	}
	if leaderboardID == "" {
		return fmt.Errorf("leaderboardID is required")
	}

	payload := struct {
		Data ResourceData `json:"data"`
	}{
		Data: ResourceData{
			Type: ResourceTypeGameCenterLeaderboards,
			ID:   leaderboardID,
		},
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterChallenges/%s/relationships/leaderboard", challengeID)
	_, err = c.do(ctx, http.MethodPatch, path, body)
	return err
}

// UpdateGameCenterChallengeLeaderboardV2Relationship updates the leaderboardV2 relationship on a challenge.
func (c *Client) UpdateGameCenterChallengeLeaderboardV2Relationship(ctx context.Context, challengeID, leaderboardID string) error {
	challengeID = strings.TrimSpace(challengeID)
	leaderboardID = strings.TrimSpace(leaderboardID)
	if challengeID == "" {
		return fmt.Errorf("challengeID is required")
	}
	if leaderboardID == "" {
		return fmt.Errorf("leaderboardID is required")
	}

	payload := struct {
		Data ResourceData `json:"data"`
	}{
		Data: ResourceData{
			Type: ResourceTypeGameCenterLeaderboards,
			ID:   leaderboardID,
		},
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterChallenges/%s/relationships/leaderboardV2", challengeID)
	_, err = c.do(ctx, http.MethodPatch, path, body)
	return err
}

func (c *Client) getGameCenterChallengeLinkages(ctx context.Context, challengeID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		challengeID,
		relationship,
		"challengeID",
		"/v1/gameCenterChallenges/%s/relationships/%s",
		"gameCenterChallengeRelationships",
		opts...,
	)
}

func (c *Client) getGameCenterChallengeVersionLinkages(ctx context.Context, versionID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		versionID,
		relationship,
		"versionID",
		"/v1/gameCenterChallengeVersions/%s/relationships/%s",
		"gameCenterChallengeVersionRelationships",
		opts...,
	)
}

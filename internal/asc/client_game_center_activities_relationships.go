package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GameCenterActivityLocalizationImageLinkageResponse is the response for activity localization image relationships.
type GameCenterActivityLocalizationImageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GameCenterActivityVersionDefaultImageLinkageResponse is the response for activity version defaultImage relationships.
type GameCenterActivityVersionDefaultImageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GetGameCenterActivityVersionsRelationships retrieves version linkages for an activity.
func (c *Client) GetGameCenterActivityVersionsRelationships(ctx context.Context, activityID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getGameCenterActivityLinkages(ctx, activityID, "versions", opts...)
}

// AddGameCenterActivityAchievementsV2 adds v2 achievements to an activity.
func (c *Client) AddGameCenterActivityAchievementsV2(ctx context.Context, activityID string, achievementIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterAchievements, achievementIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterActivities/%s/relationships/achievementsV2", strings.TrimSpace(activityID))
	_, err = c.do(ctx, http.MethodPost, path, body)
	return err
}

// RemoveGameCenterActivityAchievementsV2 removes v2 achievements from an activity.
func (c *Client) RemoveGameCenterActivityAchievementsV2(ctx context.Context, activityID string, achievementIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterAchievements, achievementIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterActivities/%s/relationships/achievementsV2", strings.TrimSpace(activityID))
	_, err = c.do(ctx, http.MethodDelete, path, body)
	return err
}

// AddGameCenterActivityLeaderboardsV2 adds v2 leaderboards to an activity.
func (c *Client) AddGameCenterActivityLeaderboardsV2(ctx context.Context, activityID string, leaderboardIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterActivities/%s/relationships/leaderboardsV2", strings.TrimSpace(activityID))
	_, err = c.do(ctx, http.MethodPost, path, body)
	return err
}

// RemoveGameCenterActivityLeaderboardsV2 removes v2 leaderboards from an activity.
func (c *Client) RemoveGameCenterActivityLeaderboardsV2(ctx context.Context, activityID string, leaderboardIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterActivities/%s/relationships/leaderboardsV2", strings.TrimSpace(activityID))
	_, err = c.do(ctx, http.MethodDelete, path, body)
	return err
}

// GetGameCenterActivityLocalizationImageRelationship retrieves the image linkage for an activity localization.
func (c *Client) GetGameCenterActivityLocalizationImageRelationship(ctx context.Context, localizationID string) (*GameCenterActivityLocalizationImageLinkageResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/gameCenterActivityLocalizations/%s/relationships/image", localizationID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityLocalizationImageLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterActivityVersionDefaultImageRelationship retrieves the default image linkage for an activity version.
func (c *Client) GetGameCenterActivityVersionDefaultImageRelationship(ctx context.Context, versionID string) (*GameCenterActivityVersionDefaultImageLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/gameCenterActivityVersions/%s/relationships/defaultImage", versionID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterActivityVersionDefaultImageLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterActivityVersionLocalizationsRelationships retrieves localization linkages for an activity version.
func (c *Client) GetGameCenterActivityVersionLocalizationsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getGameCenterActivityVersionLinkages(ctx, versionID, "localizations", opts...)
}

func (c *Client) getGameCenterActivityLinkages(ctx context.Context, activityID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		activityID,
		relationship,
		"activityID",
		"/v1/gameCenterActivities/%s/relationships/%s",
		"gameCenterActivityRelationships",
		opts...,
	)
}

func (c *Client) getGameCenterActivityVersionLinkages(ctx context.Context, versionID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		versionID,
		relationship,
		"versionID",
		"/v1/gameCenterActivityVersions/%s/relationships/%s",
		"gameCenterActivityVersionRelationships",
		opts...,
	)
}

package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GameCenterAchievementLocalizationAchievementLinkageResponse is the response for achievement localization achievement relationships.
type GameCenterAchievementLocalizationAchievementLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GameCenterAchievementLocalizationImageLinkageResponse is the response for achievement localization image relationships.
type GameCenterAchievementLocalizationImageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GameCenterAchievementGroupAchievementLinkageResponse is the response for groupAchievement relationships.
type GameCenterAchievementGroupAchievementLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GetGameCenterAchievementLocalizationAchievementRelationship retrieves the achievement linkage for a localization.
func (c *Client) GetGameCenterAchievementLocalizationAchievementRelationship(ctx context.Context, localizationID string) (*GameCenterAchievementLocalizationAchievementLinkageResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/gameCenterAchievementLocalizations/%s/relationships/gameCenterAchievement", localizationID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementLocalizationAchievementLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterAchievementLocalizationImageRelationship retrieves the image linkage for a localization.
func (c *Client) GetGameCenterAchievementLocalizationImageRelationship(ctx context.Context, localizationID string) (*GameCenterAchievementLocalizationImageLinkageResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/gameCenterAchievementLocalizations/%s/relationships/gameCenterAchievementImage", localizationID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementLocalizationImageLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterAchievementGroupAchievementRelationship retrieves the group achievement linkage for an achievement.
func (c *Client) GetGameCenterAchievementGroupAchievementRelationship(ctx context.Context, achievementID string) (*GameCenterAchievementGroupAchievementLinkageResponse, error) {
	achievementID = strings.TrimSpace(achievementID)
	if achievementID == "" {
		return nil, fmt.Errorf("achievementID is required")
	}

	path := fmt.Sprintf("/v1/gameCenterAchievements/%s/relationships/groupAchievement", achievementID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAchievementGroupAchievementLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetGameCenterAchievementLocalizationsRelationships retrieves localization linkages for an achievement.
func (c *Client) GetGameCenterAchievementLocalizationsRelationships(ctx context.Context, achievementID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getGameCenterAchievementLinkages(ctx, achievementID, "localizations", opts...)
}

// GetGameCenterAchievementReleasesRelationships retrieves release linkages for an achievement.
func (c *Client) GetGameCenterAchievementReleasesRelationships(ctx context.Context, achievementID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getGameCenterAchievementLinkages(ctx, achievementID, "releases", opts...)
}

// UpdateGameCenterAchievementActivityRelationship updates the activity relationship on an achievement.
func (c *Client) UpdateGameCenterAchievementActivityRelationship(ctx context.Context, achievementID, activityID string) error {
	achievementID = strings.TrimSpace(achievementID)
	activityID = strings.TrimSpace(activityID)
	if achievementID == "" {
		return fmt.Errorf("achievementID is required")
	}
	if activityID == "" {
		return fmt.Errorf("activityID is required")
	}

	payload := struct {
		Data ResourceData `json:"data"`
	}{
		Data: ResourceData{
			Type: ResourceTypeGameCenterActivities,
			ID:   activityID,
		},
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterAchievements/%s/relationships/activity", achievementID)
	_, err = c.do(ctx, http.MethodPatch, path, body)
	return err
}

// UpdateGameCenterAchievementGroupAchievementRelationship updates the groupAchievement relationship on an achievement.
func (c *Client) UpdateGameCenterAchievementGroupAchievementRelationship(ctx context.Context, achievementID, groupAchievementID string) error {
	achievementID = strings.TrimSpace(achievementID)
	groupAchievementID = strings.TrimSpace(groupAchievementID)
	if achievementID == "" {
		return fmt.Errorf("achievementID is required")
	}
	if groupAchievementID == "" {
		return fmt.Errorf("groupAchievementID is required")
	}

	payload := struct {
		Data ResourceData `json:"data"`
	}{
		Data: ResourceData{
			Type: ResourceTypeGameCenterAchievements,
			ID:   groupAchievementID,
		},
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterAchievements/%s/relationships/groupAchievement", achievementID)
	_, err = c.do(ctx, http.MethodPatch, path, body)
	return err
}

// GetGameCenterAchievementVersionsRelationshipsV2 retrieves version linkages for a v2 achievement.
func (c *Client) GetGameCenterAchievementVersionsRelationshipsV2(ctx context.Context, achievementID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getGameCenterAchievementLinkagesV2(ctx, achievementID, "versions", opts...)
}

// UpdateGameCenterAchievementActivityRelationshipV2 updates the activity relationship on a v2 achievement.
func (c *Client) UpdateGameCenterAchievementActivityRelationshipV2(ctx context.Context, achievementID, activityID string) error {
	achievementID = strings.TrimSpace(achievementID)
	activityID = strings.TrimSpace(activityID)
	if achievementID == "" {
		return fmt.Errorf("achievementID is required")
	}
	if activityID == "" {
		return fmt.Errorf("activityID is required")
	}

	payload := struct {
		Data ResourceData `json:"data"`
	}{
		Data: ResourceData{
			Type: ResourceTypeGameCenterActivities,
			ID:   activityID,
		},
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v2/gameCenterAchievements/%s/relationships/activity", achievementID)
	_, err = c.do(ctx, http.MethodPatch, path, body)
	return err
}

func (c *Client) getGameCenterAchievementLinkages(ctx context.Context, achievementID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		achievementID,
		relationship,
		"achievementID",
		"/v1/gameCenterAchievements/%s/relationships/%s",
		"gameCenterAchievementRelationships",
		opts...,
	)
}

func (c *Client) getGameCenterAchievementLinkagesV2(ctx context.Context, achievementID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		achievementID,
		relationship,
		"achievementID",
		"/v2/gameCenterAchievements/%s/relationships/%s",
		"gameCenterAchievementRelationshipsV2",
		opts...,
	)
}

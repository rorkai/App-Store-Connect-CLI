package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// AddGameCenterActivityAchievementsV2 adds v2 achievements to an activity.
func (c *Client) AddGameCenterActivityAchievementsV2(ctx context.Context, activityID string, achievementIDs []string) error {
	activityID = strings.TrimSpace(activityID)
	achievementIDs = normalizeList(achievementIDs)
	if activityID == "" {
		return fmt.Errorf("activityID is required")
	}
	if len(achievementIDs) == 0 {
		return fmt.Errorf("achievementIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterAchievements, achievementIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path, err := resourcePath("/v1/gameCenterActivities/%s/relationships/achievementsV2", activityID)
	if err != nil {
		return err
	}

	_, err = c.do(ctx, http.MethodPost, path, body)
	return err
}

// RemoveGameCenterActivityAchievementsV2 removes v2 achievements from an activity.
func (c *Client) RemoveGameCenterActivityAchievementsV2(ctx context.Context, activityID string, achievementIDs []string) error {
	activityID = strings.TrimSpace(activityID)
	achievementIDs = normalizeList(achievementIDs)
	if activityID == "" {
		return fmt.Errorf("activityID is required")
	}
	if len(achievementIDs) == 0 {
		return fmt.Errorf("achievementIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterAchievements, achievementIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path, err := resourcePath("/v1/gameCenterActivities/%s/relationships/achievementsV2", activityID)
	if err != nil {
		return err
	}

	_, err = c.do(ctx, http.MethodDelete, path, body)
	return err
}

// AddGameCenterActivityLeaderboardsV2 adds v2 leaderboards to an activity.
func (c *Client) AddGameCenterActivityLeaderboardsV2(ctx context.Context, activityID string, leaderboardIDs []string) error {
	activityID = strings.TrimSpace(activityID)
	leaderboardIDs = normalizeList(leaderboardIDs)
	if activityID == "" {
		return fmt.Errorf("activityID is required")
	}
	if len(leaderboardIDs) == 0 {
		return fmt.Errorf("leaderboardIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path, err := resourcePath("/v1/gameCenterActivities/%s/relationships/leaderboardsV2", activityID)
	if err != nil {
		return err
	}

	_, err = c.do(ctx, http.MethodPost, path, body)
	return err
}

// RemoveGameCenterActivityLeaderboardsV2 removes v2 leaderboards from an activity.
func (c *Client) RemoveGameCenterActivityLeaderboardsV2(ctx context.Context, activityID string, leaderboardIDs []string) error {
	activityID = strings.TrimSpace(activityID)
	leaderboardIDs = normalizeList(leaderboardIDs)
	if activityID == "" {
		return fmt.Errorf("activityID is required")
	}
	if len(leaderboardIDs) == 0 {
		return fmt.Errorf("leaderboardIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterLeaderboards, leaderboardIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path, err := resourcePath("/v1/gameCenterActivities/%s/relationships/leaderboardsV2", activityID)
	if err != nil {
		return err
	}

	_, err = c.do(ctx, http.MethodDelete, path, body)
	return err
}

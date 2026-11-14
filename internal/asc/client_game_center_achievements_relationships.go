package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

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
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/gameCenterAchievements/%s/relationships/activity", achievementID), payload)
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
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/gameCenterAchievements/%s/relationships/groupAchievement", achievementID), payload)
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
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v2/gameCenterAchievements/%s/relationships/activity", achievementID), payload)
}

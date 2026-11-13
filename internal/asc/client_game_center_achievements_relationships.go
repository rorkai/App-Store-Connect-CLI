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

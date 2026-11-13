package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// UpdateGameCenterDetailChallengesMinimumPlatformVersionsRelationship replaces the challengesMinimumPlatformVersions relationship.
func (c *Client) UpdateGameCenterDetailChallengesMinimumPlatformVersionsRelationship(ctx context.Context, detailID string, versionIDs []string) error {
	return c.updateGameCenterDetailToManyRelationship(ctx, detailID, "challengesMinimumPlatformVersions", ResourceTypeAppStoreVersions, versionIDs)
}

// UpdateGameCenterDetailGameCenterAchievementsRelationship replaces the gameCenterAchievements relationship.
func (c *Client) UpdateGameCenterDetailGameCenterAchievementsRelationship(ctx context.Context, detailID string, achievementIDs []string) error {
	return c.updateGameCenterDetailToManyRelationship(ctx, detailID, "gameCenterAchievements", ResourceTypeGameCenterAchievements, achievementIDs)
}

// UpdateGameCenterDetailGameCenterAchievementsV2Relationship replaces the gameCenterAchievementsV2 relationship.
func (c *Client) UpdateGameCenterDetailGameCenterAchievementsV2Relationship(ctx context.Context, detailID string, achievementIDs []string) error {
	return c.updateGameCenterDetailToManyRelationship(ctx, detailID, "gameCenterAchievementsV2", ResourceTypeGameCenterAchievements, achievementIDs)
}

// UpdateGameCenterDetailGameCenterLeaderboardSetsRelationship replaces the gameCenterLeaderboardSets relationship.
func (c *Client) UpdateGameCenterDetailGameCenterLeaderboardSetsRelationship(ctx context.Context, detailID string, setIDs []string) error {
	return c.updateGameCenterDetailToManyRelationship(ctx, detailID, "gameCenterLeaderboardSets", ResourceTypeGameCenterLeaderboardSets, setIDs)
}

// UpdateGameCenterDetailGameCenterLeaderboardSetsV2Relationship replaces the gameCenterLeaderboardSetsV2 relationship.
func (c *Client) UpdateGameCenterDetailGameCenterLeaderboardSetsV2Relationship(ctx context.Context, detailID string, setIDs []string) error {
	return c.updateGameCenterDetailToManyRelationship(ctx, detailID, "gameCenterLeaderboardSetsV2", ResourceTypeGameCenterLeaderboardSets, setIDs)
}

// UpdateGameCenterDetailGameCenterLeaderboardsRelationship replaces the gameCenterLeaderboards relationship.
func (c *Client) UpdateGameCenterDetailGameCenterLeaderboardsRelationship(ctx context.Context, detailID string, leaderboardIDs []string) error {
	return c.updateGameCenterDetailToManyRelationship(ctx, detailID, "gameCenterLeaderboards", ResourceTypeGameCenterLeaderboards, leaderboardIDs)
}

// UpdateGameCenterDetailGameCenterLeaderboardsV2Relationship replaces the gameCenterLeaderboardsV2 relationship.
func (c *Client) UpdateGameCenterDetailGameCenterLeaderboardsV2Relationship(ctx context.Context, detailID string, leaderboardIDs []string) error {
	return c.updateGameCenterDetailToManyRelationship(ctx, detailID, "gameCenterLeaderboardsV2", ResourceTypeGameCenterLeaderboards, leaderboardIDs)
}

func (c *Client) updateGameCenterDetailToManyRelationship(ctx context.Context, detailID, relationship string, resourceType ResourceType, ids []string) error {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return fmt.Errorf("detailID is required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(resourceType, ids),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterDetails/%s/relationships/%s", detailID, relationship)
	_, err = c.do(ctx, http.MethodPatch, path, body)
	return err
}

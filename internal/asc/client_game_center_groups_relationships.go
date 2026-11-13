package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// UpdateGameCenterGroupGameCenterLeaderboardSetsRelationship replaces the gameCenterLeaderboardSets relationship.
func (c *Client) UpdateGameCenterGroupGameCenterLeaderboardSetsRelationship(ctx context.Context, groupID string, setIDs []string) error {
	return c.updateGameCenterGroupToManyRelationship(ctx, groupID, "gameCenterLeaderboardSets", ResourceTypeGameCenterLeaderboardSets, setIDs)
}

// UpdateGameCenterGroupGameCenterLeaderboardSetsV2Relationship replaces the gameCenterLeaderboardSetsV2 relationship.
func (c *Client) UpdateGameCenterGroupGameCenterLeaderboardSetsV2Relationship(ctx context.Context, groupID string, setIDs []string) error {
	return c.updateGameCenterGroupToManyRelationship(ctx, groupID, "gameCenterLeaderboardSetsV2", ResourceTypeGameCenterLeaderboardSets, setIDs)
}

func (c *Client) updateGameCenterGroupToManyRelationship(ctx context.Context, groupID, relationship string, resourceType ResourceType, ids []string) error {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return fmt.Errorf("groupID is required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(resourceType, ids),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterGroups/%s/relationships/%s", groupID, relationship)
	_, err = c.do(ctx, http.MethodPatch, path, body)
	return err
}

package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// AddGameCenterEnabledVersionCompatibleVersions adds compatible enabled versions.
func (c *Client) AddGameCenterEnabledVersionCompatibleVersions(ctx context.Context, enabledVersionID string, compatibleIDs []string) error {
	enabledVersionID = strings.TrimSpace(enabledVersionID)
	compatibleIDs = normalizeList(compatibleIDs)
	if enabledVersionID == "" {
		return fmt.Errorf("enabledVersionID is required")
	}
	if len(compatibleIDs) == 0 {
		return fmt.Errorf("compatibleIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterEnabledVersions, compatibleIDs),
	}
	return sendNoContent(ctx, c, http.MethodPost, fmt.Sprintf("/v1/gameCenterEnabledVersions/%s/relationships/compatibleVersions", enabledVersionID), payload)
}

// RemoveGameCenterEnabledVersionCompatibleVersions removes compatible enabled versions.
func (c *Client) RemoveGameCenterEnabledVersionCompatibleVersions(ctx context.Context, enabledVersionID string, compatibleIDs []string) error {
	enabledVersionID = strings.TrimSpace(enabledVersionID)
	compatibleIDs = normalizeList(compatibleIDs)
	if enabledVersionID == "" {
		return fmt.Errorf("enabledVersionID is required")
	}
	if len(compatibleIDs) == 0 {
		return fmt.Errorf("compatibleIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterEnabledVersions, compatibleIDs),
	}
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterEnabledVersions/%s/relationships/compatibleVersions", enabledVersionID), payload)
}

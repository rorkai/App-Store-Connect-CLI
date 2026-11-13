package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// AddGameCenterAppVersionCompatibilityVersions adds compatibility versions to a Game Center app version.
func (c *Client) AddGameCenterAppVersionCompatibilityVersions(ctx context.Context, appVersionID string, compatibleIDs []string) error {
	appVersionID = strings.TrimSpace(appVersionID)
	compatibleIDs = normalizeList(compatibleIDs)
	if appVersionID == "" {
		return fmt.Errorf("appVersionID is required")
	}
	if len(compatibleIDs) == 0 {
		return fmt.Errorf("compatibleIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterAppVersions, compatibleIDs),
	}
	return sendNoContent(ctx, c, http.MethodPost, fmt.Sprintf("/v1/gameCenterAppVersions/%s/relationships/compatibilityVersions", appVersionID), payload)
}

// RemoveGameCenterAppVersionCompatibilityVersions removes compatibility versions from a Game Center app version.
func (c *Client) RemoveGameCenterAppVersionCompatibilityVersions(ctx context.Context, appVersionID string, compatibleIDs []string) error {
	appVersionID = strings.TrimSpace(appVersionID)
	compatibleIDs = normalizeList(compatibleIDs)
	if appVersionID == "" {
		return fmt.Errorf("appVersionID is required")
	}
	if len(compatibleIDs) == 0 {
		return fmt.Errorf("compatibleIDs are required")
	}

	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterAppVersions, compatibleIDs),
	}
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/gameCenterAppVersions/%s/relationships/compatibilityVersions", appVersionID), payload)
}

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
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterAppVersions/%s/relationships/compatibilityVersions", appVersionID)
	_, err = c.do(ctx, http.MethodPost, path, body)
	return err
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
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterAppVersions/%s/relationships/compatibilityVersions", appVersionID)
	_, err = c.do(ctx, http.MethodDelete, path, body)
	return err
}

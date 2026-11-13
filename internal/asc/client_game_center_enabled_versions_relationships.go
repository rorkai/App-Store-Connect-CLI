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
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterEnabledVersions/%s/relationships/compatibleVersions", enabledVersionID)
	_, err = c.do(ctx, http.MethodPost, path, body)
	return err
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
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterEnabledVersions/%s/relationships/compatibleVersions", enabledVersionID)
	_, err = c.do(ctx, http.MethodDelete, path, body)
	return err
}

// UpdateGameCenterEnabledVersionCompatibleVersionsRelationship replaces the compatibleVersions relationship.
func (c *Client) UpdateGameCenterEnabledVersionCompatibleVersionsRelationship(ctx context.Context, enabledVersionID string, compatibleIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterEnabledVersions, compatibleIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterEnabledVersions/%s/relationships/compatibleVersions", strings.TrimSpace(enabledVersionID))
	_, err = c.do(ctx, http.MethodPatch, path, body)
	return err
}

package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// GetGameCenterEnabledVersionCompatibleVersionsRelationships retrieves compatible enabled version linkages.
func (c *Client) GetGameCenterEnabledVersionCompatibleVersionsRelationships(ctx context.Context, enabledVersionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getGameCenterEnabledVersionLinkages(ctx, enabledVersionID, "compatibleVersions", opts...)
}

// AddGameCenterEnabledVersionCompatibleVersions adds compatible enabled versions.
func (c *Client) AddGameCenterEnabledVersionCompatibleVersions(ctx context.Context, enabledVersionID string, compatibleIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterEnabledVersions, compatibleIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterEnabledVersions/%s/relationships/compatibleVersions", strings.TrimSpace(enabledVersionID))
	_, err = c.do(ctx, http.MethodPost, path, body)
	return err
}

// RemoveGameCenterEnabledVersionCompatibleVersions removes compatible enabled versions.
func (c *Client) RemoveGameCenterEnabledVersionCompatibleVersions(ctx context.Context, enabledVersionID string, compatibleIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterEnabledVersions, compatibleIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterEnabledVersions/%s/relationships/compatibleVersions", strings.TrimSpace(enabledVersionID))
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

func (c *Client) getGameCenterEnabledVersionLinkages(ctx context.Context, enabledVersionID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		enabledVersionID,
		relationship,
		"enabledVersionID",
		"/v1/gameCenterEnabledVersions/%s/relationships/%s",
		"gameCenterEnabledVersionRelationships",
		opts...,
	)
}

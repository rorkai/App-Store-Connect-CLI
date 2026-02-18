package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GameCenterAppVersionAppStoreVersionLinkageResponse is the response for appStoreVersion relationships.
type GameCenterAppVersionAppStoreVersionLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GetGameCenterAppVersionAppStoreVersionRelationship retrieves the app store version linkage for a Game Center app version.
func (c *Client) GetGameCenterAppVersionAppStoreVersionRelationship(ctx context.Context, appVersionID string) (*GameCenterAppVersionAppStoreVersionLinkageResponse, error) {
	appVersionID = strings.TrimSpace(appVersionID)
	if appVersionID == "" {
		return nil, fmt.Errorf("appVersionID is required")
	}

	path := fmt.Sprintf("/v1/gameCenterAppVersions/%s/relationships/appStoreVersion", appVersionID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response GameCenterAppVersionAppStoreVersionLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetGameCenterAppVersionCompatibilityVersionsRelationships retrieves compatibility version linkages for a Game Center app version.
func (c *Client) GetGameCenterAppVersionCompatibilityVersionsRelationships(ctx context.Context, appVersionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getGameCenterAppVersionLinkages(ctx, appVersionID, "compatibilityVersions", opts...)
}

// AddGameCenterAppVersionCompatibilityVersions adds compatibility versions to a Game Center app version.
func (c *Client) AddGameCenterAppVersionCompatibilityVersions(ctx context.Context, appVersionID string, compatibleIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterAppVersions, compatibleIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterAppVersions/%s/relationships/compatibilityVersions", strings.TrimSpace(appVersionID))
	_, err = c.do(ctx, http.MethodPost, path, body)
	return err
}

// RemoveGameCenterAppVersionCompatibilityVersions removes compatibility versions from a Game Center app version.
func (c *Client) RemoveGameCenterAppVersionCompatibilityVersions(ctx context.Context, appVersionID string, compatibleIDs []string) error {
	payload := RelationshipRequest{
		Data: buildRelationshipData(ResourceTypeGameCenterAppVersions, compatibleIDs),
	}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/gameCenterAppVersions/%s/relationships/compatibilityVersions", strings.TrimSpace(appVersionID))
	_, err = c.do(ctx, http.MethodDelete, path, body)
	return err
}

func (c *Client) getGameCenterAppVersionLinkages(ctx context.Context, appVersionID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		appVersionID,
		relationship,
		"appVersionID",
		"/v1/gameCenterAppVersions/%s/relationships/%s",
		"gameCenterAppVersionRelationships",
		opts...,
	)
}

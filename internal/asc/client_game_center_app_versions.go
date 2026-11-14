package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// CreateGameCenterAppVersion creates a new Game Center app version.
func (c *Client) CreateGameCenterAppVersion(ctx context.Context, appStoreVersionID string) (*GameCenterAppVersionResponse, error) {
	appStoreVersionID = strings.TrimSpace(appStoreVersionID)
	if appStoreVersionID == "" {
		return nil, fmt.Errorf("appStoreVersionID is required")
	}

	payload := GameCenterAppVersionCreateRequest{
		Data: GameCenterAppVersionCreateData{
			Type: ResourceTypeGameCenterAppVersions,
			Relationships: &GameCenterAppVersionRelationships{
				AppStoreVersion: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreVersions,
						ID:   appStoreVersionID,
					},
				},
			},
		},
	}

	return sendJSON[GameCenterAppVersionResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/gameCenterAppVersions", Payload: payload})
}

// UpdateGameCenterAppVersion updates an existing Game Center app version.
func (c *Client) UpdateGameCenterAppVersion(ctx context.Context, appVersionID string, attrs GameCenterAppVersionUpdateAttributes) (*GameCenterAppVersionResponse, error) {
	appVersionID = strings.TrimSpace(appVersionID)
	if appVersionID == "" {
		return nil, fmt.Errorf("appVersionID is required")
	}
	if attrs.Enabled == nil {
		return nil, fmt.Errorf("at least one attribute is required")
	}

	payload := GameCenterAppVersionUpdateRequest{
		Data: GameCenterAppVersionUpdateData{
			Type:       ResourceTypeGameCenterAppVersions,
			ID:         appVersionID,
			Attributes: &attrs,
		},
	}

	return sendJSON[GameCenterAppVersionResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/gameCenterAppVersions/%s", appVersionID), Payload: payload})
}

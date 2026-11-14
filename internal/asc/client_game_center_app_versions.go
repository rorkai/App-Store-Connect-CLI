package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// GetGameCenterAppVersions retrieves the list of Game Center app versions.
func (c *Client) GetGameCenterAppVersions(ctx context.Context, opts ...GCAppVersionsOption) (*GameCenterAppVersionsResponse, error) {
	query := applyOptions(opts)
	return getJSON[GameCenterAppVersionsResponse](ctx, c, readRequest{
		Path:        "/v1/gameCenterAppVersions",
		Query:       buildGCAppVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-app-versions",
	})
}

// GetGameCenterAppVersion retrieves a Game Center app version by ID.
func (c *Client) GetGameCenterAppVersion(ctx context.Context, appVersionID string) (*GameCenterAppVersionResponse, error) {
	return getJSON[GameCenterAppVersionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterAppVersions/%s", strings.TrimSpace(appVersionID))})
}

// GetGameCenterDetailGameCenterAppVersions retrieves Game Center app versions for a detail.
func (c *Client) GetGameCenterDetailGameCenterAppVersions(ctx context.Context, detailID string, opts ...GCAppVersionsOption) (*GameCenterAppVersionsResponse, error) {
	query := applyOptions(opts)
	detailID = strings.TrimSpace(detailID)
	if query.nextURL == "" && detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}
	return getJSON[GameCenterAppVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterDetails/%s/gameCenterAppVersions", detailID),
		Query:       buildGCAppVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-detail-app-versions",
	})
}

// GetGameCenterAppVersionAppStoreVersion retrieves the related App Store version.
func (c *Client) GetGameCenterAppVersionAppStoreVersion(ctx context.Context, appVersionID string) (*AppStoreVersionResponse, error) {
	return getJSON[AppStoreVersionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/gameCenterAppVersions/%s/appStoreVersion", strings.TrimSpace(appVersionID))})
}

// GetGameCenterAppVersionCompatibilityVersions retrieves compatible Game Center app versions.
func (c *Client) GetGameCenterAppVersionCompatibilityVersions(ctx context.Context, appVersionID string, opts ...GCAppVersionsOption) (*GameCenterAppVersionsResponse, error) {
	query := applyOptions(opts)
	appVersionID = strings.TrimSpace(appVersionID)
	if query.nextURL == "" && appVersionID == "" {
		return nil, fmt.Errorf("appVersionID is required")
	}
	return getJSON[GameCenterAppVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterAppVersions/%s/compatibilityVersions", appVersionID),
		Query:       buildGCAppVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-app-version-compatibility",
	})
}

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

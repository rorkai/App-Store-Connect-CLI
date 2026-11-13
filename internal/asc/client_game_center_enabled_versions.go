package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetAppGameCenterEnabledVersions retrieves enabled Game Center versions for an app.
func (c *Client) GetAppGameCenterEnabledVersions(ctx context.Context, appID string, opts ...GCEnabledVersionsOption) (*GameCenterEnabledVersionsResponse, error) {
	query := applyOptions(opts)
	appID = strings.TrimSpace(appID)
	if query.nextURL == "" && appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	return getJSON[GameCenterEnabledVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/apps/%s/gameCenterEnabledVersions", appID),
		Query:       buildGCEnabledVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-enabled-versions",
	})
}

// GetGameCenterEnabledVersionCompatibleVersions retrieves compatible enabled versions.
func (c *Client) GetGameCenterEnabledVersionCompatibleVersions(ctx context.Context, enabledVersionID string, opts ...GCEnabledVersionsOption) (*GameCenterEnabledVersionsResponse, error) {
	query := applyOptions(opts)
	enabledVersionID = strings.TrimSpace(enabledVersionID)
	if query.nextURL == "" && enabledVersionID == "" {
		return nil, fmt.Errorf("enabledVersionID is required")
	}
	return getJSON[GameCenterEnabledVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/gameCenterEnabledVersions/%s/compatibleVersions", enabledVersionID),
		Query:       buildGCEnabledVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "game-center-enabled-versions-compatibility",
	})
}

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

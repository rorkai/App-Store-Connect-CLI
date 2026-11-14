package asc

import (
	"context"
	"fmt"
	"strings"
)

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

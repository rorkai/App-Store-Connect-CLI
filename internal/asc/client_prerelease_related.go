package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetPreReleaseVersionApp retrieves the app for a pre-release version.
func (c *Client) GetPreReleaseVersionApp(ctx context.Context, versionID string) (*AppResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	return getJSON[AppResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/preReleaseVersions/%s/app", versionID)})
}

// GetPreReleaseVersionBuilds retrieves builds for a pre-release version.
func (c *Client) GetPreReleaseVersionBuilds(ctx context.Context, versionID string, opts ...PreReleaseVersionBuildsOption) (*BuildsResponse, error) {
	query := applyOptions(opts)

	versionID = strings.TrimSpace(versionID)
	if query.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	return getJSON[BuildsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/preReleaseVersions/%s/builds", versionID),
		Query:       buildListQuery(query),
		Next:        query.nextURL,
		NextContext: "preReleaseVersionBuilds",
	})
}

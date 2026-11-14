package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetBundleIDApp retrieves the app for a bundle ID.
func (c *Client) GetBundleIDApp(ctx context.Context, bundleID string) (*AppResponse, error) {
	bundleID = strings.TrimSpace(bundleID)
	return getJSON[AppResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/bundleIds/%s/app", bundleID)})
}

// GetBundleIDProfiles retrieves profiles for a bundle ID.
func (c *Client) GetBundleIDProfiles(ctx context.Context, bundleID string, opts ...BundleIDProfilesOption) (*ProfilesResponse, error) {
	query := applyOptions(opts)
	return getJSON[ProfilesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/bundleIds/%s/profiles", strings.TrimSpace(bundleID)),
		Query:       buildBundleIDProfilesQuery(query),
		Next:        query.nextURL,
		NextContext: "bundleIdProfiles",
	})
}

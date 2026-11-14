package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetAppStoreVersionAppClipDefaultExperience retrieves the default App Clip experience for a version.
func (c *Client) GetAppStoreVersionAppClipDefaultExperience(ctx context.Context, versionID string) (*AppClipDefaultExperienceResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	return getJSON[AppClipDefaultExperienceResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appStoreVersions/%s/appClipDefaultExperience", versionID)})
}

// GetAppStoreVersionExperimentsV2ForVersion retrieves v2 experiments for an app store version.
func (c *Client) GetAppStoreVersionExperimentsV2ForVersion(ctx context.Context, versionID string, opts ...AppStoreVersionExperimentsV2Option) (*AppStoreVersionExperimentsV2Response, error) {
	query := applyOptions(opts)
	versionID = strings.TrimSpace(versionID)
	if query.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	return getJSON[AppStoreVersionExperimentsV2Response](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appStoreVersions/%s/appStoreVersionExperimentsV2", versionID),
		Query:       buildAppStoreVersionExperimentsV2Query(query),
		Next:        query.nextURL,
		NextContext: "appStoreVersionExperimentsV2",
	})
}

// GetAppStoreVersionCustomerReviews retrieves customer reviews for an app store version.
func (c *Client) GetAppStoreVersionCustomerReviews(ctx context.Context, versionID string, opts ...ReviewOption) (*ReviewsResponse, error) {
	query := applyOptions(opts)
	versionID = strings.TrimSpace(versionID)
	if query.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	return getJSON[ReviewsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/appStoreVersions/%s/customerReviews", versionID),
		Query:       buildReviewQuery(opts),
		Next:        query.nextURL,
		NextContext: "customerReviews",
	})
}

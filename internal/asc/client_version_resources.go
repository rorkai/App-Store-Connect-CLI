package asc

import (
	"context"
	"fmt"
	"strings"
)

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

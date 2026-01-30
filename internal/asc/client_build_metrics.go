package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetBuildBetaUsagesMetrics retrieves beta build usage metrics for a build.
func (c *Client) GetBuildBetaUsagesMetrics(ctx context.Context, buildID string, opts ...BetaBuildUsagesOption) (*BetaBuildUsagesResponse, error) {
	query := &betaBuildUsagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	buildID = strings.TrimSpace(buildID)
	if query.nextURL == "" && buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	path := fmt.Sprintf("/v1/builds/%s/metrics/betaBuildUsages", buildID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaBuildUsages: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBetaBuildUsagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	return &BetaBuildUsagesResponse{Data: data}, nil
}

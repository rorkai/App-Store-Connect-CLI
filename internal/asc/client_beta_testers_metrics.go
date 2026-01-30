package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetBetaTesterUsagesMetrics retrieves beta tester usage metrics for a tester.
func (c *Client) GetBetaTesterUsagesMetrics(ctx context.Context, testerID string, opts ...BetaTesterUsagesOption) (*BetaTesterUsagesResponse, error) {
	query := &betaTesterUsagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	testerID = strings.TrimSpace(testerID)
	if query.nextURL == "" && testerID == "" {
		return nil, fmt.Errorf("testerID is required")
	}
	if query.nextURL == "" && strings.TrimSpace(query.appID) == "" {
		return nil, fmt.Errorf("appID is required")
	}

	path := fmt.Sprintf("/v1/betaTesters/%s/metrics/betaTesterUsages", testerID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaTesterUsages: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBetaTesterUsagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	return &BetaTesterUsagesResponse{Data: data}, nil
}

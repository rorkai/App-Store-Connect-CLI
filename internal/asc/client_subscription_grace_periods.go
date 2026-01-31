package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetSubscriptionGracePeriod retrieves a subscription grace period by ID.
func (c *Client) GetSubscriptionGracePeriod(ctx context.Context, gracePeriodID string) (*SubscriptionGracePeriodResponse, error) {
	gracePeriodID = strings.TrimSpace(gracePeriodID)
	if gracePeriodID == "" {
		return nil, fmt.Errorf("grace period ID is required")
	}

	path := fmt.Sprintf("/v1/subscriptionGracePeriods/%s", gracePeriodID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionGracePeriodResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// UpdateSubscriptionGracePeriod updates a subscription grace period by ID.
func (c *Client) UpdateSubscriptionGracePeriod(ctx context.Context, gracePeriodID string, attrs SubscriptionGracePeriodUpdateAttributes) (*SubscriptionGracePeriodResponse, error) {
	payload := SubscriptionGracePeriodUpdateRequest{
		Data: SubscriptionGracePeriodUpdateData{
			Type:       ResourceTypeSubscriptionGracePeriods,
			ID:         strings.TrimSpace(gracePeriodID),
			Attributes: attrs,
		},
	}

	return sendJSON[SubscriptionGracePeriodResponse](ctx, c, writeRequest{
		Method:  http.MethodPatch,
		Path:    fmt.Sprintf("/v1/subscriptionGracePeriods/%s", strings.TrimSpace(gracePeriodID)),
		Payload: payload,
	})
}

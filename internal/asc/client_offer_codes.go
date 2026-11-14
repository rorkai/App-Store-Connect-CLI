package asc

import (
	"context"
	"net/http"
)

// CreateSubscriptionOfferCodeCustomCode creates a custom code.
func (c *Client) CreateSubscriptionOfferCodeCustomCode(ctx context.Context, req SubscriptionOfferCodeCustomCodeCreateRequest) (*SubscriptionOfferCodeCustomCodeResponse, error) {
	return sendJSON[SubscriptionOfferCodeCustomCodeResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/subscriptionOfferCodeCustomCodes", Payload: req})
}

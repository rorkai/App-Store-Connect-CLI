package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// GetSubscriptionOfferCodeCustomCode retrieves a custom code by ID.
func (c *Client) GetSubscriptionOfferCodeCustomCode(ctx context.Context, customCodeID string) (*SubscriptionOfferCodeCustomCodeResponse, error) {
	return getJSON[SubscriptionOfferCodeCustomCodeResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/subscriptionOfferCodeCustomCodes/%s", strings.TrimSpace(customCodeID))})
}

// CreateSubscriptionOfferCodeCustomCode creates a custom code.
func (c *Client) CreateSubscriptionOfferCodeCustomCode(ctx context.Context, req SubscriptionOfferCodeCustomCodeCreateRequest) (*SubscriptionOfferCodeCustomCodeResponse, error) {
	return sendJSON[SubscriptionOfferCodeCustomCodeResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/subscriptionOfferCodeCustomCodes", Payload: req})
}

// UpdateSubscriptionOfferCodeCustomCode updates a custom code.
func (c *Client) UpdateSubscriptionOfferCodeCustomCode(ctx context.Context, customCodeID string, attrs SubscriptionOfferCodeCustomCodeUpdateAttributes) (*SubscriptionOfferCodeCustomCodeResponse, error) {
	payload := SubscriptionOfferCodeCustomCodeUpdateRequest{
		Data: SubscriptionOfferCodeCustomCodeUpdateData{
			Type:       ResourceTypeSubscriptionOfferCodeCustomCodes,
			ID:         strings.TrimSpace(customCodeID),
			Attributes: attrs,
		},
	}

	return sendJSON[SubscriptionOfferCodeCustomCodeResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/subscriptionOfferCodeCustomCodes/%s", strings.TrimSpace(customCodeID)), Payload: payload})
}

// UpdateSubscriptionOfferCodeOneTimeUseCode updates a one-time use code batch.
func (c *Client) UpdateSubscriptionOfferCodeOneTimeUseCode(ctx context.Context, oneTimeUseCodeID string, attrs SubscriptionOfferCodeOneTimeUseCodeUpdateAttributes) (*SubscriptionOfferCodeOneTimeUseCodeResponse, error) {
	payload := SubscriptionOfferCodeOneTimeUseCodeUpdateRequest{
		Data: SubscriptionOfferCodeOneTimeUseCodeUpdateData{
			Type:       ResourceTypeSubscriptionOfferCodeOneTimeUseCodes,
			ID:         strings.TrimSpace(oneTimeUseCodeID),
			Attributes: attrs,
		},
	}

	return sendJSON[SubscriptionOfferCodeOneTimeUseCodeResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/subscriptionOfferCodeOneTimeUseCodes/%s", strings.TrimSpace(oneTimeUseCodeID)), Payload: payload})
}

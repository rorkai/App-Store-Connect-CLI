package asc

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// MarketplaceSearchDetailsResponse is the response for marketplace search detail list formatting.
type MarketplaceSearchDetailsResponse = Response[MarketplaceSearchDetailAttributes]

// GetMarketplaceWebhook retrieves a marketplace webhook by ID from the
// supported collection endpoint.
func (c *Client) GetMarketplaceWebhook(ctx context.Context, webhookID string) (*MarketplaceWebhookResponse, error) {
	webhookID = strings.TrimSpace(webhookID)
	if webhookID == "" {
		return nil, fmt.Errorf("webhookID is required")
	}

	firstPage, err := c.GetMarketplaceWebhooks(ctx, Limit[MarketplaceWebhooksOption](200))
	if err != nil {
		return nil, err
	}

	stop := errors.New("marketplace webhook found")
	var resource Resource[MarketplaceWebhookAttributes]
	err = PaginateEach(ctx, firstPage, func(ctx context.Context, nextURL string) (PaginatedResponse, error) {
		return c.GetMarketplaceWebhooks(ctx, NextURL[MarketplaceWebhooksOption](nextURL))
	}, func(page PaginatedResponse) error {
		webhooks, ok := page.(*MarketplaceWebhooksResponse)
		if !ok {
			return fmt.Errorf("unexpected marketplace webhooks response type %T", page)
		}
		for i := range webhooks.Data {
			if webhooks.Data[i].ID == webhookID {
				resource = webhooks.Data[i]
				return stop
			}
		}
		return nil
	})
	if errors.Is(err, stop) {
		return &MarketplaceWebhookResponse{Data: resource}, nil
	}
	if err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("marketplace webhook %q not found: %w", webhookID, ErrNotFound)
}

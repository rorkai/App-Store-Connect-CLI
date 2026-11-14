package asc

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// MarketplaceSearchDetailsResponse is the response for marketplace search detail list formatting.
type MarketplaceSearchDetailsResponse = Response[MarketplaceSearchDetailAttributes]

// MarketplaceSearchDetailDeleteResult represents CLI output for search detail deletions.
type MarketplaceSearchDetailDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// MarketplaceWebhookDeleteResult represents CLI output for webhook deletions.
type MarketplaceWebhookDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// CreateMarketplaceSearchDetail creates marketplace search details for an app.
func (c *Client) CreateMarketplaceSearchDetail(ctx context.Context, appID, catalogURL string) (*MarketplaceSearchDetailResponse, error) {
	appID = strings.TrimSpace(appID)
	catalogURL = strings.TrimSpace(catalogURL)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	if catalogURL == "" {
		return nil, fmt.Errorf("catalogURL is required")
	}

	payload := MarketplaceSearchDetailCreateRequest{
		Data: MarketplaceSearchDetailCreateData{
			Type:       ResourceTypeMarketplaceSearchDetails,
			Attributes: MarketplaceSearchDetailCreateAttributes{CatalogURL: catalogURL},
			Relationships: &MarketplaceSearchDetailCreateRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
			},
		},
	}

	return sendJSON[MarketplaceSearchDetailResponse](ctx, c, writeRequest{
		Method:   "POST",
		Path:     "/v1/marketplaceSearchDetails",
		Payload:  payload,
		ParseErr: "failed to parse marketplace search detail response",
	})
}

// UpdateMarketplaceSearchDetail updates marketplace search details by ID.
func (c *Client) UpdateMarketplaceSearchDetail(ctx context.Context, detailID string, attrs MarketplaceSearchDetailUpdateAttributes) (*MarketplaceSearchDetailResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}

	payload := MarketplaceSearchDetailUpdateRequest{
		Data: MarketplaceSearchDetailUpdateData{
			Type: ResourceTypeMarketplaceSearchDetails,
			ID:   detailID,
		},
	}
	if attrs.CatalogURL != nil {
		payload.Data.Attributes = &attrs
	}

	return sendJSON[MarketplaceSearchDetailResponse](ctx, c, writeRequest{
		Method:   "PATCH",
		Path:     fmt.Sprintf("/v1/marketplaceSearchDetails/%s", detailID),
		Payload:  payload,
		ParseErr: "failed to parse marketplace search detail response",
	})
}

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

// CreateMarketplaceWebhook creates a marketplace webhook.
func (c *Client) CreateMarketplaceWebhook(ctx context.Context, endpointURL, secret string) (*MarketplaceWebhookResponse, error) {
	endpointURL = strings.TrimSpace(endpointURL)
	secret = strings.TrimSpace(secret)
	if endpointURL == "" {
		return nil, fmt.Errorf("endpointURL is required")
	}
	if secret == "" {
		return nil, fmt.Errorf("secret is required")
	}

	payload := MarketplaceWebhookCreateRequest{
		Data: MarketplaceWebhookCreateData{
			Type: ResourceTypeMarketplaceWebhooks,
			Attributes: MarketplaceWebhookCreateAttributes{
				EndpointURL: endpointURL,
				Secret:      secret,
			},
		},
	}

	return sendJSON[MarketplaceWebhookResponse](ctx, c, writeRequest{
		Method:   "POST",
		Path:     "/v1/marketplaceWebhooks",
		Payload:  payload,
		ParseErr: "failed to parse marketplace webhook response",
	})
}

// UpdateMarketplaceWebhook updates a marketplace webhook by ID.
func (c *Client) UpdateMarketplaceWebhook(ctx context.Context, webhookID string, attrs MarketplaceWebhookUpdateAttributes) (*MarketplaceWebhookResponse, error) {
	webhookID = strings.TrimSpace(webhookID)
	if webhookID == "" {
		return nil, fmt.Errorf("webhookID is required")
	}

	payload := MarketplaceWebhookUpdateRequest{
		Data: MarketplaceWebhookUpdateData{
			Type: ResourceTypeMarketplaceWebhooks,
			ID:   webhookID,
		},
	}
	if attrs.EndpointURL != nil || attrs.Secret != nil {
		payload.Data.Attributes = &attrs
	}

	return sendJSON[MarketplaceWebhookResponse](ctx, c, writeRequest{
		Method:   "PATCH",
		Path:     fmt.Sprintf("/v1/marketplaceWebhooks/%s", webhookID),
		Payload:  payload,
		ParseErr: "failed to parse marketplace webhook response",
	})
}

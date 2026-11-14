package asc

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// MarketplaceSearchDetailAttributes describes marketplace search detail attributes.
type MarketplaceSearchDetailAttributes struct {
	CatalogURL string `json:"catalogUrl,omitempty"`
}

// MarketplaceSearchDetailsResponse is the response for marketplace search detail list formatting.
type MarketplaceSearchDetailsResponse = Response[MarketplaceSearchDetailAttributes]

// MarketplaceSearchDetailResponse is the response for marketplace search detail endpoints.
type MarketplaceSearchDetailResponse = SingleResponse[MarketplaceSearchDetailAttributes]

// MarketplaceSearchDetailCreateAttributes describes attributes for creating search details.
type MarketplaceSearchDetailCreateAttributes struct {
	CatalogURL string `json:"catalogUrl"`
}

// MarketplaceSearchDetailCreateRelationships describes relationships for create requests.
type MarketplaceSearchDetailCreateRelationships struct {
	App *Relationship `json:"app"`
}

// MarketplaceSearchDetailCreateData is the data payload for create requests.
type MarketplaceSearchDetailCreateData struct {
	Type          ResourceType                                `json:"type"`
	Attributes    MarketplaceSearchDetailCreateAttributes     `json:"attributes"`
	Relationships *MarketplaceSearchDetailCreateRelationships `json:"relationships"`
}

// MarketplaceSearchDetailCreateRequest is a request to create search details.
type MarketplaceSearchDetailCreateRequest struct {
	Data MarketplaceSearchDetailCreateData `json:"data"`
}

// MarketplaceSearchDetailUpdateAttributes describes fields for updating search details.
type MarketplaceSearchDetailUpdateAttributes struct {
	CatalogURL *string `json:"catalogUrl,omitempty"`
}

// MarketplaceSearchDetailUpdateData is the data payload for update requests.
type MarketplaceSearchDetailUpdateData struct {
	Type       ResourceType                             `json:"type"`
	ID         string                                   `json:"id"`
	Attributes *MarketplaceSearchDetailUpdateAttributes `json:"attributes,omitempty"`
}

// MarketplaceSearchDetailUpdateRequest is a request to update search details.
type MarketplaceSearchDetailUpdateRequest struct {
	Data MarketplaceSearchDetailUpdateData `json:"data"`
}

// MarketplaceSearchDetailDeleteResult represents CLI output for search detail deletions.
type MarketplaceSearchDetailDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// MarketplaceWebhookAttributes describes marketplace webhook attributes.
type MarketplaceWebhookAttributes struct {
	EndpointURL string `json:"endpointUrl,omitempty"`
}

// MarketplaceWebhooksResponse is the response from marketplace webhooks list endpoint.
type MarketplaceWebhooksResponse = Response[MarketplaceWebhookAttributes]

// MarketplaceWebhookResponse is the response from marketplace webhook detail endpoint.
type MarketplaceWebhookResponse = SingleResponse[MarketplaceWebhookAttributes]

// MarketplaceWebhookCreateAttributes describes attributes for creating a webhook.
type MarketplaceWebhookCreateAttributes struct {
	EndpointURL string `json:"endpointUrl"`
	Secret      string `json:"secret"`
}

// MarketplaceWebhookCreateData is the data payload for create requests.
type MarketplaceWebhookCreateData struct {
	Type       ResourceType                       `json:"type"`
	Attributes MarketplaceWebhookCreateAttributes `json:"attributes"`
}

// MarketplaceWebhookCreateRequest is a request to create a webhook.
type MarketplaceWebhookCreateRequest struct {
	Data MarketplaceWebhookCreateData `json:"data"`
}

// MarketplaceWebhookUpdateAttributes describes fields for updating a webhook.
type MarketplaceWebhookUpdateAttributes struct {
	EndpointURL *string `json:"endpointUrl,omitempty"`
	Secret      *string `json:"secret,omitempty"`
}

// MarketplaceWebhookUpdateData is the data payload for update requests.
type MarketplaceWebhookUpdateData struct {
	Type       ResourceType                        `json:"type"`
	ID         string                              `json:"id"`
	Attributes *MarketplaceWebhookUpdateAttributes `json:"attributes,omitempty"`
}

// MarketplaceWebhookUpdateRequest is a request to update a webhook.
type MarketplaceWebhookUpdateRequest struct {
	Data MarketplaceWebhookUpdateData `json:"data"`
}

// MarketplaceWebhookDeleteResult represents CLI output for webhook deletions.
type MarketplaceWebhookDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GetMarketplaceSearchDetailForApp retrieves marketplace search details for an app.
func (c *Client) GetMarketplaceSearchDetailForApp(ctx context.Context, appID string, fields []string) (*MarketplaceSearchDetailResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	return getJSON[MarketplaceSearchDetailResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/apps/%s/marketplaceSearchDetail", appID),
		Query:    buildMarketplaceSearchDetailsFieldsQuery(fields),
		ParseErr: "failed to parse marketplace search detail response",
	})
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

// DeleteMarketplaceSearchDetail deletes marketplace search details by ID.
func (c *Client) DeleteMarketplaceSearchDetail(ctx context.Context, detailID string) error {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return fmt.Errorf("detailID is required")
	}

	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/marketplaceSearchDetails/%s", detailID), nil)
}

// GetMarketplaceWebhooks retrieves marketplace webhooks.
func (c *Client) GetMarketplaceWebhooks(ctx context.Context, opts ...MarketplaceWebhooksOption) (*MarketplaceWebhooksResponse, error) {
	query := applyOptions(opts)

	return getJSON[MarketplaceWebhooksResponse](ctx, c, readRequest{
		Path:        "/v1/marketplaceWebhooks",
		Query:       buildMarketplaceWebhooksQuery(query),
		Next:        query.nextURL,
		NextContext: "marketplaceWebhooks",
		ParseErr:    "failed to parse marketplace webhooks response",
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

// DeleteMarketplaceWebhook deletes a marketplace webhook by ID.
func (c *Client) DeleteMarketplaceWebhook(ctx context.Context, webhookID string) error {
	webhookID = strings.TrimSpace(webhookID)
	if webhookID == "" {
		return fmt.Errorf("webhookID is required")
	}

	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/marketplaceWebhooks/%s", webhookID), nil)
}

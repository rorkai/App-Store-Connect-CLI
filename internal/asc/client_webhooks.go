package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// GetAppWebhooks retrieves webhooks for an app.
func (c *Client) GetAppWebhooks(ctx context.Context, appID string, opts ...WebhooksOption) (*WebhooksResponse, error) {
	query := applyOptions(opts)
	appID = strings.TrimSpace(appID)
	return getJSON[WebhooksResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/apps/%s/webhooks", appID),
		Query:       buildWebhooksQuery(query),
		Next:        query.nextURL,
		NextContext: "webhooks",
		ParseErr:    "failed to parse webhooks response",
	})
}

// GetWebhook retrieves a webhook by ID.
func (c *Client) GetWebhook(ctx context.Context, webhookID string) (*WebhookResponse, error) {
	webhookID = strings.TrimSpace(webhookID)
	return getJSON[WebhookResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/webhooks/%s", webhookID),
		ParseErr: "failed to parse webhook response",
	})
}

// CreateWebhook creates a webhook for an app.
func (c *Client) CreateWebhook(ctx context.Context, appID string, attrs WebhookCreateAttributes) (*WebhookResponse, error) {
	payload := WebhookCreateRequest{
		Data: WebhookCreateData{
			Type:       ResourceTypeWebhooks,
			Attributes: attrs,
			Relationships: WebhookCreateRelationships{
				App: Relationship{
					Data: ResourceData{Type: ResourceTypeApps, ID: strings.TrimSpace(appID)},
				},
			},
		},
	}
	return sendJSON[WebhookResponse](ctx, c, writeRequest{
		Method:   http.MethodPost,
		Path:     "/v1/webhooks",
		Payload:  payload,
		ParseErr: "failed to parse webhook response",
	})
}

// UpdateWebhook updates a webhook by ID.
func (c *Client) UpdateWebhook(ctx context.Context, webhookID string, attrs WebhookUpdateAttributes) (*WebhookResponse, error) {
	webhookID = strings.TrimSpace(webhookID)
	payload := WebhookUpdateRequest{
		Data: WebhookUpdateData{
			Type:       ResourceTypeWebhooks,
			ID:         webhookID,
			Attributes: &attrs,
		},
	}
	return sendJSON[WebhookResponse](ctx, c, writeRequest{
		Method:   http.MethodPatch,
		Path:     fmt.Sprintf("/v1/webhooks/%s", webhookID),
		Payload:  payload,
		ParseErr: "failed to parse webhook response",
	})
}

// DeleteWebhook deletes a webhook by ID.
func (c *Client) DeleteWebhook(ctx context.Context, webhookID string) error {
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/webhooks/%s", strings.TrimSpace(webhookID)), nil)
}

// GetWebhookDeliveries retrieves deliveries for a webhook.
func (c *Client) GetWebhookDeliveries(ctx context.Context, webhookID string, opts ...WebhookDeliveriesOption) (*WebhookDeliveriesResponse, error) {
	query := applyOptions(opts)
	return getJSON[WebhookDeliveriesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/webhooks/%s/deliveries", strings.TrimSpace(webhookID)),
		Query:       buildWebhookDeliveriesQuery(query),
		Next:        query.nextURL,
		NextContext: "webhookDeliveries",
		ParseErr:    "failed to parse webhook deliveries response",
	})
}

// GetWebhookDeliveriesRelationships retrieves delivery relationships for a webhook.
func (c *Client) GetWebhookDeliveriesRelationships(ctx context.Context, webhookID string, opts ...LinkagesOption) (*WebhookDeliveriesLinkagesResponse, error) {
	query := applyOptions(opts)
	return getJSON[WebhookDeliveriesLinkagesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/webhooks/%s/relationships/deliveries", strings.TrimSpace(webhookID)),
		Query:       buildLinkagesQuery(query),
		Next:        query.nextURL,
		NextContext: "webhookDeliveries relationships",
		ParseErr:    "failed to parse webhook deliveries relationships response",
	})
}

// CreateWebhookDelivery creates a webhook delivery (redelivery) from a template delivery.
func (c *Client) CreateWebhookDelivery(ctx context.Context, deliveryID string) (*WebhookDeliveryResponse, error) {
	payload := WebhookDeliveryCreateRequest{
		Data: WebhookDeliveryCreateData{
			Type: ResourceTypeWebhookDeliveries,
			Relationships: WebhookDeliveryCreateRelationships{
				Template: Relationship{
					Data: ResourceData{Type: ResourceTypeWebhookDeliveries, ID: strings.TrimSpace(deliveryID)},
				},
			},
		},
	}
	return sendJSON[WebhookDeliveryResponse](ctx, c, writeRequest{
		Method:   http.MethodPost,
		Path:     "/v1/webhookDeliveries",
		Payload:  payload,
		ParseErr: "failed to parse webhook delivery response",
	})
}

// CreateWebhookPing creates a webhook ping for a webhook.
func (c *Client) CreateWebhookPing(ctx context.Context, webhookID string) (*WebhookPingResponse, error) {
	payload := WebhookPingCreateRequest{
		Data: WebhookPingCreateData{
			Type: ResourceTypeWebhookPings,
			Relationships: WebhookPingCreateRelationships{
				Webhook: Relationship{
					Data: ResourceData{Type: ResourceTypeWebhooks, ID: strings.TrimSpace(webhookID)},
				},
			},
		},
	}
	return sendJSON[WebhookPingResponse](ctx, c, writeRequest{
		Method:   http.MethodPost,
		Path:     "/v1/webhookPings",
		Payload:  payload,
		ParseErr: "failed to parse webhook ping response",
	})
}

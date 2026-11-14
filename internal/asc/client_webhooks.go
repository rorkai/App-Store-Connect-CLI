package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

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

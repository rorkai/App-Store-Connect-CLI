package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetAppWebhooks retrieves webhooks for an app.
func (c *Client) GetAppWebhooks(ctx context.Context, appID string, opts ...WebhooksOption) (*WebhooksResponse, error) {
	query := &webhooksQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	path := fmt.Sprintf("/v1/apps/%s/webhooks", appID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("webhooks: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildWebhooksQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response WebhooksResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse webhooks response: %w", err)
	}

	return &response, nil
}

// GetWebhook retrieves a webhook by ID.
func (c *Client) GetWebhook(ctx context.Context, webhookID string) (*WebhookResponse, error) {
	webhookID = strings.TrimSpace(webhookID)
	path := fmt.Sprintf("/v1/webhooks/%s", webhookID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response WebhookResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse webhook response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/webhooks", body)
	if err != nil {
		return nil, err
	}

	var response WebhookResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse webhook response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/v1/webhooks/%s", webhookID), body)
	if err != nil {
		return nil, err
	}

	var response WebhookResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse webhook response: %w", err)
	}

	return &response, nil
}

// DeleteWebhook deletes a webhook by ID.
func (c *Client) DeleteWebhook(ctx context.Context, webhookID string) error {
	path := fmt.Sprintf("/v1/webhooks/%s", strings.TrimSpace(webhookID))
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetWebhookDeliveries retrieves deliveries for a webhook.
func (c *Client) GetWebhookDeliveries(ctx context.Context, webhookID string, opts ...WebhookDeliveriesOption) (*WebhookDeliveriesResponse, error) {
	query := &webhookDeliveriesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/webhooks/%s/deliveries", strings.TrimSpace(webhookID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("webhookDeliveries: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildWebhookDeliveriesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response WebhookDeliveriesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse webhook deliveries response: %w", err)
	}

	return &response, nil
}

// GetWebhookDeliveriesRelationships retrieves delivery relationships for a webhook.
func (c *Client) GetWebhookDeliveriesRelationships(ctx context.Context, webhookID string, opts ...LinkagesOption) (*WebhookDeliveriesLinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/webhooks/%s/relationships/deliveries", strings.TrimSpace(webhookID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("webhookDeliveries relationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response WebhookDeliveriesLinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse webhook deliveries relationships response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/webhookDeliveries", body)
	if err != nil {
		return nil, err
	}

	var response WebhookDeliveryResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse webhook delivery response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/webhookPings", body)
	if err != nil {
		return nil, err
	}

	var response WebhookPingResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse webhook ping response: %w", err)
	}

	return &response, nil
}

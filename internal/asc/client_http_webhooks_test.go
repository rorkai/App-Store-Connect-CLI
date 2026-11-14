package asc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestCreateWebhook_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"webhooks","id":"wh-1","attributes":{"name":"Build Updates","url":"https://example.com/webhook","enabled":true}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/webhooks" {
			t.Fatalf("expected path /v1/webhooks, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload WebhookCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeWebhooks {
			t.Fatalf("expected type webhooks, got %q", payload.Data.Type)
		}
		if payload.Data.Attributes.Name != "Build Updates" {
			t.Fatalf("expected name Build Updates, got %q", payload.Data.Attributes.Name)
		}
		if payload.Data.Attributes.URL != "https://example.com/webhook" {
			t.Fatalf("expected url https://example.com/webhook, got %q", payload.Data.Attributes.URL)
		}
		if payload.Data.Attributes.Secret != "secret123" {
			t.Fatalf("expected secret secret123, got %q", payload.Data.Attributes.Secret)
		}
		if !payload.Data.Attributes.Enabled {
			t.Fatalf("expected enabled true")
		}
		if len(payload.Data.Attributes.EventTypes) != 1 || payload.Data.Attributes.EventTypes[0] != WebhookEventBuildUploadStateUpdated {
			t.Fatalf("expected event type BUILD_UPLOAD_STATE_UPDATED, got %v", payload.Data.Attributes.EventTypes)
		}
		if payload.Data.Relationships.App.Data.ID != "app-1" {
			t.Fatalf("expected app id app-1, got %q", payload.Data.Relationships.App.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	attrs := WebhookCreateAttributes{
		Enabled:    true,
		EventTypes: []WebhookEventType{WebhookEventBuildUploadStateUpdated},
		Name:       "Build Updates",
		Secret:     "secret123",
		URL:        "https://example.com/webhook",
	}
	if _, err := client.CreateWebhook(context.Background(), "app-1", attrs); err != nil {
		t.Fatalf("CreateWebhook() error: %v", err)
	}
}

func TestUpdateWebhook_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"webhooks","id":"wh-1","attributes":{"name":"Build Updates"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/webhooks/wh-1" {
			t.Fatalf("expected path /v1/webhooks/wh-1, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload WebhookUpdateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeWebhooks {
			t.Fatalf("expected type webhooks, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "wh-1" {
			t.Fatalf("expected id wh-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes == nil || payload.Data.Attributes.URL == nil {
			t.Fatalf("expected url attribute to be set")
		}
		if *payload.Data.Attributes.URL != "https://example.com/new" {
			t.Fatalf("expected url https://example.com/new, got %q", *payload.Data.Attributes.URL)
		}
		assertAuthorized(t, req)
	}, response)

	urlValue := "https://example.com/new"
	attrs := WebhookUpdateAttributes{URL: &urlValue}
	if _, err := client.UpdateWebhook(context.Background(), "wh-1", attrs); err != nil {
		t.Fatalf("UpdateWebhook() error: %v", err)
	}
}

func TestCreateWebhookDelivery_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"webhookDeliveries","id":"d1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/webhookDeliveries" {
			t.Fatalf("expected path /v1/webhookDeliveries, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload WebhookDeliveryCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeWebhookDeliveries {
			t.Fatalf("expected type webhookDeliveries, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships.Template.Data.ID != "d1" {
			t.Fatalf("expected delivery template id d1, got %q", payload.Data.Relationships.Template.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateWebhookDelivery(context.Background(), "d1"); err != nil {
		t.Fatalf("CreateWebhookDelivery() error: %v", err)
	}
}

func TestCreateWebhookPing_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusCreated, `{"data":{"type":"webhookPings","id":"p1"}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/v1/webhookPings" {
			t.Fatalf("expected path /v1/webhookPings, got %s", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload WebhookPingCreateRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.Data.Type != ResourceTypeWebhookPings {
			t.Fatalf("expected type webhookPings, got %q", payload.Data.Type)
		}
		if payload.Data.Relationships.Webhook.Data.ID != "wh-1" {
			t.Fatalf("expected webhook id wh-1, got %q", payload.Data.Relationships.Webhook.Data.ID)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.CreateWebhookPing(context.Background(), "wh-1"); err != nil {
		t.Fatalf("CreateWebhookPing() error: %v", err)
	}
}

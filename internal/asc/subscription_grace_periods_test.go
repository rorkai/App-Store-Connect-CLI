package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestGetAppSubscriptionGracePeriod(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionGracePeriods","id":"grace-1","attributes":{"optIn":true,"sandboxOptIn":false,"duration":"DAY_16","renewalType":"ALL_RENEWALS"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/subscriptionGracePeriod" {
			t.Fatalf("expected path /v1/apps/app-1/subscriptionGracePeriod, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetAppSubscriptionGracePeriod(context.Background(), "app-1"); err != nil {
		t.Fatalf("GetAppSubscriptionGracePeriod() error: %v", err)
	}
}

func TestGetSubscriptionGracePeriod(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionGracePeriods","id":"grace-1","attributes":{"optIn":true,"sandboxOptIn":true,"duration":"DAY_28","renewalType":"PAID_TO_PAID_ONLY"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionGracePeriods/grace-1" {
			t.Fatalf("expected path /v1/subscriptionGracePeriods/grace-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetSubscriptionGracePeriod(context.Background(), "grace-1"); err != nil {
		t.Fatalf("GetSubscriptionGracePeriod() error: %v", err)
	}
}

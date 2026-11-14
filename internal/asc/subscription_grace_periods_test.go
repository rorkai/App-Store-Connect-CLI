package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestUpdateSubscriptionGracePeriod(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionGracePeriods","id":"grace-1","attributes":{"optIn":true,"sandboxOptIn":false,"duration":"SIXTEEN_DAYS","renewalType":"ALL_RENEWALS"}}}`)
	client := newTestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionGracePeriods/grace-1" {
			t.Fatalf("expected path /v1/subscriptionGracePeriods/grace-1, got %s", req.URL.Path)
		}
		var payload SubscriptionGracePeriodUpdateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Data.Type != ResourceTypeSubscriptionGracePeriods {
			t.Fatalf("expected type subscriptionGracePeriods, got %q", payload.Data.Type)
		}
		if payload.Data.ID != "grace-1" {
			t.Fatalf("expected id grace-1, got %q", payload.Data.ID)
		}
		if payload.Data.Attributes.OptIn == nil || !*payload.Data.Attributes.OptIn {
			t.Fatalf("expected optIn=true, got %+v", payload.Data.Attributes.OptIn)
		}
		if payload.Data.Attributes.SandboxOptIn == nil || *payload.Data.Attributes.SandboxOptIn {
			t.Fatalf("expected sandboxOptIn=false, got %+v", payload.Data.Attributes.SandboxOptIn)
		}
		if payload.Data.Attributes.Duration == nil || *payload.Data.Attributes.Duration != "SIXTEEN_DAYS" {
			t.Fatalf("expected duration SIXTEEN_DAYS, got %+v", payload.Data.Attributes.Duration)
		}
		if payload.Data.Attributes.RenewalType == nil || *payload.Data.Attributes.RenewalType != "ALL_RENEWALS" {
			t.Fatalf("expected renewalType ALL_RENEWALS, got %+v", payload.Data.Attributes.RenewalType)
		}
		assertAuthorized(t, req)
	}, response)

	optIn := true
	sandboxOptIn := false
	duration := "SIXTEEN_DAYS"
	renewalType := "ALL_RENEWALS"
	attrs := SubscriptionGracePeriodUpdateAttributes{
		OptIn:        &optIn,
		SandboxOptIn: &sandboxOptIn,
		Duration:     &duration,
		RenewalType:  &renewalType,
	}

	if _, err := client.UpdateSubscriptionGracePeriod(context.Background(), "grace-1", attrs); err != nil {
		t.Fatalf("UpdateSubscriptionGracePeriod() error: %v", err)
	}
}

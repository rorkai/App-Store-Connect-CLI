package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func stubWebSubscriptionsSession(t *testing.T) {
	t.Helper()

	t.Setenv("ASC_APP_ID", "")
	t.Cleanup(SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{Client: &http.Client{}}, "cache", nil
	}))
}

func resetWebSubscriptionAvailabilityStubs(t *testing.T) {
	t.Helper()
	origList := listWebSubscriptionPlanAvailabilitiesFn
	origRemove := removeWebSubscriptionPlanAvailabilityFromSaleFn
	t.Cleanup(func() {
		listWebSubscriptionPlanAvailabilitiesFn = origList
		removeWebSubscriptionPlanAvailabilityFromSaleFn = origRemove
	})
}

func TestWebSubscriptionsAvailabilityRemoveFromSaleCommand(t *testing.T) {
	stubWebSubscriptionsSession(t)
	resetWebSubscriptionAvailabilityStubs(t)

	listCalls := 0
	listWebSubscriptionPlanAvailabilitiesFn = func(ctx context.Context, client *webcore.Client, subscriptionID string) ([]webcore.SubscriptionPlanAvailability, error) {
		if subscriptionID != "sub-1" {
			t.Fatalf("expected subscription sub-1, got %q", subscriptionID)
		}
		synctest.Wait()
		listCalls++
		if listCalls > 1 {
			return []webcore.SubscriptionPlanAvailability{
				{ID: "plan-1", PlanType: "UPFRONT", AvailableInNewTerritories: false, AvailableTerritoriesLoaded: true},
			}, nil
		}
		return []webcore.SubscriptionPlanAvailability{
			{ID: "plan-ignored", PlanType: "OTHER", AvailableInNewTerritories: true, AvailableTerritories: []string{"USA"}, AvailableTerritoriesLoaded: true},
			{ID: "plan-1", PlanType: "UPFRONT", AvailableInNewTerritories: true, AvailableTerritories: []string{"USA"}, AvailableTerritoriesLoaded: true},
		}, nil
	}
	removeWebSubscriptionPlanAvailabilityFromSaleFn = func(ctx context.Context, client *webcore.Client, planAvailabilityID string) (*webcore.SubscriptionPlanAvailability, error) {
		synctest.Wait()
		if planAvailabilityID != "plan-1" {
			t.Fatalf("expected plan-1, got %q", planAvailabilityID)
		}
		return &webcore.SubscriptionPlanAvailability{ID: "plan-1", PlanType: "UPFRONT", AvailableInNewTerritories: false}, nil
	}

	d, depsStdout, depsStderr := newWebSpinnerTestDeps(t)
	cmd := newWebSubscriptionsAvailabilityRemoveFromSaleCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--subscription-id", "sub-1",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
		synctest.Test(t, func(t *testing.T) {
			if err := cmd.Exec(context.Background(), nil); err != nil {
				t.Fatalf("exec error: %v", err)
			}
		})
	})

	var payload webSubscriptionRemoveFromSaleResult
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode stdout: %v\nstdout=%s", err, stdout)
	}
	if payload.SubscriptionID != "sub-1" || payload.PlanAvailabilityID != "plan-1" || !payload.RemovedFromSale {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if payload.AvailableInNewTerritories {
		t.Fatalf("expected availableInNewTerritories=false: %#v", payload)
	}
	wantLabels := []string{"Loading subscription plan availability", "Removing subscription from sale", "Verifying subscription removal from sale"}
	labels := spinnerLabels(stderr)
	for i, want := range wantLabels {
		if len(labels) <= i || labels[i] != want {
			t.Fatalf("expected labels %v, got %v", wantLabels, labels)
		}
	}
}

func TestWebSubscriptionsAvailabilityRemoveFromSaleCommandUsesDirectPlanAvailabilityID(t *testing.T) {
	_ = stubWebProgressLabels(t)
	stubWebSubscriptionsSession(t)
	resetWebSubscriptionAvailabilityStubs(t)

	listCalls := 0
	listWebSubscriptionPlanAvailabilitiesFn = func(ctx context.Context, client *webcore.Client, subscriptionID string) ([]webcore.SubscriptionPlanAvailability, error) {
		listCalls++
		return []webcore.SubscriptionPlanAvailability{
			{ID: "plan-direct", PlanType: "UPFRONT", AvailableInNewTerritories: listCalls == 1, AvailableTerritoriesLoaded: true},
		}, nil
	}
	removeWebSubscriptionPlanAvailabilityFromSaleFn = func(ctx context.Context, client *webcore.Client, planAvailabilityID string) (*webcore.SubscriptionPlanAvailability, error) {
		if planAvailabilityID != "plan-direct" {
			t.Fatalf("expected plan-direct, got %q", planAvailabilityID)
		}
		return &webcore.SubscriptionPlanAvailability{ID: "plan-direct", AvailableInNewTerritories: false}, nil
	}

	d, depsStdout, depsStderr := newWebTestDeps()
	cmd := newWebSubscriptionsAvailabilityRemoveFromSaleCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--subscription-id", "sub-1",
		"--plan-availability-id", "plan-direct",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	runWithWebTestDeps(depsStdout, depsStderr, func() {
		if err := cmd.Exec(context.Background(), nil); err != nil {
			t.Fatalf("exec error: %v", err)
		}
	})
	if listCalls != 2 {
		t.Fatalf("expected direct plan availability id to verify before and after patch, got %d list calls", listCalls)
	}
}

func TestWebSubscriptionsAvailabilityRemoveFromSaleCommandRejectsMismatchedDirectPlanAvailabilityID(t *testing.T) {
	_ = stubWebProgressLabels(t)
	stubWebSubscriptionsSession(t)
	resetWebSubscriptionAvailabilityStubs(t)

	removeCalled := false
	listWebSubscriptionPlanAvailabilitiesFn = func(ctx context.Context, client *webcore.Client, subscriptionID string) ([]webcore.SubscriptionPlanAvailability, error) {
		return []webcore.SubscriptionPlanAvailability{{ID: "plan-other", PlanType: "UPFRONT", AvailableInNewTerritories: true, AvailableTerritories: []string{"USA"}, AvailableTerritoriesLoaded: true}}, nil
	}
	removeWebSubscriptionPlanAvailabilityFromSaleFn = func(ctx context.Context, client *webcore.Client, planAvailabilityID string) (*webcore.SubscriptionPlanAvailability, error) {
		removeCalled = true
		return nil, nil
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebSubscriptionsAvailabilityRemoveFromSaleCommand(d)
	if err := cmd.FlagSet.Parse([]string{
		"--subscription-id", "sub-1",
		"--plan-availability-id", "plan-direct",
		"--confirm",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), `plan availability "plan-direct" was not found for subscription "sub-1"`) {
		t.Fatalf("expected mismatched plan availability error, got %v", err)
	}
	if removeCalled {
		t.Fatal("expected mismatched direct plan availability id to stop before removal")
	}
}

func TestWebSubscriptionsAvailabilityRemoveFromSaleCommandFailsWhenReadbackStillOnSale(t *testing.T) {
	_ = stubWebProgressLabels(t)
	stubWebSubscriptionsSession(t)
	resetWebSubscriptionAvailabilityStubs(t)

	listCalls := 0
	listWebSubscriptionPlanAvailabilitiesFn = func(ctx context.Context, client *webcore.Client, subscriptionID string) ([]webcore.SubscriptionPlanAvailability, error) {
		listCalls++
		return []webcore.SubscriptionPlanAvailability{
			{ID: "plan-1", PlanType: "UPFRONT", AvailableInNewTerritories: true, AvailableTerritories: []string{"USA"}, AvailableTerritoriesLoaded: true},
		}, nil
	}
	removeWebSubscriptionPlanAvailabilityFromSaleFn = func(ctx context.Context, client *webcore.Client, planAvailabilityID string) (*webcore.SubscriptionPlanAvailability, error) {
		return &webcore.SubscriptionPlanAvailability{ID: "plan-1", PlanType: "UPFRONT", AvailableInNewTerritories: false}, nil
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebSubscriptionsAvailabilityRemoveFromSaleCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--subscription-id", "sub-1", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "is still available after patch") {
		t.Fatalf("expected verification error, got %v", err)
	}
	if listCalls != 2 {
		t.Fatalf("expected preflight and readback list calls, got %d", listCalls)
	}
}

func TestWebSubscriptionsAvailabilityRemoveFromSaleCommandRequiresFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing subscription id", args: []string{"--confirm"}, wantErr: "--subscription-id is required"},
		{name: "missing confirm", args: []string{"--subscription-id", "sub-1"}, wantErr: "--confirm is required"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			d, depsStdout, depsStderr := newWebTestDeps()
			cmd := newWebSubscriptionsAvailabilityRemoveFromSaleCommand(d)
			if err := cmd.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			_, stderr := runWithWebTestDeps(depsStdout, depsStderr, func() {
				err := cmd.Exec(context.Background(), nil)
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp, got %v", err)
				}
			})
			if !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", tt.wantErr, stderr)
			}
		})
	}
}

func TestWebSubscriptionsAvailabilityRemoveFromSaleCommandAccountHolderError(t *testing.T) {
	_ = stubWebProgressLabels(t)
	stubWebSubscriptionsSession(t)
	resetWebSubscriptionAvailabilityStubs(t)

	listWebSubscriptionPlanAvailabilitiesFn = func(ctx context.Context, client *webcore.Client, subscriptionID string) ([]webcore.SubscriptionPlanAvailability, error) {
		return []webcore.SubscriptionPlanAvailability{{ID: "plan-1", PlanType: "UPFRONT"}}, nil
	}
	removeWebSubscriptionPlanAvailabilityFromSaleFn = func(ctx context.Context, client *webcore.Client, planAvailabilityID string) (*webcore.SubscriptionPlanAvailability, error) {
		return nil, &webcore.APIError{Status: http.StatusForbidden}
	}

	d, _, _ := newWebTestDeps()
	cmd := newWebSubscriptionsAvailabilityRemoveFromSaleCommand(d)
	if err := cmd.FlagSet.Parse([]string{"--subscription-id", "sub-1", "--confirm"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected Account Holder error")
	}
	if !strings.Contains(err.Error(), "requires the App Store Connect Account Holder role") {
		t.Fatalf("expected Account Holder guidance, got %v", err)
	}
	if strings.Contains(err.Error(), "unauthorized or expired") {
		t.Fatalf("expected specific Account Holder guidance, got %v", err)
	}
}

func TestSelectSubscriptionPlanAvailabilityAmbiguous(t *testing.T) {
	_, err := selectSubscriptionPlanAvailability([]webcore.SubscriptionPlanAvailability{
		{ID: "plan-1", PlanType: "ONE"},
		{ID: "plan-2", PlanType: "TWO"},
	})
	if err == nil || !strings.Contains(err.Error(), "multiple subscription plan availabilities matched") {
		t.Fatalf("expected ambiguous plan availability error, got %v", err)
	}
}

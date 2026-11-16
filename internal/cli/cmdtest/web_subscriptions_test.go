package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	webcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestWebSubscriptionsAvailabilityRemoveFromSaleRunWithAppSelector(t *testing.T) {
	availabilityListCalls := 0
	patchCalls := 0
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return webSubscriptionsAvailabilityResponse(t, req, &availabilityListCalls, &patchCalls)
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	stdout, stderr, code := runWithDeps(t, nil,
		"--profile", "test-web",
		"web", "subscriptions", "availability", "remove-from-sale",
		"--output", "json",
		"--app", "app-1",
		"--subscription-id", "availability",
		"--confirm",
	)
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		SubscriptionID            string   `json:"subscriptionId"`
		PlanAvailabilityID        string   `json:"planAvailabilityId"`
		RemovedFromSale           bool     `json:"removedFromSale"`
		AvailableInNewTerritories bool     `json:"availableInNewTerritories"`
		AvailableTerritories      []string `json:"availableTerritories"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.SubscriptionID != "sub-1" || payload.PlanAvailabilityID != "plan-1" || !payload.RemovedFromSale {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.AvailableInNewTerritories || len(payload.AvailableTerritories) != 0 {
		t.Fatalf("expected subscription to be removed from sale, got %+v", payload)
	}
	if availabilityListCalls != 2 {
		t.Fatalf("expected pre-patch and post-patch availability reads, got %d", availabilityListCalls)
	}
	if patchCalls != 1 {
		t.Fatalf("expected one remove-from-sale patch, got %d", patchCalls)
	}
}

func TestWebSubscriptionsAvailabilityRemoveFromSaleRunRejectsUnownedPlanAvailabilityID(t *testing.T) {
	availabilityListCalls := 0
	patchCalls := 0
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return webSubscriptionsAvailabilityResponse(t, req, &availabilityListCalls, &patchCalls)
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	stdout, stderr, code := runWithDeps(t, nil,
		"--profile", "test-web",
		"web", "subscriptions", "availability", "remove-from-sale",
		"--app", "app-1",
		"--subscription-id", "availability",
		"--plan-availability-id", "plan-other",
		"--confirm",
	)
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `plan availability "plan-other" was not found for subscription "sub-1"`) {
		t.Fatalf("expected plan ownership error, got %q", stderr)
	}
	if availabilityListCalls != 1 {
		t.Fatalf("expected one availability read before rejection, got %d", availabilityListCalls)
	}
	if patchCalls != 0 {
		t.Fatalf("expected no patch for unowned plan availability, got %d", patchCalls)
	}
}

func TestWebSubscriptionsAvailabilityRemoveFromSaleRunUsesOwnedPlanAvailabilityID(t *testing.T) {
	availabilityListCalls := 0
	patchCalls := 0
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return webSubscriptionsAvailabilityResponse(t, req, &availabilityListCalls, &patchCalls)
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	stdout, stderr, code := runWithDeps(t, nil,
		"--profile", "test-web",
		"web", "subscriptions", "availability", "remove-from-sale",
		"--output", "json",
		"--app", "app-1",
		"--subscription-id", "availability",
		"--plan-availability-id", "plan-1",
		"--confirm",
	)
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		PlanAvailabilityID string `json:"planAvailabilityId"`
		RemovedFromSale    bool   `json:"removedFromSale"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.PlanAvailabilityID != "plan-1" || !payload.RemovedFromSale {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if availabilityListCalls != 2 {
		t.Fatalf("expected ownership and readback availability reads, got %d", availabilityListCalls)
	}
	if patchCalls != 1 {
		t.Fatalf("expected one remove-from-sale patch, got %d", patchCalls)
	}
}

func TestWebSubscriptionsAvailabilityRemoveFromSaleRunFailsWhenReadbackStillOnSale(t *testing.T) {
	availabilityListCalls := 0
	patchCalls := 0
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return webSubscriptionsAvailabilityResponse(t, req, &availabilityListCalls, &patchCalls, false)
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	stdout, stderr, code := runWithDeps(t, nil,
		"--profile", "test-web",
		"web", "subscriptions", "availability", "remove-from-sale",
		"--output", "json",
		"--app", "app-1",
		"--subscription-id", "availability",
		"--confirm",
	)
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `plan availability "plan-1" is still available after patch`) {
		t.Fatalf("expected readback verification error, got %q", stderr)
	}
	if availabilityListCalls != 2 {
		t.Fatalf("expected pre-patch and post-patch availability reads, got %d", availabilityListCalls)
	}
	if patchCalls != 1 {
		t.Fatalf("expected one remove-from-sale patch before verification failed, got %d", patchCalls)
	}
}

func TestWebSubscriptionsAvailabilityRemoveFromSaleRunUsageErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "missing subscription id",
			args: []string{
				"web", "subscriptions", "availability", "remove-from-sale",
				"--confirm",
			},
			wantErr: "--subscription-id is required",
		},
		{
			name: "missing confirm",
			args: []string{
				"web", "subscriptions", "availability", "remove-from-sale",
				"--subscription-id", "sub-1",
			},
			wantErr: "--confirm is required",
		},
		{
			name: "invalid output",
			args: []string{
				"web", "subscriptions", "availability", "remove-from-sale",
				"--subscription-id", "sub-1",
				"--confirm",
				"--output", "yaml",
			},
			wantErr: `(got "yaml")`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, stderr := captureOutput(t, func() {
				code := cmd.Run(test.args, "1.0.0")
				if code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
				}
			})
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestWebSubscriptionsPricingMonthlyCommitmentBootstrapRunCreatesAvailabilityAndPrices(t *testing.T) {
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: monthlyCommitmentBootstrapTransport(t, monthlyCommitmentBootstrapHTTPOptions{})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	stdout, stderr, code := runWithDeps(t, nil,
		"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
		"--subscription-id", "sub-1",
		"--territory", "NOR",
		"--upfront-price-point-id", "upfront-point",
		"--monthly-price-point-id", "monthly-point",
		"--confirm",
		"--output", "json",
	)
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	payload := decodeMonthlyCommitmentBootstrapReceipt(t, stdout)
	if !payload.PlanAvailabilityCreated || !payload.PricesCreated || !payload.Verified || payload.CompletedStage != "verified" {
		t.Fatalf("unexpected payload: %+v stdout=%q", payload, stdout)
	}
}

func TestWebSubscriptionsPricingMonthlyCommitmentBootstrapExitsWhenReadbackPricesAreStale(t *testing.T) {
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: monthlyCommitmentBootstrapTransport(t, monthlyCommitmentBootstrapHTTPOptions{
				pricesBody: monthlyCommitmentPairedPricesJSON("stale-upfront", "stale-monthly", ""),
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	stdout, _, code := runWithDeps(t, nil,
		"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
		"--subscription-id", "sub-1",
		"--territory", "NOR",
		"--upfront-price-point-id", "upfront-point",
		"--monthly-price-point-id", "monthly-point",
		"--confirm",
		"--output", "json",
	)
	if code == cmd.ExitSuccess {
		t.Fatal("expected non-zero exit when readback prices do not match")
	}
	payload := decodeMonthlyCommitmentBootstrapReceipt(t, stdout)
	if payload.Verified || payload.CompletedStage != "prices" {
		t.Fatalf("expected unverified prices stage, got %+v stdout=%q", payload, stdout)
	}
	if !payload.PlanAvailabilityCreated || !payload.PricesCreated {
		t.Fatalf("stale readback should still report applied mutations: %+v", payload)
	}
}

func TestWebSubscriptionsPricingMonthlyCommitmentBootstrapReportsPlanAvailabilityStageWhenPricesPatchFails(t *testing.T) {
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: monthlyCommitmentBootstrapTransport(t, monthlyCommitmentBootstrapHTTPOptions{
				patchStatus: http.StatusUnprocessableEntity,
				patchBody:   `{"errors":[{"code":"ENTITY_ERROR","detail":"invalid prices"}]}`,
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	stdout, _, code := runWithDeps(t, nil,
		"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
		"--subscription-id", "sub-1",
		"--territory", "NOR",
		"--upfront-price-point-id", "upfront-point",
		"--monthly-price-point-id", "monthly-point",
		"--confirm",
		"--output", "json",
	)
	if code == cmd.ExitSuccess {
		t.Fatal("expected non-zero exit when paired price creation fails")
	}
	payload := decodeMonthlyCommitmentBootstrapReceipt(t, stdout)
	if payload.Verified || payload.CompletedStage != "plan_availability" {
		t.Fatalf("expected completed plan_availability stage, got %+v stdout=%q", payload, stdout)
	}
	if !payload.PlanAvailabilityCreated || payload.PricesCreated {
		t.Fatalf("price PATCH failure should leave pricesCreated false: %+v", payload)
	}
	if !strings.Contains(payload.Failure, "paired price creation failed") {
		t.Fatalf("expected failure to name the price stage, got %+v", payload)
	}
}

func TestWebSubscriptionsPricingMonthlyCommitmentBootstrapExitsWhenReadbackPricesAreMissing(t *testing.T) {
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: monthlyCommitmentBootstrapTransport(t, monthlyCommitmentBootstrapHTTPOptions{
				pricesBody: `{"data":[]}`,
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	stdout, _, code := runWithDeps(t, nil,
		"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
		"--subscription-id", "sub-1",
		"--territory", "NOR",
		"--upfront-price-point-id", "upfront-point",
		"--monthly-price-point-id", "monthly-point",
		"--confirm",
		"--output", "json",
	)
	if code == cmd.ExitSuccess {
		t.Fatal("expected non-zero exit when readback prices are missing")
	}
	payload := decodeMonthlyCommitmentBootstrapReceipt(t, stdout)
	if payload.Verified || payload.CompletedStage != "prices" || !payload.PricesCreated {
		t.Fatalf("expected unverified prices stage, got %+v stdout=%q", payload, stdout)
	}
}

func TestWebSubscriptionsPricingMonthlyCommitmentBootstrapVerifiesScheduledStartDate(t *testing.T) {
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: monthlyCommitmentBootstrapTransport(t, monthlyCommitmentBootstrapHTTPOptions{
				pricesBody: monthlyCommitmentPairedPricesJSON("upfront-point", "monthly-point", "2026-07-01"),
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	stdout, stderr, code := runWithDeps(t, nil,
		"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
		"--subscription-id", "sub-1",
		"--territory", "NOR",
		"--upfront-price-point-id", "upfront-point",
		"--monthly-price-point-id", "monthly-point",
		"--start-date", "2026-07-01",
		"--confirm",
		"--output", "json",
	)
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	payload := decodeMonthlyCommitmentBootstrapReceipt(t, stdout)
	if !payload.Verified || payload.CompletedStage != "verified" {
		t.Fatalf("expected verified scheduled bootstrap, got %+v stdout=%q", payload, stdout)
	}
}

func TestWebSubscriptionsPricingMonthlyCommitmentBootstrapDryRunReportsPreviewWithoutCreation(t *testing.T) {
	requests := 0
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != http.MethodGet || req.URL.Path != "/iris/v1/subscriptions/sub-1/planAvailabilities" {
					t.Fatalf("unexpected dry-run request: %s %s", req.Method, req.URL.Path)
				}
				return webSubscriptionsJSONResponse(`{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT"},"relationships":{"availableTerritories":{"data":[{"type":"territories","id":"NOR"}]}}}]}`), nil
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	stdout, stderr, code := runWithDeps(t, nil,
		"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
		"--subscription-id", "sub-1",
		"--territory", "NOR",
		"--upfront-price-point-id", "upfront-point",
		"--monthly-price-point-id", "monthly-point",
		"--dry-run",
		"--output", "json",
	)
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var payload struct {
		PlanAvailabilityCreated     bool `json:"planAvailabilityCreated"`
		PlanAvailabilityWouldCreate bool `json:"planAvailabilityWouldCreate"`
		PricesCreated               bool `json:"pricesCreated"`
		DryRun                      bool `json:"dryRun"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.PlanAvailabilityCreated || !payload.PlanAvailabilityWouldCreate || payload.PricesCreated || !payload.DryRun {
		t.Fatalf("unexpected dry-run payload: %+v", payload)
	}
	if requests != 1 {
		t.Fatalf("expected one read and no mutations, got %d requests", requests)
	}
}

func TestWebSubscriptionsPricingMonthlyCommitmentBootstrapUsageExitCodes(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "missing subscription id",
			args: []string{
				"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
			},
			wantErr: "--subscription-id is required",
		},
		{
			name: "missing confirm",
			args: []string{
				"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
				"--subscription-id", "sub-1",
				"--territory", "NOR",
				"--upfront-price-point-id", "upfront",
				"--monthly-price-point-id", "monthly",
			},
			wantErr: "--confirm is required",
		},
		{
			name: "preserve requires start date",
			args: []string{
				"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
				"--subscription-id", "sub-1",
				"--territory", "NOR",
				"--upfront-price-point-id", "upfront",
				"--monthly-price-point-id", "monthly",
				"--preserve-current-price",
				"--confirm",
			},
			wantErr: "--preserve-current-price requires --start-date",
		},
		{
			name: "rejects excluded territory",
			args: []string{
				"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
				"--subscription-id", "sub-1",
				"--territory", "USA",
				"--upfront-price-point-id", "upfront",
				"--monthly-price-point-id", "monthly",
				"--confirm",
			},
			wantErr: "--territory cannot be USA for monthly-commitment pricing",
		},
		{
			name: "accepts Singapore",
			args: []string{
				"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
				"--subscription-id", "sub-1",
				"--territory", "SGP",
				"--upfront-price-point-id", "upfront",
				"--monthly-price-point-id", "monthly",
			},
			wantErr: "--confirm is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestWebSubscriptionsPricingAdjustedEqualizationsViewRun(t *testing.T) {
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/iris/v1/subscriptionPricePoints/monthly-point/adjustedEqualizations" {
					t.Fatalf("unexpected adjusted equalizations request: %s %s", req.Method, req.URL.String())
				}
				if got := req.URL.Query().Get("filter[planType]"); got != "MONTHLY" {
					t.Fatalf("expected MONTHLY plan type, got %q", got)
				}
				return &http.Response{
					StatusCode: http.StatusConflict,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body: io.NopCloser(strings.NewReader(
						`{"errors":[{"code":"STATE_ERROR.EQUALIZATION_FAILED","detail":"No compatible price point","meta":{"associatedErrors":{"prices":[{"code":"STATE_ERROR.NO_TIER_IN_TERRITORY","detail":"DEU"}]}}}]}`,
					)),
				}, nil
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	stdout, stderr, code := runWithDeps(t, nil,
		"web", "subscriptions", "pricing", "adjusted-equalizations", "view",
		"--price-point-id", "monthly-point",
		"--plan-type", "MONTHLY",
		"--output", "json",
	)
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var payload struct {
		PlanType              string   `json:"planType"`
		Status                int      `json:"status"`
		Available             bool     `json:"available"`
		Code                  string   `json:"code"`
		MissingTerritoryCount int      `json:"missingTerritoryCount"`
		MissingTerritories    []string `json:"missingTerritories"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if payload.PlanType != "MONTHLY" || payload.Status != http.StatusConflict || payload.Available {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Code != "STATE_ERROR.EQUALIZATION_FAILED" || payload.MissingTerritoryCount != 1 ||
		len(payload.MissingTerritories) != 1 || payload.MissingTerritories[0] != "DEU" {
		t.Fatalf("unexpected equalization failure details: %+v", payload)
	}
}

func TestWebSubscriptionsPricingAdjustedEqualizationsRejectsUpfrontPlanType(t *testing.T) {
	assertUsageExit(t, []string{
		"web", "subscriptions", "pricing", "adjusted-equalizations", "view",
		"--price-point-id", "upfront-point",
		"--plan-type", "UPFRONT",
	}, `--plan-type only supports "MONTHLY"`)
}

func webSubscriptionsAvailabilityResponse(t *testing.T, req *http.Request, availabilityListCalls *int, patchCalls *int, postPatchRemoved ...bool) (*http.Response, error) {
	t.Helper()

	shouldReturnRemovedAfterPatch := true
	if len(postPatchRemoved) > 0 {
		shouldReturnRemovedAfterPatch = postPatchRemoved[0]
	}

	switch {
	case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/app-1/subscriptionGroups":
		if req.URL.Query().Get("include") != "subscriptions" {
			t.Fatalf("expected subscriptions include, got %q", req.URL.RawQuery)
		}
		return webSubscriptionsJSONResponse(`{
			"data": [{
				"id": "group-1",
				"type": "subscriptionGroups",
				"attributes": {"referenceName": "Premium"},
				"relationships": {
					"subscriptions": {
						"data": [{"type": "subscriptions", "id": "sub-1"}]
					}
				}
			}],
			"included": [{
				"id": "sub-1",
				"type": "subscriptions",
				"attributes": {
					"productId": "availability",
					"name": "Monthly",
					"state": "APPROVED"
				}
			}]
		}`), nil
	case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/subscriptions/sub-1/planAvailabilities":
		*availabilityListCalls++
		if *availabilityListCalls == 1 || !shouldReturnRemovedAfterPatch {
			return webSubscriptionsJSONResponse(`{
				"data": [{
					"id": "plan-1",
					"type": "subscriptionPlanAvailabilities",
					"attributes": {
						"availableInNewTerritories": true,
						"planType": "UPFRONT"
					},
					"relationships": {
						"availableTerritories": {"data": [{"type": "territories", "id": "USA"}]}
					}
				}]
			}`), nil
		}
		return webSubscriptionsJSONResponse(`{
			"data": [{
				"id": "plan-1",
				"type": "subscriptionPlanAvailabilities",
				"attributes": {
					"availableInNewTerritories": false,
					"planType": "UPFRONT"
				},
				"relationships": {
					"availableTerritories": {"data": []}
				}
			}]
		}`), nil
	case req.Method == http.MethodPatch && req.URL.Path == "/iris/v1/subscriptionPlanAvailabilities/plan-1":
		*patchCalls++
		rawBody, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var payload struct {
			Data struct {
				Attributes struct {
					AvailableInNewTerritories bool `json:"availableInNewTerritories"`
				} `json:"attributes"`
				Relationships struct {
					AvailableTerritories struct {
						Data []any `json:"data"`
					} `json:"availableTerritories"`
				} `json:"relationships"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			t.Fatalf("decode request body: %v\nbody=%s", err, string(rawBody))
		}
		if payload.Data.Attributes.AvailableInNewTerritories {
			t.Fatal("expected availableInNewTerritories=false")
		}
		if len(payload.Data.Relationships.AvailableTerritories.Data) != 0 {
			t.Fatalf("expected availableTerritories.data to be empty, got %#v", payload.Data.Relationships.AvailableTerritories.Data)
		}
		return webSubscriptionsJSONResponse(`{
			"data": {
				"id": "plan-1",
				"type": "subscriptionPlanAvailabilities",
				"attributes": {
					"availableInNewTerritories": false,
					"planType": "UPFRONT"
				}
			}
		}`), nil
	default:
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	}
}

func webSubscriptionsJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

type monthlyCommitmentBootstrapHTTPOptions struct {
	patchStatus int
	patchBody   string
	pricesBody  string
}

type monthlyCommitmentBootstrapReceipt struct {
	PlanAvailabilityCreated bool   `json:"planAvailabilityCreated"`
	PricesCreated           bool   `json:"pricesCreated"`
	Verified                bool   `json:"verified"`
	CompletedStage          string `json:"completedStage"`
	Failure                 string `json:"failure"`
}

func decodeMonthlyCommitmentBootstrapReceipt(t *testing.T, stdout string) monthlyCommitmentBootstrapReceipt {
	t.Helper()
	var payload monthlyCommitmentBootstrapReceipt
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	return payload
}

func monthlyCommitmentPairedPricesJSON(upfrontPoint, monthlyPoint, startDate string) string {
	startAttr := ""
	if startDate != "" {
		startAttr = `,"startDate":"` + startDate + `"`
	}
	return `{
		"data": [
			{
				"type": "subscriptionPrices",
				"id": "price-upfront",
				"attributes": {"planType": "UPFRONT"` + startAttr + `},
				"relationships": {
					"territory": {"data": {"type": "territories", "id": "NOR"}},
					"subscriptionPricePoint": {"data": {"type": "subscriptionPricePoints", "id": "` + upfrontPoint + `"}}
				}
			},
			{
				"type": "subscriptionPrices",
				"id": "price-monthly",
				"attributes": {"planType": "MONTHLY"` + startAttr + `},
				"relationships": {
					"territory": {"data": {"type": "territories", "id": "NOR"}},
					"subscriptionPricePoint": {"data": {"type": "subscriptionPricePoints", "id": "` + monthlyPoint + `"}}
				}
			}
		]
	}`
}

func monthlyCommitmentBootstrapTransport(t *testing.T, opts monthlyCommitmentBootstrapHTTPOptions) http.RoundTripper {
	t.Helper()
	availabilityLists := 0
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/subscriptions/sub-1/planAvailabilities":
			availabilityLists++
			if availabilityLists == 1 {
				return webSubscriptionsJSONResponse(`{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT"},"relationships":{"availableTerritories":{"data":[{"type":"territories","id":"NOR"}]}}}]}`), nil
			}
			return webSubscriptionsJSONResponse(`{"data":[
				{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT"},"relationships":{"availableTerritories":{"data":[{"type":"territories","id":"NOR"}]}}},
				{"type":"subscriptionPlanAvailabilities","id":"plan-monthly","attributes":{"planType":"MONTHLY"},"relationships":{"availableTerritories":{"data":[{"type":"territories","id":"NOR"}]}}}
			]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/subscriptionPlanAvailabilities":
			return webSubscriptionsJSONResponse(`{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-monthly","attributes":{"planType":"MONTHLY"},"relationships":{"availableTerritories":{"data":[{"type":"territories","id":"NOR"}]}}}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/iris/v1/subscriptions/sub-1":
			status := opts.patchStatus
			if status == 0 {
				status = http.StatusOK
			}
			body := opts.patchBody
			if body == "" {
				body = `{"data":{"type":"subscriptions","id":"sub-1"}}`
			}
			return &http.Response{
				StatusCode: status,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/subscriptions/sub-1/prices":
			body := opts.pricesBody
			if body == "" {
				body = monthlyCommitmentPairedPricesJSON("upfront-point", "monthly-point", "")
			}
			return webSubscriptionsJSONResponse(body), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
}

func TestWebSubscriptionsPricingMonthlyCommitmentBootstrapPointsAtPlanAvailabilitySet(t *testing.T) {
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method == http.MethodGet && req.URL.Path == "/iris/v1/subscriptions/sub-1/planAvailabilities" {
					return webSubscriptionsJSONResponse(`{"data":[
						{"type":"subscriptionPlanAvailabilities","id":"plan-monthly","attributes":{"planType":"MONTHLY"},"relationships":{"availableTerritories":{"data":[{"type":"territories","id":"DEU"}]}}}
					]}`), nil
				}
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				return nil, nil
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	stdout, stderr, code := runWithDeps(t, nil,
		"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
		"--subscription-id", "sub-1",
		"--territory", "NOR",
		"--upfront-price-point-id", "upfront-point",
		"--monthly-price-point-id", "monthly-point",
		"--confirm",
		"--output", "json",
	)
	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "asc subscriptions pricing plan-availability set --subscription-id sub-1 --plan-type MONTHLY") {
		t.Fatalf("expected the error to point at plan-availability set, got %q", stderr)
	}
}

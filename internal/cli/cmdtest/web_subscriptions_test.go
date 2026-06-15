package cmdtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"
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

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"--profile", "test-web",
			"web", "subscriptions", "availability", "remove-from-sale",
			"--output", "json",
			"--app", "app-1",
			"--subscription-id", "availability",
			"--confirm",
		}, "1.0.0")
		if code != cmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
		}
	})
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

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"--profile", "test-web",
			"web", "subscriptions", "availability", "remove-from-sale",
			"--app", "app-1",
			"--subscription-id", "availability",
			"--plan-availability-id", "plan-other",
			"--confirm",
		}, "1.0.0")
		if code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
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

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"--profile", "test-web",
			"web", "subscriptions", "availability", "remove-from-sale",
			"--output", "json",
			"--app", "app-1",
			"--subscription-id", "availability",
			"--plan-availability-id", "plan-1",
			"--confirm",
		}, "1.0.0")
		if code != cmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
		}
	})
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

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"--profile", "test-web",
			"web", "subscriptions", "availability", "remove-from-sale",
			"--output", "json",
			"--app", "app-1",
			"--subscription-id", "availability",
			"--confirm",
		}, "1.0.0")
		if code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
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
			wantErr: "unsupported format: yaml",
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
		requests := 0
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				switch requests {
				case 1:
					return webSubscriptionsJSONResponse(`{"data":[{"type":"subscriptionPlanAvailabilities","id":"plan-upfront","attributes":{"planType":"UPFRONT"},"relationships":{"availableTerritories":{"data":[{"type":"territories","id":"NOR"}]}}}]}`), nil
				case 2:
					if req.Method != http.MethodPost || req.URL.Path != "/iris/v1/subscriptionPlanAvailabilities" {
						t.Fatalf("unexpected availability request: %s %s", req.Method, req.URL.Path)
					}
					return webSubscriptionsJSONResponse(`{"data":{"type":"subscriptionPlanAvailabilities","id":"plan-monthly","attributes":{"planType":"MONTHLY"},"relationships":{"availableTerritories":{"data":[{"type":"territories","id":"NOR"}]}}}}`), nil
				case 3:
					if req.Method != http.MethodPatch || req.URL.Path != "/iris/v1/subscriptions/sub-1" {
						t.Fatalf("unexpected pricing request: %s %s", req.Method, req.URL.Path)
					}
					return webSubscriptionsJSONResponse(`{"data":{"type":"subscriptions","id":"sub-1"}}`), nil
				default:
					t.Fatalf("unexpected request %d: %s %s", requests, req.Method, req.URL.Path)
					return nil, nil
				}
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
			"--subscription-id", "sub-1",
			"--territory", "NOR",
			"--upfront-price-point-id", "upfront-point",
			"--monthly-price-point-id", "monthly-point",
			"--confirm",
			"--output", "json",
		}, "1.0.0")
		if code != cmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var payload struct {
		PlanAvailabilityCreated bool `json:"planAvailabilityCreated"`
		PricesCreated           bool `json:"pricesCreated"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if !payload.PlanAvailabilityCreated || !payload.PricesCreated {
		t.Fatalf("unexpected payload: %+v", payload)
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

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
			"--subscription-id", "sub-1",
			"--territory", "NOR",
			"--upfront-price-point-id", "upfront-point",
			"--monthly-price-point-id", "monthly-point",
			"--dry-run",
			"--output", "json",
		}, "1.0.0")
		if code != cmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
		}
	})
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

func TestWebSubscriptionsPricingMonthlyCommitmentBootstrapRunUsageErrors(t *testing.T) {
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
			name: "rejects United States",
			args: []string{
				"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
				"--subscription-id", "sub-1",
				"--territory", "USA",
				"--upfront-price-point-id", "upfront",
				"--monthly-price-point-id", "monthly",
				"--confirm",
			},
			wantErr: "--territory cannot be USA or Singapore for monthly-commitment pricing",
		},
		{
			name: "rejects Singapore",
			args: []string{
				"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
				"--subscription-id", "sub-1",
				"--territory", "SGP",
				"--upfront-price-point-id", "upfront",
				"--monthly-price-point-id", "monthly",
				"--confirm",
			},
			wantErr: "--territory cannot be USA or Singapore for monthly-commitment pricing",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				code := cmd.Run(test.args, "1.0.0")
				if code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestWebSubscriptionsPricingMonthlyCommitmentBootstrapUsageExitCodes(t *testing.T) {
	binaryPath := buildASCBlackBoxBinary(t)

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
			wantErr: "--territory cannot be USA or Singapore for monthly-commitment pricing",
		},
		{
			name: "rejects Singapore",
			args: []string{
				"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap",
				"--subscription-id", "sub-1",
				"--territory", "SGP",
				"--upfront-price-point-id", "upfront",
				"--monthly-price-point-id", "monthly",
				"--confirm",
			},
			wantErr: "--territory cannot be USA or Singapore for monthly-commitment pricing",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, test.args...)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("expected process exit error, got %v", err)
			}
			if exitErr.ExitCode() != 2 {
				t.Fatalf("expected exit code 2, got %d", exitErr.ExitCode())
			}
			if stdout.String() != "" {
				t.Fatalf("expected empty stdout, got %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr.String())
			}
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

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"web", "subscriptions", "pricing", "adjusted-equalizations", "view",
			"--price-point-id", "monthly-point",
			"--plan-type", "MONTHLY",
			"--output", "json",
		}, "1.0.0")
		if code != cmd.ExitSuccess {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
		}
	})
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
	binaryPath := buildASCBlackBoxBinary(t)
	command := exec.Command(
		binaryPath,
		"web", "subscriptions", "pricing", "adjusted-equalizations", "view",
		"--price-point-id", "upfront-point",
		"--plan-type", "UPFRONT",
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected process exit error, got %v", err)
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.ExitCode())
	}
	if stdout.String() != "" {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), `--plan-type only supports "MONTHLY"`) {
		t.Fatalf("expected MONTHLY-only error, got %q", stderr.String())
	}
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

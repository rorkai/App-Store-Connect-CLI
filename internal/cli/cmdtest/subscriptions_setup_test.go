package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type subscriptionsSetupOutput struct {
	Status               string `json:"status"`
	GroupID              string `json:"groupId,omitempty"`
	GroupLocalizationID  string `json:"groupLocalizationId,omitempty"`
	SubscriptionID       string `json:"subscriptionId,omitempty"`
	LocalizationID       string `json:"localizationId,omitempty"`
	ReviewScreenshotID   string `json:"reviewScreenshotId,omitempty"`
	AvailabilityID       string `json:"availabilityId,omitempty"`
	ResolvedPricePointID string `json:"resolvedPricePointId,omitempty"`
	Error                string `json:"error,omitempty"`
	FailedStep           string `json:"failedStep,omitempty"`
	Verification         struct {
		Status                  string   `json:"status"`
		SubscriptionState       string   `json:"subscriptionState,omitempty"`
		GroupExists             *bool    `json:"groupExists,omitempty"`
		SubscriptionExists      bool     `json:"subscriptionExists,omitempty"`
		LocalizationExists      *bool    `json:"localizationExists,omitempty"`
		PriceVerified           *bool    `json:"priceVerified,omitempty"`
		AvailabilityVerified    *bool    `json:"availabilityVerified,omitempty"`
		PriceCoverageVerified   *bool    `json:"priceCoverageVerified,omitempty"`
		PricedTerritories       []string `json:"pricedTerritories,omitempty"`
		MissingPriceTerritories []string `json:"missingPriceTerritories,omitempty"`
		PriceTerritory          string   `json:"priceTerritory,omitempty"`
		CurrentPrice            *struct {
			Amount   string `json:"amount"`
			Currency string `json:"currency"`
		} `json:"currentPrice,omitempty"`
	} `json:"verification,omitempty"`
	Steps []struct {
		Name    string `json:"name"`
		Status  string `json:"status"`
		Message string `json:"message,omitempty"`
	} `json:"steps"`
}

func TestSubscriptionsHelpShowsSetupCommand(t *testing.T) {
	root := RootCommand("1.2.3")

	subscriptionsCmd := findSubcommand(root, "subscriptions")
	if subscriptionsCmd == nil {
		t.Fatal("expected subscriptions command")
		return
	}
	subscriptionsUsage := subscriptionsCmd.UsageFunc(subscriptionsCmd)
	if !usageListsSubcommand(subscriptionsUsage, "setup") {
		t.Fatalf("expected subscriptions help to list setup, got %q", subscriptionsUsage)
	}

	setupCmd := findSubcommand(root, "subscriptions", "setup")
	if setupCmd == nil {
		t.Fatal("expected subscriptions setup command")
		return
	}
	setupUsage := setupCmd.UsageFunc(setupCmd)
	if !strings.Contains(setupUsage, "--group-reference-name") {
		t.Fatalf("expected subscriptions setup help to show --group-reference-name, got %q", setupUsage)
	}
	if !strings.Contains(setupUsage, "--display-name") {
		t.Fatalf("expected subscriptions setup help to show --display-name, got %q", setupUsage)
	}
	if !strings.Contains(setupUsage, "--price-territory") {
		t.Fatalf("expected subscriptions setup help to show --price-territory, got %q", setupUsage)
	}
	if !strings.Contains(setupUsage, "--no-verify") {
		t.Fatalf("expected subscriptions setup help to show --no-verify, got %q", setupUsage)
	}
	if !strings.Contains(setupUsage, "--enable-monthly-commitment") {
		t.Fatalf("expected subscriptions setup help to show --enable-monthly-commitment, got %q", setupUsage)
	}
	for _, flagName := range []string{
		"--group-locale",
		"--group-display-name",
		"--group-custom-app-name",
		"--review-screenshot",
		"--repair",
	} {
		if !strings.Contains(setupUsage, flagName) {
			t.Fatalf("expected subscriptions setup help to show %s, got %q", flagName, setupUsage)
		}
	}
}

func TestSubscriptionsSetupValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "missing group target",
			args: []string{
				"subscriptions", "setup",
				"--reference-name", "Pro Monthly",
				"--product-id", "com.example.pro.monthly",
			},
			wantErr: "one of --group-id or --group-reference-name is required",
		},
		{
			name: "group-id and group-reference-name mutually exclusive",
			args: []string{
				"subscriptions", "setup",
				"--app", "APP_ID",
				"--group-id", "GROUP_ID",
				"--group-reference-name", "Pro",
				"--reference-name", "Pro Monthly",
				"--product-id", "com.example.pro.monthly",
			},
			wantErr: "--group-id and --group-reference-name are mutually exclusive",
		},
		{
			name: "missing app when creating group",
			args: []string{
				"subscriptions", "setup",
				"--group-reference-name", "Pro",
				"--reference-name", "Pro Monthly",
				"--product-id", "com.example.pro.monthly",
			},
			wantErr: "--app is required when creating a new group",
		},
		{
			name: "missing display name when localization requested",
			args: []string{
				"subscriptions", "setup",
				"--group-id", "GROUP_ID",
				"--reference-name", "Pro Monthly",
				"--product-id", "com.example.pro.monthly",
				"--locale", "en-US",
			},
			wantErr: "--display-name is required when localization flags are provided",
		},
		{
			name: "missing locale when localization requested",
			args: []string{
				"subscriptions", "setup",
				"--group-id", "GROUP_ID",
				"--reference-name", "Pro Monthly",
				"--product-id", "com.example.pro.monthly",
				"--display-name", "Pro Monthly",
			},
			wantErr: "--locale is required when localization flags are provided",
		},
		{
			name: "missing price territory when pricing requested",
			args: []string{
				"subscriptions", "setup",
				"--group-id", "GROUP_ID",
				"--reference-name", "Pro Monthly",
				"--product-id", "com.example.pro.monthly",
				"--price", "3.99",
			},
			wantErr: "--price-territory is required when pricing flags are provided",
		},
		{
			name: "missing pricing selector when pricing flags requested",
			args: []string{
				"subscriptions", "setup",
				"--group-id", "GROUP_ID",
				"--reference-name", "Pro Monthly",
				"--product-id", "com.example.pro.monthly",
				"--price-territory", "USA",
			},
			wantErr: "one of --price-point-id, --tier, or --price is required when pricing flags are provided",
		},
		{
			name: "missing group display name when group localization requested",
			args: []string{
				"subscriptions", "setup",
				"--group-id", "GROUP_ID",
				"--reference-name", "Pro Monthly",
				"--product-id", "com.example.pro.monthly",
				"--group-locale", "en-US",
			},
			wantErr: "--group-display-name is required when group localization flags are provided",
		},
		{
			name: "repair requires pricing",
			args: []string{
				"subscriptions", "setup",
				"--group-id", "GROUP_ID",
				"--reference-name", "Pro Monthly",
				"--product-id", "com.example.pro.monthly",
				"--repair",
			},
			wantErr: "--repair requires pricing flags",
		},
		{
			name: "pricing selectors are mutually exclusive",
			args: []string{
				"subscriptions", "setup",
				"--group-id", "GROUP_ID",
				"--reference-name", "Pro Monthly",
				"--product-id", "com.example.pro.monthly",
				"--price-territory", "USA",
				"--price", "3.99",
				"--price-point-id", "pp-1",
			},
			wantErr: "--price-point-id, --tier, and --price are mutually exclusive",
		},
		{
			name: "availability flag requires territories",
			args: []string{
				"subscriptions", "setup",
				"--group-id", "GROUP_ID",
				"--reference-name", "Pro Monthly",
				"--product-id", "com.example.pro.monthly",
				"--available-in-new-territories",
			},
			wantErr: "--territories is required when availability flags are provided unless --price-territory can be used to derive availability",
		},
		{
			name: "monthly commitment requires one year",
			args: []string{
				"subscriptions", "setup",
				"--group-id", "GROUP_ID",
				"--reference-name", "Pro Monthly",
				"--product-id", "com.example.pro.monthly",
				"--subscription-period", "ONE_MONTH",
				"--enable-monthly-commitment",
			},
			wantErr: "--enable-monthly-commitment requires --subscription-period ONE_YEAR",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsSetupCreateOnlySuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/subscriptionGroups" {
				t.Fatalf("unexpected group lookup request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[],"links":{"next":""}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionGroups" {
				t.Fatalf("unexpected group create request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.SubscriptionGroupCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode group payload: %v", err)
			}
			if payload.Data.Attributes.ReferenceName != "Pro" {
				t.Fatalf("expected group reference Pro, got %q", payload.Data.Attributes.ReferenceName)
			}
			body := `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptions" {
				t.Fatalf("unexpected subscription create request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.SubscriptionCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode subscription payload: %v", err)
			}
			if payload.Data.Relationships.Group.Data.ID != "group-1" {
				t.Fatalf("expected group-1 relationship, got %q", payload.Data.Relationships.Group.Data.ID)
			}
			if payload.Data.Attributes.Name != "Pro Monthly" || payload.Data.Attributes.ProductID != "com.example.pro.monthly" {
				t.Fatalf("unexpected subscription attrs: %+v", payload.Data.Attributes)
			}
			body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"READY_TO_SUBMIT","familySharable":false}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1" {
				t.Fatalf("unexpected verify group request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 5:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1" {
				t.Fatalf("unexpected verify subscription request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"READY_TO_SUBMIT","familySharable":false}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result subscriptionsSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--app", "app-1",
			"--group-reference-name", "Pro",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 5 {
		t.Fatalf("expected create and verify requests, got %d", requestCount)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "ok" || result.GroupID != "group-1" || result.SubscriptionID != "sub-1" {
		t.Fatalf("unexpected create-only setup result: %+v", result)
	}
	if result.Verification.Status != "verified" || result.Verification.GroupExists == nil || !*result.Verification.GroupExists || !result.Verification.SubscriptionExists {
		t.Fatalf("expected verified group/subscription create-only result, got %+v", result.Verification)
	}
}

func TestSubscriptionsSetupRejectsAppleMissingMetadataState(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case 2:
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA"}}}`), nil
		case 3:
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`), nil
		case 4:
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA"}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var result subscriptionsSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--group-id", "group-1",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "MISSING_METADATA") {
			t.Fatalf("expected MISSING_METADATA failure, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected structured error without stderr duplication, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "error" || result.FailedStep != "verify_state" || result.Verification.Status != "failed" || result.Verification.SubscriptionState != "MISSING_METADATA" {
		t.Fatalf("expected truthful Apple state failure, got %+v", result)
	}
}

func TestSubscriptionsSetupVerifiesAllExistingAvailabilityPriceCoverage(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"READY_TO_SUBMIT"}}],"links":{"next":""}}`), nil
		case 2:
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`), nil
		case 3:
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"READY_TO_SUBMIT"}}}`), nil
		case 4:
			if req.URL.Path != "/v1/subscriptions/sub-1/subscriptionAvailability" {
				t.Fatalf("unexpected availability request: %s", req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":true}}}`), nil
		case 5:
			if req.URL.Path != "/v1/subscriptionAvailabilities/availability-1/availableTerritories" {
				t.Fatalf("unexpected availability territories request: %s", req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{"next":""}}`), nil
		case 6:
			if req.URL.Path != "/v1/subscriptions/sub-1/prices" || req.URL.Query().Get("filter[planType]") != "UPFRONT" {
				t.Fatalf("unexpected price coverage request: %s", req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-usa","attributes":{"planType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-usa"}}}}],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var result subscriptionsSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--group-id", "group-1",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "CAN") {
			t.Fatalf("expected missing CAN price coverage failure, got %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected structured error without stderr duplication, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Verification.PriceCoverageVerified == nil || *result.Verification.PriceCoverageVerified || len(result.Verification.MissingPriceTerritories) != 1 || result.Verification.MissingPriceTerritories[0] != "CAN" {
		t.Fatalf("expected missing CAN coverage in verification, got %+v", result.Verification)
	}
}

func TestSubscriptionsSetupExistingGroupNoVerifySuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1/subscriptions" {
				t.Fatalf("unexpected subscription lookup request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[productId]"); got != "com.example.pro.monthly" {
				t.Fatalf("expected product ID lookup, got %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptions" {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.SubscriptionCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode subscription payload: %v", err)
			}
			if payload.Data.Relationships.Group.Data.ID != "group-1" {
				t.Fatalf("expected group-1 relationship, got %q", payload.Data.Relationships.Group.Data.ID)
			}
			body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA"}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result subscriptionsSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--group-id", "group-1",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--no-verify",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 2 {
		t.Fatalf("expected only subscription create request with --no-verify, got %d", requestCount)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "ok" || result.GroupID != "group-1" || result.SubscriptionID != "sub-1" {
		t.Fatalf("unexpected existing-group no-verify result: %+v", result)
	}
	if result.Verification.Status != "skipped" {
		t.Fatalf("expected skipped verification with --no-verify, got %+v", result.Verification)
	}
}

func TestSubscriptionsSetupReusesExistingGroupSubscriptionAndLocalization(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method == http.MethodPost {
			t.Fatalf("setup should not create existing resources, got POST %s", req.URL.Path)
		}
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/subscriptionGroups" {
				t.Fatalf("unexpected group lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}],"links":{"next":""}}`), nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1/subscriptions" {
				t.Fatalf("unexpected subscription lookup request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[productId]"); got != "com.example.pro.monthly" {
				t.Fatalf("expected product ID lookup, got %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA"}}],"links":{"next":""}}`), nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/subscriptionLocalizations" {
				t.Fatalf("unexpected localization lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro Monthly","locale":"en-US","description":"All premium features."}}],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result subscriptionsSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--app", "app-1",
			"--group-reference-name", "Pro",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--locale", "en-US",
			"--display-name", "Pro Monthly",
			"--description", "All premium features.",
			"--no-verify",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 3 {
		t.Fatalf("expected only lookup requests, got %d", requestCount)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "ok" || result.GroupID != "group-1" || result.SubscriptionID != "sub-1" || result.LocalizationID != "loc-1" {
		t.Fatalf("unexpected reused setup result: %+v", result)
	}
	wantMessages := map[string]string{
		"ensure_group":        "used existing group",
		"create_subscription": "used existing subscription",
		"create_localization": "used existing localization",
	}
	seenMessages := map[string]bool{}
	for _, step := range result.Steps {
		if want, ok := wantMessages[step.Name]; ok && step.Message != want {
			t.Fatalf("expected %s message %q, got %q", step.Name, want, step.Message)
		}
		if _, ok := wantMessages[step.Name]; ok {
			seenMessages[step.Name] = true
		}
	}
	for name := range wantMessages {
		if !seenMessages[name] {
			t.Fatalf("expected reuse step %q in %+v", name, result.Steps)
		}
	}
}

func TestSubscriptionsSetupRejectsMismatchedExistingSubscription(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/subscriptionGroups" {
				t.Fatalf("unexpected group lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}],"links":{"next":""}}`), nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1/subscriptions" {
				t.Fatalf("unexpected subscription lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Old Name","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH"}}],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--app", "app-1",
			"--group-reference-name", "Pro",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--no-verify",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "different reference name") {
		t.Fatalf("expected different reference name error, got %v", runErr)
	}

	var result subscriptionsSetupOutput
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "error" || result.FailedStep != "create_subscription" {
		t.Fatalf("unexpected setup result: %+v", result)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 2 {
		t.Fatalf("expected lookup requests only, got %d", requestCount)
	}
}

func TestSubscriptionsSetupRejectsMismatchedExistingSubscriptionFamilySharingDefault(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1/subscriptions" {
			t.Fatalf("unexpected subscription lookup request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","familySharable":true}}],"links":{"next":""}}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--group-id", "group-1",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--no-verify",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "different family sharing setting") {
		t.Fatalf("expected different family sharing error, got %v", runErr)
	}

	var result subscriptionsSetupOutput
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "error" || result.FailedStep != "create_subscription" {
		t.Fatalf("unexpected setup result: %+v", result)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 1 {
		t.Fatalf("expected only subscription lookup request, got %d", requestCount)
	}
}

func TestSubscriptionsSetupRejectsAmbiguousExistingGroupReference(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/subscriptionGroups" {
			t.Fatalf("unexpected group lookup request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}},{"type":"subscriptionGroups","id":"group-2","attributes":{"referenceName":"Pro"}}],"links":{"next":""}}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--app", "app-1",
			"--group-reference-name", "Pro",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--no-verify",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "multiple subscription groups match reference name") {
		t.Fatalf("expected ambiguous group reference error, got %v", runErr)
	}

	var result subscriptionsSetupOutput
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "error" || result.FailedStep != "ensure_group" {
		t.Fatalf("unexpected setup result: %+v", result)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 1 {
		t.Fatalf("expected only group lookup request, got %d", requestCount)
	}
}

func TestSubscriptionsSetupRejectsMismatchedExistingLocalization(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/subscriptionGroups" {
				t.Fatalf("unexpected group lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}],"links":{"next":""}}`), nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1/subscriptions" {
				t.Fatalf("unexpected subscription lookup request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[productId]"); got != "com.example.pro.monthly" {
				t.Fatalf("expected product ID lookup, got %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH"}}],"links":{"next":""}}`), nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/subscriptionLocalizations" {
				t.Fatalf("unexpected localization lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Old Name","locale":"en-US","description":"All premium features."}}],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--app", "app-1",
			"--group-reference-name", "Pro",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--locale", "en-US",
			"--display-name", "Pro Monthly",
			"--description", "All premium features.",
			"--no-verify",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "different display name") {
		t.Fatalf("expected different display name error, got %v", runErr)
	}

	var result subscriptionsSetupOutput
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "error" || result.FailedStep != "create_localization" {
		t.Fatalf("unexpected setup result: %+v", result)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 3 {
		t.Fatalf("expected lookup requests only, got %d", requestCount)
	}
}

func TestSubscriptionsSetupRejectsMismatchedExistingLocalizationDescription(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/subscriptionGroups" {
				t.Fatalf("unexpected group lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}],"links":{"next":""}}`), nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1/subscriptions" {
				t.Fatalf("unexpected subscription lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH"}}],"links":{"next":""}}`), nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/subscriptionLocalizations" {
				t.Fatalf("unexpected localization lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro Monthly","locale":"en-US","description":"Old description."}}],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--app", "app-1",
			"--group-reference-name", "Pro",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--locale", "en-US",
			"--display-name", "Pro Monthly",
			"--description", "All premium features.",
			"--no-verify",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "different description") {
		t.Fatalf("expected different description error, got %v", runErr)
	}

	var result subscriptionsSetupOutput
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "error" || result.FailedStep != "create_localization" {
		t.Fatalf("unexpected setup result: %+v", result)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 3 {
		t.Fatalf("expected lookup requests only, got %d", requestCount)
	}
}

func TestSubscriptionsSetupReusesExistingPriceAndAvailability(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method == http.MethodPost || req.Method == http.MethodPatch {
			t.Fatalf("setup rerun should not mutate matching existing state, got %s %s", req.Method, req.URL.Path)
		}
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1/subscriptions" {
				t.Fatalf("unexpected subscription lookup request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[productId]"); got != "com.example.pro.monthly" {
				t.Fatalf("expected product ID lookup, got %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH"}}],"links":{"next":""}}`), nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/prices" {
				t.Fatalf("unexpected price lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-1","attributes":{"planType":"UPFRONT"},"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"price-point-1"}},"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{"next":""}}`), nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/subscriptionAvailability" {
				t.Fatalf("unexpected availability lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":true}}}`), nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionAvailabilities/availability-1/availableTerritories" {
				t.Fatalf("unexpected availability territories request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("cursor"); got != "" {
				t.Fatalf("expected first availability territories page, got cursor %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"CAN"}],"links":{"next":"/v1/subscriptionAvailabilities/availability-1/availableTerritories?cursor=page-2"}}`), nil
		case 5:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionAvailabilities/availability-1/availableTerritories" {
				t.Fatalf("unexpected availability territories page request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("cursor"); got != "page-2" {
				t.Fatalf("expected second availability territories page, got cursor %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"}],"links":{"next":""}}`), nil
		case 6:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionPricePoints/price-point-1/equalizations" {
				t.Fatalf("unexpected equalizations request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case 7:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/prices" {
				t.Fatalf("unexpected price matrix read: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-1","attributes":{"planType":"UPFRONT"},"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"price-point-1"}},"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result subscriptionsSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--group-id", "group-1",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--price-point-id", "price-point-1",
			"--price-territory", "USA",
			"--available-in-new-territories",
			"--no-verify",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "ok" || result.SubscriptionID != "sub-1" || result.AvailabilityID != "availability-1" {
		t.Fatalf("unexpected setup result: %+v", result)
	}
	wantMessages := map[string]string{
		"set_price":        "verified complete price matrix across 1 territories",
		"set_availability": "used existing availability",
	}
	for name, want := range wantMessages {
		found := false
		for _, step := range result.Steps {
			if step.Name == name {
				found = true
				if step.Message != want {
					t.Fatalf("expected %s message %q, got %q", name, want, step.Message)
				}
			}
		}
		if !found {
			t.Fatalf("expected step %q in %+v", name, result.Steps)
		}
	}
	if requestCount != 7 {
		t.Fatalf("expected lookup requests only, got %d", requestCount)
	}
}

func TestSubscriptionsSetupRepairReplacesMismatchedPrice(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH"}}],"links":{"next":""}}`), nil
		case 2:
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-1","attributes":{"planType":"UPFRONT"},"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"old-price-point"}},"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{"next":""}}`), nil
		case 3:
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`), nil
		case 4:
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"territories","id":"USA"}],"links":{"next":""}}`), nil
		case 5:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionPricePoints/price-point-1/equalizations" {
				t.Fatalf("expected repair equalizations GET, got %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		case 6:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/prices" {
				t.Fatalf("expected repair state GET, got %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-1","attributes":{"planType":"UPFRONT"},"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"old-price-point"}},"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{"next":""}}`), nil
		case 7:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/subscriptions/sub-1" {
				t.Fatalf("expected repair matrix PATCH, got %s %s", req.Method, req.URL.String())
			}
			var payload asc.SubscriptionUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode repair matrix payload: %v", err)
			}
			if len(payload.Included) != 1 || payload.Included[0].Attributes == nil || payload.Included[0].Attributes.PlanType != asc.SubscriptionPlanTypeUpfront {
				t.Fatalf("expected one UPFRONT repair matrix row, got %+v", payload.Included)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"sub-1"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var result subscriptionsSetupOutput
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--group-id", "group-1",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--price-point-id", "price-point-1",
			"--price-territory", "USA",
			"--repair",
			"--no-verify",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v", err)
	}
	foundRepair := false
	for _, step := range result.Steps {
		if step.Name == "set_price" && step.Message == "materialized complete price matrix across 1 territories" {
			foundRepair = true
		}
	}
	if !foundRepair {
		t.Fatalf("expected repair step, got %+v", result.Steps)
	}
}

func TestSubscriptionsSetupRejectsInitialPricePatchWhenExistingPricesMismatch(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method == http.MethodPost || req.Method == http.MethodPatch {
			t.Fatalf("setup rerun should not mutate priced subscription with nonmatching price, got %s %s", req.Method, req.URL.Path)
		}
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1/subscriptions" {
				t.Fatalf("unexpected subscription lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH"}}],"links":{"next":""}}`), nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/prices" {
				t.Fatalf("unexpected price lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-1","attributes":{"planType":"MONTHLY"},"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"price-point-other"}},"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--group-id", "group-1",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--locale", "en-US",
			"--display-name", "Pro Monthly",
			"--description", "All premium features.",
			"--price-point-id", "price-point-1",
			"--price-territory", "USA",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "already has prices but none match") {
			t.Fatalf("expected existing prices mismatch error, got %v", err)
		}
	})

	if requestCount != 2 {
		t.Fatalf("expected subscription and price lookup only, got %d requests", requestCount)
	}
}

func TestSubscriptionsSetupRejectsStaleExistingAvailabilityBeforeLocalization(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method == http.MethodPost || req.Method == http.MethodPatch {
			t.Fatalf("setup rerun should not mutate before stale availability failure, got %s %s", req.Method, req.URL.Path)
		}
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1/subscriptions" {
				t.Fatalf("unexpected subscription lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","familySharable":false}}],"links":{"next":""}}`), nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/subscriptionAvailability" {
				t.Fatalf("unexpected availability lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptionAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, _ = captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--group-id", "group-1",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--locale", "en-US",
			"--display-name", "Pro Monthly",
			"--description", "All premium features.",
			"--available-in-new-territories",
			"--territories", "USA",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil || !strings.Contains(err.Error(), "already has availability") {
			t.Fatalf("expected stale availability error, got %v", err)
		}
	})

	if requestCount != 2 {
		t.Fatalf("expected subscription and availability lookup only, got %d requests", requestCount)
	}
}

func TestSubscriptionsSetupPricingAutoEnablesPriceTerritoryAvailability(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("HOME", t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/pp-nok-19/equalizations" {
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		}
		if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/prices" {
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		}
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/subscriptionGroups" {
				t.Fatalf("unexpected group lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionGroups" {
				t.Fatalf("unexpected group create request: %s %s", req.Method, req.URL.Path)
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`), nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptions" {
				t.Fatalf("unexpected subscription create request: %s %s", req.Method, req.URL.Path)
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA"}}}`), nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/pricePoints" {
				t.Fatalf("unexpected price-point lookup request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[territory]"); got != "NOR" {
				t.Fatalf("expected filter[territory]=NOR, got %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"pp-nok-19","attributes":{"customerPrice":"19.00","proceeds":"14.00","proceedsYear2":"14.00"}}],"links":{"next":""}}`), nil
		case 5:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/subscriptions/sub-1" {
				t.Fatalf("unexpected initial price request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.SubscriptionUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode initial price payload: %v", err)
			}
			if len(payload.Included) != 1 {
				t.Fatalf("expected one included price resource, got %d", len(payload.Included))
			}
			if payload.Included[0].Attributes == nil || payload.Included[0].Attributes.PlanType != asc.SubscriptionPlanTypeUpfront {
				t.Fatalf("expected UPFRONT initial price, got %+v", payload.Included[0].Attributes)
			}
			if payload.Included[0].Relationships.Territory == nil || payload.Included[0].Relationships.Territory.Data.ID != "NOR" {
				t.Fatalf("expected pricing territory NOR, got %+v", payload.Included[0].Relationships.Territory)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA"}}}`), nil
		case 6:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionAvailabilities" {
				t.Fatalf("unexpected availability request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.SubscriptionAvailabilityCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode availability payload: %v", err)
			}
			if payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
				t.Fatalf("expected availability to target sub-1, got %q", payload.Data.Relationships.Subscription.Data.ID)
			}
			if !payload.Data.Attributes.AvailableInNewTerritories {
				t.Fatalf("expected availableInNewTerritories true")
			}
			if len(payload.Data.Relationships.AvailableTerritories.Data) != 1 {
				t.Fatalf("expected one auto-enabled territory, got %+v", payload.Data.Relationships.AvailableTerritories.Data)
			}
			if got := payload.Data.Relationships.AvailableTerritories.Data[0].ID; got != "NOR" {
				t.Fatalf("expected auto-enabled territory NOR, got %q", got)
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result subscriptionsSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--app", "app-1",
			"--group-reference-name", "Pro",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--price", "19",
			"--price-territory", "Norway",
			"--available-in-new-territories",
			"--no-verify",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 6 {
		t.Fatalf("expected create, price, and auto-availability requests, got %d", requestCount)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "ok" || result.AvailabilityID != "avail-1" || result.ResolvedPricePointID != "pp-nok-19" {
		t.Fatalf("unexpected pricing auto-availability result: %+v", result)
	}
	foundAutoAvailabilityMessage := false
	for _, step := range result.Steps {
		if step.Name != "set_availability" {
			continue
		}
		if !strings.Contains(step.Message, `auto-enabled pricing territory "NOR"`) {
			t.Fatalf("expected auto-availability step message, got %q", step.Message)
		}
		foundAutoAvailabilityMessage = true
	}
	if !foundAutoAvailabilityMessage {
		t.Fatalf("expected set_availability step with auto-enabled pricing territory message, got %+v", result.Steps)
	}
	if result.Verification.Status != "skipped" {
		t.Fatalf("expected skipped verification with --no-verify, got %+v", result.Verification)
	}
}

func TestSubscriptionsSetupCreateLocalizationPricingAndAvailabilitySuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("HOME", t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	coveragePriceReads := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/pp-399/equalizations" {
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"pp-can-399","attributes":{"customerPrice":"5.49"},"relationships":{"territory":{"data":{"type":"territories","id":"CAN"}}}}],"links":{"next":""}}`), nil
		}
		if req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptions/sub-1" && coveragePriceReads > 0 {
			var payload asc.SubscriptionUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode CAN starting price payload: %v", err)
			}
			if len(payload.Included) != 2 {
				t.Fatalf("expected complete USA/CAN matrix, got %+v", payload.Included)
			}
			for _, included := range payload.Included {
				if included.Attributes == nil || included.Attributes.PlanType != asc.SubscriptionPlanTypeUpfront {
					t.Fatalf("expected each matrix row to include UPFRONT planType, got %+v", included.Attributes)
				}
			}
			requestCount++ // replace the historical single-price PATCH slot
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"state":"MISSING_METADATA"}}}`), nil
		}
		if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/prices" && req.URL.Query().Get("filter[planType]") == "UPFRONT" && req.URL.Query().Get("filter[territory]") == "" {
			coveragePriceReads++
			canada := ""
			if coveragePriceReads > 1 {
				canada = `,{"type":"subscriptionPrices","id":"price-can","attributes":{"startDate":"2026-03-01","planType":"UPFRONT"},"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-can-399"}},"territory":{"data":{"type":"territories","id":"CAN"}}}}`
			}
			body := `{"data":[{"type":"subscriptionPrices","id":"price-usa","attributes":{"startDate":"2026-03-01","planType":"UPFRONT"},"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-399"}},"territory":{"data":{"type":"territories","id":"USA"}}}}` + canada + `],"links":{"next":""}}`
			return jsonHTTPResponse(http.StatusOK, body), nil
		}
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/subscriptionGroups" {
				t.Fatalf("unexpected group lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionGroups" {
				t.Fatalf("unexpected group create request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptions" {
				t.Fatalf("unexpected subscription create request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.SubscriptionCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode subscription payload: %v", err)
			}
			if payload.Data.Relationships.Group.Data.ID != "group-1" {
				t.Fatalf("expected group-1 relationship, got %q", payload.Data.Relationships.Group.Data.ID)
			}
			if payload.Data.Attributes.Name != "Pro Monthly" || payload.Data.Attributes.ProductID != "com.example.pro.monthly" {
				t.Fatalf("unexpected subscription attrs: %+v", payload.Data.Attributes)
			}
			if payload.Data.Attributes.SubscriptionPeriod != "ONE_MONTH" {
				t.Fatalf("expected subscription period ONE_MONTH, got %q", payload.Data.Attributes.SubscriptionPeriod)
			}
			if payload.Data.Attributes.FamilySharable == nil || !*payload.Data.Attributes.FamilySharable {
				t.Fatalf("expected family-sharable true")
			}
			body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA","familySharable":true}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 4:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionLocalizations" {
				t.Fatalf("unexpected localization request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.SubscriptionLocalizationCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode localization payload: %v", err)
			}
			if payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
				t.Fatalf("expected localization to target sub-1, got %q", payload.Data.Relationships.Subscription.Data.ID)
			}
			if payload.Data.Attributes.Name != "Pro Monthly" || payload.Data.Attributes.Locale != "en-US" || payload.Data.Attributes.Description != "All premium features." {
				t.Fatalf("unexpected localization attrs: %+v", payload.Data.Attributes)
			}
			body := `{"data":{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro Monthly","locale":"en-US","description":"All premium features."}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 5:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/pricePoints" {
				t.Fatalf("unexpected price-point lookup request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("filter[territory]") != "USA" {
				t.Fatalf("expected USA territory filter, got %q", req.URL.Query().Get("filter[territory]"))
			}
			body := `{"data":[
				{"type":"subscriptionPricePoints","id":"pp-199","attributes":{"customerPrice":"1.99","proceeds":"1.39","proceedsYear2":"1.39"}},
				{"type":"subscriptionPricePoints","id":"pp-399","attributes":{"customerPrice":"3.99","proceeds":"3.39","proceedsYear2":"3.39"}}
			],"links":{"next":""}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 6:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/subscriptions/sub-1" {
				t.Fatalf("unexpected initial price request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.SubscriptionUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode initial price payload: %v", err)
			}
			if len(payload.Included) != 1 || payload.Included[0].Relationships.SubscriptionPricePoint.Data.ID != "pp-399" {
				t.Fatalf("expected resolved price point pp-399, got %+v", payload.Included)
			}
			if payload.Included[0].Attributes == nil || payload.Included[0].Attributes.PlanType != asc.SubscriptionPlanTypeUpfront {
				t.Fatalf("expected UPFRONT initial price, got %+v", payload.Included[0].Attributes)
			}
			body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA","familySharable":true}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 7:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionAvailabilities" {
				t.Fatalf("unexpected availability request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.SubscriptionAvailabilityCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode availability payload: %v", err)
			}
			if payload.Data.Relationships.Subscription.Data.ID != "sub-1" {
				t.Fatalf("expected availability to target sub-1, got %q", payload.Data.Relationships.Subscription.Data.ID)
			}
			if payload.Data.Attributes.AvailableInNewTerritories {
				t.Fatalf("expected availableInNewTerritories false")
			}
			if len(payload.Data.Relationships.AvailableTerritories.Data) != 2 {
				t.Fatalf("expected two availability territories, got %+v", payload.Data.Relationships.AvailableTerritories.Data)
			}
			body := `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 8:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1" {
				t.Fatalf("unexpected verify group request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 9:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1" {
				t.Fatalf("unexpected verify subscription request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"READY_TO_SUBMIT","familySharable":true}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 10:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/subscriptionLocalizations" {
				t.Fatalf("unexpected verify localizations request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":[{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro Monthly","locale":"en-US","description":"All premium features."}}],"links":{"next":""}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 11:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/prices" {
				t.Fatalf("unexpected verify pricing request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":[{"type":"subscriptionPrices","id":"price-1","attributes":{"startDate":"2026-03-01"},"relationships":{"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-399"}},"territory":{"data":{"type":"territories","id":"USA"}}}}],
				"included":[
					{"type":"subscriptionPricePoints","id":"pp-399","attributes":{"customerPrice":"3.99","proceeds":"3.39","proceedsYear2":"3.39"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 12:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/subscriptionAvailability" {
				t.Fatalf("unexpected verify availability request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 13:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionAvailabilities/avail-1/availableTerritories" {
				t.Fatalf("unexpected verify availability territories request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}],"links":{"next":""}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result subscriptionsSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "setup",
			"--app", "app-1",
			"--group-reference-name", "Pro",
			"--reference-name", "Pro Monthly",
			"--product-id", "com.example.pro.monthly",
			"--subscription-period", "ONE_MONTH",
			"--family-sharable",
			"--available-in-new-territories", "false",
			"--locale", "en-US",
			"--display-name", "Pro Monthly",
			"--description", "All premium features.",
			"--price", "3.99",
			"--price-territory", "USA",
			"--start-date", "2026-03-01",
			"--territories", "USA,CAN",
			"--refresh",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 13 {
		t.Fatalf("expected full create and verify flow, got %d requests", requestCount)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "ok" || result.GroupID != "group-1" || result.SubscriptionID != "sub-1" || result.LocalizationID != "loc-1" || result.AvailabilityID != "avail-1" || result.ResolvedPricePointID != "pp-399" {
		t.Fatalf("unexpected full setup result: %+v", result)
	}
	if result.Verification.Status != "verified" || result.Verification.GroupExists == nil || !*result.Verification.GroupExists || !result.Verification.SubscriptionExists {
		t.Fatalf("expected verified group/subscription state, got %+v", result.Verification)
	}
	if result.Verification.LocalizationExists == nil || !*result.Verification.LocalizationExists {
		t.Fatalf("expected localization verification, got %+v", result.Verification)
	}
	if result.Verification.PriceVerified == nil || !*result.Verification.PriceVerified {
		t.Fatalf("expected price verification, got %+v", result.Verification)
	}
	if result.Verification.AvailabilityVerified == nil || !*result.Verification.AvailabilityVerified {
		t.Fatalf("expected availability verification, got %+v", result.Verification)
	}
	if result.Verification.SubscriptionState != "READY_TO_SUBMIT" {
		t.Fatalf("expected Apple state READY_TO_SUBMIT, got %+v", result.Verification)
	}
	if result.Verification.PriceCoverageVerified == nil || !*result.Verification.PriceCoverageVerified || len(result.Verification.MissingPriceTerritories) != 0 {
		t.Fatalf("expected complete price coverage, got %+v", result.Verification)
	}
	if result.Verification.CurrentPrice == nil || result.Verification.CurrentPrice.Amount != "3.99" || result.Verification.CurrentPrice.Currency != "USD" {
		t.Fatalf("expected verified current price 3.99 USD, got %+v", result.Verification.CurrentPrice)
	}
}

func TestSubscriptionsSetupNormalizesTerritories(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("HOME", t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPricePoints/pp-399/equalizations" {
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"pp-fra-399","attributes":{"customerPrice":"3.99"},"relationships":{"territory":{"data":{"type":"territories","id":"FRA"}}}}],"links":{}}`), nil
		}
		if req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/prices" {
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`), nil
		}
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/subscriptionGroups" {
				t.Fatalf("unexpected group lookup request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionGroups" {
				t.Fatalf("unexpected group create request: %s %s", req.Method, req.URL.Path)
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`), nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptions" {
				t.Fatalf("unexpected subscription create request: %s %s", req.Method, req.URL.Path)
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA"}}}`), nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/pricePoints" {
				t.Fatalf("unexpected price-point lookup request: %s %s", req.Method, req.URL.String())
			}
			if got := req.URL.Query().Get("filter[territory]"); got != "USA" {
				t.Fatalf("expected normalized filter[territory]=USA, got %q", got)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"pp-399","attributes":{"customerPrice":"3.99","proceeds":"3.39","proceedsYear2":"3.39"}}],"links":{"next":""}}`), nil
		case 5:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/subscriptions/sub-1" {
				t.Fatalf("unexpected initial price request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.SubscriptionUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode full price matrix: %v", err)
			}
			if len(payload.Included) != 2 {
				t.Fatalf("expected USA/FRA price matrix, got %+v", payload.Included)
			}
			requestCount = 8
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA"}}}`), nil
		case 6:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionPricePoints/pp-399/equalizations" {
				t.Fatalf("unexpected equalizations request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"pp-fra-399","attributes":{"customerPrice":"3.99"},"relationships":{"territory":{"data":{"type":"territories","id":"FRA"}}}}],"links":{"next":""}}`), nil
		case 7:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/prices" {
				t.Fatalf("unexpected price coverage request: %s %s", req.Method, req.URL.String())
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"subscriptionPrices","id":"price-usa","attributes":{"planType":"UPFRONT"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-399"}}}}],"links":{"next":""}}`), nil
		case 8:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/subscriptions/sub-1" {
				t.Fatalf("unexpected FRA starting price request: %s %s", req.Method, req.URL.String())
			}
			var payload asc.SubscriptionUpdateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode FRA starting price payload: %v", err)
			}
			if len(payload.Included) != 1 || payload.Included[0].Relationships.Territory == nil || payload.Included[0].Relationships.Territory.Data.ID != "FRA" || payload.Included[0].Relationships.SubscriptionPricePoint.Data.ID != "pp-fra-399" {
				t.Fatalf("unexpected FRA starting price payload: %+v", payload.Included)
			}
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"state":"MISSING_METADATA"}}}`), nil
		case 9:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionAvailabilities" {
				t.Fatalf("unexpected availability request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.SubscriptionAvailabilityCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode availability payload: %v", err)
			}
			if len(payload.Data.Relationships.AvailableTerritories.Data) != 2 {
				t.Fatalf("expected two availability territories, got %+v", payload.Data.Relationships.AvailableTerritories.Data)
			}
			if got := payload.Data.Relationships.AvailableTerritories.Data[0].ID; got != "USA" {
				t.Fatalf("expected first territory USA, got %q", got)
			}
			if got := payload.Data.Relationships.AvailableTerritories.Data[1].ID; got != "FRA" {
				t.Fatalf("expected second territory FRA, got %q", got)
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"subscriptions", "setup",
		"--app", "app-1",
		"--group-reference-name", "Pro",
		"--reference-name", "Pro Monthly",
		"--product-id", "com.example.pro.monthly",
		"--subscription-period", "ONE_MONTH",
		"--price", "3.99",
		"--price-territory", "United States",
		"--territories", "US,France",
		"--no-verify",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
	if requestCount != 9 {
		t.Fatalf("expected 9 setup requests, got %d", requestCount)
	}
}

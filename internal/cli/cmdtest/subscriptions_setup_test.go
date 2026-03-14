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
	SubscriptionID       string `json:"subscriptionId,omitempty"`
	LocalizationID       string `json:"localizationId,omitempty"`
	AvailabilityID       string `json:"availabilityId,omitempty"`
	ResolvedPricePointID string `json:"resolvedPricePointId,omitempty"`
	Error                string `json:"error,omitempty"`
	FailedStep           string `json:"failedStep,omitempty"`
	Verification         struct {
		Status               string `json:"status"`
		GroupExists          *bool  `json:"groupExists,omitempty"`
		SubscriptionExists   bool   `json:"subscriptionExists,omitempty"`
		LocalizationExists   *bool  `json:"localizationExists,omitempty"`
		PriceVerified        *bool  `json:"priceVerified,omitempty"`
		AvailabilityVerified *bool  `json:"availabilityVerified,omitempty"`
		PriceTerritory       string `json:"priceTerritory,omitempty"`
		CurrentPrice         *struct {
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
	}
	subscriptionsUsage := subscriptionsCmd.UsageFunc(subscriptionsCmd)
	if !usageListsSubcommand(subscriptionsUsage, "setup") {
		t.Fatalf("expected subscriptions help to list setup, got %q", subscriptionsUsage)
	}

	setupCmd := findSubcommand(root, "subscriptions", "setup")
	if setupCmd == nil {
		t.Fatal("expected subscriptions setup command")
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
			wantErr: "--territories is required when availability flags are provided",
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
		case 2:
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
			body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA","familySharable":false}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1" {
				t.Fatalf("unexpected verify group request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1" {
				t.Fatalf("unexpected verify subscription request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA","familySharable":false}}}`
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
	if requestCount != 4 {
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

func TestSubscriptionsSetupExistingGroupNoVerifySuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
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
	if requestCount != 1 {
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

func TestSubscriptionsSetupCreateLocalizationPricingAndAvailabilitySuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("HOME", t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/subscriptionGroups" {
				t.Fatalf("unexpected group create request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 2:
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
		case 3:
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
		case 4:
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
		case 5:
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
			body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA","familySharable":true}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
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
			if payload.Data.Attributes.AvailableInNewTerritories {
				t.Fatalf("expected availableInNewTerritories false")
			}
			if len(payload.Data.Relationships.AvailableTerritories.Data) != 2 {
				t.Fatalf("expected two availability territories, got %+v", payload.Data.Relationships.AvailableTerritories.Data)
			}
			body := `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 7:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionGroups/group-1" {
				t.Fatalf("unexpected verify group request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Pro"}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 8:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1" {
				t.Fatalf("unexpected verify subscription request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA","familySharable":true}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 9:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/subscriptionLocalizations" {
				t.Fatalf("unexpected verify localizations request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":[{"type":"subscriptionLocalizations","id":"loc-1","attributes":{"name":"Pro Monthly","locale":"en-US","description":"All premium features."}}],"links":{"next":""}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 10:
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
		case 11:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/subscriptionAvailability" {
				t.Fatalf("unexpected verify availability request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 12:
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
	if requestCount != 12 {
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
	if result.Verification.CurrentPrice == nil || result.Verification.CurrentPrice.Amount != "3.99" || result.Verification.CurrentPrice.Currency != "USD" {
		t.Fatalf("expected verified current price 3.99 USD, got %+v", result.Verification.CurrentPrice)
	}
}

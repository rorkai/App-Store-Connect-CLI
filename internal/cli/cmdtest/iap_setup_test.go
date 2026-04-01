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
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type iapSetupOutput struct {
	Status               string `json:"status"`
	IAPID                string `json:"iapId"`
	LocalizationID       string `json:"localizationId,omitempty"`
	PriceScheduleID      string `json:"priceScheduleId,omitempty"`
	ResolvedPricePointID string `json:"resolvedPricePointId,omitempty"`
	Verification         struct {
		Status             string `json:"status"`
		IAPExists          bool   `json:"iapExists,omitempty"`
		LocalizationExists *bool  `json:"localizationExists,omitempty"`
		PriceVerified      *bool  `json:"priceVerified,omitempty"`
		BaseTerritory      string `json:"baseTerritory,omitempty"`
		CurrentPrice       *struct {
			Amount   string `json:"amount"`
			Currency string `json:"currency"`
		} `json:"currentPrice,omitempty"`
		ScheduledPrice *struct {
			Amount   string `json:"amount"`
			Currency string `json:"currency"`
		} `json:"scheduledPrice,omitempty"`
		ScheduledStartDate string `json:"scheduledStartDate,omitempty"`
	} `json:"verification,omitempty"`
	Error      string `json:"error,omitempty"`
	FailedStep string `json:"failedStep,omitempty"`
	Steps      []struct {
		Name    string `json:"name"`
		Status  string `json:"status"`
		Message string `json:"message,omitempty"`
	} `json:"steps"`
}

func TestIAPHelpShowsSetupCommand(t *testing.T) {
	root := RootCommand("1.2.3")

	iapCmd := findSubcommand(root, "iap")
	if iapCmd == nil {
		t.Fatal("expected iap command")
	}
	iapUsage := iapCmd.UsageFunc(iapCmd)
	if !usageListsSubcommand(iapUsage, "setup") {
		t.Fatalf("expected iap help to list setup, got %q", iapUsage)
	}

	setupCmd := findSubcommand(root, "iap", "setup")
	if setupCmd == nil {
		t.Fatal("expected iap setup command")
	}
	setupUsage := setupCmd.UsageFunc(setupCmd)
	if !strings.Contains(setupUsage, "--reference-name") {
		t.Fatalf("expected iap setup help to show --reference-name, got %q", setupUsage)
	}
	if !strings.Contains(setupUsage, "--display-name") {
		t.Fatalf("expected iap setup help to show --display-name, got %q", setupUsage)
	}
	if strings.Contains(setupUsage, "--ref-name") {
		t.Fatalf("expected iap setup help to hide --ref-name alias, got %q", setupUsage)
	}
	if strings.Contains(setupUsage, "\n  --name") {
		t.Fatalf("expected iap setup help to hide --name alias, got %q", setupUsage)
	}
	if !strings.Contains(setupUsage, "--no-verify") {
		t.Fatalf("expected iap setup help to show --no-verify, got %q", setupUsage)
	}
}

func TestIAPSetupValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "missing app",
			args: []string{
				"iap", "setup",
				"--type", "NON_CONSUMABLE",
				"--reference-name", "Pro Lifetime",
				"--product-id", "lifetime",
			},
			wantErr: "--app is required",
		},
		{
			name: "missing display name when localization requested",
			args: []string{
				"iap", "setup",
				"--app", "APP_ID",
				"--type", "NON_CONSUMABLE",
				"--reference-name", "Pro Lifetime",
				"--product-id", "lifetime",
				"--locale", "en-US",
			},
			wantErr: "--display-name is required when localization flags are provided",
		},
		{
			name: "missing locale when localization requested",
			args: []string{
				"iap", "setup",
				"--app", "APP_ID",
				"--type", "NON_CONSUMABLE",
				"--reference-name", "Pro Lifetime",
				"--product-id", "lifetime",
				"--display-name", "Second Draft Pro",
			},
			wantErr: "--locale is required when localization flags are provided",
		},
		{
			name: "missing base territory when pricing requested",
			args: []string{
				"iap", "setup",
				"--app", "APP_ID",
				"--type", "NON_CONSUMABLE",
				"--reference-name", "Pro Lifetime",
				"--product-id", "lifetime",
				"--price", "3.99",
			},
			wantErr: "--base-territory is required when pricing flags are provided",
		},
		{
			name: "missing pricing selector when pricing flags requested",
			args: []string{
				"iap", "setup",
				"--app", "APP_ID",
				"--type", "NON_CONSUMABLE",
				"--reference-name", "Pro Lifetime",
				"--product-id", "lifetime",
				"--base-territory", "USA",
			},
			wantErr: "one of --price-point-id, --tier, or --price is required when pricing flags are provided",
		},
		{
			name: "pricing selectors are mutually exclusive",
			args: []string{
				"iap", "setup",
				"--app", "APP_ID",
				"--type", "NON_CONSUMABLE",
				"--reference-name", "Pro Lifetime",
				"--product-id", "lifetime",
				"--base-territory", "USA",
				"--price", "3.99",
				"--price-point-id", "pp-1",
			},
			wantErr: "--price-point-id, --tier, and --price are mutually exclusive",
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

func TestIAPSetupCreateOnlySuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v2/inAppPurchases" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.Path)
			}

			var payload asc.InAppPurchaseV2CreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode create payload: %v", err)
			}
			if payload.Data.Attributes.Name != "Pro Lifetime" {
				t.Fatalf("expected reference name, got %q", payload.Data.Attributes.Name)
			}
			if payload.Data.Attributes.ProductID != "lifetime" {
				t.Fatalf("expected product id lifetime, got %q", payload.Data.Attributes.ProductID)
			}
			if payload.Data.Attributes.InAppPurchaseType != "NON_CONSUMABLE" {
				t.Fatalf("expected NON_CONSUMABLE type, got %q", payload.Data.Attributes.InAppPurchaseType)
			}

			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro Lifetime","productId":"lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1" {
				t.Fatalf("unexpected verify request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro Lifetime","productId":"lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result iapSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "setup",
			"--app", "app-1",
			"--type", "NON_CONSUMABLE",
			"--reference-name", "Pro Lifetime",
			"--product-id", "lifetime",
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
		t.Fatalf("expected create plus verify readback, got %d requests", requestCount)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "ok" {
		t.Fatalf("expected ok status, got %q", result.Status)
	}
	if result.IAPID != "iap-1" {
		t.Fatalf("expected iapId iap-1, got %q", result.IAPID)
	}
	if result.LocalizationID != "" || result.PriceScheduleID != "" || result.ResolvedPricePointID != "" {
		t.Fatalf("expected no localization/pricing ids for create-only run, got %+v", result)
	}
	if result.Verification.Status != "verified" || !result.Verification.IAPExists {
		t.Fatalf("expected verified create-only result, got %+v", result.Verification)
	}
}

func TestIAPSetupCreateOnlyNoVerifySkipsReadback(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodPost || req.URL.Path != "/v2/inAppPurchases" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro Lifetime","productId":"lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result iapSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "setup",
			"--app", "app-1",
			"--type", "NON_CONSUMABLE",
			"--reference-name", "Pro Lifetime",
			"--product-id", "lifetime",
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
		t.Fatalf("expected only create request with --no-verify, got %d", requestCount)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Verification.Status != "skipped" {
		t.Fatalf("expected skipped verification with --no-verify, got %+v", result.Verification)
	}
}

func TestIAPSetupClientInitFailureProducesStructuredError(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_PROFILE", "")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result iapSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "setup",
			"--app", "app-1",
			"--type", "NON_CONSUMABLE",
			"--reference-name", "Pro Lifetime",
			"--product-id", "lifetime",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr for reported json error, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "error" || result.FailedStep != "create_iap" {
		t.Fatalf("expected create_iap failure result, got %+v", result)
	}
	if strings.TrimSpace(result.Error) == "" {
		t.Fatalf("expected top-level error message, got %+v", result)
	}
	if len(result.Steps) == 0 || result.Steps[0].Status != "failed" || strings.TrimSpace(result.Steps[0].Message) == "" {
		t.Fatalf("expected structured failed step message, got %+v", result.Steps)
	}
}

func TestIAPSetupCreateLocalizationAndPricingSuccess(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("HOME", t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v2/inAppPurchases" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.InAppPurchaseV2CreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode create payload: %v", err)
			}
			if payload.Data.Attributes.Name != "Pro Lifetime" || payload.Data.Attributes.ProductID != "lifetime" || payload.Data.Attributes.InAppPurchaseType != "NON_CONSUMABLE" {
				t.Fatalf("unexpected create payload: %+v", payload.Data.Attributes)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro Lifetime","productId":"lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/inAppPurchaseLocalizations" {
				t.Fatalf("unexpected localization request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.InAppPurchaseLocalizationCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode localization payload: %v", err)
			}
			if payload.Data.Relationships.InAppPurchaseV2.Data.ID != "iap-1" {
				t.Fatalf("expected localization to target iap-1, got %q", payload.Data.Relationships.InAppPurchaseV2.Data.ID)
			}
			if payload.Data.Attributes.Name != "Second Draft Pro" || payload.Data.Attributes.Locale != "en-US" || payload.Data.Attributes.Description != "Lifetime access" {
				t.Fatalf("unexpected localization payload: %+v", payload.Data.Attributes)
			}
			body := `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1","attributes":{"name":"Second Draft Pro","locale":"en-US","description":"Lifetime access"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
				t.Fatalf("unexpected price-points request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("filter[territory]") != "USA" {
				t.Fatalf("expected USA territory filter, got %q", req.URL.Query().Get("filter[territory]"))
			}
			body := `{"data":[
				{"type":"inAppPurchasePricePoints","id":"pp-199","attributes":{"customerPrice":"1.99","proceeds":"1.39"}},
				{"type":"inAppPurchasePricePoints","id":"pp-399","attributes":{"customerPrice":"3.99","proceeds":"2.79"}},
				{"type":"inAppPurchasePricePoints","id":"pp-499","attributes":{"customerPrice":"4.99","proceeds":"3.49"}}
			],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/inAppPurchasePriceSchedules" {
				t.Fatalf("unexpected price schedule request: %s %s", req.Method, req.URL.Path)
			}
			var payload asc.InAppPurchasePriceScheduleCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode price schedule payload: %v", err)
			}
			if payload.Data.Relationships.InAppPurchase.Data.ID != "iap-1" {
				t.Fatalf("expected price schedule to target iap-1, got %q", payload.Data.Relationships.InAppPurchase.Data.ID)
			}
			if payload.Data.Relationships.BaseTerritory.Data.ID != "USA" {
				t.Fatalf("expected base territory USA, got %q", payload.Data.Relationships.BaseTerritory.Data.ID)
			}
			if len(payload.Included) != 1 || payload.Included[0].Relationships.InAppPurchasePricePoint.Data.ID != "pp-399" {
				t.Fatalf("expected resolved price point pp-399, got %+v", payload.Included)
			}
			if payload.Included[0].Attributes.StartDate != "2026-03-01" {
				t.Fatalf("expected start date 2026-03-01, got %q", payload.Included[0].Attributes.StartDate)
			}
			body := `{"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","attributes":{}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 5:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1" {
				t.Fatalf("unexpected verify iap request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro Lifetime","productId":"lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 6:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/inAppPurchaseLocalizations" {
				t.Fatalf("unexpected verify localizations request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":[{"type":"inAppPurchaseLocalizations","id":"loc-1","attributes":{"name":"Second Draft Pro","locale":"en-US","description":"Lifetime access"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 7:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/iapPriceSchedule" {
				t.Fatalf("unexpected verify schedule request: %s %s", req.Method, req.URL.String())
			}
			query := req.URL.Query()
			if query.Get("include") != "baseTerritory,manualPrices,automaticPrices" {
				t.Fatalf("unexpected schedule include query: %q", query.Get("include"))
			}
			body := `{
				"data":{
					"type":"inAppPurchasePriceSchedules",
					"id":"sched-1",
					"relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}
				},
				"included":[
					{
						"type":"inAppPurchasePrices",
						"id":"price-1",
						"attributes":{"startDate":"2026-03-01","manual":true},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-399"}}
						}
					},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 8:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
				t.Fatalf("unexpected verify price-points request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("filter[territory]") != "USA" {
				t.Fatalf("expected verify territory filter USA, got %q", req.URL.Query().Get("filter[territory]"))
			}
			body := `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-399","attributes":{"customerPrice":"3.99","proceeds":"2.79"}}],"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result iapSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "setup",
			"--app", "app-1",
			"--type", "NON_CONSUMABLE",
			"--reference-name", "Pro Lifetime",
			"--product-id", "lifetime",
			"--locale", "en-US",
			"--display-name", "Second Draft Pro",
			"--description", "Lifetime access",
			"--price", "3.99",
			"--base-territory", "USA",
			"--start-date", "2026-03-01",
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
	if requestCount != 8 {
		t.Fatalf("expected create, localization, resolution, schedule, and verify reads, got %d requests", requestCount)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "ok" {
		t.Fatalf("expected ok status, got %q", result.Status)
	}
	if result.IAPID != "iap-1" || result.LocalizationID != "loc-1" || result.PriceScheduleID != "sched-1" || result.ResolvedPricePointID != "pp-399" {
		t.Fatalf("unexpected setup result: %+v", result)
	}
	if result.Verification.Status != "verified" || !result.Verification.IAPExists {
		t.Fatalf("expected verified setup result, got %+v", result.Verification)
	}
	if result.Verification.LocalizationExists == nil || !*result.Verification.LocalizationExists {
		t.Fatalf("expected localization verification, got %+v", result.Verification)
	}
	if result.Verification.PriceVerified == nil || !*result.Verification.PriceVerified {
		t.Fatalf("expected pricing verification, got %+v", result.Verification)
	}
	if result.Verification.CurrentPrice == nil || result.Verification.CurrentPrice.Amount != "3.99" || result.Verification.CurrentPrice.Currency != "USD" {
		t.Fatalf("expected verified current price 3.99 USD, got %+v", result.Verification.CurrentPrice)
	}
}

func TestIAPSetupRefreshesContextsAcrossPricingAndVerification(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ASC_TIMEOUT", "80ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var priceScheduleDeadlineRemaining time.Duration
	var verifyDeadlineRemaining time.Duration
	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v2/inAppPurchases" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Budget Lifetime","productId":"budget.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
				t.Fatalf("unexpected price-point lookup request: %s %s", req.Method, req.URL.String())
			}
			time.Sleep(60 * time.Millisecond)
			body := `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-399","attributes":{"customerPrice":"3.99","proceeds":"2.79"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/inAppPurchasePriceSchedules" {
				t.Fatalf("unexpected price schedule request: %s %s", req.Method, req.URL.Path)
			}
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected price schedule request to carry a timeout deadline")
			}
			priceScheduleDeadlineRemaining = time.Until(deadline)
			if priceScheduleDeadlineRemaining < 35*time.Millisecond {
				t.Fatalf("expected fresh price schedule context after tier resolution, got only %v remaining", priceScheduleDeadlineRemaining)
			}
			body := `{"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","attributes":{}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1" {
				t.Fatalf("unexpected verify iap request: %s %s", req.Method, req.URL.Path)
			}
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected verify request to carry a timeout deadline")
			}
			verifyDeadlineRemaining = time.Until(deadline)
			if verifyDeadlineRemaining < 35*time.Millisecond {
				t.Fatalf("expected fresh verify context after pricing setup, got only %v remaining", verifyDeadlineRemaining)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Budget Lifetime","productId":"budget.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 5:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/iapPriceSchedule" {
				t.Fatalf("unexpected verify schedule request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":{
					"type":"inAppPurchasePriceSchedules",
					"id":"sched-1",
					"relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}
				},
				"included":[
					{
						"type":"inAppPurchasePrices",
						"id":"price-1",
						"attributes":{"startDate":"2026-03-01","manual":true},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-399"}}
						}
					},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 6:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
				t.Fatalf("unexpected verify price-points request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-399","attributes":{"customerPrice":"3.99","proceeds":"2.79"}}],"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result iapSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "setup",
			"--app", "app-1",
			"--type", "NON_CONSUMABLE",
			"--reference-name", "Budget Lifetime",
			"--product-id", "budget.lifetime",
			"--price", "3.99",
			"--base-territory", "USA",
			"--start-date", "2026-03-01",
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
	if priceScheduleDeadlineRemaining == 0 {
		t.Fatal("expected price schedule request to run")
	}
	if verifyDeadlineRemaining == 0 {
		t.Fatal("expected verify request to run")
	}
	if requestCount != 6 {
		t.Fatalf("expected create, lookup, schedule, and verify reads, got %d requests", requestCount)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "ok" || result.Verification.Status != "verified" {
		t.Fatalf("expected verified setup result, got %+v", result)
	}
}

func TestIAPSetupRefreshesContextsAcrossLocalizationCreationAndVerification(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_TIMEOUT", "80ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var localizationCreateDeadlineRemaining time.Duration
	var localizationVerifyDeadlineRemaining time.Duration
	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v2/inAppPurchases" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.Path)
			}
			// Consume most of the create-step timeout so follow-up requests only
			// succeed when setup refreshes request contexts between steps.
			time.Sleep(60 * time.Millisecond)
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Localized Lifetime","productId":"localized.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/inAppPurchaseLocalizations" {
				t.Fatalf("unexpected localization create request: %s %s", req.Method, req.URL.Path)
			}
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected localization create request to carry a timeout deadline")
			}
			localizationCreateDeadlineRemaining = time.Until(deadline)
			if localizationCreateDeadlineRemaining < 35*time.Millisecond {
				t.Fatalf("expected fresh localization create context, got only %v remaining", localizationCreateDeadlineRemaining)
			}
			body := `{"data":{"type":"inAppPurchaseLocalizations","id":"loc-1","attributes":{"name":"Localized Pro","locale":"en-US","description":"Localized description"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1" {
				t.Fatalf("unexpected verify iap request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Localized Lifetime","productId":"localized.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/inAppPurchaseLocalizations" {
				t.Fatalf("unexpected localization verify request: %s %s", req.Method, req.URL.Path)
			}
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected localization verify request to carry a timeout deadline")
			}
			localizationVerifyDeadlineRemaining = time.Until(deadline)
			if localizationVerifyDeadlineRemaining < 35*time.Millisecond {
				t.Fatalf("expected fresh localization verify context, got only %v remaining", localizationVerifyDeadlineRemaining)
			}
			body := `{"data":[{"type":"inAppPurchaseLocalizations","id":"loc-1","attributes":{"name":"Localized Pro","locale":"en-US","description":"Localized description"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result iapSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "setup",
			"--app", "app-1",
			"--type", "NON_CONSUMABLE",
			"--reference-name", "Localized Lifetime",
			"--product-id", "localized.lifetime",
			"--locale", "en-US",
			"--display-name", "Localized Pro",
			"--description", "Localized description",
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
		t.Fatalf("expected create, localization create, and verify reads, got %d requests", requestCount)
	}
	if localizationCreateDeadlineRemaining == 0 {
		t.Fatal("expected localization create request to run")
	}
	if localizationVerifyDeadlineRemaining == 0 {
		t.Fatal("expected localization verify request to run")
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "ok" || result.Verification.Status != "verified" {
		t.Fatalf("expected verified setup result, got %+v", result)
	}
	if result.Verification.LocalizationExists == nil || !*result.Verification.LocalizationExists {
		t.Fatalf("expected localization verification success, got %+v", result.Verification)
	}
}

func TestIAPSetupFutureStartDateVerificationSucceeds(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("HOME", t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v2/inAppPurchases" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Future Lifetime","productId":"future.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
				t.Fatalf("unexpected price-point lookup request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-399","attributes":{"customerPrice":"3.99","proceeds":"2.79"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/inAppPurchasePriceSchedules" {
				t.Fatalf("unexpected price schedule request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","attributes":{}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1" {
				t.Fatalf("unexpected verify iap request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Future Lifetime","productId":"future.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 5:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/iapPriceSchedule" {
				t.Fatalf("unexpected verify schedule request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":{
					"type":"inAppPurchasePriceSchedules",
					"id":"sched-1",
					"relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}
				},
				"included":[
					{
						"type":"inAppPurchasePrices",
						"id":"price-1",
						"attributes":{"startDate":"2099-01-01","manual":true},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-399"}}
						}
					},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 6:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/iapPriceSchedule" {
				t.Fatalf("unexpected scheduled verification schedule request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":{
					"type":"inAppPurchasePriceSchedules",
					"id":"sched-1",
					"relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}
				},
				"included":[
					{
						"type":"inAppPurchasePrices",
						"id":"price-1",
						"attributes":{"startDate":"2099-01-01","manual":true},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-399"}}
						}
					},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 7:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
				t.Fatalf("unexpected scheduled verification price-point request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-399","attributes":{"customerPrice":"3.99","proceeds":"2.79"}}],"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result iapSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "setup",
			"--app", "app-1",
			"--type", "NON_CONSUMABLE",
			"--reference-name", "Future Lifetime",
			"--product-id", "future.lifetime",
			"--price", "3.99",
			"--base-territory", "USA",
			"--start-date", "2099-01-01",
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
	if requestCount != 7 {
		t.Fatalf("expected create, lookup, schedule, and future verification reads, got %d requests", requestCount)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "ok" {
		t.Fatalf("expected ok status, got %q", result.Status)
	}
	if result.Verification.Status != "verified" {
		t.Fatalf("expected verified future-schedule result, got %+v", result.Verification)
	}
	if result.Verification.PriceVerified == nil || !*result.Verification.PriceVerified {
		t.Fatalf("expected future scheduled price to be verified, got %+v", result.Verification)
	}
	if result.Verification.CurrentPrice != nil {
		t.Fatalf("expected no current price for future schedule, got %+v", result.Verification.CurrentPrice)
	}
	if result.Verification.ScheduledPrice == nil || result.Verification.ScheduledPrice.Amount != "3.99" || result.Verification.ScheduledPrice.Currency != "USD" {
		t.Fatalf("expected scheduled price 3.99 USD, got %+v", result.Verification.ScheduledPrice)
	}
	if result.Verification.ScheduledStartDate != "2099-01-01" {
		t.Fatalf("expected scheduled start date 2099-01-01, got %q", result.Verification.ScheduledStartDate)
	}
}

func TestIAPSetupCurrentPriceVerificationUsesNumericComparison(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("HOME", t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v2/inAppPurchases" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Numeric Lifetime","productId":"numeric.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
				t.Fatalf("unexpected price-point lookup request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-390","attributes":{"customerPrice":"3.90","proceeds":"2.72"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/inAppPurchasePriceSchedules" {
				t.Fatalf("unexpected price schedule request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","attributes":{}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1" {
				t.Fatalf("unexpected verify iap request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Numeric Lifetime","productId":"numeric.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 5:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/iapPriceSchedule" {
				t.Fatalf("unexpected verify schedule request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":{
					"type":"inAppPurchasePriceSchedules",
					"id":"sched-1",
					"relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}
				},
				"included":[
					{
						"type":"inAppPurchasePrices",
						"id":"price-1",
						"attributes":{"startDate":"2026-03-01","manual":true},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-390"}}
						}
					},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				]
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 6:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
				t.Fatalf("unexpected verify price-points request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-390","attributes":{"customerPrice":"3.90","proceeds":"2.72"}}],"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result iapSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "setup",
			"--app", "app-1",
			"--type", "NON_CONSUMABLE",
			"--reference-name", "Numeric Lifetime",
			"--product-id", "numeric.lifetime",
			"--price", "3.9",
			"--base-territory", "USA",
			"--start-date", "2026-03-01",
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
	if requestCount != 6 {
		t.Fatalf("expected create, lookup, schedule, and verification reads, got %d requests", requestCount)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "ok" || result.Verification.Status != "verified" {
		t.Fatalf("expected verified success result, got %+v", result)
	}
	if result.Verification.PriceVerified == nil || !*result.Verification.PriceVerified {
		t.Fatalf("expected numeric price verification success, got %+v", result.Verification)
	}
	if result.Verification.CurrentPrice == nil || result.Verification.CurrentPrice.Amount != "3.90" {
		t.Fatalf("expected verified current price 3.90, got %+v", result.Verification.CurrentPrice)
	}
}

func TestIAPSetupTierVerificationFailsOnWrongAppliedPrice(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("HOME", t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v2/inAppPurchases" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Tier Lifetime","productId":"tier.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
				t.Fatalf("unexpected setup tier lookup request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[
				{"type":"inAppPurchasePricePoints","id":"pp-199","attributes":{"customerPrice":"1.99","proceeds":"1.39"}},
				{"type":"inAppPurchasePricePoints","id":"pp-299","attributes":{"customerPrice":"2.99","proceeds":"2.09"}}
			],"links":{"next":""}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 3:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/inAppPurchasePriceSchedules" {
				t.Fatalf("unexpected price schedule request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","attributes":{}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1" {
				t.Fatalf("unexpected verify iap request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Tier Lifetime","productId":"tier.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 5:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
				t.Fatalf("unexpected verify tier lookup request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[
				{"type":"inAppPurchasePricePoints","id":"pp-199","attributes":{"customerPrice":"1.99","proceeds":"1.39"}},
				{"type":"inAppPurchasePricePoints","id":"pp-299","attributes":{"customerPrice":"2.99","proceeds":"2.09"}}
			],"links":{"next":""}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 6:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/iapPriceSchedule" {
				t.Fatalf("unexpected verify schedule request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}},
				"included":[
					{"type":"inAppPurchasePrices","id":"price-1","attributes":{"startDate":"2026-03-01","manual":true},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-199"}}}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				]
			}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 7:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
				t.Fatalf("unexpected verify current price-point request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-199","attributes":{"customerPrice":"1.99","proceeds":"1.39"}}],"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],"links":{"next":""}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result iapSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "setup",
			"--app", "app-1",
			"--type", "NON_CONSUMABLE",
			"--reference-name", "Tier Lifetime",
			"--product-id", "tier.lifetime",
			"--tier", "2",
			"--base-territory", "USA",
			"--start-date", "2026-03-01",
			"--refresh",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err == nil {
			t.Fatal("expected verification error, got nil")
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "error" || result.FailedStep != "verify_state" {
		t.Fatalf("expected verify_state failure, got %+v", result)
	}
	if !strings.Contains(result.Error, "current price mismatch") {
		t.Fatalf("expected numeric verification mismatch, got %+v", result)
	}
}

func TestIAPSetupExplicitPricePointVerificationFailsOnWrongAppliedPrice(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("HOME", t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodPost || req.URL.Path != "/v2/inAppPurchases" {
				t.Fatalf("unexpected create request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Point Lifetime","productId":"point.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/inAppPurchasePriceSchedules" {
				t.Fatalf("unexpected price schedule request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","attributes":{}}}`
			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1" {
				t.Fatalf("unexpected verify iap request: %s %s", req.Method, req.URL.Path)
			}
			body := `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Point Lifetime","productId":"point.lifetime","inAppPurchaseType":"NON_CONSUMABLE"}}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
				t.Fatalf("unexpected verify explicit point lookup request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-399","attributes":{"customerPrice":"3.99","proceeds":"2.79"}}],"links":{"next":""}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 5:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/iapPriceSchedule" {
				t.Fatalf("unexpected verify schedule request: %s %s", req.Method, req.URL.String())
			}
			body := `{
				"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","relationships":{"baseTerritory":{"data":{"type":"territories","id":"USA"}}}},
				"included":[
					{"type":"inAppPurchasePrices","id":"price-1","attributes":{"startDate":"2026-03-01","manual":true},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"pp-199"}}}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				]
			}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		case 6:
			if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/iap-1/pricePoints" {
				t.Fatalf("unexpected verify current price-point request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-199","attributes":{"customerPrice":"1.99","proceeds":"1.39"}}],"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],"links":{"next":""}}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var result iapSetupOutput
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "setup",
			"--app", "app-1",
			"--type", "NON_CONSUMABLE",
			"--reference-name", "Point Lifetime",
			"--product-id", "point.lifetime",
			"--price-point-id", "pp-399",
			"--base-territory", "USA",
			"--start-date", "2026-03-01",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err == nil {
			t.Fatal("expected verification error, got nil")
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse setup result: %v\nstdout=%q", err, stdout)
	}
	if result.Status != "error" || result.FailedStep != "verify_state" {
		t.Fatalf("expected verify_state failure, got %+v", result)
	}
	if !strings.Contains(result.Error, "current price mismatch") {
		t.Fatalf("expected explicit price-point mismatch, got %+v", result)
	}
}

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

type iapSetupOutput struct {
	Status               string `json:"status"`
	IAPID                string `json:"iapId"`
	LocalizationID       string `json:"localizationId,omitempty"`
	PriceScheduleID      string `json:"priceScheduleId,omitempty"`
	ResolvedPricePointID string `json:"resolvedPricePointId,omitempty"`
	FailedStep           string `json:"failedStep,omitempty"`
	Steps                []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
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
		if req.Method != http.MethodPost || req.URL.Path != "/v2/inAppPurchases" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
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
	if requestCount != 1 {
		t.Fatalf("expected one request, got %d", requestCount)
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
	if requestCount != 4 {
		t.Fatalf("expected four requests, got %d", requestCount)
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
}

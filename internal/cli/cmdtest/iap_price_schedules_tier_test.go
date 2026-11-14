package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestIAPPriceSchedulesCreateValidationErrors(t *testing.T) {
	setupUsageExitCodeEnv(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "tier with prices",
			args:    []string{"iap", "pricing", "schedules", "create", "--iap-id", "9000000003", "--base-territory", "USA", "--tier", "5", "--prices", "PP:2026-03-01"},
			wantErr: "--prices and --tier/--price are mutually exclusive",
		},
		{
			name:    "tier with price",
			args:    []string{"iap", "pricing", "schedules", "create", "--iap-id", "9000000003", "--base-territory", "USA", "--tier", "5", "--price", "4.99"},
			wantErr: "--tier and --price are mutually exclusive",
		},
		{
			name:    "non-numeric price",
			args:    []string{"iap", "pricing", "schedules", "create", "--iap-id", "9000000003", "--base-territory", "USA", "--price", "abc"},
			wantErr: "--price must be a number",
		},
		{
			name:    "invalid tier start date",
			args:    []string{"iap", "pricing", "schedules", "create", "--iap-id", "9000000003", "--base-territory", "USA", "--tier", "1", "--start-date", "invalid"},
			wantErr: "--start-date must be in YYYY-MM-DD format",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitCode(t, test.args, test.wantErr)
		})
	}
}

func TestIAPPriceSchedulesCreate_TierUsesIAPPricePoints(t *testing.T) {
	setupAuth(t)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000003/pricePoints", "filter[territory]=USA", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"inAppPurchasePricePoints","id":"iap-pp-1","attributes":{"customerPrice":"0.99","proceeds":"0.70"}},
				{"type":"inAppPurchasePricePoints","id":"iap-pp-2","attributes":{"customerPrice":"1.99","proceeds":"1.40"}},
				{"type":"inAppPurchasePricePoints","id":"iap-pp-3","attributes":{"customerPrice":"2.99","proceeds":"2.10"}}
			],
			"links":{"next":""}
		}`))
	reg.Register(asctest.POST("/v1/inAppPurchasePriceSchedules").JSONBody(`{"data":{"type":"inAppPurchasePriceSchedules","relationships":{"inAppPurchase":{"data":{"type":"inAppPurchases","id":"9000000003"}},"baseTerritory":{"data":{"type":"territories","id":"USA"}},"manualPrices":{"data":[{"type":"inAppPurchasePrices","id":"${local-manual-price-1}"}]}}},"included":[{"type":"inAppPurchasePrices","id":"${local-manual-price-1}","attributes":{"startDate":"2026-03-01"},"relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"9000000003"}},"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"iap-pp-2"}}}}]}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","attributes":{}}}`))

	t.Setenv("HOME", t.TempDir())
	stdout, stderr, err := runCommand(t, []string{
		"iap", "pricing", "schedules", "create",
		"--iap-id", "9000000003",
		"--base-territory", "USA",
		"--tier", "2",
		"--start-date", "2026-03-01",
		"--refresh",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"sched-1"`) {
		t.Fatalf("expected schedule output, got %q", stdout)
	}
}

func TestIAPPriceSchedulesCreateUsesCanonicalErrorPrefix(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/inAppPurchasePriceSchedules").JSONBody(`{"data":{"type":"inAppPurchasePriceSchedules","relationships":{"inAppPurchase":{"data":{"type":"inAppPurchases","id":"9000000003"}},"baseTerritory":{"data":{"type":"territories","id":"USA"}},"manualPrices":{"data":[{"type":"inAppPurchasePrices","id":"${local-manual-price-1}"}]}}},"included":[{"type":"inAppPurchasePrices","id":"${local-manual-price-1}","attributes":{},"relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"9000000003"}},"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"PP1"}}}}]}`),
		asctest.JSON(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"boom"}]}`))

	_, _, err := runCommand(t, []string{
		"iap", "pricing", "schedules", "create",
		"--iap-id", "9000000003",
		"--base-territory", "USA",
		"--prices", "PP1",
	})
	if err == nil {
		t.Fatal("expected create error")
	}
	if !strings.Contains(err.Error(), "iap pricing schedules create: failed to create:") {
		t.Fatalf("expected canonical error prefix, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "iap price-schedules create:") {
		t.Fatalf("expected legacy error prefix to be removed, got %q", err.Error())
	}
}

func TestIAPPriceSchedulesCreateRefreshesContextAfterTierResolution(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "80ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var createDeadlineRemaining time.Duration
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/inAppPurchases/9000000003/pricePoints"):
			time.Sleep(60 * time.Millisecond)
			body := `{
				"data":[
					{"type":"inAppPurchasePricePoints","id":"iap-pp-1","attributes":{"customerPrice":"0.99","proceeds":"0.70"}},
					{"type":"inAppPurchasePricePoints","id":"iap-pp-2","attributes":{"customerPrice":"1.99","proceeds":"1.40"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/inAppPurchasePriceSchedules"):
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatal("expected create request to carry a timeout deadline")
			}
			createDeadlineRemaining = time.Until(deadline)
			if createDeadlineRemaining != 80*time.Millisecond {
				t.Fatalf("expected fresh create context after tier resolution, got %v remaining, want 80ms", createDeadlineRemaining)
			}

			resp := `{"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","attributes":{}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(resp)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	t.Setenv("HOME", t.TempDir())
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "pricing", "schedules", "create",
			"--iap-id", "9000000003",
			"--base-territory", "USA",
			"--tier", "2",
			"--start-date", "2026-03-01",
			"--refresh",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		var runErr error
		synctest.Test(t, func(*testing.T) { runErr = root.Run(context.Background()) })
		if runErr != nil {
			t.Fatalf("run error: %v", runErr)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if createDeadlineRemaining == 0 {
		t.Fatal("expected create request to run")
	}
	if !strings.Contains(stdout, `"id":"sched-1"`) {
		t.Fatalf("expected schedule output, got %q", stdout)
	}
}

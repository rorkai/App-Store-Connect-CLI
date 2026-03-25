package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestIAPPriceSchedulesGetRejectsInvalidInclude(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "pricing", "schedules", "view",
			"--schedule-id", "schedule-1",
			"--include", "unknown",
		}); err != nil {
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
	if !strings.Contains(stderr, "--include must be one of: baseTerritory, manualPrices, automaticPrices") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestIAPPriceSchedulesGetByIDWithIncludeOptions(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/inAppPurchasePriceSchedules/schedule-1" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}

		query := req.URL.Query()
		if query.Get("include") != "baseTerritory,manualPrices,automaticPrices" {
			t.Fatalf("unexpected include query: %q", query.Get("include"))
		}
		if query.Get("fields[inAppPurchasePriceSchedules]") != "baseTerritory,manualPrices,automaticPrices" {
			t.Fatalf("unexpected schedule fields query: %q", query.Get("fields[inAppPurchasePriceSchedules]"))
		}
		if query.Get("fields[territories]") != "currency" {
			t.Fatalf("unexpected territory fields query: %q", query.Get("fields[territories]"))
		}
		if query.Get("fields[inAppPurchasePrices]") != "startDate,endDate,manual,inAppPurchasePricePoint,territory" {
			t.Fatalf("unexpected price fields query: %q", query.Get("fields[inAppPurchasePrices]"))
		}
		if query.Get("limit[manualPrices]") != "25" {
			t.Fatalf("unexpected manual limit query: %q", query.Get("limit[manualPrices]"))
		}
		if query.Get("limit[automaticPrices]") != "25" {
			t.Fatalf("unexpected automatic limit query: %q", query.Get("limit[automaticPrices]"))
		}

		body := `{"data":{"type":"inAppPurchasePriceSchedules","id":"schedule-1"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "pricing", "schedules", "view",
			"--schedule-id", "schedule-1",
			"--include", "baseTerritory,manualPrices,automaticPrices",
			"--schedule-fields", "baseTerritory,manualPrices,automaticPrices",
			"--territory-fields", "currency",
			"--price-fields", "startDate,endDate,manual,inAppPurchasePricePoint,territory",
			"--manual-prices-limit", "25",
			"--automatic-prices-limit", "25",
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
	if !strings.Contains(stdout, `"id":"schedule-1"`) {
		t.Fatalf("expected schedule id in output, got %q", stdout)
	}
}

func TestIAPPriceSchedulesGetUsesCanonicalErrorPrefix(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"boom"}]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"iap", "pricing", "schedules", "view",
		"--iap-id", "iap-1",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := root.Run(context.Background())
	if err == nil {
		t.Fatal("expected fetch error")
	}
	if !strings.Contains(err.Error(), "iap pricing schedules view: failed to fetch:") {
		t.Fatalf("expected canonical error prefix, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "iap price-schedules get:") {
		t.Fatalf("expected legacy error prefix to be removed, got %q", err.Error())
	}
}

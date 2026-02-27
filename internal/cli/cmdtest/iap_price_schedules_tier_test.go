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

func TestIAPPriceSchedulesCreate_TierAndPricesMutualExclusion(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "price-schedules", "create",
			"--iap-id", "IAP_ID",
			"--base-territory", "USA",
			"--tier", "5",
			"--prices", "PP:2026-03-01",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %q", stderr)
	}
}

func TestIAPPriceSchedulesCreate_TierAndPriceMutualExclusion(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "price-schedules", "create",
			"--iap-id", "IAP_ID",
			"--base-territory", "USA",
			"--tier", "5",
			"--price", "4.99",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %q", stderr)
	}
}

func TestIAPPriceSchedulesCreate_TierUsesIAPPricePoints(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var resolvedPricePointID string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/inAppPurchases/IAP_ID/pricePoints"):
			body := `{
				"data":[
					{"type":"inAppPurchasePricePoints","id":"iap-pp-1","attributes":{"customerPrice":"0.99","proceeds":"0.70"}},
					{"type":"inAppPurchasePricePoints","id":"iap-pp-2","attributes":{"customerPrice":"1.99","proceeds":"1.40"}},
					{"type":"inAppPurchasePricePoints","id":"iap-pp-3","attributes":{"customerPrice":"2.99","proceeds":"2.10"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/apps/"):
			t.Fatalf("unexpected app price points request: %s", req.URL.Path)
			return nil, nil
		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/inAppPurchasePriceSchedules"):
			bodyBytes, _ := io.ReadAll(req.Body)
			bodyStr := string(bodyBytes)
			if strings.Contains(bodyStr, "iap-pp-2") {
				resolvedPricePointID = "iap-pp-2"
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
			"iap", "price-schedules", "create",
			"--iap-id", "IAP_ID",
			"--base-territory", "USA",
			"--tier", "2",
			"--start-date", "2026-03-01",
			"--refresh",
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
	if resolvedPricePointID != "iap-pp-2" {
		t.Fatalf("expected tier 2 to resolve iap-pp-2, got %q", resolvedPricePointID)
	}
	if !strings.Contains(stdout, `"id":"sched-1"`) {
		t.Fatalf("expected schedule output, got %q", stdout)
	}
}

func TestIAPPriceSchedulesCreate_InvalidPriceValue(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "price-schedules", "create",
			"--iap-id", "IAP_ID",
			"--base-territory", "USA",
			"--price", "abc",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--price must be a number") {
		t.Fatalf("expected invalid --price error, got %q", stderr)
	}
}

func TestIAPPriceSchedulesCreate_InvalidTierStartDate(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "price-schedules", "create",
			"--iap-id", "IAP_ID",
			"--base-territory", "USA",
			"--tier", "1",
			"--start-date", "invalid",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--start-date must be in YYYY-MM-DD format") {
		t.Fatalf("expected invalid --start-date error, got %q", stderr)
	}
}

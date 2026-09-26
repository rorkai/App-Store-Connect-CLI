package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestIAPPriceSchedulesCreateSendsNormalizedBaseTerritory(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var sentBaseTerritory string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/inAppPurchasePriceSchedules" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		var payload asc.InAppPurchasePriceScheduleCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		sentBaseTerritory = payload.Data.Relationships.BaseTerritory.Data.ID
		return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","attributes":{}}}`), nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "pricing", "schedules", "create",
			"--iap-id", "9000000003",
			"--base-territory", "US",
			"--prices", "PP1:2026-03-01",
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
	if sentBaseTerritory != "USA" {
		t.Fatalf("expected baseTerritory USA, got %q", sentBaseTerritory)
	}
	if !strings.Contains(stdout, `"id":"sched-1"`) {
		t.Fatalf("expected schedule output, got %q", stdout)
	}
}

func TestIAPPriceSchedulesCreateResolvesTiersWithNormalizedBaseTerritory(t *testing.T) {
	setupAuth(t)
	t.Setenv("HOME", t.TempDir())

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var (
		tierTerritory     string
		sentBaseTerritory string
	)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/inAppPurchases/9000000003/pricePoints"):
			tierTerritory = req.URL.Query().Get("filter[territory]")
			return jsonHTTPResponse(http.StatusOK, `{"data":[{"type":"inAppPurchasePricePoints","id":"iap-pp-1","attributes":{"customerPrice":"0.99","proceeds":"0.70"}}],"links":{"next":""}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/inAppPurchasePriceSchedules":
			var payload asc.InAppPurchasePriceScheduleCreateRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			sentBaseTerritory = payload.Data.Relationships.BaseTerritory.Data.ID
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","attributes":{}}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "pricing", "schedules", "create",
			"--iap-id", "9000000003",
			"--base-territory", "United States",
			"--tier", "1",
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
	if tierTerritory != "USA" {
		t.Fatalf("expected tier resolution filter[territory]=USA, got %q", tierTerritory)
	}
	if sentBaseTerritory != "USA" {
		t.Fatalf("expected baseTerritory USA, got %q", sentBaseTerritory)
	}
}

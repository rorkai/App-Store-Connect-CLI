package cmdtest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestIAPPricePointsListPaginateUsesMaxPageSizeAndPerPageTimeout(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "120ms")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	const base = "https://api.appstoreconnect.apple.com/v2/inAppPurchases/6760268101/pricePoints"
	pages := map[string]string{
		"limit=8000":           `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-1"}],"links":{"next":"` + base + `?cursor=AQ&limit=8000"}}`,
		"cursor=AQ&limit=8000": `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-2"}],"links":{"next":"` + base + `?cursor=BQ&limit=8000"}}`,
		"cursor=BQ&limit=8000": `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-3"}],"links":{}}`,
	}
	requests := 0
	var previousPageCtx context.Context
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if deadline, ok := req.Context().Deadline(); !ok || time.Until(deadline) != 120*time.Millisecond {
			t.Errorf("page %d did not get a 120ms deadline: ok=%v remaining=%s", requests, ok, time.Until(deadline))
			return nil, context.DeadlineExceeded
		}
		if previousPageCtx != nil && previousPageCtx.Err() == nil {
			t.Errorf("page %d shares a deadline with page %d", requests, requests-1)
			return nil, context.DeadlineExceeded
		}
		previousPageCtx = req.Context()

		body, ok := pages[req.URL.RawQuery]
		if req.Method != http.MethodGet || req.URL.Path != "/v2/inAppPurchases/6760268101/pricePoints" || !ok {
			t.Errorf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, errors.New("unexpected test transport request")
		}
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
			"iap", "pricing", "price-points", "list",
			"--iap-id", "6760268101",
			"--paginate",
			"--output", "json",
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
	if requests != 3 {
		t.Fatalf("expected 3 paginated requests, got %d", requests)
	}
	for _, id := range []string{"pp-1", "pp-2", "pp-3"} {
		if !strings.Contains(stdout, `"id":"`+id+`"`) {
			t.Fatalf("expected %s in aggregated output, got %q", id, stdout)
		}
	}
}

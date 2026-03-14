package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSubscriptionsPricePointsListPaginateUsesPerPageTimeout(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_TIMEOUT", "120ms")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		time.Sleep(70 * time.Millisecond)

		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}

		switch req.URL.RawQuery {
		case "limit=200":
			if req.URL.Path != "/v1/subscriptions/sub-1/pricePoints" {
				t.Fatalf("unexpected first page path: %s", req.URL.Path)
			}
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=AQ&limit=200"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "cursor=AQ&limit=200":
			if req.URL.Path != "/v1/subscriptions/sub-1/pricePoints" {
				t.Fatalf("unexpected second page path: %s", req.URL.Path)
			}
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-2"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=BQ&limit=200"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "cursor=BQ&limit=200":
			if req.URL.Path != "/v1/subscriptions/sub-1/pricePoints" {
				t.Fatalf("unexpected third page path: %s", req.URL.Path)
			}
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-3"}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request path/query: %s?%s", req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "price-points", "list",
			"--subscription-id", "sub-1",
			"--paginate",
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
	if requests != 3 {
		t.Fatalf("expected 3 paginated requests, got %d", requests)
	}
	if !strings.Contains(stdout, `"id":"pp-1"`) || !strings.Contains(stdout, `"id":"pp-2"`) || !strings.Contains(stdout, `"id":"pp-3"`) {
		t.Fatalf("expected aggregated paginated output, got %q", stdout)
	}
}

func TestSubscriptionsPricePointsListStreamRequiresPaginate(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "price-points", "list",
			"--subscription-id", "sub-1",
			"--stream",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatalf("expected error for --stream without --paginate")
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--stream requires --paginate") {
		t.Fatalf("expected --stream requires --paginate error, got %q", stderr)
	}
}

func TestSubscriptionsPricePointsListStreamOutput(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/subscriptions/sub-1/pricePoints" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}

		query := req.URL.RawQuery
		switch {
		case strings.Contains(query, "limit=200") && !strings.Contains(query, "cursor="):
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-1","attributes":{"customerPrice":"1.99"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=AQ&limit=200"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case strings.Contains(query, "cursor=AQ"):
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-2","attributes":{"customerPrice":"2.99"}}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected query: %s", query)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "price-points", "list",
			"--subscription-id", "sub-1",
			"--paginate",
			"--stream",
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

	// Streaming should produce multiple JSON lines (NDJSON), not one aggregated blob
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines (one per page), got %d: %q", len(lines), stdout)
	}
	if !strings.Contains(lines[0], `"id":"pp-1"`) {
		t.Fatalf("expected first page to contain pp-1, got %q", lines[0])
	}
	if !strings.Contains(lines[1], `"id":"pp-2"`) {
		t.Fatalf("expected second page to contain pp-2, got %q", lines[1])
	}
}

func TestSubscriptionsPricePointsListStreamRejectsRepeatedNextURL(t *testing.T) {
	setupAuth(t)

	const repeatedNextURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=AQ&limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/pricePoints" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-1"}],"links":{"next":"` + repeatedNextURL + `"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != repeatedNextURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-2"}],"links":{"next":"` + repeatedNextURL + `"}}`
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

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "price-points", "list",
			"--subscription-id", "sub-1",
			"--paginate",
			"--stream",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "subscriptions pricing price-points list:") {
		t.Fatalf("expected subscriptions pricing price-points list context, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "detected repeated pagination URL") {
		t.Fatalf("expected repeated pagination URL error, got %v", runErr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two streamed pages before repeated URL detection, got %d: %q", len(lines), stdout)
	}
	if !strings.Contains(lines[0], `"id":"pp-1"`) || !strings.Contains(lines[1], `"id":"pp-2"`) {
		t.Fatalf("expected streamed pages pp-1 and pp-2, got %q", stdout)
	}
}

func TestSubscriptionsPricePointsListStreamReturnsSecondPageFailure(t *testing.T) {
	setupAuth(t)

	const nextURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=AQ&limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/sub-1/pricePoints" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-1"}],"links":{"next":"` + nextURL + `"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != nextURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"errors":[{"status":"500","title":"Server Error","detail":"page 2 failed"}]}`
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
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

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "price-points", "list",
			"--subscription-id", "sub-1",
			"--paginate",
			"--stream",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), "subscriptions pricing price-points list:") {
		t.Fatalf("expected subscriptions pricing price-points list context, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "page 2 failed") {
		t.Fatalf("expected second page failure detail, got %v", runErr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one streamed page before second-page failure, got %d: %q", len(lines), stdout)
	}
	if !strings.Contains(lines[0], `"id":"pp-1"`) {
		t.Fatalf("expected first streamed page to contain pp-1, got %q", stdout)
	}
}

func TestSubscriptionsPricePointsListRejectsInvalidNextURL(t *testing.T) {
	tests := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name:    "invalid scheme",
			next:    "http://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=AQ",
			wantErr: "subscriptions pricing price-points list: --next must be an App Store Connect URL",
		},
		{
			name:    "malformed URL",
			next:    "https://api.appstoreconnect.apple.com/%zz",
			wantErr: "subscriptions pricing price-points list: --next must be a valid URL:",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"subscriptions", "pricing", "price-points", "list",
					"--next", test.next,
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(runErr.Error(), test.wantErr) {
				t.Fatalf("expected error %q, got %v", test.wantErr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestSubscriptionsPricePointsListPaginateFromNextWithoutSubscription(t *testing.T) {
	setupAuth(t)

	const firstURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=BQ&limit=200"

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.String() != firstURL {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-next-1"}],"links":{"next":"` + secondURL + `"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.String() != secondURL {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-next-2"}],"links":{"next":""}}`
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

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "price-points", "list",
			"--paginate",
			"--next", firstURL,
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
	if !strings.Contains(stdout, `"id":"pp-next-1"`) || !strings.Contains(stdout, `"id":"pp-next-2"`) {
		t.Fatalf("expected paginated price points in output, got %q", stdout)
	}
}

func TestSubscriptionsPricePointsListTerritoryFilter(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/subscriptions/sub-1/pricePoints" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		query := req.URL.Query()
		if query.Get("filter[territory]") != "USA" {
			t.Fatalf("expected filter[territory]=USA, got %q", query.Get("filter[territory]"))
		}

		body := `{"data":[{"type":"subscriptionPricePoints","id":"pp-usa","attributes":{"customerPrice":"9.99","proceeds":"8.49"}}],"links":{}}`
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
			"subscriptions", "pricing", "price-points", "list",
			"--subscription-id", "sub-1",
			"--territory", "USA",
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
	if !strings.Contains(stdout, `"id":"pp-usa"`) {
		t.Fatalf("expected filtered output, got %q", stdout)
	}
}

func TestSubscriptionsPricePointsEqualizationsPaginate(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++

		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}

		switch req.URL.RawQuery {
		case "limit=200":
			if req.URL.Path != "/v1/subscriptionPricePoints/pp-1/equalizations" {
				t.Fatalf("unexpected first page path: %s", req.URL.Path)
			}
			body := `{"data":[{"type":"subscriptionPricePointEqualizations","id":"eq-1","attributes":{"territory":"USA"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptionPricePoints/pp-1/equalizations?cursor=AQ&limit=200"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "cursor=AQ&limit=200":
			if req.URL.Path != "/v1/subscriptionPricePoints/pp-1/equalizations" {
				t.Fatalf("unexpected second page path: %s", req.URL.Path)
			}
			body := `{"data":[{"type":"subscriptionPricePointEqualizations","id":"eq-2","attributes":{"territory":"GBR"}}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request path/query: %s?%s", req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "price-points", "equalizations",
			"--price-point-id", "pp-1",
			"--paginate",
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
	if requests != 2 {
		t.Fatalf("expected 2 paginated requests, got %d", requests)
	}
	if !strings.Contains(stdout, `"id":"eq-1"`) || !strings.Contains(stdout, `"id":"eq-2"`) {
		t.Fatalf("expected aggregated paginated output, got %q", stdout)
	}
}

func TestSubscriptionsPricePointsEqualizationsWithoutPaginateUsesSinglePage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++

		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/subscriptionPricePoints/pp-1/equalizations" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		if req.URL.RawQuery != "" {
			t.Fatalf("expected empty query without --paginate, got %q", req.URL.RawQuery)
		}

		// Include next to ensure command does not follow when --paginate is absent.
		body := `{"data":[{"type":"subscriptionPricePointEqualizations","id":"eq-1","attributes":{"territory":"USA"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptionPricePoints/pp-1/equalizations?cursor=AQ&limit=200"}}`
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
			"subscriptions", "pricing", "price-points", "equalizations",
			"--price-point-id", "pp-1",
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
	if requests != 1 {
		t.Fatalf("expected exactly one request without --paginate, got %d", requests)
	}
	if !strings.Contains(stdout, `"id":"eq-1"`) {
		t.Fatalf("expected first page result in output, got %q", stdout)
	}
}

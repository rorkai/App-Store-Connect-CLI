package itunes

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	retryLookupPayload = `{"resultCount":1,"results":[{"trackId":123,` +
		`"trackName":"Focus","bundleId":"com.example.focus",` +
		`"averageUserRating":4.5,"userRatingCount":10}]}`
	retryHistogramHTML            = `<div class="vote"><span class="total">61</span></div>`
	retryStorefrontSearchPayload  = `{"pageData":{"bubbles":[{"results":[{"id":"1234567890","entity":"tvSoftware"}]}]}}`
	retryStorefrontCustomerPrefix = "/customer-reviews/"
)

// retryTestServer answers every public endpoint the client uses, failing the
// first failures requests with a fixed status.
type retryTestServer struct {
	t          *testing.T
	remaining  atomic.Int32
	requests   atomic.Int32
	status     int
	retryAfter func() string
}

func newRetryTestServer(t *testing.T, failures int32, status int, retryAfter func() string) (*httptest.Server, *retryTestServer) {
	t.Helper()

	state := &retryTestServer{t: t, status: status, retryAfter: retryAfter}
	state.remaining.Store(failures)
	server := httptest.NewServer(state)
	t.Cleanup(server.Close)
	return server, state
}

func (s *retryTestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.requests.Add(1)

	if s.remaining.Add(-1) >= 0 {
		if s.retryAfter != nil {
			if value := s.retryAfter(); value != "" {
				w.Header().Set("Retry-After", value)
			}
		}
		w.WriteHeader(s.status)
		return
	}

	switch {
	case r.URL.Path == "/search", r.URL.Path == "/lookup":
		w.Header().Set("Content-Type", "application/json")
		writeBody(s.t, w, retryLookupPayload)
	case strings.Contains(r.URL.Path, retryStorefrontCustomerPrefix):
		w.Header().Set("Content-Type", "text/html")
		writeBody(s.t, w, retryHistogramHTML)
	case strings.HasSuffix(r.URL.Path, storefrontSearchPath):
		w.Header().Set("Content-Type", "application/json")
		writeBody(s.t, w, retryStorefrontSearchPayload)
	default:
		http.NotFound(w, r)
	}
}

func retryTestClient(server *httptest.Server) *Client {
	return &Client{
		HTTPClient:              server.Client(),
		BaseURL:                 server.URL,
		StorefrontSearchBaseURL: server.URL,
	}
}

type retryRoundTripFunc func(*http.Request) (*http.Response, error)

func (f retryRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// useFastRetries pins retry options so retry tests stay fast and hermetic.
func useFastRetries(t *testing.T, maxRetries string) {
	t.Helper()

	t.Setenv("ASC_MAX_RETRIES", maxRetries)
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "5ms")
}

func TestPublicRequestsRetryTransientStatuses(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		wantRequests int32
		call         func(*Client) error
	}{
		{
			name:         "search rate limited",
			status:       http.StatusTooManyRequests,
			wantRequests: 2,
			call: func(client *Client) error {
				results, err := client.SearchApps(context.Background(), "focus", "us", 20)
				if err != nil {
					return err
				}
				if len(results) != 1 || results[0].AppID != 123 {
					t.Fatalf("SearchApps() = %+v, want the retried payload", results)
				}
				return nil
			},
		},
		{
			name:         "search server error",
			status:       http.StatusInternalServerError,
			wantRequests: 2,
			call: func(client *Client) error {
				_, err := client.SearchApps(context.Background(), "focus", "us", 20)
				return err
			},
		},
		{
			name:         "lookup service unavailable",
			status:       http.StatusServiceUnavailable,
			wantRequests: 2,
			call: func(client *Client) error {
				app, err := client.LookupApp(context.Background(), "123", LookupOptions{Country: "us"})
				if err != nil {
					return err
				}
				if app.AppID != 123 {
					t.Fatalf("LookupApp() = %+v, want app 123", app)
				}
				return nil
			},
		},
		{
			name:         "lookup by bundle ID rate limited",
			status:       http.StatusTooManyRequests,
			wantRequests: 2,
			call: func(client *Client) error {
				app, err := client.LookupAppByBundleID(context.Background(), "com.example.focus", LookupOptions{})
				if err != nil {
					return err
				}
				if app == nil || app.AppID != 123 {
					t.Fatalf("LookupAppByBundleID() = %+v, want app 123", app)
				}
				return nil
			},
		},
		{
			name:         "ratings bad gateway",
			status:       http.StatusBadGateway,
			wantRequests: 3,
			call: func(client *Client) error {
				ratings, err := client.GetRatings(context.Background(), "123", "us")
				if err != nil {
					return err
				}
				if ratings.Histogram[5] != 61 {
					t.Fatalf("GetRatings() histogram = %v, want 5-star count 61", ratings.Histogram)
				}
				return nil
			},
		},
		{
			name:         "storefront search rate limited",
			status:       http.StatusTooManyRequests,
			wantRequests: 2,
			call: func(client *Client) error {
				result, err := client.RankApp(
					context.Background(),
					"1234567890",
					"focus timer",
					"us",
					PublicSearchPlatformTVOS,
				)
				if err != nil {
					return err
				}
				if !result.Found || result.Rank == nil || *result.Rank != 1 {
					t.Fatalf("RankApp() = %+v, want rank 1", result)
				}
				return nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useFastRetries(t, "3")

			server, state := newRetryTestServer(t, 1, test.status, nil)
			if err := test.call(retryTestClient(server)); err != nil {
				t.Fatalf("call after %d retry: %v", 1, err)
			}
			if got := state.requests.Load(); got != test.wantRequests {
				t.Fatalf("requests = %d, want %d", got, test.wantRequests)
			}
		})
	}
}

func TestPublicRequestsDoNotRetryTerminalStatuses(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantError string
		call      func(*Client) error
	}{
		{
			name:      "lookup not found",
			status:    http.StatusNotFound,
			wantError: "lookup request returned status 404",
			call: func(client *Client) error {
				_, err := client.LookupApps(context.Background(), []string{"123"}, LookupOptions{})
				return err
			},
		},
		{
			name:      "search forbidden",
			status:    http.StatusForbidden,
			wantError: "search request returned status 403",
			call: func(client *Client) error {
				_, err := client.SearchApps(context.Background(), "focus", "us", 20)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useFastRetries(t, "3")

			server, state := newRetryTestServer(t, 1, test.status, nil)
			err := test.call(retryTestClient(server))
			if err == nil {
				t.Fatal("expected terminal failure")
			}
			if err.Error() != test.wantError {
				t.Fatalf("error = %q, want %q", err, test.wantError)
			}
			if got := state.requests.Load(); got != 1 {
				t.Fatalf("requests = %d, want 1 (no retry)", got)
			}

			var statusError interface{ HTTPStatusCode() int }
			if !errors.As(err, &statusError) || statusError.HTTPStatusCode() != test.status {
				t.Fatalf("error %T does not expose status %d", err, test.status)
			}
			var storefrontError interface{ PublicStorefrontError() bool }
			if !errors.As(err, &storefrontError) || !storefrontError.PublicStorefrontError() {
				t.Fatalf("error %T does not retain public storefront semantics", err)
			}
		})
	}
}

func TestPublicRetryHonorsRetryAfterHeader(t *testing.T) {
	tests := []struct {
		name       string
		retryAfter func() string
	}{
		{
			name:       "seconds",
			retryAfter: func() string { return "30" },
		},
		{
			name: "http date",
			retryAfter: func() string {
				return time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_MAX_RETRIES", "3")
			t.Setenv("ASC_BASE_DELAY", "1ms")
			t.Setenv("ASC_MAX_DELAY", "50ms")

			server, state := newRetryTestServer(t, 1, http.StatusTooManyRequests, test.retryAfter)
			start := time.Now()
			if _, err := retryTestClient(server).SearchApps(context.Background(), "focus", "us", 20); err != nil {
				t.Fatalf("SearchApps() error: %v", err)
			}
			elapsed := time.Since(start)

			if got := state.requests.Load(); got != 2 {
				t.Fatalf("requests = %d, want 2", got)
			}
			if elapsed < 50*time.Millisecond {
				t.Fatalf("elapsed = %s, want at least the capped Retry-After delay of 50ms", elapsed)
			}
			if elapsed > 5*time.Second {
				t.Fatalf("elapsed = %s, want the Retry-After delay capped to ASC_MAX_DELAY", elapsed)
			}
		})
	}
}

func TestPublicRetryStopsWhenRetryAfterOutlastsDeadline(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "3")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "30s")

	server, state := newRetryTestServer(t, 100, http.StatusTooManyRequests, func() string { return "30" })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	_, err := retryTestClient(server).SearchApps(ctx, "focus", "us", 20)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected rate limit failure")
	}
	if err.Error() != "search request returned status 429" {
		t.Fatalf("error = %q, want the public storefront status error", err)
	}
	if got := state.requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1 (Retry-After outlasts the deadline)", got)
	}
	if elapsed >= 250*time.Millisecond {
		t.Fatalf("elapsed = %s, want the call to fail materially before the deadline", elapsed)
	}
}

func TestPublicRetryExplicitCancellationWinsWhenRetryAfterOutlastsDeadline(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "3")
	t.Setenv("ASC_MAX_DELAY", "30s")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	client := &Client{
		BaseURL: "https://example.test",
		HTTPClient: &http.Client{Transport: retryRoundTripFunc(func(*http.Request) (*http.Response, error) {
			cancel()
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Retry-After": {"30"}},
				Body:       io.NopCloser(strings.NewReader("rate limited")),
			}, nil
		})},
	}

	_, err := client.SearchApps(ctx, "focus", "us", 20)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want explicit context cancellation", err)
	}
}

func TestPublicRetryStopsWhenFallbackBackoffOutlastsDeadline(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "3")
	t.Setenv("ASC_BASE_DELAY", "2s")
	t.Setenv("ASC_MAX_DELAY", "2s")

	server, state := newRetryTestServer(t, 100, http.StatusTooManyRequests, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := retryTestClient(server).SearchApps(ctx, "focus", "us", 20)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected rate limit failure")
	}
	if err.Error() != "search request returned status 429" {
		t.Fatalf("error = %q, want the public storefront status error", err)
	}
	if got := state.requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1 (fallback backoff outlasts the deadline)", got)
	}
	if elapsed >= 100*time.Millisecond {
		t.Fatalf("elapsed = %s, want the fallback backoff to fail materially before the deadline", elapsed)
	}
}

func TestPublicRetryFinalFallbackCancellationWins(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "5ms")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var requests atomic.Int32
	client := &Client{
		BaseURL: "https://example.test",
		HTTPClient: &http.Client{Transport: retryRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if requests.Add(1) == 2 {
				cancel()
			}
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader("rate limited")),
				Request:    req,
			}, nil
		})},
	}

	_, err := client.SearchApps(ctx, "focus", "us", 20)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want explicit context cancellation", err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want the final allowed retry", got)
	}
}

func TestPublicRetryDrainsFailureBodyForConnectionReuse(t *testing.T) {
	useFastRetries(t, "1")

	var requests atomic.Int32
	var connections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, "retry after this response")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, retryLookupPayload)
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	server.Start()
	t.Cleanup(server.Close)

	if _, err := retryTestClient(server).SearchApps(context.Background(), "focus", "us", 20); err != nil {
		t.Fatalf("SearchApps() error: %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
	if got := connections.Load(); got != 1 {
		t.Fatalf("connections = %d, want 1 reused connection", got)
	}
}

func TestPublicRetryDelayParsesRetryAfterHeader(t *testing.T) {
	now := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
	const maxDelay = 30 * time.Second

	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "missing header", value: "", want: 0},
		{name: "seconds", value: "2", want: 2 * time.Second},
		{name: "seconds capped", value: "600", want: maxDelay},
		{name: "seconds overflow capped", value: "9223372036854775807", want: maxDelay},
		{name: "seconds beyond int64 capped", value: "9223372036854775808", want: maxDelay},
		{name: "zero seconds", value: "0", want: 0},
		{name: "negative seconds", value: "-5", want: 0},
		{name: "unparsable", value: "soon", want: 0},
		{
			name:  "http date",
			value: now.Add(10 * time.Second).Format(http.TimeFormat),
			want:  10 * time.Second,
		},
		{
			name:  "http date capped",
			value: now.Add(10 * time.Minute).Format(http.TimeFormat),
			want:  maxDelay,
		},
		{
			name:  "http date in the past",
			value: now.Add(-time.Minute).Format(http.TimeFormat),
			want:  0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			headers := http.Header{}
			if test.value != "" {
				headers.Set("Retry-After", test.value)
			}
			if got := publicRetryDelay(headers, now, maxDelay); got != test.want {
				t.Fatalf("publicRetryDelay(%q) = %s, want %s", test.value, got, test.want)
			}
		})
	}
}

func TestPublicRetriesRespectResolveRetryOptions(t *testing.T) {
	tests := []struct {
		name         string
		maxRetries   string
		wantRequests int32
	}{
		{name: "retries disabled", maxRetries: "0", wantRequests: 1},
		{name: "single retry", maxRetries: "1", wantRequests: 2},
		{name: "resolved default", maxRetries: "", wantRequests: int32(asc.DefaultMaxRetries) + 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useFastRetries(t, test.maxRetries)

			server, state := newRetryTestServer(t, 100, http.StatusServiceUnavailable, nil)
			_, err := retryTestClient(server).SearchApps(context.Background(), "focus", "us", 20)
			if err == nil {
				t.Fatal("expected exhausted retry failure")
			}
			if err.Error() != "search request returned status 503" {
				t.Fatalf("error = %q, want the original public storefront status error", err)
			}
			if got := state.requests.Load(); got != test.wantRequests {
				t.Fatalf("requests = %d, want %d", got, test.wantRequests)
			}

			var storefrontError interface{ PublicStorefrontError() bool }
			if !errors.As(err, &storefrontError) || !storefrontError.PublicStorefrontError() {
				t.Fatalf("error %T does not retain public storefront semantics", err)
			}
		})
	}
}

func TestGetRatingsRetriesThrottledHistogram(t *testing.T) {
	useFastRetries(t, "3")

	var histogramRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/lookup" {
			writeBody(t, w, retryLookupPayload)
			return
		}
		// Apple throttles the customer-reviews page with 403, not 429.
		if histogramRequests.Add(1) == 1 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		writeBody(t, w, retryHistogramHTML)
	}))
	t.Cleanup(server.Close)

	ratings, err := retryTestClient(server).GetRatings(context.Background(), "123", "us")
	if err != nil {
		t.Fatalf("GetRatings() error: %v", err)
	}
	if ratings.Histogram[5] != 61 {
		t.Fatalf("histogram = %v, want 5-star count 61 from the retried request", ratings.Histogram)
	}
	if got := histogramRequests.Load(); got != 2 {
		t.Fatalf("histogram requests = %d, want 2", got)
	}
}

package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"
	"time"
)

func newMutationRetryTestClient(t *testing.T, httpClient *http.Client) *Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	return &Client{
		httpClient: httpClient,
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}
}

func setFastRetryEnv(t *testing.T, maxRetries string) {
	t.Helper()

	t.Setenv("ASC_MAX_RETRIES", maxRetries)
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	resetConfigCacheForTest()
	t.Cleanup(resetConfigCacheForTest)
}

func TestMutationDispatchObserverIgnoresLocalFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "authentication", err: ErrMissingKeyID},
		{name: "canceled before request", err: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setFastRetryEnv(t, "0")
			client := newMutationRetryTestClient(t, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Error("local failure reached HTTP transport")
				return nil, errors.New("unexpected request")
			})})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if errors.Is(tc.err, context.Canceled) {
				cancel()
			} else {
				client.keyID = ""
			}
			dispatched := false
			ctx = WithMutationDispatchObserver(ctx, func() { dispatched = true })
			_, err := client.do(ctx, http.MethodPost, "/v1/subscriptionGroups", nil)
			if !errors.Is(err, tc.err) || dispatched {
				t.Fatalf("dispatched=%v err=%v, want no dispatch and %v", dispatched, err, tc.err)
			}
		})
	}
}

func TestGuardedMutationCancellationPreservesRejection(t *testing.T) {
	setFastRetryEnv(t, "1")
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = context.WithValue(ctx, requestObservationKey{}, &requestObservation{retry: func(error) { cancel() }})
		dispatched := false
		ctx = WithMutationDispatchObserver(ctx, func() { dispatched = true })
		requests := 0
		client := newMutationRetryTestClient(t, &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests++
			return &http.Response{StatusCode: http.StatusTooManyRequests, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		})})
		attempts := 0
		_, err := RetryGuardedMutation(ctx, func(ctx context.Context, _ bool) ([]byte, error) {
			attempts++
			return client.do(ctx, http.MethodDelete, "/v1/subscriptionIntroductoryOffers/offer-1", nil)
		})
		var status interface{ HTTPStatusCode() int }
		if !errors.Is(err, context.Canceled) || !errors.As(err, &status) || status.HTTPStatusCode() != http.StatusTooManyRequests || !dispatched || requests != 1 || attempts != 1 {
			t.Fatalf("canceled rejection: err=%v dispatched=%v requests=%d attempts=%d", err, dispatched, requests, attempts)
		}
	})
}

// A 429 is a rejection: App Store Connect never processed the request, so
// replaying the identical payload cannot double-apply the mutation.
func TestClientDo_RetriesRateLimitedMutation(t *testing.T) {
	setFastRetryEnv(t, "3")
	// Retry-After is only honored within the retry cap, so the cap has to clear
	// the 1s the server asks for below.
	t.Setenv("ASC_MAX_DELAY", "5s")

	const payload = `{"data":{"type":"appStoreVersionLocalizations","attributes":{"description":"hello"}}}`

	var attempts atomic.Int32
	var (
		bodiesMu sync.Mutex
		bodies   []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		attempt := int(attempts.Add(1))
		if req.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", req.Method)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("read request body on attempt %d: %v", attempt, err)
		}
		bodiesMu.Lock()
		bodies = append(bodies, string(body))
		bodiesMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch attempt {
		case 1:
			// Retry-After must win over the configured 1ms backoff.
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED","title":"Too many requests"}]}`)
		case 2:
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED","title":"Too many requests"}]}`)
		default:
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"data":{"id":"loc-1"}}`)
		}
	}))
	t.Cleanup(server.Close)

	client := newMutationRetryTestClient(t, server.Client())

	start := time.Now()
	data, err := client.do(context.Background(), http.MethodPost, server.URL+"/v1/appStoreVersionLocalizations", strings.NewReader(payload))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("do() error: %v", err)
	}
	if got, want := string(data), `{"data":{"id":"loc-1"}}`; got != want {
		t.Fatalf("do() = %q, want %q", got, want)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
	if elapsed < 900*time.Millisecond {
		t.Fatalf("expected Retry-After: 1 to be honored, retried after %s", elapsed)
	}
	bodiesMu.Lock()
	defer bodiesMu.Unlock()
	if len(bodies) != 3 {
		t.Fatalf("expected 3 recorded bodies, got %d", len(bodies))
	}
	for i, body := range bodies {
		if body != payload {
			t.Fatalf("attempt %d sent body %q, want %q", i+1, body, payload)
		}
	}
}

func TestClientDo_RetriesRateLimitForEveryWriteMethod(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			setFastRetryEnv(t, "3")

			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != method {
					t.Errorf("expected %s, got %s", method, req.Method)
				}
				if int(attempts.Add(1)) == 1 {
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":{"id":"1"}}`)
			}))
			t.Cleanup(server.Close)

			client := newMutationRetryTestClient(t, server.Client())

			if _, err := client.do(context.Background(), method, server.URL+"/v1/apps/1", nil); err != nil {
				t.Fatalf("do() error: %v", err)
			}
			if got := attempts.Load(); got != 2 {
				t.Fatalf("expected 2 attempts, got %d", got)
			}
		})
	}
}

func TestClientDo_RateLimitedMutationExhaustsRetriesWithStatus(t *testing.T) {
	setFastRetryEnv(t, "1")

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED","title":"Too many requests"}]}`)
	}))
	t.Cleanup(server.Close)

	client := newMutationRetryTestClient(t, server.Client())

	_, err := client.do(context.Background(), http.MethodPost, server.URL+"/v1/apps", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
	if !IsRetryable(err) {
		t.Fatalf("expected retryable error after exhaustion, got %v", err)
	}
	if !IsRetryBudgetExhausted(err) {
		t.Fatalf("expected retry budget exhaustion marker, got %v", err)
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected *APIError in chain, got %v", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, apiErr.StatusCode)
	}
}

func TestClientDo_RateLimitTimeoutPreservesRetryableCause(t *testing.T) {
	setFastRetryEnv(t, "3")

	// The deadline must still be pending when the 429 arrives. A real socket
	// and a 20ms wall-clock budget can time out before the first request on CI.
	synctest.Test(t, func(t *testing.T) {
		var attempts atomic.Int32
		client := newMutationRetryTestClient(t, &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			attempts.Add(1)
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Retry-After": {"1"}},
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED"}]}`)),
			}, nil
		})})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err := client.do(ctx, http.MethodPost, "https://api.appstoreconnect.apple.com/v1/apps", nil)
		if err == nil {
			t.Fatal("expected retry wait to exceed the request deadline")
		}
		if got := attempts.Load(); got != 1 {
			t.Fatalf("expected no second request before Retry-After elapsed, got %d attempts", got)
		}
		if message := err.Error(); !strings.Contains(message, "retry cap") || !strings.Contains(message, "context deadline") {
			t.Fatalf("expected retry-cap and context-budget diagnostics, got %v", err)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("did not expect context deadline classification before the parent context expires, got %v", err)
		}
		if !IsRetryable(err) {
			t.Fatalf("expected original 429 retryable classification to be preserved, got %v", err)
		}
		if IsRetryBudgetExhausted(err) {
			t.Fatalf("did not expect retry budget exhaustion before a retry was attempted, got %v", err)
		}
		if got := GetRetryAfter(err); got != time.Second {
			t.Fatalf("expected Retry-After to be preserved, got %s", got)
		}
		var statusErr interface{ HTTPStatusCode() int }
		if !errors.As(err, &statusErr) || statusErr.HTTPStatusCode() != http.StatusTooManyRequests {
			t.Fatalf("expected status 429 to remain inspectable, got %v", err)
		}
	})
}

// 5xx failures are ambiguous for a write: App Store Connect may already have
// applied the mutation, so replaying it could duplicate work.
func TestClientDo_DoesNotRetryAmbiguousMutationFailures(t *testing.T) {
	statuses := []int{
		http.StatusRequestTimeout,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}

	for _, status := range statuses {
		t.Run(http.StatusText(status), func(t *testing.T) {
			setFastRetryEnv(t, "3")

			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"errors":[{"code":"UNEXPECTED_ERROR","title":"Server error"}]}`)
			}))
			t.Cleanup(server.Close)

			client := newMutationRetryTestClient(t, server.Client())

			_, err := client.do(context.Background(), http.MethodPost, server.URL+"/v1/apps", nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := attempts.Load(); got != 1 {
				t.Fatalf("expected exactly 1 attempt, got %d", got)
			}
			// Callers that reconcile ambiguous writes still need the retryable
			// classification and the HTTP status.
			if !IsRetryable(err) {
				t.Fatalf("expected retryable classification to be preserved, got %v", err)
			}
			apiErr, ok := errors.AsType[*APIError](err)
			if !ok {
				t.Fatalf("expected *APIError in chain, got %v", err)
			}
			if apiErr.StatusCode != status {
				t.Fatalf("expected status %d, got %d", status, apiErr.StatusCode)
			}
		})
	}
}

func TestClientDo_DoesNotRetryMutationTransportFailure(t *testing.T) {
	setFastRetryEnv(t, "3")

	var attempts atomic.Int32
	client := newMutationRetryTestClient(t, &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return nil, syscall.ECONNRESET
	})})

	_, err := client.do(context.Background(), http.MethodPost, "/v1/apps", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected exactly 1 attempt for an ambiguous transport failure, got %d", got)
	}
}

func TestClientDo_ReadRetryPolicyUnchanged(t *testing.T) {
	setFastRetryEnv(t, "3")

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", req.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		if int(attempts.Add(1)) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"errors":[{"code":"UNEXPECTED_ERROR"}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	t.Cleanup(server.Close)

	client := newMutationRetryTestClient(t, server.Client())

	if _, err := client.do(context.Background(), http.MethodGet, server.URL+"/v1/apps", nil); err != nil {
		t.Fatalf("do() error: %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}

// doIdempotentMutation opts into the full transient-failure policy. It must not
// stack a second retry loop on top of the rate-limit retries in do().
func TestClientDoIdempotentMutation_RetriesTransientFailuresOnce(t *testing.T) {
	setFastRetryEnv(t, "1")

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"errors":[{"code":"UNEXPECTED_ERROR"}]}`)
	}))
	t.Cleanup(server.Close)

	client := newMutationRetryTestClient(t, server.Client())

	if _, err := client.doIdempotentMutation(context.Background(), http.MethodPatch, server.URL+"/v1/territoryAvailabilities/ta-1", nil); err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts (1 retry), got %d", got)
	}
}

func TestClientDoIdempotentMutation_RateLimitRetriesAreNotStacked(t *testing.T) {
	setFastRetryEnv(t, "1")

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED"}]}`)
	}))
	t.Cleanup(server.Close)

	client := newMutationRetryTestClient(t, server.Client())

	if _, err := client.doIdempotentMutation(context.Background(), http.MethodPatch, server.URL+"/v1/territoryAvailabilities/ta-1", nil); err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts (1 retry), got %d", got)
	}
}

func TestClientDo_RateLimitRetriesDisabledByMaxRetriesZero(t *testing.T) {
	setFastRetryEnv(t, "0")

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(server.Close)

	client := newMutationRetryTestClient(t, server.Client())

	if _, err := client.do(context.Background(), http.MethodPost, server.URL+"/v1/apps", nil); err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected exactly 1 attempt with retries disabled, got %d", got)
	}
}

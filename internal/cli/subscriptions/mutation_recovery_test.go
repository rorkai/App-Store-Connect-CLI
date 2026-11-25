package subscriptions

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestRunReconciledMutationRetriesOnlyAfterNegativeReadback(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")

	mutations := 0
	readbacks := 0
	status, err := runReconciledMutation(
		context.Background(),
		shared.DefaultDeps(),
		func(context.Context) (bool, error) {
			readbacks++
			return false, nil
		},
		func(context.Context) error {
			mutations++
			if mutations == 1 {
				return &asc.RetryableError{Err: errors.New("temporary failure")}
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runReconciledMutation() error: %v", err)
	}
	if status != reconciledMutationCreated || mutations != 2 || readbacks != 2 {
		t.Fatalf("unexpected recovery: status=%q mutations=%d readbacks=%d", status, mutations, readbacks)
	}
}

func TestIsTransientMutationErrorSkipsExhaustedClientRateLimit(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")

	_, err := asc.WithRetry(context.Background(), func() (string, error) {
		return "", &asc.RetryableError{
			Err: &asc.APIError{Code: "RATE_LIMIT_EXCEEDED", StatusCode: http.StatusTooManyRequests},
		}
	}, asc.ResolveRetryOptions())
	if err == nil {
		t.Fatal("expected exhausted rate-limit error")
	}
	if !asc.IsRetryBudgetExhausted(err) {
		t.Fatalf("expected client retry budget marker, got %v", err)
	}
	if shared.IsTransientMutationError(context.Background(), err) {
		t.Fatalf("expected exhausted client rate limit not to enter the outer pricing retry loop: %v", err)
	}
}

func TestPartitionEqualizeFailuresKeepsUnhonoredRetryDelayFinal(t *testing.T) {
	_, err := asc.WithRetry(context.Background(), func() (struct{}, error) {
		return struct{}{}, &asc.RetryableError{
			Err:        &asc.APIError{Code: "RATE_LIMIT_EXCEEDED", StatusCode: http.StatusTooManyRequests},
			RetryAfter: time.Hour,
		}
	}, asc.RetryOptions{MaxRetries: 1, BaseDelay: time.Millisecond, MaxDelay: time.Second})
	if err == nil {
		t.Fatal("expected unhonored Retry-After error, got nil")
	}

	retryable, final := partitionEqualizeFailures(context.Background(), []equalizeAttemptFailure{{Target: equalization{Territory: "USA"}, Err: err}})
	if len(retryable) != 0 || len(final) != 1 {
		t.Fatalf("expected one final failure and no retryable failures, got retryable=%d final=%d", len(retryable), len(final))
	}
}

func TestEqualizeRateLimitReplayUsesFreshRequestDeadlines(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "100ms")
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")
	asc.ResetConfigCacheForTest()
	t.Cleanup(asc.ResetConfigCacheForTest)

	const post = "POST /v1/subscriptionPrices"
	const prices = "GET /v1/subscriptions/sub-1/prices"
	const tiers = "GET /v1/subscriptionPriceTiers"
	var calls []string
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = resolvedPricesRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		call := req.Method + " " + req.URL.Path
		calls = append(calls, call)
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(60 * time.Millisecond):
		}
		switch call {
		case post:
			if len(calls) == 1 {
				response := resolvedPricesJSONResponse(`{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED"}]}`)
				response.StatusCode = http.StatusTooManyRequests
				return response, nil
			}
			response := resolvedPricesJSONResponse(`{"data":{"type":"subscriptionPrices","id":"created"}}`)
			response.StatusCode = http.StatusCreated
			return response, nil
		case prices, tiers:
			return resolvedPricesJSONResponse(`{"data":[]}`), nil
		default:
			t.Fatalf("unexpected request %s", call)
			return nil, nil
		}
	})
	client, err := asc.NewClientFromPEM("KEY123", "issuer", introImportTestPrivateKeyPEM(t))
	if err != nil {
		t.Fatal(err)
	}
	synctest.Test(t, func(t *testing.T) {
		succeeded, failures := applyEqualizedPrices(context.Background(), shared.DefaultDeps(), client, "sub-1",
			[]equalization{{Territory: "CAN", PricePointID: "can-point"}}, 1, asc.SubscriptionPriceCreateAttributes{}, time.Now(), false)
		if succeeded != 1 || len(failures) != 0 || !slices.Equal(calls, []string{post, prices, tiers, post}) {
			t.Fatalf("succeeded=%d failures=%v requests=%v", succeeded, failures, calls)
		}
	})
}

func TestApplyEqualizedPricesDoesNotReplayExhaustedClientRateLimit(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "3")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	postAttempts := 0
	readbacks := 0
	http.DefaultTransport = resolvedPricesRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			postAttempts++
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED"}]}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionPriceTiers":
			return resolvedPricesJSONResponse(`{"data":[]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/prices":
			readbacks++
			return resolvedPricesJSONResponse(`{"data":[],"included":[],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	client, err := asc.NewClientFromPEM("KEY123", "issuer", introImportTestPrivateKeyPEM(t))
	if err != nil {
		t.Fatalf("NewClientFromPEM() error: %v", err)
	}

	succeeded, failures := applyEqualizedPrices(
		context.Background(),
		shared.DefaultDeps(),
		client,
		"sub-1",
		[]equalization{{Territory: "CAN", Price: "1.29", PricePointID: "price-point-can"}},
		1,
		asc.SubscriptionPriceCreateAttributes{},
		time.Now().UTC(), false,
	)
	if succeeded != 0 || len(failures) != 1 {
		t.Fatalf("unexpected equalize result: succeeded=%d failures=%d", succeeded, len(failures))
	}
	if want := asc.DefaultMaxRetries + 1; postAttempts != want {
		t.Fatalf("expected one client retry budget (%d POSTs), got %d; outer pricing recovery replayed an exhausted 429", want, postAttempts)
	}
	if readbacks != postAttempts {
		t.Fatalf("expected a readback before each replay and after exhaustion, got %d for %d POSTs", readbacks, postAttempts)
	}
}

func TestApplyEqualizedPricesDoesNotSleepOnUnhonoredRetryAfter(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "3")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1s")
	asc.ResetConfigCacheForTest()
	t.Cleanup(asc.ResetConfigCacheForTest)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	postAttempts := 0
	readbacks := 0
	http.DefaultTransport = resolvedPricesRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionPrices":
			postAttempts++
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"429","code":"RATE_LIMIT_EXCEEDED"}]}`)),
				Header:     http.Header{"Content-Type": []string{"application/json"}, "Retry-After": []string{"3600"}},
			}, nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/sub-1/prices":
			readbacks++
			return resolvedPricesJSONResponse(`{"data":[],"included":[],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	client, err := asc.NewClientFromPEM("KEY123", "issuer", introImportTestPrivateKeyPEM(t))
	if err != nil {
		t.Fatalf("NewClientFromPEM() error: %v", err)
	}

	var succeeded int
	var failures []equalizeAttemptFailure
	synctest.Test(t, func(*testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		succeeded, failures = applyEqualizedPrices(
			ctx,
			shared.DefaultDeps(),
			client,
			"sub-1",
			[]equalization{{Territory: "CAN", Price: "1.29", PricePointID: "price-point-can"}},
			1,
			asc.SubscriptionPriceCreateAttributes{},
			time.Now().UTC(), false,
		)
	})
	if succeeded != 0 || len(failures) != 1 {
		t.Fatalf("unexpected equalize result: succeeded=%d failures=%d", succeeded, len(failures))
	}
	if postAttempts != 1 {
		t.Fatalf("expected one POST when Retry-After exceeds cap, got %d", postAttempts)
	}
	if readbacks != 1 {
		t.Fatalf("expected one final reconciliation readback, got %d", readbacks)
	}
}

func TestRunReconciledMutationRetriesChildDeadlineAfterNegativeReadback(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")

	mutations := 0
	readbacks := 0
	status, err := runReconciledMutation(
		context.Background(),
		shared.DefaultDeps(),
		func(context.Context) (bool, error) {
			readbacks++
			return false, nil
		},
		func(context.Context) error {
			mutations++
			if mutations == 1 {
				return context.DeadlineExceeded
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runReconciledMutation() error: %v", err)
	}
	if status != reconciledMutationCreated || mutations != 2 || readbacks != 2 {
		t.Fatalf("unexpected recovery: status=%q mutations=%d readbacks=%d", status, mutations, readbacks)
	}
}

func TestRunReconciledMutationReadsAgainBeforeReplay(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1ms")
	t.Setenv("ASC_MAX_DELAY", "1ms")

	mutations := 0
	readbacks := 0
	status, err := runReconciledMutation(
		context.Background(),
		shared.DefaultDeps(),
		func(context.Context) (bool, error) {
			readbacks++
			return readbacks == 2, nil
		},
		func(context.Context) error {
			mutations++
			return &asc.RetryableError{Err: errors.New("ambiguous failure")}
		},
	)
	if err != nil {
		t.Fatalf("runReconciledMutation() error: %v", err)
	}
	if status != reconciledMutationReconciled || mutations != 1 || readbacks != 2 {
		t.Fatalf("unexpected recovery: status=%q mutations=%d readbacks=%d", status, mutations, readbacks)
	}
}

func TestRunReconciledMutationStopsWhenReadbackFails(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "3")
	mutations := 0
	_, err := runReconciledMutation(
		context.Background(),
		shared.DefaultDeps(),
		func(context.Context) (bool, error) {
			return false, errors.New("readback unavailable")
		},
		func(context.Context) error {
			mutations++
			return &asc.RetryableError{Err: errors.New("ambiguous failure")}
		},
	)
	if err == nil || mutations != 1 {
		t.Fatalf("expected one mutation and a readback error, mutations=%d err=%v", mutations, err)
	}
}

func TestRunReconciledMutationRespectsCancellationDuringBackoff(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "1")
	t.Setenv("ASC_BASE_DELAY", "1h")

	ctx, cancel := context.WithCancel(context.Background())
	mutations := 0
	readbacks := 0
	_, err := runReconciledMutation(
		ctx,
		shared.DefaultDeps(),
		func(context.Context) (bool, error) {
			readbacks++
			cancel()
			return false, nil
		},
		func(context.Context) error {
			mutations++
			return &asc.RetryableError{Err: errors.New("temporary failure"), RetryAfter: time.Hour}
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if mutations != 1 || readbacks != 1 {
		t.Fatalf("expected cancellation during first backoff, mutations=%d readbacks=%d", mutations, readbacks)
	}
}

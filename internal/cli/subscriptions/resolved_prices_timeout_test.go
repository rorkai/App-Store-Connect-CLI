package subscriptions

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type resolvedPricesRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn resolvedPricesRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestFetchResolvedSubscriptionPricesUsesFreshDeadlinePerPage(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "100ms")
	t.Setenv("ASC_MAX_RETRIES", "0")
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	calls := 0
	http.DefaultTransport = resolvedPricesRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			time.Sleep(60 * time.Millisecond)
			return resolvedPricesJSONResponse(`{"data":[],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/prices?cursor=next"}}`), nil
		}
		deadline, ok := req.Context().Deadline()
		if !ok || time.Until(deadline) < 70*time.Millisecond {
			t.Errorf("expected fresh second-page deadline, remaining=%s", time.Until(deadline))
			return nil, context.DeadlineExceeded
		}
		return resolvedPricesJSONResponse(`{"data":[],"links":{"next":""}}`), nil
	})
	client, err := asc.NewClientFromPEM("KEY123", "issuer", introImportTestPrivateKeyPEM(t))
	if err != nil {
		t.Fatalf("NewClientFromPEM() error: %v", err)
	}

	var result *shared.ResolvedPricesResult
	synctest.Test(t, func(*testing.T) {
		result, err = fetchResolvedSubscriptionPrices(context.Background(), client, "sub-1", 200, "", time.Now().UTC(), "", "")
	})
	if err != nil {
		t.Fatalf("fetchResolvedSubscriptionPrices() error: %v", err)
	}
	if calls != 2 || len(result.Prices) != 0 {
		t.Fatalf("unexpected pagination result: calls=%d result=%+v", calls, result)
	}
}

func TestMonthlyCommitmentResolvedPriceCallersUseFreshDeadlinePerPage(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, *asc.Client) error
	}{
		{
			name: "validate upfront prices",
			run: func(ctx context.Context, client *asc.Client) error {
				return validateMonthlyCommitmentUpfrontPrices(ctx, client, "sub-1", nil, "9.99")
			},
		},
		{
			name: "prepare monthly prices",
			run: func(ctx context.Context, client *asc.Client) error {
				_, err := prepareMonthlySubscriptionPrices(ctx, client, "sub-1", nil, "9.99", subscriptionPricingToday())
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_TIMEOUT", "100ms")
			t.Setenv("ASC_MAX_RETRIES", "0")
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			calls := 0
			http.DefaultTransport = resolvedPricesRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					time.Sleep(60 * time.Millisecond)
					return resolvedPricesJSONResponse(`{"data":[],"links":{"next":"/v1/subscriptions/sub-1/prices?cursor=next"}}`), nil
				}
				deadline, ok := req.Context().Deadline()
				if !ok || time.Until(deadline) < 70*time.Millisecond {
					t.Errorf("expected fresh second-page deadline, remaining=%s", time.Until(deadline))
					return nil, context.DeadlineExceeded
				}
				return resolvedPricesJSONResponse(`{"data":[],"links":{"next":""}}`), nil
			})
			client, err := asc.NewClientFromPEM("KEY123", "issuer", introImportTestPrivateKeyPEM(t))
			if err != nil {
				t.Fatalf("NewClientFromPEM() error: %v", err)
			}

			synctest.Test(t, func(*testing.T) { err = test.run(context.Background(), client) })
			if err != nil {
				t.Fatalf("caller error: %v", err)
			}
			if calls != 2 {
				t.Fatalf("expected two page requests, got %d", calls)
			}
		})
	}
}

func resolvedPricesJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

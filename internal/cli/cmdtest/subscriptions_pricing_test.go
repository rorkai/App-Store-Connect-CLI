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

func TestSubscriptionsPricingValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app and subscription-id",
			args:    []string{"subscriptions", "pricing", "summary"},
			wantErr: "Error: --app or --subscription-id is required",
		},
		{
			name:    "app and subscription-id both set",
			args:    []string{"subscriptions", "pricing", "summary", "--app", "APP_ID", "--subscription-id", "SUB_ID"},
			wantErr: "Error: --app and --subscription-id are mutually exclusive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected error %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsPricingByIDSuccess(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/sub-1" && req.Method == http.MethodGet:
			body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly","subscriptionPeriod":"ONE_MONTH","state":"APPROVED"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.URL.Path == "/v1/subscriptions/sub-1/prices":
			query := req.URL.Query()
			if query.Get("filter[territory]") != "USA" {
				t.Fatalf("expected filter[territory]=USA, got %q", query.Get("filter[territory]"))
			}
			if query.Get("include") != "subscriptionPricePoint,territory" {
				t.Fatalf("expected include=subscriptionPricePoint,territory, got %q", query.Get("include"))
			}
			body := `{
				"data":[
					{
						"type":"subscriptionPrices","id":"price-1",
						"attributes":{"startDate":"2024-01-01"},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-1"}}
						}
					}
				],
				"included":[
					{"type":"subscriptionPricePoints","id":"pp-1","attributes":{"customerPrice":"9.99","proceeds":"7.00","proceedsYear2":"8.49"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "summary", "--subscription-id", "sub-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"sub-1"`) {
		t.Fatalf("expected sub id in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"currentPrice":{"amount":"9.99","currency":"USD"}`) {
		t.Fatalf("expected current price in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"proceeds":{"amount":"7.00","currency":"USD"}`) {
		t.Fatalf("expected proceeds in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"proceedsYear2":{"amount":"8.49","currency":"USD"}`) {
		t.Fatalf("expected proceeds year 2 in output, got %q", stdout)
	}
}

func TestSubscriptionsPricingTableOutput(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/sub-1" && req.Method == http.MethodGet:
			body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly","subscriptionPeriod":"ONE_MONTH","state":"APPROVED"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.URL.Path == "/v1/subscriptions/sub-1/prices":
			body := `{
				"data":[{
					"type":"subscriptionPrices","id":"price-1",
					"attributes":{"startDate":"2024-01-01"},
					"relationships":{
						"territory":{"data":{"type":"territories","id":"USA"}},
						"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-1"}}
					}
				}],
				"included":[
					{"type":"subscriptionPricePoints","id":"pp-1","attributes":{"customerPrice":"9.99","proceeds":"7.00","proceedsYear2":"8.49"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "summary", "--subscription-id", "sub-1", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Current Price") {
		t.Fatalf("expected table headers in output, got %q", stdout)
	}
	if !strings.Contains(stdout, "9.99 USD") {
		t.Fatalf("expected formatted price in output, got %q", stdout)
	}
}

func TestSubscriptionsPricingUsesLatestEffectivePriceAsCurrent(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/subscriptions/sub-1" && req.Method == http.MethodGet:
			body := `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly","subscriptionPeriod":"ONE_MONTH","state":"APPROVED"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.URL.Path == "/v1/subscriptions/sub-1/prices":
			body := `{
				"data":[
					{
						"type":"subscriptionPrices","id":"price-old",
						"attributes":{"startDate":"2020-01-01"},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-old"}}
						}
					},
					{
						"type":"subscriptionPrices","id":"price-current",
						"attributes":{"startDate":"2024-01-01"},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-current"}}
						}
					}
				],
				"included":[
					{"type":"subscriptionPricePoints","id":"pp-old","attributes":{"customerPrice":"1.99","proceeds":"1.40","proceedsYear2":"1.60"}},
					{"type":"subscriptionPricePoints","id":"pp-current","attributes":{"customerPrice":"9.99","proceeds":"7.00","proceedsYear2":"8.49"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "pricing", "summary", "--subscription-id", "sub-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"currentPrice":{"amount":"9.99","currency":"USD"}`) {
		t.Fatalf("expected latest effective price as current, got %q", stdout)
	}
	if strings.Contains(stdout, `"currentPrice":{"amount":"1.99","currency":"USD"}`) {
		t.Fatalf("unexpected stale price selected as current, got %q", stdout)
	}
}

func TestSubscriptionsPricingReturnsWorkerErrorNotContextCancelled(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/v1/apps/app-1/subscriptionGroups" && req.Method == http.MethodGet:
			body := `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Main Group"}}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions" && req.Method == http.MethodGet:
			body := `{
				"data":[
					{"type":"subscriptions","id":"sub-ok","attributes":{"name":"OK","productId":"com.example.ok","subscriptionPeriod":"ONE_MONTH","state":"APPROVED"}},
					{"type":"subscriptions","id":"sub-fail","attributes":{"name":"FAIL","productId":"com.example.fail","subscriptionPeriod":"ONE_MONTH","state":"APPROVED"}}
				],
				"links":{}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.URL.Path == "/v1/subscriptions/sub-ok/prices" && req.Method == http.MethodGet:
			body := `{
				"data":[
					{
						"type":"subscriptionPrices","id":"price-ok",
						"attributes":{"startDate":"2024-01-01"},
						"relationships":{
							"territory":{"data":{"type":"territories","id":"USA"}},
							"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"pp-ok"}}
						}
					}
				],
				"included":[
					{"type":"subscriptionPricePoints","id":"pp-ok","attributes":{"customerPrice":"9.99","proceeds":"7.00","proceedsYear2":"8.49"}},
					{"type":"territories","id":"USA","attributes":{"currency":"USD"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		case req.URL.Path == "/v1/subscriptions/sub-fail/prices" && req.Method == http.MethodGet:
			body := `{"errors":[{"status":"500","title":"Server Error","detail":"boom"}]}`
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil

		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{"subscriptions", "pricing", "summary", "--app", "app-1"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := root.Run(context.Background())
	if err == nil {
		t.Fatalf("expected command to fail")
	}
	if strings.Contains(strings.ToLower(err.Error()), "context cancelled") {
		t.Fatalf("expected root cause error, got masked cancellation: %v", err)
	}
	if !strings.Contains(err.Error(), "fetch prices") {
		t.Fatalf("expected fetch prices context in error, got %v", err)
	}
}

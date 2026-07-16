package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestSubscriptionsAdjustedEqualizationsSendsExactFilters(t *testing.T) {
	setupAuth(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionPricePoints/base-1/adjustedEqualizations" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		query := req.URL.Query()
		for key, want := range map[string]string{
			"filter[territory]":               "USA,FRA",
			"filter[subscription]":            "sub-1,sub-2",
			"filter[upfrontPricePointId]":     "upfront-1,upfront-2",
			"filter[planType]":                "MONTHLY",
			"fields[subscriptionPricePoints]": "customerPrice,adjustedEqualizations",
			"fields[territories]":             "currency",
			"include":                         "territory",
			"limit":                           "50",
		} {
			if got := query.Get(key); got != want {
				t.Fatalf("%s=%q, want %q; raw query=%s", key, got, want, req.URL.RawQuery)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"subscriptionPricePoints","id":"adjusted-1","attributes":{"customerPrice":"4.99"}}],"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],"links":{}}`)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "price-points", "adjusted-equalizations",
			"--price-point-id", "base-1",
			"--territory", "US,France",
			"--subscription-id", "sub-1,sub-2",
			"--upfront-price-point-id", "upfront-1,upfront-2",
			"--plan-type", "monthly",
			"--fields", "customerPrice,adjustedEqualizations",
			"--territory-fields", "currency",
			"--include", "territory",
			"--limit", "50",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run: %v; stderr=%q", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var response asc.SubscriptionPricePointsResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse stdout: %v; stdout=%q", err, stdout)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "adjusted-1" {
		t.Fatalf("unexpected response: %#v", response.Data)
	}
}

func TestSubscriptionsPricePointsListSends441Filters(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/pricePoints" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		query := req.URL.Query()
		for key, want := range map[string]string{
			"filter[upfrontPricePointId]":     "upfront-1,upfront-2",
			"filter[planType]":                "MONTHLY,UPFRONT",
			"fields[subscriptionPricePoints]": "customerPrice,adjustedEqualizations",
			"fields[territories]":             "currency",
			"include":                         "territory",
		} {
			if got := query.Get(key); got != want {
				t.Fatalf("%s=%q, want %q", key, got, want)
			}
		}
		return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"point-1"}],"links":{}}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "price-points", "list",
			"--subscription-id", "8000000001",
			"--upfront-price-point-id", "upfront-1,upfront-2",
			"--plan-type", "monthly,upfront",
			"--fields", "customerPrice,adjustedEqualizations",
			"--territory-fields", "currency",
			"--include", "territory",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var response asc.SubscriptionPricePointsResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse stdout: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "point-1" {
		t.Fatalf("unexpected output: %#v", response.Data)
	}
}

func TestSubscriptionsAdjustedEqualizationsUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing price point", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations"}, want: "--price-point-id is required"},
		{name: "invalid limit", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--limit", "8001"}, want: "--limit must be between 1 and 8000"},
		{name: "invalid fields", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--fields", "bogus"}, want: "--fields must be one of"},
		{name: "invalid include", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--include", "subscription"}, want: "--include must be one of: territory"},
		{name: "empty territory", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--territory", ""}, want: "invalid value for --territory: cannot be empty"},
		{name: "unknown territory", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--territory", "Atlantis"}, want: "could not be mapped"},
		{name: "empty plan type", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--plan-type", ""}, want: "invalid value for --plan-type: cannot be empty"},
		{name: "unsupported adjusted plan type", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--plan-type", "UPFRONT"}, want: "--plan-type must be MONTHLY for adjusted equalizations"},
		{name: "invalid territory fields", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--territory-fields", "name"}, want: "--territory-fields must be one of: currency"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.want)
		})
	}
}

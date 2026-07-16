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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
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
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"subscriptionPricePoints","id":"point-1"}],"links":{}}`)
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
		{name: "next with filter", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptionPricePoints/base-1/adjustedEqualizations?cursor=next", "--territory", "USA"}, want: "--next cannot be combined"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.want)
		})
	}
}

func TestSubscriptionsPricePointsListValidates441FlagsBeforeLookup(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("invalid fields must fail before HTTP lookup: %s %s", req.Method, req.URL)
		return nil, nil
	}))

	assertUsageExit(t, []string{
		"subscriptions", "pricing", "price-points", "list",
		"--subscription-id", "human-readable-product-id",
		"--fields", "bogus",
	}, "--fields must be one of")
}

func TestSubscriptionsPricePointsListRejectsNextQueryConflictsBeforeLookup(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("next conflicts must fail before HTTP lookup: %s %s", req.Method, req.URL)
		return nil, nil
	}))

	assertUsageExit(t, []string{
		"subscriptions", "pricing", "price-points", "list",
		"--next", "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=next",
		"--fields", "customerPrice",
	}, "--next cannot be combined")
}

func TestSubscriptionsPricePointsListKeepsCustomerPriceForClientSideFiltering(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/subscriptions/8000000001/pricePoints" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		if got := req.URL.Query().Get("fields[subscriptionPricePoints]"); got != "proceeds,customerPrice" {
			t.Fatalf("fields[subscriptionPricePoints]=%q, want proceeds,customerPrice", got)
		}
		return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"point-1","attributes":{"customerPrice":"4.99","proceeds":"3.50"}}],"links":{}}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "price-points", "list",
			"--subscription-id", "8000000001",
			"--price", "4.99",
			"--fields", "proceeds",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	var response asc.SubscriptionPricePointsResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse stdout: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "point-1" {
		t.Fatalf("expected matching filtered point, got %#v", response.Data)
	}
}

func TestSubscriptionsPricePointsListKeepsFullPayloadWhenFieldsAreOmitted(t *testing.T) {
	setupAuth(t)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/subscriptions/8000000001/pricePoints" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		if _, ok := req.URL.Query()["fields[subscriptionPricePoints]"]; ok {
			t.Fatalf("unexpected sparse fields query: %s", req.URL.RawQuery)
		}
		return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"point-1","attributes":{"customerPrice":"4.99","proceeds":"3.50","proceedsYear2":"3.75"}}],"links":{}}`)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "price-points", "list",
			"--subscription-id", "8000000001",
			"--price", "4.99",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	var response asc.SubscriptionPricePointsResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse stdout: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].Attributes.ProceedsYear2 != "3.75" {
		t.Fatalf("expected full matching price-point payload, got %#v", response.Data)
	}
}

func TestSubscriptionsPricePointsPaginationPreservesAppleNextLimit(t *testing.T) {
	setupAuth(t)
	requestCount := 0
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.URL.Path != "/v1/subscriptions/8000000001/pricePoints" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		if got := req.URL.Query().Get("limit"); got != "200" {
			t.Fatalf("request %d limit=%q, want Apple cursor limit 200", requestCount, got)
		}
		switch requestCount {
		case 1:
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"point-1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptions/8000000001/pricePoints?cursor=next&limit=200"}}`)
		case 2:
			if got := req.URL.Query().Get("cursor"); got != "next" {
				t.Fatalf("cursor=%q, want next", got)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"point-2"}],"links":{}}`)
		default:
			t.Fatalf("unexpected request %d: %s", requestCount, req.URL)
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "price-points", "list",
			"--subscription-id", "8000000001",
			"--limit", "17",
			"--paginate",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if requestCount != 2 {
		t.Fatalf("request count=%d, want 2", requestCount)
	}
}

package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	subscriptionscli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/subscriptions"
)

func TestSubscriptionsAdjustedEqualizationsSendsExactFilters(t *testing.T) {
	setupAuth(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptionPricePoints/base-1/adjustedEqualizations" {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return
		}
		query := req.URL.Query()
		for key, want := range map[string]string{
			"filter[territory]":               "USA,FRA",
			"filter[subscription]":            "sub-1,sub-2",
			"filter[upfrontPricePointId]":     "upfront-1,upfront-2",
			"filter[planType]":                "MONTHLY",
			"fields[subscriptionPricePoints]": "customerPrice,adjustedEqualizations,territory",
			"fields[territories]":             "currency",
			"include":                         "territory",
			"limit":                           "50",
		} {
			if got := query.Get(key); got != want {
				t.Errorf("%s=%q, want %q; raw query=%s", key, got, want, req.URL.RawQuery)
				return
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
			"fields[subscriptionPricePoints]": "customerPrice,adjustedEqualizations,territory",
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

func TestSubscriptionsPricePointsListKeepsTerritoryInSparseFields(t *testing.T) {
	tests := []struct {
		name      string
		fields    string
		args      []string
		wantQuery url.Values
	}{
		{
			name:   "explicit include",
			fields: "customerPrice",
			args:   []string{"--include", "territory"},
			wantQuery: url.Values{
				"fields[subscriptionPricePoints]": {"customerPrice,territory"},
				"include":                         {"territory"},
			},
		},
		{
			name:   "territory fields imply include",
			fields: "customerPrice",
			args:   []string{"--territory-fields", "currency"},
			wantQuery: url.Values{
				"fields[subscriptionPricePoints]": {"customerPrice,territory"},
				"fields[territories]":             {"currency"},
				"include":                         {"territory"},
			},
		},
		{
			name:   "territory already selected",
			fields: "customerPrice,territory",
			args:   []string{"--include", "territory"},
			wantQuery: url.Values{
				"fields[subscriptionPricePoints]": {"customerPrice,territory"},
				"include":                         {"territory"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/pricePoints" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
				}
				if got, want := req.URL.RawQuery, test.wantQuery.Encode(); got != want {
					t.Fatalf("query=%q, want %q", got, want)
				}
				return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"point-1"}],"links":{}}`)
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := []string{
				"subscriptions", "pricing", "price-points", "list",
				"--subscription-id", "8000000001",
				"--fields", test.fields,
				"--output", "json",
			}
			args = append(args, test.args...)
			_, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestSubscriptionsPricePointEqualizationsKeepTerritoryInSparseFields(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		path      string
		args      []string
		wantQuery url.Values
	}{
		{
			name:    "equalizations explicit include",
			command: "equalizations",
			path:    "equalizations",
			args:    []string{"--include", "territory"},
			wantQuery: url.Values{
				"fields[subscriptionPricePoints]": {"customerPrice,territory"},
				"include":                         {"territory"},
			},
		},
		{
			name:    "adjusted equalizations territory fields imply include",
			command: "adjusted-equalizations",
			path:    "adjustedEqualizations",
			args:    []string{"--territory-fields", "currency"},
			wantQuery: url.Values{
				"fields[subscriptionPricePoints]": {"customerPrice,territory"},
				"fields[territories]":             {"currency"},
				"include":                         {"territory"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				wantPath := "/v1/subscriptionPricePoints/base-1/" + test.path
				if req.Method != http.MethodGet || req.URL.Path != wantPath {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
				}
				if got, want := req.URL.RawQuery, test.wantQuery.Encode(); got != want {
					t.Fatalf("query=%q, want %q", got, want)
				}
				return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"point-1"}],"links":{}}`)
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := []string{
				"subscriptions", "pricing", "price-points", test.command,
				"--price-point-id", "base-1",
				"--fields", "customerPrice",
				"--output", "json",
			}
			args = append(args, test.args...)
			_, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
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
		{name: "separator-only territory", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--territory", ","}, want: "invalid value for --territory: cannot contain empty values"},
		{name: "unknown territory", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--territory", "Atlantis"}, want: "could not be mapped"},
		{name: "empty plan type", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--plan-type", ""}, want: "invalid value for --plan-type: cannot be empty"},
		{name: "separator-only plan type", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--plan-type", ","}, want: "invalid value for --plan-type: cannot contain empty values"},
		{name: "empty element in subscription IDs", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--subscription-id", "sub-1,,sub-2"}, want: "invalid value for --subscription-id: cannot contain empty values"},
		{name: "empty element in upfront IDs", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--upfront-price-point-id", "upfront-1,"}, want: "invalid value for --upfront-price-point-id: cannot contain empty values"},
		{name: "separator-only fields", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--fields", ","}, want: "invalid value for --fields: cannot contain empty values"},
		{name: "empty element in territory fields", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--territory-fields", "currency,"}, want: "invalid value for --territory-fields: cannot contain empty values"},
		{name: "empty element in include", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--include", ",territory"}, want: "invalid value for --include: cannot contain empty values"},
		{name: "unsupported adjusted plan type", args: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations", "--price-point-id", "base-1", "--plan-type", "UPFRONT"}, want: "--plan-type must be MONTHLY for adjusted equalizations"},
		{name: "unknown equalizations plan type", args: []string{"subscriptions", "pricing", "price-points", "equalizations", "--price-point-id", "base-1", "--plan-type", "annual"}, want: "--plan-type must be one of: MONTHLY, UPFRONT"},
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
	assertUsageExit(t, []string{
		"subscriptions", "pricing", "price-points", "list",
		"--subscription-id", "human-readable-product-id",
		"--plan-type", "annual",
	}, "--plan-type must be one of: MONTHLY, UPFRONT")
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

func TestSubscriptionsPricePointCommandsRejectAllOpaqueNextConflictsBeforeClient(t *testing.T) {
	tests := []struct {
		name     string
		command  []string
		nextURL  string
		conflict []string
	}{
		{
			name:     "list empty owner subscription",
			command:  []string{"subscriptions", "pricing", "price-points", "list"},
			nextURL:  "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=next",
			conflict: []string{"--subscription-id", ""},
		},
		{
			name:     "list empty owner app",
			command:  []string{"subscriptions", "pricing", "price-points", "list"},
			nextURL:  "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=next",
			conflict: []string{"--app", ""},
		},
		{
			name:     "list territory filter",
			command:  []string{"subscriptions", "pricing", "price-points", "list"},
			nextURL:  "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=next",
			conflict: []string{"--territory", "USA"},
		},
		{
			name:     "list price filter",
			command:  []string{"subscriptions", "pricing", "price-points", "list"},
			nextURL:  "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=next",
			conflict: []string{"--price", "4.99"},
		},
		{
			name:     "list minimum price filter",
			command:  []string{"subscriptions", "pricing", "price-points", "list"},
			nextURL:  "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=next",
			conflict: []string{"--min-price", "1.00"},
		},
		{
			name:     "list maximum price filter",
			command:  []string{"subscriptions", "pricing", "price-points", "list"},
			nextURL:  "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=next",
			conflict: []string{"--max-price", "9.99"},
		},
	}

	commonFilters := []struct {
		name string
		args []string
	}{
		{name: "upfront price point filter", args: []string{"--upfront-price-point-id", "upfront-1"}},
		{name: "plan type filter", args: []string{"--plan-type", "MONTHLY"}},
		{name: "sparse fields", args: []string{"--fields", "customerPrice"}},
		{name: "territory sparse fields", args: []string{"--territory-fields", "currency"}},
		{name: "include", args: []string{"--include", "territory"}},
		{name: "limit", args: []string{"--limit", "0"}},
	}
	for _, filter := range commonFilters {
		tests = append(tests, struct {
			name     string
			command  []string
			nextURL  string
			conflict []string
		}{
			name:     "list " + filter.name,
			command:  []string{"subscriptions", "pricing", "price-points", "list"},
			nextURL:  "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=next",
			conflict: filter.args,
		})
	}

	equalizationCommands := []struct {
		name    string
		command []string
		path    string
	}{
		{name: "equalizations", command: []string{"subscriptions", "pricing", "price-points", "equalizations"}, path: "equalizations"},
		{name: "adjusted equalizations", command: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations"}, path: "adjustedEqualizations"},
	}
	equalizationConflicts := append([]struct {
		name string
		args []string
	}{
		{name: "empty owner price point", args: []string{"--price-point-id", ""}},
		{name: "territory filter", args: []string{"--territory", "USA"}},
		{name: "subscription filter", args: []string{"--subscription-id", "sub-1"}},
	}, commonFilters...)
	for _, command := range equalizationCommands {
		for _, conflict := range equalizationConflicts {
			tests = append(tests, struct {
				name     string
				command  []string
				nextURL  string
				conflict []string
			}{
				name:     command.name + " " + conflict.name,
				command:  command.command,
				nextURL:  "https://api.appstoreconnect.apple.com/v1/subscriptionPricePoints/point-1/" + command.path + "?cursor=next",
				conflict: conflict.args,
			})
		}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := subscriptionscli.SetPricePointsClientFactory(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("poison price-points client factory called")
			})
			t.Cleanup(restore)

			args := append(append(append([]string{}, test.command...), "--next", test.nextURL), test.conflict...)
			assertUsageExit(t, args, "--next cannot be combined")
			if clientFactoryCalled {
				t.Fatal("next conflict reached the price-points client factory")
			}
		})
	}
}

func TestSubscriptionsPricePointCommandsContinueFromOpaqueNextURL(t *testing.T) {
	tests := []struct {
		name    string
		command []string
		path    string
		limit   string
	}{
		{
			name:    "list",
			command: []string{"subscriptions", "pricing", "price-points", "list"},
			path:    "/v1/subscriptions/sub-1/pricePoints",
			limit:   "200",
		},
		{
			name:    "equalizations",
			command: []string{"subscriptions", "pricing", "price-points", "equalizations"},
			path:    "/v1/subscriptionPricePoints/point-1/equalizations",
			limit:   "8000",
		},
		{
			name:    "adjusted equalizations",
			command: []string{"subscriptions", "pricing", "price-points", "adjusted-equalizations"},
			path:    "/v1/subscriptionPricePoints/point-1/adjustedEqualizations",
			limit:   "8000",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "6759231657")
			requestCount := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requestCount++
				if req.URL.Path != test.path {
					t.Fatalf("path=%q, want %q", req.URL.Path, test.path)
				}
				if got := req.URL.Query().Get("limit"); got != test.limit {
					t.Fatalf("limit=%q, want %q", got, test.limit)
				}
				switch requestCount {
				case 1:
					if got := req.URL.Query().Get("cursor"); got != "start" {
						t.Fatalf("cursor=%q, want start", got)
					}
					next := "https://api.appstoreconnect.apple.com" + test.path + "?cursor=finish&limit=" + test.limit
					return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"point-1"}],"links":{"next":"`+next+`"}}`)
				case 2:
					if got := req.URL.Query().Get("cursor"); got != "finish" {
						t.Fatalf("cursor=%q, want finish", got)
					}
					return jsonResponse(http.StatusOK, `{"data":[{"type":"subscriptionPricePoints","id":"point-2"}],"links":{}}`)
				default:
					t.Fatalf("unexpected request %d: %s", requestCount, req.URL)
					return nil, nil
				}
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			next := "https://api.appstoreconnect.apple.com" + test.path + "?cursor=start&limit=" + test.limit
			stdout, stderr := captureOutput(t, func() {
				args := append(append([]string{}, test.command...), "--next", next, "--paginate", "--output", "json")
				if err := root.Parse(args); err != nil {
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
			var response asc.SubscriptionPricePointsResponse
			if err := json.Unmarshal([]byte(stdout), &response); err != nil {
				t.Fatalf("parse stdout: %v; stdout=%q", err, stdout)
			}
			if len(response.Data) != 2 || response.Data[0].ID != "point-1" || response.Data[1].ID != "point-2" {
				t.Fatalf("unexpected response: %#v", response.Data)
			}
		})
	}
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
		if got := req.URL.Query().Get("include"); got != "territory" {
			t.Fatalf("include=%q, want territory", got)
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
	stdout, stderr := captureOutput(t, func() {
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
	var response asc.SubscriptionPricePointsResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("parse stdout: %v; stdout=%q", err, stdout)
	}
	if len(response.Data) != 2 || response.Data[0].ID != "point-1" || response.Data[1].ID != "point-2" {
		t.Fatalf("unexpected paginated response: %#v", response.Data)
	}
}

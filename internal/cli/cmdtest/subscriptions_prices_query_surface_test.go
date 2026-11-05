package cmdtest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type subscriptionPricesListQueryCapture struct {
	calls    int
	path     string
	query    url.Values
	response func(http.ResponseWriter, *http.Request)
}

func subscriptionPricesListQuerySurfaceStub(t *testing.T) *subscriptionPricesListQueryCapture {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	captured := &subscriptionPricesListQueryCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		captured.calls++
		captured.path = req.URL.Path
		captured.query = req.URL.Query()
		if captured.response != nil {
			captured.response(w, req)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"subscriptionPrices","id":"price-1"}],"links":{"next":""}}`)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Scheme + "://" + req.URL.Host; got != asc.BaseURL {
			t.Errorf("request origin = %s, want %s", got, asc.BaseURL)
		}
		routed := req.Clone(req.Context())
		routed.URL.Scheme = serverURL.Scheme
		routed.URL.Host = serverURL.Host
		routed.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(routed)
	})
	client, err := asc.NewClientWithHTTPClient(
		"TEST_KEY",
		"TEST_ISSUER",
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: transport},
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))

	return captured
}

func (c *subscriptionPricesListQueryCapture) assertNoRequest(t *testing.T) {
	t.Helper()
	if c.calls != 0 {
		t.Fatalf("expected validation to short-circuit before HTTP, got %d call(s) to %s?%s", c.calls, c.path, c.query.Encode())
	}
}

func runSubscriptionPricesListQuerySurface(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	return stdout, stderr, runErr
}

func TestSubscriptionsPricingPricesListSendsQueryControls(t *testing.T) {
	captured := subscriptionPricesListQuerySurfaceStub(t)

	stdout, stderr, err := runSubscriptionPricesListQuerySurface(
		t,
		"subscriptions", "pricing", "prices", "list",
		"--subscription-id", "8000000001",
		"--price-point-id", "point-1, point-2",
		"--plan-type", "monthly",
		"--territory", "United States,gb",
		"--fields", "startDate,preserved",
		"--territory-fields", "currency",
		"--price-point-fields", "customerPrice,proceeds",
		"--include", "territory,subscriptionPricePoint",
		"--limit", "25",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if captured.path != "/v1/subscriptions/8000000001/prices" {
		t.Fatalf("expected subscription prices path, got %q", captured.path)
	}
	wantQuery := url.Values{
		"filter[planType]":                {"MONTHLY"},
		"filter[subscriptionPricePoint]":  {"point-1,point-2"},
		"filter[territory]":               {"USA,GBR"},
		"fields[subscriptionPrices]":      {"startDate,preserved,territory,subscriptionPricePoint"},
		"fields[territories]":             {"currency"},
		"fields[subscriptionPricePoints]": {"customerPrice,proceeds"},
		"include":                         {"territory,subscriptionPricePoint"},
		"limit":                           {"25"},
	}
	if got := captured.query.Encode(); got != wantQuery.Encode() {
		t.Fatalf("query = %q, want %q", got, wantQuery.Encode())
	}
	if !strings.Contains(stdout, `"id":"price-1"`) {
		t.Fatalf("expected raw response envelope, got %q", stdout)
	}
}

func TestSubscriptionsPricingPricesListPreservesExplicitFalseFields(t *testing.T) {
	captured := subscriptionPricesListQuerySurfaceStub(t)
	captured.response = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"type":"subscriptionPrices","id":"price-1","attributes":{"preserved":false}}],"links":{"next":""}}`)
	}

	stdout, stderr, err := runSubscriptionPricesListQuerySurface(
		t,
		"subscriptions", "pricing", "prices", "list",
		"--subscription-id", "8000000001",
		"--fields", "preserved",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"preserved":false`) {
		t.Fatalf("expected explicit false preserved field in output, got %q", stdout)
	}
}

func TestSubscriptionsPricingPricesListRendersIncludedPricePointRelationships(t *testing.T) {
	for _, outputFormat := range []string{"table", "markdown", "json"} {
		t.Run(outputFormat, func(t *testing.T) {
			captured := subscriptionPricesListQuerySurfaceStub(t)
			captured.response = func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":[{"type":"subscriptionPrices","id":"price-1","relationships":{"territory":{"data":{"type":"territories","id":"USA"}},"subscriptionPricePoint":{"data":{"type":"subscriptionPricePoints","id":"PRICE_POINT_1"}}}}],"included":[{"type":"subscriptionPricePoints","id":"PRICE_POINT_1","attributes":{"customerPrice":"9.99"},"relationships":{"territory":{"data":{"type":"territories","id":"GBR"}},"equalizations":{"links":{"related":"/v1/subscriptionPricePoints/PRICE_POINT_1/equalizations"}},"adjustedEqualizations":{"links":{"self":"/v1/subscriptionPricePoints/PRICE_POINT_1/adjustedEqualizations"}}}}],"links":{"next":""}}`)
			}

			stdout, stderr, err := runSubscriptionPricesListQuerySurface(
				t,
				"subscriptions", "pricing", "prices", "list",
				"--subscription-id", "8000000001",
				"--price-point-fields", "territory,equalizations,adjustedEqualizations",
				"--output", outputFormat,
			)
			if err != nil {
				t.Fatalf("run error: %v (stderr=%q)", err, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}

			for _, want := range []string{
				"Price Point Territory ID", "GBR",
				"Equalizations URL", "/v1/subscriptionPricePoints/PRICE_POINT_1/equalizations",
				"Adjusted Equalizations URL", "/v1/subscriptionPricePoints/PRICE_POINT_1/adjustedEqualizations",
			} {
				if outputFormat != "json" && !strings.Contains(stdout, want) {
					t.Fatalf("expected %q in %s output, got %q", want, outputFormat, stdout)
				}
			}
			if outputFormat == "json" {
				for _, want := range []string{
					`"territory":{"data":{"type":"territories","id":"GBR"}}`,
					`"equalizations":{"links":{"related":"/v1/subscriptionPricePoints/PRICE_POINT_1/equalizations"}}`,
					`"adjustedEqualizations":{"links":{"self":"/v1/subscriptionPricePoints/PRICE_POINT_1/adjustedEqualizations"}}`,
				} {
					if !strings.Contains(stdout, want) {
						t.Fatalf("expected raw relationship %q in JSON output, got %q", want, stdout)
					}
				}
			}
		})
	}
}

func TestSubscriptionsPricingPricesListPaginatePreservesQueryControls(t *testing.T) {
	captured := subscriptionPricesListQuerySurfaceStub(t)
	captured.response = func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if captured.calls == 1 {
			_, _ = io.WriteString(w, `{"data":[{"type":"subscriptionPrices","id":"price-1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptions/8000000001/prices?cursor=next"}}`)
			return
		}
		if captured.calls != 2 {
			t.Fatalf("unexpected request count: %d", captured.calls)
		}
		if got := req.URL.Query().Get("cursor"); got != "next" {
			t.Fatalf("cursor = %q, want next", got)
		}
		wantQuery := url.Values{
			"cursor":                     {"next"},
			"fields[subscriptionPrices]": {"startDate,preserved,territory"},
			"fields[territories]":        {"currency"},
			"include":                    {"territory"},
		}
		if got := req.URL.Query().Encode(); got != wantQuery.Encode() {
			t.Fatalf("continuation query = %q, want %q", got, wantQuery.Encode())
		}
		_, _ = io.WriteString(w, `{"data":[{"type":"subscriptionPrices","id":"price-2"}],"links":{"next":""}}`)
	}

	stdout, stderr, err := runSubscriptionPricesListQuerySurface(
		t,
		"subscriptions", "pricing", "prices", "list",
		"--subscription-id", "8000000001",
		"--fields", "startDate,preserved",
		"--territory-fields", "currency",
		"--include", "territory",
		"--paginate",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if captured.calls != 2 {
		t.Fatalf("expected two requests, got %d", captured.calls)
	}
	if !strings.Contains(stdout, `"id":"price-2"`) {
		t.Fatalf("expected aggregated second-page response, got %q", stdout)
	}
}

func TestSubscriptionsPricingPricesListRejectsInvalidQuerySelections(t *testing.T) {
	tests := []struct {
		name  string
		flag  string
		value string
		want  string
	}{
		{name: "include", flag: "include", value: "app", want: "--include must be one of"},
		{name: "subscription prices fields", flag: "fields", value: "name", want: "--fields must be one of"},
		{name: "territory fields", flag: "territory-fields", value: "name", want: "--territory-fields must be one of"},
		{name: "price point fields", flag: "price-point-fields", value: "name", want: "--price-point-fields must be one of"},
		{name: "territory", flag: "territory", value: "USA,Narnia", want: `territory "Narnia" could not be mapped`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			captured := subscriptionPricesListQuerySurfaceStub(t)
			_, stderr, err := runSubscriptionPricesListQuerySurface(
				t,
				"subscriptions", "pricing", "prices", "list",
				"--subscription-id", "8000000001",
				"--"+test.flag, test.value,
			)
			if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("expected stderr to contain %q, got %q", test.want, stderr)
			}
			captured.assertNoRequest(t)
		})
	}
}

func TestSubscriptionsPricingPricesListRejectsQueryControlsWithNextOrResolved(t *testing.T) {
	modes := []struct {
		flag string
		args []string
	}{
		{flag: "--next", args: []string{"--next", "https://api.appstoreconnect.apple.com/v1/subscriptions/8000000001/prices?cursor=next"}},
		{flag: "--resolved", args: []string{"--subscription-id", "8000000001", "--resolved"}},
	}

	for _, mode := range modes {
		for _, flagName := range []string{"price-point-id", "fields", "territory-fields", "price-point-fields", "include"} {
			t.Run(mode.flag+" with "+flagName, func(t *testing.T) {
				captured := subscriptionPricesListQuerySurfaceStub(t)
				args := append([]string{"subscriptions", "pricing", "prices", "list"}, mode.args...)
				_, stderr, err := runSubscriptionPricesListQuerySurface(t, append(args, "--"+flagName, "value")...)
				if got := rootcmd.ExitCodeFromError(err); got != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, err)
				}
				if want := mode.flag + " cannot be combined with --" + flagName; !strings.Contains(stderr, want) {
					t.Fatalf("expected %q, got %q", want, stderr)
				}
				captured.assertNoRequest(t)
			})
		}
	}
}

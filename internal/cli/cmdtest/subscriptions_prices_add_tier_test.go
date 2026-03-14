package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSubscriptionsPricesAdd_TierAndPricePointMutualExclusion(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "SUB_ID",
			"--price-point", "PP",
			"--tier", "5",
			"--territory", "USA",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %q", stderr)
	}
}

func TestSubscriptionsPricesAdd_TierUsesSubscriptionPricePoints(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var resolvedPricePointID string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/relationships/prices"):
			body := `{"data":[{"type":"subscriptionPrices","id":"existing-price-1"}],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/subscriptions/SUB_ID/pricePoints"):
			body := `{
				"data":[
					{"type":"subscriptionPricePoints","id":"sub-pp-1","attributes":{"customerPrice":"0.99","proceeds":"0.70"}},
					{"type":"subscriptionPricePoints","id":"sub-pp-2","attributes":{"customerPrice":"1.99","proceeds":"1.40"}},
					{"type":"subscriptionPricePoints","id":"sub-pp-3","attributes":{"customerPrice":"2.99","proceeds":"2.10"}}
				],
				"links":{"next":""}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/apps/"):
			t.Fatalf("unexpected app price points request: %s", req.URL.Path)
			return nil, nil
		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/subscriptionPrices"):
			bodyBytes, _ := io.ReadAll(req.Body)
			bodyStr := string(bodyBytes)
			if strings.Contains(bodyStr, "sub-pp-2") {
				resolvedPricePointID = "sub-pp-2"
			}
			resp := `{"data":{"type":"subscriptionPrices","id":"sub-price-1","attributes":{}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(resp)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	t.Setenv("HOME", t.TempDir())
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "SUB_ID",
			"--tier", "2",
			"--territory", "USA",
			"--refresh",
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
	if resolvedPricePointID != "sub-pp-2" {
		t.Fatalf("expected tier 2 to resolve sub-pp-2, got %q", resolvedPricePointID)
	}
	if !strings.Contains(stdout, `"id":"sub-price-1"`) {
		t.Fatalf("expected create output, got %q", stdout)
	}
}

func TestSubscriptionsPricesAdd_ProbeErrorReturnsWithoutWrite(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/relationships/prices"):
			body := `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"An unexpected error occurred.","detail":"An unexpected error occurred on the server side."}]}`
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPatch && strings.Contains(req.URL.Path, "/v1/subscriptions/"):
			t.Fatalf("unexpected PATCH write request after failed probe: %s", req.URL.Path)
			return nil, nil
		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/subscriptionPrices"):
			t.Fatalf("unexpected POST write request after failed probe: %s", req.URL.Path)
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	t.Setenv("HOME", t.TempDir())
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "SUB_ID",
			"--price-point", "PP_ID",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected command to fail when prices probe fails")
		}
		if !strings.Contains(err.Error(), "failed to check existing prices") {
			t.Fatalf("expected probe error message, got %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestSubscriptionsPricesAdd_InitialPriceForwardsAttributes(t *testing.T) {
	setupAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/relationships/prices"):
			body := `{"data":[],"links":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPatch && strings.Contains(req.URL.Path, "/v1/subscriptions/SUB_ID"):
			var payload map[string]any
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("failed to decode patch payload: %v", err)
			}

			includedAny, ok := payload["included"].([]any)
			if !ok || len(includedAny) != 1 {
				t.Fatalf("expected one included resource, got %#v", payload["included"])
			}
			included, ok := includedAny[0].(map[string]any)
			if !ok {
				t.Fatalf("expected included resource object, got %#v", includedAny[0])
			}

			attrs, ok := included["attributes"].(map[string]any)
			if !ok {
				t.Fatalf("expected included attributes object, got %#v", included["attributes"])
			}
			if attrs["startDate"] != "2026-05-01" {
				t.Fatalf("expected startDate 2026-05-01, got %#v", attrs["startDate"])
			}
			if attrs["preserveCurrentPrice"] != true {
				t.Fatalf("expected preserveCurrentPrice true, got %#v", attrs["preserveCurrentPrice"])
			}

			relationships, ok := included["relationships"].(map[string]any)
			if !ok {
				t.Fatalf("expected included relationships object, got %#v", included["relationships"])
			}
			territory, ok := relationships["territory"].(map[string]any)
			if !ok {
				t.Fatalf("expected territory relationship object, got %#v", relationships["territory"])
			}
			territoryData, ok := territory["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected territory.data object, got %#v", territory["data"])
			}
			if territoryData["id"] != "USA" {
				t.Fatalf("expected territory id USA, got %#v", territoryData["id"])
			}

			resp := `{"data":{"type":"subscriptions","id":"SUB_ID","attributes":{"name":"Monthly","productId":"com.example.sub.monthly"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(resp)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/subscriptionPrices"):
			t.Fatalf("unexpected POST request; initial pricing should use PATCH flow")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	t.Setenv("HOME", t.TempDir())
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "SUB_ID",
			"--price-point", "PP_ID",
			"--territory", "USA",
			"--start-date", "2026-05-01",
			"--preserved",
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
	if !strings.Contains(stdout, `"id":"SUB_ID"`) {
		t.Fatalf("expected subscription response in stdout, got %q", stdout)
	}
}

func TestSubscriptionsPricesAdd_TierRequiresTerritory(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "SUB_ID",
			"--tier", "5",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--territory is required") {
		t.Fatalf("expected --territory required error, got %q", stderr)
	}
}

func TestSubscriptionsPricesAdd_InvalidPriceValue(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "SUB_ID",
			"--price", "abc",
			"--territory", "USA",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--price must be a number") {
		t.Fatalf("expected invalid --price error, got %q", stderr)
	}
}

func TestSubscriptionsPricesAdd_NegativeTier(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "pricing", "prices", "set",
			"--subscription-id", "SUB_ID",
			"--tier", "-1",
			"--territory", "USA",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--tier must be a positive integer") {
		t.Fatalf("expected invalid --tier error, got %q", stderr)
	}
}

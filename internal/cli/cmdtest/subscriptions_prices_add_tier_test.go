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

func TestSubscriptionsPricesAdd_TierAndPricePointMutualExclusion(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "prices", "add",
			"--id", "SUB_ID",
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
			"subscriptions", "prices", "add",
			"--id", "SUB_ID",
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

func TestSubscriptionsPricesAdd_TierRequiresTerritory(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "prices", "add",
			"--id", "SUB_ID",
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
			"subscriptions", "prices", "add",
			"--id", "SUB_ID",
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
			"subscriptions", "prices", "add",
			"--id", "SUB_ID",
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

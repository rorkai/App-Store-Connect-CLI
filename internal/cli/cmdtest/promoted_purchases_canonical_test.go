package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestIAPPromotedPurchasesCreateAcceptsNormalizedProductTypeAlias(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/promotedPurchases" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}

		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		body := string(bodyBytes)
		if !strings.Contains(body, `"inAppPurchaseV2"`) {
			t.Fatalf("expected in-app purchase relationship in body, got %s", body)
		}

		resp := `{"data":{"type":"promotedPurchases","id":"promo-1","attributes":{"enabled":true}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(resp)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "promoted-purchases", "create",
			"--app", "APP_ID",
			"--product-id", "IAP_ID",
			"--product-type", "in-app-purchase",
			"--visible-for-all-users", "true",
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
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "promo-1" {
		t.Fatalf("expected promoted purchase id promo-1, got %q", out.Data.ID)
	}
}

func TestSubscriptionsPromotedPurchasesCreateAcceptsNormalizedProductTypeAlias(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/promotedPurchases" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}

		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		body := string(bodyBytes)
		if !strings.Contains(body, `"subscription"`) {
			t.Fatalf("expected subscription relationship in body, got %s", body)
		}

		resp := `{"data":{"type":"promotedPurchases","id":"promo-2","attributes":{"enabled":true}}}`
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(strings.NewReader(resp)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "promoted-purchases", "create",
			"--app", "APP_ID",
			"--product-id", "SUB_ID",
			"--product-type", "subscription",
			"--visible-for-all-users", "true",
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
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "promo-2" {
		t.Fatalf("expected promoted purchase id promo-2, got %q", out.Data.ID)
	}
}

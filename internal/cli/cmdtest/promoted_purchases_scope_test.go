package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestIAPPromotedPurchasesListFiltersToInAppPurchases(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/promotedPurchases" {
			t.Fatalf("expected app promoted purchases path, got %s", req.URL.Path)
		}

		body := `{"data":[` +
			`{"type":"promotedPurchases","id":"promo-iap","relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"iap-1"}}}},` +
			`{"type":"promotedPurchases","id":"promo-sub","relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-1"}}}}` +
			`],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "promoted-purchases", "list", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"promo-iap"`) {
		t.Fatalf("expected output to contain iap promoted purchase, got %q", stdout)
	}
	if strings.Contains(stdout, `"id":"promo-sub"`) {
		t.Fatalf("expected output to exclude subscription promoted purchase, got %q", stdout)
	}
}

func TestSubscriptionsPromotedPurchasesListFiltersToSubscriptions(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/promotedPurchases" {
			t.Fatalf("expected app promoted purchases path, got %s", req.URL.Path)
		}

		body := `{"data":[` +
			`{"type":"promotedPurchases","id":"promo-iap","relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"iap-1"}}}},` +
			`{"type":"promotedPurchases","id":"promo-sub","relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-1"}}}}` +
			`],"links":{"next":""}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "promoted-purchases", "list", "--app", "app-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"promo-sub"`) {
		t.Fatalf("expected output to contain subscription promoted purchase, got %q", stdout)
	}
	if strings.Contains(stdout, `"id":"promo-iap"`) {
		t.Fatalf("expected output to exclude in-app promoted purchase, got %q", stdout)
	}
}

func TestIAPPromotedPurchasesGetRejectsSubscriptionPromotedPurchase(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/promotedPurchases/promo-sub" {
			t.Fatalf("expected promoted purchase detail path, got %s", req.URL.Path)
		}

		body := `{"data":{"type":"promotedPurchases","id":"promo-sub","relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-1"}}}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "promoted-purchases", "view", "--promoted-purchase-id", "promo-sub"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(runErr.Error(), `belongs to subscription "sub-1", not an in-app purchase`) {
		t.Fatalf("expected scope mismatch error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestIAPPromotedPurchasesLinkPreservesSubscriptionPromotedPurchases(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.String() != "https://api.appstoreconnect.apple.com/v1/apps/app-1/promotedPurchases?limit=200" {
				t.Fatalf("unexpected current promoted purchases request: %s", req.URL.String())
			}
			body := `{"data":[` +
				`{"type":"promotedPurchases","id":"promo-sub","relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-1"}}}},` +
				`{"type":"promotedPurchases","id":"promo-iap-old","relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"iap-old"}}}}` +
				`],"links":{"next":""}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/promotedPurchases/promo-iap-new" {
				t.Fatalf("unexpected promoted purchase validation path: %s", req.URL.Path)
			}
			body := `{"data":{"type":"promotedPurchases","id":"promo-iap-new","relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"iap-new"}}}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodPatch {
				t.Fatalf("expected PATCH, got %s", req.Method)
			}
			if req.URL.Path != "/v1/apps/app-1/relationships/promotedPurchases" {
				t.Fatalf("unexpected promoted purchase link path: %s", req.URL.Path)
			}

			var payload asc.RelationshipRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode relationship payload: %v", err)
			}
			if len(payload.Data) != 2 {
				t.Fatalf("expected preserved subscription and replacement iap id, got %d items", len(payload.Data))
			}
			if payload.Data[0].ID != "promo-sub" || payload.Data[1].ID != "promo-iap-new" {
				t.Fatalf("unexpected promoted purchase IDs: %+v", payload.Data)
			}

			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"iap", "promoted-purchases", "link", "--app", "app-1", "--promoted-purchase-id", "promo-iap-new"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"promo-sub"`) || !strings.Contains(stdout, `"promo-iap-new"`) {
		t.Fatalf("expected output to contain preserved subscription and new iap IDs, got %q", stdout)
	}
	if strings.Contains(stdout, `"promo-iap-old"`) {
		t.Fatalf("expected output to exclude replaced iap ID, got %q", stdout)
	}
}

package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func setupStableSelectorAuth(t *testing.T) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
}

// selectorLookupHangGuard bounds request budgets once a selector lookup has
// timed out. It only stops a hung test; no assertion depends on it.
const selectorLookupHangGuard = 30 * time.Second

// expireSelectorLookup gives the command a 10 ms request budget so its selector
// lookup times out for real, and returns the transport handler for that lookup.
// The handler waits for the lookup's own deadline, then raises ASC_TIMEOUT to
// selectorLookupHangGuard, so the requests that follow the fallback cannot time
// out on a loaded host. A fallback request that reused the lookup's context or
// budget would still see it expired, so fresh-context assertions keep their
// meaning. Call it after the auth setup, which the client factory reads.
func expireSelectorLookup(t *testing.T) func(*http.Request) (*http.Response, error) {
	t.Helper()
	t.Setenv("ASC_TIMEOUT", "10ms")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	// The client fixes its whole-request HTTP timeout when it is built, so build
	// it with the hang guard; only the per-request budgets start at 10 ms.
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return asc.NewClientWithTimeout(
			os.Getenv("ASC_KEY_ID"),
			os.Getenv("ASC_ISSUER_ID"),
			os.Getenv("ASC_PRIVATE_KEY_PATH"),
			selectorLookupHangGuard,
		)
	}))
	return func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		t.Setenv("ASC_TIMEOUT", selectorLookupHangGuard.String())
		return nil, req.Context().Err()
	}
}

func selectorJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func TestIAPContentGetResolvesStableSelectorViaASCAppID(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "app-123")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch req.URL.Path {
		case "/v1/apps/app-123/inAppPurchasesV2":
			if req.URL.Query().Get("filter[productId]") != "com.example.pro" {
				t.Fatalf("expected product filter on lookup request, got %q", req.URL.Query().Get("filter[productId]"))
			}
			return selectorJSONResponse(`{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"CONSUMABLE"}}]}`), nil
		case "/v2/inAppPurchases/iap-1/content":
			return selectorJSONResponse(`{"data":{"type":"inAppPurchaseContents","id":"content-1"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{"iap", "content", "view", "--iap-id", "com.example.pro"})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 2 {
		t.Fatalf("expected 2 requests, got %d", requests)
	}

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "content-1" {
		t.Fatalf("expected content id content-1, got %q", out.Data.ID)
	}
}

func TestIAPContentGetResolvesNumericStableSelectorWhenAppProvided(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch req.URL.Path {
		case "/v1/apps/app-123/inAppPurchasesV2":
			if req.URL.Query().Get("filter[productId]") != "2024" {
				t.Fatalf("expected numeric product filter on lookup request, got %q", req.URL.Query().Get("filter[productId]"))
			}
			return selectorJSONResponse(`{"data":[{"type":"inAppPurchases","id":"iap-2024","attributes":{"name":"Spring Sale","productId":"2024","inAppPurchaseType":"CONSUMABLE"}}]}`), nil
		case "/v2/inAppPurchases/iap-2024/content":
			return selectorJSONResponse(`{"data":{"type":"inAppPurchaseContents","id":"content-2024"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"iap", "content", "view",
		"--app", "app-123",
		"--iap-id", "2024",
	})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 2 {
		t.Fatalf("expected 2 requests, got %d", requests)
	}

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "content-2024" {
		t.Fatalf("expected content id content-2024, got %q", out.Data.ID)
	}
}

func TestIAPContentGetFallsBackToNumericIDWhenLookupErrors(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch req.URL.Path {
		case "/v1/apps/app-123/inAppPurchasesV2":
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Body: io.NopCloser(strings.NewReader(`{
					"errors":[{"status":"403","code":"FORBIDDEN.REQUIRED_ROLE","detail":"forbidden"}]
				}`)),
				Header: http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v2/inAppPurchases/2024/content":
			return selectorJSONResponse(`{"data":{"type":"inAppPurchaseContents","id":"content-2024"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"iap", "content", "view",
		"--app", "app-123",
		"--iap-id", "2024",
	})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 2 {
		t.Fatalf("expected lookup attempt followed by direct fetch, got %d requests", requests)
	}

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "content-2024" {
		t.Fatalf("expected content id content-2024, got %q", out.Data.ID)
	}
}

func TestStableSelectorFallsBackToNumericIDAfterLookupTimeout(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		lookupPath string
		fetchPath  string
		body       string
		wantStdout string
	}{
		{
			name:       "iap content view",
			args:       []string{"iap", "content", "view", "--app", "app-123", "--iap-id", "2024"},
			lookupPath: "/v1/apps/app-123/inAppPurchasesV2",
			fetchPath:  "/v2/inAppPurchases/2024/content",
			body:       `{"data":{"type":"inAppPurchaseContents","id":"content-2024"}}`,
			wantStdout: `"id":"content-2024"`,
		},
		{
			name:       "subscription versions list",
			args:       []string{"subscriptions", "versions", "list", "--app", "app-123", "--subscription-id", "2024"},
			lookupPath: "/v1/apps/app-123/subscriptionGroups",
			fetchPath:  "/v1/subscriptions/2024/versions",
			body:       `{"data":[]}`,
			wantStdout: `"data"`,
		},
		{
			name:       "win-back offer links",
			args:       []string{"subscriptions", "offers", "win-back", "links", "--app", "app-123", "--subscription-id", "2024"},
			lookupPath: "/v1/apps/app-123/subscriptionGroups",
			fetchPath:  "/v1/subscriptions/2024/relationships/winBackOffers",
			body:       `{"data":[]}`,
			wantStdout: `"data"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupStableSelectorAuth(t)
			t.Setenv("ASC_APP_ID", "")
			lookupTimeout := expireSelectorLookup(t)

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })

			requests := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				switch req.URL.Path {
				case test.lookupPath:
					return lookupTimeout(req)
				case test.fetchPath:
					if err := req.Context().Err(); err != nil {
						t.Fatalf("expected fresh fetch context after lookup timeout, got %v", err)
					}
					return selectorJSONResponse(test.body), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})

			stdout, stderr, runErr := runRootCommand(t, test.args)
			if runErr != nil {
				t.Fatalf("expected nil error, got %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if requests != 2 {
				t.Fatalf("expected lookup timeout followed by direct fetch, got %d requests", requests)
			}
			if !strings.Contains(stdout, test.wantStdout) {
				t.Fatalf("expected stdout to contain %q, got %q", test.wantStdout, stdout)
			}
		})
	}
}

func TestIAPContentViewDoesNotSuppressNumericAmbiguity(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch req.URL.Path {
		case "/v1/apps/app-123/inAppPurchasesV2":
			switch req.URL.Query().Get("filter[productId]") {
			case "2024":
				return selectorJSONResponse(`{"data":[]}`), nil
			case "":
				if req.URL.Query().Get("filter[name]") != "2024" {
					t.Fatalf("expected name filter on second lookup request, got %q", req.URL.Query().Encode())
				}
				return selectorJSONResponse(`{"data":[
					{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"2024","productId":"com.example.one","inAppPurchaseType":"CONSUMABLE"}},
					{"type":"inAppPurchases","id":"iap-2","attributes":{"name":"2024","productId":"com.example.two","inAppPurchaseType":"CONSUMABLE"}}
				]}`), nil
			default:
				t.Fatalf("unexpected lookup query: %s", req.URL.RawQuery)
				return nil, nil
			}
		case "/v2/inAppPurchases/2024/content":
			t.Fatal("expected ambiguity to stop before direct numeric ID fetch")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	_, _, runErr := runRootCommand(t, []string{
		"iap", "content", "view",
		"--app", "app-123",
		"--iap-id", "2024",
	})
	if runErr == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(runErr.Error(), "pass --iap-id with one of:") {
		t.Fatalf("expected disambiguation guidance, got %v", runErr)
	}
	if requests != 2 {
		t.Fatalf("expected product-id and name lookup requests before error, got %d", requests)
	}
}

func TestSubscriptionReviewScreenshotGetResolvesStableSelectorWithAppFlag(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch req.URL.Path {
		case "/v1/apps/app-1/subscriptionGroups":
			return selectorJSONResponse(`{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}]}`), nil
		case "/v1/subscriptionGroups/group-1/subscriptions":
			if req.URL.Query().Get("filter[productId]") != "com.example.monthly" {
				t.Fatalf("expected product filter on lookup request, got %q", req.URL.Query().Get("filter[productId]"))
			}
			return selectorJSONResponse(`{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly"}}]}`), nil
		case "/v1/subscriptions/sub-1/appStoreReviewScreenshot":
			return selectorJSONResponse(`{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"subscriptions", "review", "app-store-screenshot", "view",
		"--app", "app-1",
		"--subscription-id", "com.example.monthly",
	})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 3 {
		t.Fatalf("expected 3 requests, got %d", requests)
	}

	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.Data.ID != "shot-1" {
		t.Fatalf("expected screenshot id shot-1, got %q", out.Data.ID)
	}
}

func TestSubscriptionPromotedPurchaseViewResolvesStableSelectorWithAppFlag(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet {
			t.Fatalf("request method = %s, want GET", req.Method)
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/subscriptionGroups":
			query := req.URL.Query()
			if len(query) != 1 || len(query["limit"]) != 1 || query["limit"][0] != "200" {
				t.Fatalf("subscription-group query = %v, want exactly limit=200", query)
			}
			return selectorJSONResponse(`{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}]}`), nil
		case "/v1/subscriptionGroups/group-1/subscriptions":
			query := req.URL.Query()
			if len(query) != 2 || len(query["limit"]) != 1 || query["limit"][0] != "200" {
				t.Fatalf("subscription query = %v, want exactly product filter and limit=200", query)
			}
			if got := query["filter[productId]"]; len(got) != 1 || got[0] != "com.example.monthly" {
				t.Fatalf("product filter = %v, want exactly [com.example.monthly]", got)
			}
			return selectorJSONResponse(`{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly"}}]}`), nil
		case "/v1/subscriptions/sub-1/promotedPurchase":
			query := req.URL.Query()
			want := map[string]string{
				"fields[inAppPurchases]": "versions",
				"fields[subscriptions]":  "versions",
				"include":                "inAppPurchaseV2,subscription",
			}
			if len(query) != len(want) {
				t.Fatalf("query = %v, want exactly %v", query, want)
			}
			for key, wantValue := range want {
				if got := query[key]; len(got) != 1 || got[0] != wantValue {
					t.Fatalf("query[%q] = %v, want exactly [%q]", key, got, wantValue)
				}
			}
			return selectorJSONResponse(`{"data":{"type":"promotedPurchases","id":"promo-1"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"subscriptions", "promoted-purchases", "view",
		"--app", "app-1",
		"--subscription-id", "com.example.monthly",
		"--iap-fields", "versions",
		"--subscription-fields", "versions",
	})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 3 || !strings.Contains(stdout, `"id":"promo-1"`) {
		t.Fatalf("requests=%d stdout=%q", requests, stdout)
	}
}

func TestIAPOfferCodesCreateStopsBeforeMutationWhenLookupFails(t *testing.T) {
	setupStableSelectorAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method == http.MethodPost {
			t.Fatalf("unexpected mutation request during failed lookup: %s %s", req.Method, req.URL.String())
		}
		if req.URL.Path != "/v1/apps/app-1/inAppPurchasesV2" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return selectorJSONResponse(`{"data":[]}`), nil
	})

	_, _, runErr := runRootCommand(t, []string{
		"iap", "offer-codes", "create",
		"--app", "app-1",
		"--iap-id", "com.example.missing",
		"--name", "SPRING",
		"--prices", "usa:pp-us",
	})
	if runErr == nil {
		t.Fatal("expected lookup error")
	}
	if !strings.Contains(runErr.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", runErr)
	}
	if requests == 0 {
		t.Fatal("expected lookup requests before failure")
	}
}

func TestSubscriptionsOfferCodesCreateStopsBeforeMutationWhenLookupFails(t *testing.T) {
	setupStableSelectorAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionOfferCodes" {
			t.Fatalf("unexpected mutation request during failed lookup: %s %s", req.Method, req.URL.String())
		}
		switch req.URL.Path {
		case "/v1/apps/app-1/subscriptionGroups":
			return selectorJSONResponse(`{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}]}`), nil
		case "/v1/subscriptionGroups/group-1/subscriptions":
			return selectorJSONResponse(`{"data":[]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	_, _, runErr := runRootCommand(t, []string{
		"subscriptions", "offers", "offer-codes", "create",
		"--app", "app-1",
		"--subscription-id", "com.example.missing",
		"--name", "SPRING",
		"--offer-eligibility", "STACK_WITH_INTRO_OFFERS",
		"--customer-eligibilities", "NEW",
		"--offer-duration", "ONE_MONTH",
		"--offer-mode", "PAY_AS_YOU_GO",
		"--number-of-periods", "1",
		"--prices", "usa:pp-us",
	})
	if runErr == nil {
		t.Fatal("expected lookup error")
	}
	if !strings.Contains(runErr.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", runErr)
	}
	if requests == 0 {
		t.Fatal("expected lookup requests before failure")
	}
}

func TestSubscriptionsOfferCodesCreateFreeTrialResolvesStableSelector(t *testing.T) {
	setupStableSelectorAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-1/subscriptionGroups":
			return selectorJSONResponse(`{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}]}`), nil
		case "/v1/subscriptionGroups/group-1/subscriptions":
			if req.URL.Query().Get("filter[productId]") != "com.example.monthly" {
				t.Fatalf("expected product filter on lookup request, got %q", req.URL.Query().Get("filter[productId]"))
			}
			return selectorJSONResponse(`{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly"}}]}`), nil
		case "/v1/subscriptionOfferCodes":
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST to /v1/subscriptionOfferCodes, got %s", req.Method)
			}
			rawBody, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			if !strings.Contains(string(rawBody), `"subscription":{"data":{"type":"subscriptions","id":"sub-1"}}`) {
				t.Fatalf("expected resolved subscription sub-1 in request body, got %s", rawBody)
			}
			body := `{"data":{"type":"subscriptionOfferCodes","id":"sub-offer-ft-2","attributes":{"name":"SPRING","active":true}}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"subscriptions", "offers", "offer-codes", "create",
		"--app", "app-1",
		"--subscription-id", "com.example.monthly",
		"--name", "SPRING",
		"--offer-eligibility", "STACK_WITH_INTRO_OFFERS",
		"--customer-eligibilities", "NEW",
		"--offer-duration", "ONE_MONTH",
		"--offer-mode", "FREE_TRIAL",
		"--number-of-periods", "1",
		"--prices", "DE",
	})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var resp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if resp.Data.ID != "sub-offer-ft-2" {
		t.Fatalf("expected offer code id sub-offer-ft-2, got %q", resp.Data.ID)
	}
}

func TestSubscriptionsOfferCodesCreateFreeTrialWithPricePointIsRejected(t *testing.T) {
	_, stderr, runErr := runRootCommand(t, []string{
		"subscriptions", "offers", "offer-codes", "create",
		"--subscription-id", "8000000001",
		"--name", "SPRING",
		"--offer-eligibility", "STACK_WITH_INTRO_OFFERS",
		"--customer-eligibilities", "NEW",
		"--offer-duration", "ONE_MONTH",
		"--offer-mode", "FREE_TRIAL",
		"--number-of-periods", "1",
		"--prices", "usa:pp-us",
	})
	if runErr == nil {
		t.Fatal("expected error for FREE_TRIAL with a price point, got nil")
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp (exit 2), got %v", runErr)
	}
	if !strings.Contains(stderr, "--prices for FREE_TRIAL must use TERRITORY entries without price point IDs") {
		t.Fatalf("expected validation message in stderr, got %q", stderr)
	}
}

func TestWinBackOffersLinksResolvesStableSelector(t *testing.T) {
	setupStableSelectorAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch req.URL.Path {
		case "/v1/apps/app-1/subscriptionGroups":
			return selectorJSONResponse(`{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}]}`), nil
		case "/v1/subscriptionGroups/group-1/subscriptions":
			if req.URL.Query().Get("filter[productId]") != "com.example.monthly" {
				t.Fatalf("expected product filter on lookup request, got %q", req.URL.Query().Get("filter[productId]"))
			}
			return selectorJSONResponse(`{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly"}}]}`), nil
		case "/v1/subscriptions/sub-1/relationships/winBackOffers":
			return selectorJSONResponse(`{"data":[]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"subscriptions", "offers", "win-back", "links",
		"--app", "app-1",
		"--subscription-id", "com.example.monthly",
	})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 3 {
		t.Fatalf("expected 3 requests, got %d", requests)
	}
	if !strings.Contains(stdout, `"data"`) {
		t.Fatalf("expected JSON output, got %q", stdout)
	}
}

func TestStableSelectorNextBypassesLookup(t *testing.T) {
	setupStableSelectorAuth(t)

	tests := []struct {
		name string
		args []string
		path string
	}{
		{
			name: "iap price points",
			args: []string{"iap", "pricing", "price-points", "list", "--next", "https://api.appstoreconnect.apple.com/v2/inAppPurchases/iap-1/pricePoints?cursor=abc"},
			path: "/v2/inAppPurchases/iap-1/pricePoints",
		},
		{
			name: "subscription price points",
			args: []string{"subscriptions", "pricing", "price-points", "list", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/pricePoints?cursor=abc"},
			path: "/v1/subscriptions/sub-1/pricePoints",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })

			requests := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.URL.Path != test.path {
					t.Fatalf("expected direct next request to %s, got %s", test.path, req.URL.Path)
				}
				if req.URL.Query().Get("cursor") != "abc" {
					t.Fatalf("expected cursor query, got %q", req.URL.RawQuery)
				}
				return selectorJSONResponse(`{"data":[]}`), nil
			})

			stdout, stderr, runErr := runRootCommand(t, test.args)
			if runErr != nil {
				t.Fatalf("expected nil error, got %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if requests != 1 {
				t.Fatalf("expected exactly one request, got %d", requests)
			}
			if !strings.Contains(stdout, `"data"`) {
				t.Fatalf("expected JSON output, got %q", stdout)
			}
		})
	}
}

func TestStableSelectorAlternateSelectorsBypassLookup(t *testing.T) {
	setupStableSelectorAuth(t)

	tests := []struct {
		name string
		args []string
		path string
		body string
	}{
		{
			name: "content id",
			args: []string{"iap", "content", "view", "--content-id", "content-1"},
			path: "/v1/inAppPurchaseContents/content-1",
			body: `{"data":{"type":"inAppPurchaseContents","id":"content-1"}}`,
		},
		{
			name: "schedule id",
			args: []string{"iap", "pricing", "schedules", "view", "--schedule-id", "schedule-1"},
			path: "/v1/inAppPurchasePriceSchedules/schedule-1",
			body: `{"data":{"type":"inAppPurchasePriceSchedules","id":"schedule-1"}}`,
		},
		{
			name: "screenshot id",
			args: []string{"iap", "review-screenshots", "view", "--screenshot-id", "shot-1"},
			path: "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1",
			body: `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1"}}`,
		},
		{
			name: "availability id",
			args: []string{"subscriptions", "pricing", "availability", "view", "--availability-id", "avail-1"},
			path: "/v1/subscriptionAvailabilities/avail-1",
			body: `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })

			requests := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.URL.Path != test.path {
					t.Fatalf("expected direct resource request to %s, got %s", test.path, req.URL.Path)
				}
				return selectorJSONResponse(test.body), nil
			})

			_, stderr, runErr := runRootCommand(t, test.args)
			if runErr != nil {
				t.Fatalf("expected nil error, got %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if requests != 1 {
				t.Fatalf("expected exactly one request, got %d", requests)
			}
		})
	}
}

func TestStableSelectorMissingAppContextShowsUsageError(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"subscriptions", "review", "app-store-screenshot", "view", "--subscription-id", "com.example.monthly"}); err != nil {
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
	if !strings.Contains(stderr, "Error: --app is required (or set ASC_APP_ID) when --subscription-id is a product ID or name") {
		t.Fatalf("expected missing app guidance, got %q", stderr)
	}
}

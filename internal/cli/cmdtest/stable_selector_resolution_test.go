package cmdtest

import (
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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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

func registerMonthlySubscriptionLookup(reg *asctest.Registry) {
	reg.Register(asctest.GET("/v1/apps/1/subscriptionGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}]}`))
	reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/subscriptions", "filter[productId]=com.example.monthly", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly"}}]}`))
}

func TestIAPContentGetResolvesStableSelectorViaASCAppID(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "123")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/inAppPurchasesV2", "filter[productId]=com.example.pro", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"CONSUMABLE"}}]}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/iap-1/content"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"inAppPurchaseContents","id":"content-1"}}`))

	stdout, stderr, runErr := runRootCommand(t, []string{"iap", "content", "view", "--iap-id", "com.example.pro"})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests := len(reg.Requests()); requests != 2 {
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/inAppPurchasesV2", "filter[productId]=2024", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"inAppPurchases","id":"iap-2024","attributes":{"name":"Spring Sale","productId":"2024","inAppPurchaseType":"CONSUMABLE"}}]}`))
	reg.Register(asctest.GET("/v2/inAppPurchases/iap-2024/content"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"inAppPurchaseContents","id":"content-2024"}}`))

	stdout, stderr, runErr := runRootCommand(t, []string{
		"iap", "content", "view",
		"--app", "123",
		"--iap-id", "2024",
	})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests := len(reg.Requests()); requests != 2 {
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

func TestIAPContentGetResolvesConfiguredBundleBeforeMembership(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "com.example.app")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		switch req.URL.Path {
		case "/v1/apps":
			if got := req.URL.Query().Get("filter[bundleId]"); got != "com.example.app" {
				t.Fatalf("expected bundle app lookup, got %q", got)
			}
			return selectorJSONResponse(`{"data":[{"type":"apps","id":"app-123","attributes":{"bundleId":"com.example.app","name":"Example"}}],"links":{"next":""}}`), nil
		case "/v1/apps/app-123/inAppPurchasesV2":
			if got := req.URL.Query().Get("filter[productId]"); got != "2024" {
				t.Fatalf("expected numeric product lookup, got %q", got)
			}
			return selectorJSONResponse(`{"data":[{"type":"inAppPurchases","id":"iap-2024","attributes":{"name":"Spring Sale","productId":"2024","inAppPurchaseType":"CONSUMABLE"}}],"links":{"next":""}}`), nil
		case "/v2/inAppPurchases/iap-2024/content":
			return selectorJSONResponse(`{"data":{"type":"inAppPurchaseContents","id":"content-2024"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"iap", "content", "view",
		"--iap-id", "2024",
	})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 3 {
		t.Fatalf("expected app lookup, scoped membership, and content requests, got %d", requests)
	}
	if !strings.Contains(stdout, `"id":"content-2024"`) {
		t.Fatalf("expected content output, got %q", stdout)
	}
}

func TestIAPContentRejectsConfiguredBundleNameCollision(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "com.example.app")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Path != "/v1/apps" {
			t.Fatalf("unexpected product or resource request after bundle/name collision: %s %s", req.Method, req.URL.String())
		}
		switch {
		case req.URL.Query().Get("filter[bundleId]") == "com.example.app":
			return selectorJSONResponse(`{"data":[],"links":{"next":""}}`), nil
		case req.URL.Query().Get("filter[name]") == "com.example.app":
			return selectorJSONResponse(`{"data":[{"type":"apps","id":"app-name-collision","attributes":{"bundleId":"com.other.app","name":"com.example.app"}}],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected app lookup after configured bundle miss: %s", req.URL.String())
			return nil, nil
		}
	})

	stdout, _, runErr := runRootCommand(t, []string{
		"iap", "content", "view",
		"--iap-id", "2024",
	})
	if runErr == nil {
		t.Fatal("expected configured bundle miss to reject before product lookup")
	}
	if stdout != "" {
		t.Fatalf("expected no product output after configured bundle miss, got %q", stdout)
	}
	if requests != 1 {
		t.Fatalf("expected one exact bundle lookup and no product access, got %d requests", requests)
	}
}

func TestIAPContentPreservesExplicitAppNameLookup(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps":
			switch {
			case req.URL.Query().Get("filter[bundleId]") == "Example App":
				return selectorJSONResponse(`{"data":[],"links":{"next":""}}`), nil
			case req.URL.Query().Get("filter[name]") == "Example App":
				return selectorJSONResponse(`{"data":[{"type":"apps","id":"app-123","attributes":{"bundleId":"com.example.app","name":"Example App"}}],"links":{"next":""}}`), nil
			default:
				t.Fatalf("unexpected app lookup: %s", req.URL.String())
				return nil, nil
			}
		case "/v1/apps/app-123/inAppPurchasesV2":
			if got := req.URL.Query().Get("filter[productId]"); got != "2024" {
				t.Fatalf("expected product lookup after explicit app-name resolution, got %q", got)
			}
			return selectorJSONResponse(`{"data":[{"type":"inAppPurchases","id":"iap-2024","attributes":{"name":"Spring Sale","productId":"2024","inAppPurchaseType":"CONSUMABLE"}}],"links":{"next":""}}`), nil
		case "/v2/inAppPurchases/iap-2024/content":
			return selectorJSONResponse(`{"data":{"type":"inAppPurchaseContents","id":"content-2024"}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"iap", "content", "view",
		"--app", "Example App",
		"--iap-id", "2024",
	})
	if runErr != nil {
		t.Fatalf("expected explicit app name to resolve, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"content-2024"`) {
		t.Fatalf("expected content output, got %q", stdout)
	}
}

func TestIAPContentGetRejectsNumericIDWhenLookupErrors(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/inAppPurchasesV2", "filter[productId]=2024", "limit=200"),
		asctest.JSON(http.StatusForbidden, `{
					"errors":[{"status":"403","code":"FORBIDDEN.REQUIRED_ROLE","detail":"forbidden"}]
				}`))

	stdout, stderr, runErr := runRootCommand(t, []string{
		"iap", "content", "view",
		"--app", "123",
		"--iap-id", "2024",
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "resolve in-app purchase by product ID") {
		t.Fatalf("expected lookup error before direct fetch, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if requests := len(reg.Requests()); requests != 1 {
		t.Fatalf("expected lookup attempt with no direct fetch, got %d requests", requests)
	}
}

func TestStableSelectorRejectsNumericIDAfterLookupTimeout(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		lookupPath string
	}{
		{
			name:       "iap content view",
			args:       []string{"iap", "content", "view", "--app", "123", "--iap-id", "2024"},
			lookupPath: "/v1/apps/123/inAppPurchasesV2",
		},
		{
			name:       "subscription versions list",
			args:       []string{"subscriptions", "versions", "list", "--app", "123", "--subscription-id", "2024"},
			lookupPath: "/v1/apps/123/subscriptionGroups",
		},
		{
			name:       "win-back offer links",
			args:       []string{"subscriptions", "offers", "win-back", "links", "--app", "123", "--subscription-id", "2024"},
			lookupPath: "/v1/apps/123/subscriptionGroups",
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
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			})

			stdout, stderr, runErr := runRootCommand(t, test.args)
			if runErr == nil || !strings.Contains(runErr.Error(), "request failed") {
				t.Fatalf("expected lookup timeout before direct fetch, got %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if requests != 1 {
				t.Fatalf("expected lookup timeout with no direct fetch, got %d requests", requests)
			}
		})
	}
}

func TestIAPContentViewDoesNotSuppressNumericAmbiguity(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/inAppPurchasesV2", "filter[productId]=2024", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/123/inAppPurchasesV2", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[
					{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"2024","productId":"com.example.one","inAppPurchaseType":"CONSUMABLE"}}
				],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/123/inAppPurchasesV2?cursor=page-2&limit=200"}}`))
	reg.Register(asctest.GET("/v1/apps/123/inAppPurchasesV2", "cursor=page-2", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[
					{"type":"inAppPurchases","id":"iap-2","attributes":{"name":"2024","productId":"com.example.two","inAppPurchaseType":"CONSUMABLE"}}
				],"links":{"next":""}}`))

	_, _, runErr := runRootCommand(t, []string{
		"iap", "content", "view",
		"--app", "123",
		"--iap-id", "2024",
	})
	if runErr == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(runErr.Error(), "pass --iap-id with one of:") {
		t.Fatalf("expected disambiguation guidance, got %v", runErr)
	}
	if requests := len(reg.Requests()); requests != 3 {
		t.Fatalf("expected product-id and two paginated scan requests before error, got %d", requests)
	}
}

func TestIAPContentRejectsDriftedMembershipPage(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/123/inAppPurchasesV2", "filter[productId]=2024", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/123/inAppPurchasesV2", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Spring Sale","productId":"com.example.spring"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/999/inAppPurchasesV2?cursor=drift&limit=200"}}`))

	_, _, runErr := runRootCommand(t, []string{
		"iap", "content", "view",
		"--app", "123",
		"--iap-id", "2024",
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "pagination URL escaped scoped collection") {
		t.Fatalf("expected scoped pagination rejection before content read, got %v", runErr)
	}
	if requests := len(reg.Requests()); requests != 2 {
		t.Fatalf("expected product lookup and one scoped page before rejection, got %d", requests)
	}
}

func TestSubscriptionSelectorRejectsDriftedAppGroupPage(t *testing.T) {
	setupStableSelectorAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/1/subscriptionGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1"}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/999/subscriptionGroups?cursor=drift&limit=200"}}`))

	_, _, runErr := runRootCommand(t, []string{
		"subscriptions", "review", "app-store-screenshot", "view",
		"--app", "1",
		"--subscription-id", "com.example.monthly",
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "pagination URL escaped scoped collection") {
		t.Fatalf("expected app-scoped group pagination rejection before screenshot read, got %v", runErr)
	}
	if requests := len(reg.Requests()); requests != 1 {
		t.Fatalf("expected one app group page before rejection, got %d", requests)
	}
}

func TestSubscriptionSelectorRejectsDriftedGroupSubscriptionPage(t *testing.T) {
	setupStableSelectorAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/1/subscriptionGroups", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1"}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/subscriptions", "filter[productId]=com.example.monthly", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/subscriptionGroups/group-999/subscriptions?cursor=drift&limit=200"}}`))

	_, _, runErr := runRootCommand(t, []string{
		"subscriptions", "review", "app-store-screenshot", "view",
		"--app", "1",
		"--subscription-id", "com.example.monthly",
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "pagination URL escaped scoped collection") {
		t.Fatalf("expected group-scoped subscription pagination rejection before screenshot read, got %v", runErr)
	}
	if requests := len(reg.Requests()); requests != 2 {
		t.Fatalf("expected group page and one scoped subscription page before rejection, got %d", requests)
	}
}

func TestSubscriptionReviewScreenshotGetResolvesStableSelectorWithAppFlag(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerMonthlySubscriptionLookup(reg)
	reg.Register(asctest.GET("/v1/subscriptions/sub-1/appStoreReviewScreenshot"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1"}}`))

	stdout, stderr, runErr := runRootCommand(t, []string{
		"subscriptions", "review", "app-store-screenshot", "view",
		"--app", "1",
		"--subscription-id", "com.example.monthly",
	})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests := len(reg.Requests()); requests != 3 {
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerMonthlySubscriptionLookup(reg)
	reg.Register(asctest.GET("/v1/subscriptions/sub-1/promotedPurchase", "fields[inAppPurchases]=versions", "fields[subscriptions]=versions", "include=inAppPurchaseV2,subscription"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"promotedPurchases","id":"promo-1"}}`))

	stdout, stderr, runErr := runRootCommand(t, []string{
		"subscriptions", "promoted-purchases", "view",
		"--app", "1",
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
	if requests := len(reg.Requests()); requests != 3 || !strings.Contains(stdout, `"id":"promo-1"`) {
		t.Fatalf("requests=%d stdout=%q", requests, stdout)
	}
}

func TestIAPOfferCodesCreateStopsBeforeMutationWhenLookupFails(t *testing.T) {
	setupStableSelectorAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	for _, query := range []string{"filter[productId]=com.example.missing", "filter[name]=com.example.missing", ""} {
		reg.Register(asctest.GET("/v1/apps/1/inAppPurchasesV2", query, "limit=200"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	}

	_, _, runErr := runRootCommand(t, []string{
		"iap", "offer-codes", "create",
		"--app", "1",
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
	if requests := len(reg.Requests()); requests == 0 {
		t.Fatal("expected lookup requests before failure")
	}
}

func TestSubscriptionsOfferCodesCreateStopsBeforeMutationWhenLookupFails(t *testing.T) {
	setupStableSelectorAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	for _, query := range []string{"filter[productId]=com.example.missing", "filter[name]=com.example.missing", ""} {
		reg.Register(asctest.GET("/v1/apps/1/subscriptionGroups", "limit=200"),
			asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}]}`))
		reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/subscriptions", query, "limit=200"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	}

	_, _, runErr := runRootCommand(t, []string{
		"subscriptions", "offers", "offer-codes", "create",
		"--app", "1",
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
	if requests := len(reg.Requests()); requests == 0 {
		t.Fatal("expected lookup requests before failure")
	}
}

func TestSubscriptionsOfferCodesCreateFreeTrialResolvesStableSelector(t *testing.T) {
	setupStableSelectorAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerMonthlySubscriptionLookup(reg)
	reg.Register(asctest.POST("/v1/subscriptionOfferCodes").JSONBody(`{"data":{"type":"subscriptionOfferCodes","attributes":{"name":"SPRING","offerEligibility":"STACK_WITH_INTRO_OFFERS","customerEligibilities":["NEW"],"duration":"ONE_MONTH","offerMode":"FREE_TRIAL","numberOfPeriods":1},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"sub-1"}},"prices":{"data":[{"type":"subscriptionOfferCodePrices","id":"${local-price-1}"}]}}},"included":[{"type":"subscriptionOfferCodePrices","id":"${local-price-1}","relationships":{"territory":{"data":{"type":"territories","id":"DEU"}}}}]}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"subscriptionOfferCodes","id":"sub-offer-ft-2","attributes":{"name":"SPRING","active":true}}}`))

	stdout, stderr, runErr := runRootCommand(t, []string{
		"subscriptions", "offers", "offer-codes", "create",
		"--app", "1",
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

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerMonthlySubscriptionLookup(reg)
	reg.Register(asctest.GET("/v1/subscriptions/sub-1/relationships/winBackOffers"), asctest.JSON(http.StatusOK, `{"data":[]}`))

	stdout, stderr, runErr := runRootCommand(t, []string{
		"subscriptions", "offers", "win-back", "links",
		"--app", "1",
		"--subscription-id", "com.example.monthly",
	})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests := len(reg.Requests()); requests != 3 {
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
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(test.path, "cursor=abc"), asctest.JSON(http.StatusOK, `{"data":[]}`))

			stdout, stderr, runErr := runRootCommand(t, test.args)
			if runErr != nil {
				t.Fatalf("expected nil error, got %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if requests := len(reg.Requests()); requests != 1 {
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
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET(test.path), asctest.JSON(http.StatusOK, test.body))

			_, stderr, runErr := runRootCommand(t, test.args)
			if runErr != nil {
				t.Fatalf("expected nil error, got %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if requests := len(reg.Requests()); requests != 1 {
				t.Fatalf("expected exactly one request, got %d", requests)
			}
		})
	}
}

func TestStableSelectorMissingAppContextShowsUsageError(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "")

	asctest.NewRegistry(t).Install()

	stdout, stderr, err := runCommand(t, []string{"subscriptions", "review", "app-store-screenshot", "view", "--subscription-id", "com.example.monthly"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --app is required (or set ASC_APP_ID) when --subscription-id is a product ID or name") {
		t.Fatalf("expected missing app guidance, got %q", stderr)
	}
}

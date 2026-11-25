package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestSubscriptionsAvailabilityAvailableTerritoriesResolvesSubscriptionSelector(t *testing.T) {
	setupStableSelectorAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.Header.Get("Authorization") == "" {
			t.Fatal("expected authorization header")
		}
		switch req.URL.Path {
		case "/v1/apps/1/subscriptionGroups":
			return selectorJSONResponse(`{"data":[{"type":"subscriptionGroups","id":"group-1"}]}`), nil
		case "/v1/subscriptionGroups/group-1/subscriptions":
			if got := req.URL.Query().Get("filter[productId]"); got != "com.example.monthly" {
				t.Fatalf("expected product ID filter, got %q", got)
			}
			return selectorJSONResponse(`{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","productId":"com.example.monthly"}}]}`), nil
		case "/v1/subscriptions/sub-1/subscriptionAvailability":
			return selectorJSONResponse(`{"data":{"type":"subscriptionAvailabilities","id":"avail-1"}}`), nil
		case "/v1/subscriptionAvailabilities/avail-1/availableTerritories":
			if got := req.URL.Query().Get("limit"); got != "7" {
				t.Fatalf("expected limit=7, got %q", got)
			}
			return selectorJSONResponse(`{"data":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}]}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	stdout, stderr, runErr := runRootCommand(t, []string{
		"subscriptions", "pricing", "availability", "available-territories",
		"--app", "1",
		"--subscription-id", "com.example.monthly",
		"--limit", "7",
		"--output", "json",
	})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 4 {
		t.Fatalf("expected 4 requests, got %d", requests)
	}

	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if len(out.Data) != 1 || out.Data[0].ID != "USA" {
		t.Fatalf("expected USA territory, got %#v", out.Data)
	}
}

func TestSubscriptionsAvailabilityAvailableTerritoriesRejectsConfiguredBundleNameCollision(t *testing.T) {
	setupStableSelectorAuth(t)
	t.Setenv("ASC_APP_ID", "com.example.collision")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Path != "/v1/apps" {
			t.Fatalf("unexpected resource request after configured bundle/name collision: %s %s", req.Method, req.URL.String())
		}
		switch {
		case req.URL.Query().Get("filter[bundleId]") == "com.example.collision":
			return selectorJSONResponse(`{"data":[],"links":{"next":""}}`), nil
		case req.URL.Query().Get("filter[name]") == "com.example.collision":
			return selectorJSONResponse(`{"data":[{"type":"apps","id":"app-name-collision","attributes":{"bundleId":"com.other.app","name":"com.example.collision"}}],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected app lookup after configured bundle miss: %s", req.URL.String())
			return nil, nil
		}
	})

	stdout, _, runErr := runRootCommand(t, []string{
		"subscriptions", "pricing", "availability", "available-territories",
		"--subscription-id", "2024",
	})
	if runErr == nil {
		t.Fatal("expected configured bundle miss to reject before subscription lookup")
	}
	if stdout != "" {
		t.Fatalf("expected no territory output after configured bundle miss, got %q", stdout)
	}
	if requests != 1 {
		t.Fatalf("expected one exact bundle lookup and no subscription/resource access, got %d requests", requests)
	}
}

func TestSubscriptionsAvailabilityAvailableTerritoriesReportsAvailabilityLookupFailure(t *testing.T) {
	setupStableSelectorAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/8000000001/subscriptionAvailability" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusNotFound, `{"errors":[{"status":"404","title":"Not Found"}]}`), nil
	})

	_, _, runErr := runRootCommand(t, []string{
		"subscriptions", "pricing", "availability", "available-territories",
		"--subscription-id", "8000000001",
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "failed to resolve availability") {
		t.Fatalf("expected availability lookup failure, got %v", runErr)
	}
	if requests != 1 {
		t.Fatalf("expected one failed lookup request, got %d", requests)
	}
}

func TestSubscriptionsAvailabilityAvailableTerritoriesRejectsConflictingSelectors(t *testing.T) {
	_, _, runErr := runRootCommand(t, []string{
		"subscriptions", "pricing", "availability", "available-territories",
		"--availability-id", "avail-1",
		"--subscription-id", "sub-1",
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "--availability-id and --subscription-id are mutually exclusive") {
		t.Fatalf("expected selector conflict, got %v", runErr)
	}
}

func TestSubscriptionsAvailabilityAvailableTerritoriesRejectsNextWithSelectorsBeforeClient(t *testing.T) {
	setupStableSelectorAuth(t)
	next := "https://api.appstoreconnect.apple.com/v1/subscriptionAvailabilities/avail-1/availableTerritories?cursor=next"
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "availability", args: []string{"--availability-id", "avail-1"}, want: "--availability-id"},
		{name: "subscription", args: []string{"--subscription-id", "sub-1"}, want: "--subscription-id"},
		{name: "app", args: []string{"--app", "app-1"}, want: "--app"},
		{name: "empty availability", args: []string{"--availability-id", ""}, want: "--availability-id"},
		{name: "empty subscription", args: []string{"--subscription-id", ""}, want: "--subscription-id"},
		{name: "empty app", args: []string{"--app", ""}, want: "--app must not be empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				return nil, nil
			})
			args := append([]string{"subscriptions", "pricing", "availability", "available-territories", "--next", next}, test.args...)
			_, _, runErr := runRootCommand(t, args)
			wantError := "--next cannot be combined with " + test.want
			if test.name == "empty app" {
				wantError = test.want
			}
			if runErr == nil || !strings.Contains(runErr.Error(), wantError) {
				t.Fatalf("expected %s, got %v", wantError, runErr)
			}
			if requests != 0 {
				t.Fatalf("expected zero requests, got %d", requests)
			}
		})
	}
}

func TestSubscriptionsAvailabilityAvailableTerritoriesAllowsStandaloneNextCursor(t *testing.T) {
	setupStableSelectorAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Path != "/v1/subscriptionAvailabilities/avail-1/availableTerritories" || req.URL.Query().Get("cursor") != "next" {
			t.Fatalf("unexpected standalone cursor request: %s", req.URL.String())
		}
		return selectorJSONResponse(`{"data":[{"type":"territories","id":"USA"}],"links":{"next":""}}`), nil
	})
	stdout, stderr, runErr := runRootCommand(t, []string{"subscriptions", "pricing", "availability", "available-territories", "--next", "https://api.appstoreconnect.apple.com/v1/subscriptionAvailabilities/avail-1/availableTerritories?cursor=next", "--output", "json"})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 1 {
		t.Fatalf("expected one standalone cursor request, got %d", requests)
	}
	if !strings.Contains(stdout, `"id":"USA"`) {
		t.Fatalf("expected USA output, got %q", stdout)
	}
}

func TestSubscriptionsAvailabilityAvailableTerritoriesPaginatesSameAvailability(t *testing.T) {
	setupStableSelectorAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Path != "/v1/subscriptionAvailabilities/avail-1/availableTerritories" {
			t.Fatalf("unexpected request path: %s", req.URL.String())
		}
		switch req.URL.Query().Get("cursor") {
		case "":
			if got := req.URL.Query().Get("limit"); got != "200" {
				t.Fatalf("expected limit=200, got %q", got)
			}
			return selectorJSONResponse(`{"data":[{"type":"territories","id":"USA"}],"links":{"next":"/v1/subscriptionAvailabilities/avail-1/availableTerritories?cursor=next"}}`), nil
		case "next":
			return selectorJSONResponse(`{"data":[{"type":"territories","id":"FRA"}],"links":{"next":""}}`), nil
		default:
			t.Fatalf("unexpected cursor %q", req.URL.Query().Get("cursor"))
			return nil, nil
		}
	})
	stdout, stderr, runErr := runRootCommand(t, []string{"subscriptions", "pricing", "availability", "available-territories", "--availability-id", "avail-1", "--paginate", "--output", "json"})
	if runErr != nil {
		t.Fatalf("expected nil error, got %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests != 2 {
		t.Fatalf("expected two same-availability requests, got %d", requests)
	}
	if !strings.Contains(stdout, `"id":"USA"`) || !strings.Contains(stdout, `"id":"FRA"`) {
		t.Fatalf("expected both territory pages, got %q", stdout)
	}
}

func TestSubscriptionsAvailabilityAvailableTerritoriesRejectsDriftedContinuation(t *testing.T) {
	setupStableSelectorAuth(t)
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Path != "/v1/subscriptionAvailabilities/avail-1/availableTerritories" {
			t.Fatalf("unexpected follow-up request: %s", req.URL.String())
		}
		return selectorJSONResponse(`{"data":[{"type":"territories","id":"USA"}],"links":{"next":"/v1/subscriptionAvailabilities/avail-2/availableTerritories?cursor=drift"}}`), nil
	})
	_, _, runErr := runRootCommand(t, []string{"subscriptions", "pricing", "availability", "available-territories", "--availability-id", "avail-1", "--paginate"})
	if runErr == nil || !strings.Contains(runErr.Error(), "escaped scoped collection") {
		t.Fatalf("expected scoped continuation refusal, got %v", runErr)
	}
	if requests != 1 {
		t.Fatalf("expected no follow-up request after drift, got %d requests", requests)
	}
}

func TestSubscriptionsAvailabilityAvailableTerritoriesRequiresSubscriptionForApp(t *testing.T) {
	_, _, runErr := runRootCommand(t, []string{
		"subscriptions", "pricing", "availability", "available-territories",
		"--availability-id", "avail-1",
		"--app", "1",
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "--app requires --subscription-id") {
		t.Fatalf("expected app selector error, got %v", runErr)
	}
}

func TestSubscriptionsAvailabilityAvailableTerritoriesRequiresAppBeforeAuth(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_STRICT_AUTH", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()

	for _, selector := range []string{"com.example.monthly", "Monthly"} {
		_, _, runErr := runRootCommand(t, []string{
			"subscriptions", "pricing", "availability", "available-territories",
			"--subscription-id", selector,
		})
		if runErr == nil || !strings.Contains(runErr.Error(), "--app is required (or set ASC_APP_ID)") {
			t.Fatalf("selector %q: expected app context error before authentication, got %v", selector, runErr)
		}
	}
}

func TestSubscriptionsAvailabilityAvailableTerritoriesClassifiesPaginationValidationAsUsage(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "invalid limit",
			args: []string{
				"subscriptions", "pricing", "availability", "available-territories",
				"--availability-id", "avail-1",
				"--limit", "201",
			},
			wantErr: "subscriptions pricing availability available-territories: --limit must be between 1 and 200",
		},
		{
			name: "invalid next URL",
			args: []string{
				"subscriptions", "pricing", "availability", "available-territories",
				"--next", "https://example.com/v1/subscriptionAvailabilities/avail-1/availableTerritories?cursor=x",
			},
			wantErr: "subscriptions pricing availability available-territories: --next must be an App Store Connect URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runRootCommand(t, test.args)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", runErr)
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("expected usage exit code %d, got %d", rootcmd.ExitUsage, got)
			}
			if runErr.Error() != test.wantErr {
				t.Fatalf("expected error %q, got %q", test.wantErr, runErr.Error())
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			wantStderrPrefix := "Error: " + test.wantErr + "\nDESCRIPTION\n"
			if !strings.HasPrefix(stderr, wantStderrPrefix) {
				t.Fatalf("expected stderr prefix %q, got %q", wantStderrPrefix, stderr)
			}
		})
	}
}

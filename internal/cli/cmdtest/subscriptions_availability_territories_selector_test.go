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
		case "/v1/apps/app-1/subscriptionGroups":
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
		"--app", "app-1",
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

func TestSubscriptionsAvailabilityAvailableTerritoriesRequiresSubscriptionForApp(t *testing.T) {
	_, _, runErr := runRootCommand(t, []string{
		"subscriptions", "pricing", "availability", "available-territories",
		"--availability-id", "avail-1",
		"--app", "app-1",
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

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

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

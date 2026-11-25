package cmdtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestIAPPricingAvailabilityViewInclude(t *testing.T) {
	const path = "/v2/inAppPurchases/9000000001/inAppPurchaseAvailability"
	const availability = `"data":{"type":"inAppPurchaseAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}},"links":{}`

	for _, include := range []bool{false, true} {
		name := "default"
		if include {
			name = "availableTerritories"
		}
		t.Run(name, func(t *testing.T) {
			args := []string{"iap", "pricing", "availability", "view", "--iap-id", "9000000001", "--output", "json"}
			matcher := asctest.GET(path)
			body := "{" + availability + "}"
			if include {
				args = append(args, "--include", "availableTerritories")
				matcher = asctest.GET(path, "include=availableTerritories")
				body = "{" + availability + `,"included":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}]}`
			}
			reg := asctest.NewRegistry(t)
			reg.Register(matcher, asctest.JSON(http.StatusOK, body))

			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), args...)
			if code != cmd.ExitSuccess || stderr != "" {
				t.Fatalf("view failed: exit=%d stderr=%q", code, stderr)
			}
			var response struct {
				Data     asc.Resource[asc.InAppPurchaseAvailabilityAttributes] `json:"data"`
				Included []asc.Resource[asc.TerritoryAttributes]               `json:"included"`
			}
			if err := json.Unmarshal([]byte(stdout), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Data.ID != "avail-1" {
				t.Fatalf("expected availability data, got %s", stdout)
			}
			if include && (len(response.Included) != 1 || response.Included[0].ID != "USA" || response.Included[0].Attributes.Currency != "USD") {
				t.Fatalf("expected the included USA territory and currency, got %s", stdout)
			}
			if !include && response.Included != nil {
				t.Fatalf("expected no included territories by default, got %s", stdout)
			}
		})
	}
}

func TestIAPPricingAvailabilityViewInvalidInclude(t *testing.T) {
	assertUsageExitWithDeps(t, nil, []string{"iap", "pricing", "availability", "view", "--iap-id", "9000000001", "--include", "territories"},
		`--include must be a comma-separated list of: availableTerritories (unknown value "territories")`)
}

func TestIAPPricingAvailabilitySetAcceptsSpacedBoolValue(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/inAppPurchaseAvailabilities").JSONBody(`{"data":{"type":"inAppPurchaseAvailabilities","attributes":{"availableInNewTerritories":false},"relationships":{"inAppPurchase":{"data":{"type":"inAppPurchases","id":"9000000001"}},"availableTerritories":{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}]}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"inAppPurchaseAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`))

	stdout, stderr, err := runCommand(t, []string{"iap", "pricing", "availability", "set", "--iap-id", "9000000001", "--available-in-new-territories", "false", "--territories", "USA,CAN", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"avail-1"`) {
		t.Fatalf("expected availability response, got %q", stdout)
	}
}

func TestIAPScopedSelectorRefusesRedirectBeforeAvailabilityRead(t *testing.T) {
	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/v1/apps/1234567890/inAppPurchasesV2":
			http.Redirect(w, r, "/v1/apps/other/inAppPurchasesV2", http.StatusTemporaryRedirect)
		case "/v1/apps/other/inAppPurchasesV2":
			http.Error(w, "unexpected redirected product lookup", http.StatusInternalServerError)
		default:
			http.Error(w, "unexpected product lookup", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	stdout, _, code := runWithDeps(t, stubServerClient(t, server), "iap", "pricing", "availability", "view", "--app", "1234567890", "--iap-id", "com.example.pro", "--output", "json")
	if code != cmd.ExitError || stdout != "" {
		t.Fatalf("redirected scoped product lookup exit=%d stdout=%q, want error and no output", code, stdout)
	}
	mu.Lock()
	got := append([]string(nil), requests...)
	mu.Unlock()
	if len(got) != 1 || got[0] != "GET /v1/apps/1234567890/inAppPurchasesV2" {
		t.Fatalf("scoped product lookup requests=%v, want one refusal and no follow-up", got)
	}
}

package cmdtest

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// Defensive fixture: Apple's related-resource 404 shape applied to the schedule
// read itself. Not observed live as of 2026-09-15 (see the manualPrices capture
// below for the real sequence), but accepted so a future Apple change keeps the
// same contract.
const appPriceScheduleNotConfiguredBody = `{"errors":[{"id":"7f80654e-883f-4160-ba16-fad4ff9d055c","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'appPriceSchedules' with id 'app-1'"}]}`

// Captured live on 2026-09-15 from an app without a price schedule (ID
// substituted): GET /v1/apps/{id}/appPriceSchedule returned a synthetic
// schedule whose ID is the app ID, GET .../baseTerritory returned 200, and GET
// /v1/appPriceSchedules/{id}/manualPrices returned this 404.
const appPriceScheduleSyntheticBody = `{"data":{"type":"appPriceSchedules","id":"app-1","relationships":{"baseTerritory":{"links":{"related":"https://api.appstoreconnect.apple.com/v1/appPriceSchedules/app-1/baseTerritory"}},"manualPrices":{"links":{"related":"https://api.appstoreconnect.apple.com/v1/appPriceSchedules/app-1/manualPrices"}}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/appPriceSchedules/app-1"}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appPriceSchedule"}}`

const appPriceScheduleBaseTerritoryBody = `{"data":{"type":"territories","id":"USA","attributes":{"currency":"USD"},"links":{"self":"https://api.appstoreconnect.apple.com/v1/territories/USA"}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/appPriceSchedules/app-1/baseTerritory"}}`

const appPriceScheduleManualPricesNotConfiguredBody = `{"errors":[{"id":"0f2d2c4e-2b7a-4a4b-9a1c-5f6e7d8c9b0a","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'null' with id 'app-1'"}]}`

// newSyntheticScheduleServer replays the live sequence for an app whose price
// schedule was never created.
func newSyntheticScheduleServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appPriceSchedule":
			_, _ = w.Write([]byte(appPriceScheduleSyntheticBody))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPriceSchedules/app-1/baseTerritory":
			_, _ = w.Write([]byte(appPriceScheduleBaseTerritoryBody))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPriceSchedules/app-1/manualPrices":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(appPriceScheduleManualPricesNotConfiguredBody))
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestPricingCurrentNotConfiguredIsExpectedNegativeJSON(t *testing.T) {
	tests := []struct {
		name   string
		server func(t *testing.T) *httptest.Server
	}{
		{
			name: "schedule read 404",
			server: func(t *testing.T) *httptest.Server {
				return newNotFoundServer(t, "/v1/apps/app-1/appPriceSchedule", appPriceScheduleNotConfiguredBody)
			},
		},
		{name: "synthetic schedule with manual prices 404", server: newSyntheticScheduleServer},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			useServerTransport(t, test.server(t))

			stdout, stderr, runErr := runRootCommand(t, []string{"pricing", "current", "--app", "app-1", "--output", "json"})

			assertNotConfiguredExpectedNegative(t, runErr)
			wantStderr := `App app-1 has no price schedule configured yet; create it with: asc pricing schedule create --app app-1 --free --base-territory "USA"` + "\n"
			if stderr != wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
			}
			if stdout != `{"appId":"app-1","configured":false}`+"\n" {
				t.Fatalf("unexpected stdout: %q", stdout)
			}
		})
	}
}

func TestPricingCurrentNotConfiguredTableOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	useServerTransport(t, newNotFoundServer(t, "/v1/apps/app-1/appPriceSchedule", appPriceScheduleNotConfiguredBody))

	stdout, stderr, runErr := runRootCommand(t, []string{"pricing", "current", "--app", "app-1", "--output", "table"})

	assertNotConfiguredExpectedNegative(t, runErr)
	if !strings.Contains(stderr, "App app-1 has no price schedule configured yet") {
		t.Fatalf("expected not-configured hint on stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Configured") || !strings.Contains(stdout, "false") || !strings.Contains(stdout, "app-1") {
		t.Fatalf("expected table with configured=false row, got %q", stdout)
	}
}

func TestPricingCurrentUnknownAppStillReturnsNotFound(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	useServerTransport(t, newNotFoundServer(t, "/v1/apps/app-1/appPriceSchedule", appNotFoundBody))

	stdout, stderr, runErr := runRootCommand(t, []string{"pricing", "current", "--app", "app-1", "--output", "json"})

	if runErr == nil || !errors.Is(runErr, asc.ErrNotFound) {
		t.Fatalf("expected asc.ErrNotFound, got %v", runErr)
	}
	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitNotFound {
		t.Fatalf("expected exit code %d, got %d", cmd.ExitNotFound, got)
	}
	if shared.IsValidationError(runErr) {
		t.Fatalf("unknown app must not be classified as an expected negative: %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "pricing current: get app price schedule: The specified resource does not exist: There is no resource of type 'apps' with id 'app-1'") {
		t.Fatalf("expected Apple's not-found detail, got %v", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected empty streams, got stdout=%q stderr=%q", stdout, stderr)
	}
}

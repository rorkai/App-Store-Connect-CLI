package cmdtest

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// Captured live from GET /v1/apps/{id}/appAvailabilityV2 on 2026-09-13 with
// the app ID substituted.
const appAvailabilityNotConfiguredBody = `{"errors":[{"id":"b8a2b802-0512-4f42-b46a-cb444c0dc8db","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'appAvailabilities' with id 'app-1'"}]}`

// Captured live from GET /v1/apps/999999999999/appAvailabilityV2 on 2026-09-13
// with the app ID substituted.
const appNotFoundBody = `{"errors":[{"id":"1c5bfc66-b18d-46ce-9863-635985130e62","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'apps' with id 'app-1'"}]}`

// newNotFoundServer serves one 404 body for the expected GET path and fails
// the test on any other request.
func newNotFoundServer(t *testing.T, path, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != path {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

// serverRoundTripper rewrites every request to the httptest server so the
// default client exercises the real transport and error parsing.
func serverRoundTripper(t *testing.T, server *httptest.Server) roundTripFunc {
	t.Helper()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
}

func useServerTransport(t *testing.T, server *httptest.Server) {
	t.Helper()
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = serverRoundTripper(t, server)
}

// assertNotConfiguredExpectedNegative checks the shared expected-negative
// contract: exit code 1, validation stage, already reported, no not-found
// classification, and the state_not_ready diagnostic.
func assertNotConfiguredExpectedNegative(t *testing.T, runErr error) {
	t.Helper()
	if runErr == nil {
		t.Fatal("expected expected-negative error")
	}
	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitError {
		t.Fatalf("expected exit code %d, got %d (%v)", cmd.ExitError, got, runErr)
	}
	if !shared.IsValidationError(runErr) {
		t.Fatalf("expected validation-classified error, got %v", runErr)
	}
	var reported shared.ReportedError
	if !errors.As(runErr, &reported) {
		t.Fatalf("expected reported error, got %v", runErr)
	}
	if errors.Is(runErr, asc.ErrNotFound) {
		t.Fatalf("expected-negative must not carry not-found classification: %v", runErr)
	}
	var apiErr *asc.APIError
	if errors.As(runErr, &apiErr) {
		t.Fatalf("expected-negative must not expose the API status code: %v", runErr)
	}
	diagnostic, ok := shared.DiagnosticFromError(runErr)
	if !ok || diagnostic.Code != shared.DiagnosticStateNotReady {
		t.Fatalf("expected %q diagnostic, got %+v (ok=%v)", shared.DiagnosticStateNotReady, diagnostic, ok)
	}
}

func runPricingAvailabilityView(t *testing.T, server *httptest.Server, args ...string) (string, string, error) {
	t.Helper()
	return runRootWithDeps(t, stubClient(t, serverRoundTripper(t, server)), args...)
}

func TestPricingAvailabilityViewNotConfiguredIsExpectedNegative(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	server := newNotFoundServer(t, "/v1/apps/app-1/appAvailabilityV2", appAvailabilityNotConfiguredBody)

	stdout, stderr, runErr := runPricingAvailabilityView(t, server, "pricing", "availability", "view", "--app", "app-1", "--output", "json")

	assertNotConfiguredExpectedNegative(t, runErr)
	wantStderr := `App app-1 has no availability configured yet; create it with: asc pricing availability create --app app-1 --territory "USA" --available true --available-in-new-territories true` + "\n"
	if stderr != wantStderr {
		t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestPricingAvailabilityViewUnknownAppStillReturnsNotFound(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	server := newNotFoundServer(t, "/v1/apps/app-1/appAvailabilityV2", appNotFoundBody)

	stdout, stderr, runErr := runPricingAvailabilityView(t, server, "pricing", "availability", "view", "--app", "app-1", "--output", "json")

	if runErr == nil {
		t.Fatal("expected not-found error")
	}
	if !errors.Is(runErr, asc.ErrNotFound) {
		t.Fatalf("expected asc.ErrNotFound, got %v", runErr)
	}
	var apiErr *asc.APIError
	if !errors.As(runErr, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected wrapped 404 API error, got %v", runErr)
	}
	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitNotFound {
		t.Fatalf("expected exit code %d, got %d", cmd.ExitNotFound, got)
	}
	if shared.IsValidationError(runErr) {
		t.Fatalf("unknown app must not be classified as an expected negative: %v", runErr)
	}
	// Pre-existing contract for any other 404 on the --app lookup.
	if !strings.Contains(runErr.Error(), `pricing availability view: app availability not found for app "app-1"`) {
		t.Fatalf("expected the pre-existing not-found message, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestPricingAvailabilityViewByIDMissingRecordStillReturnsNotFound(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	body := `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'appAvailabilities' with id 'availability-1'"}]}`
	server := newNotFoundServer(t, "/v2/appAvailabilities/availability-1", body)

	stdout, stderr, runErr := runPricingAvailabilityView(t, server, "pricing", "availability", "view", "--id", "availability-1", "--output", "json")

	if runErr == nil || !errors.Is(runErr, asc.ErrNotFound) {
		t.Fatalf("expected asc.ErrNotFound for an unknown availability ID, got %v", runErr)
	}
	if got := cmd.ExitCodeFromError(runErr); got != cmd.ExitNotFound {
		t.Fatalf("expected exit code %d, got %d", cmd.ExitNotFound, got)
	}
	if shared.IsValidationError(runErr) {
		t.Fatalf("--id lookup must not be classified as an expected negative: %v", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected empty streams, got stdout=%q stderr=%q", stdout, stderr)
	}
}

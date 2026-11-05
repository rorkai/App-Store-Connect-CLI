package cmdtest

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// Captured live from GET /v2/inAppPurchases/{id}/inAppPurchaseAvailability on
// 2026-09-13 with the IAP ID substituted.
const iapAvailabilityNotConfiguredBody = `{"errors":[{"id":"3ee73c04-a4b6-48e1-b29f-a6242beee89e","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'inAppPurchaseAvailabilities' with id '9000000001'"}]}`

// Captured live from GET /v2/inAppPurchases/999999999999/inAppPurchaseAvailability
// on 2026-09-13 with the IAP ID substituted.
const iapNotFoundBody = `{"errors":[{"id":"6a1d8b64-3d4e-4a1c-9d0e-2f7c3b5a9e10","status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'inAppPurchases' with id '9000000001'"}]}`

func TestIAPPricingAvailabilityViewNotConfiguredIsExpectedNegative(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	useServerTransport(t, newNotFoundServer(t, "/v2/inAppPurchases/9000000001/inAppPurchaseAvailability", iapAvailabilityNotConfiguredBody))

	stdout, stderr, runErr := runRootCommand(t, []string{"iap", "pricing", "availability", "view", "--iap-id", "9000000001", "--output", "json"})

	assertNotConfiguredExpectedNegative(t, runErr)
	wantStderr := `In-app purchase 9000000001 has no availability configured yet; create it with: asc iap pricing availability set --iap-id 9000000001 --territories "USA"` + "\n"
	if stderr != wantStderr {
		t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestIAPPricingAvailabilityViewUnknownIAPStillReturnsNotFound(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	useServerTransport(t, newNotFoundServer(t, "/v2/inAppPurchases/9000000001/inAppPurchaseAvailability", iapNotFoundBody))

	stdout, stderr, runErr := runRootCommand(t, []string{"iap", "pricing", "availability", "view", "--iap-id", "9000000001", "--output", "json"})

	if runErr == nil || !errors.Is(runErr, asc.ErrNotFound) {
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
		t.Fatalf("unknown IAP must not be classified as an expected negative: %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "iap pricing availability view: failed to fetch: The specified resource does not exist: There is no resource of type 'inAppPurchases' with id '9000000001'") {
		t.Fatalf("expected Apple's not-found detail, got %v", runErr)
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected empty streams, got stdout=%q stderr=%q", stdout, stderr)
	}
}

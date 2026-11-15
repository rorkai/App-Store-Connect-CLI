package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type confirmRequiredCase struct {
	args    []string
	wantErr string
}

// runConfirmRequiredCases runs each invalid invocation and asserts exit code 2,
// empty stdout, the usage page and wantErr as the only "Error:" line. It reads
// both os.Stderr and the deps streams, so a row keeps working when its command
// moves onto shared.Deps. With wantConfirmDiagnostic it also asserts the
// required_input_missing diagnostic for --confirm.
func runConfirmRequiredCases(t *testing.T, cases []confirmRequiredCase, wantConfirmDiagnostic bool) {
	t.Helper()

	setupUsageExitCodeEnv(t)
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var depsStdout, depsStderr string
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				depsStdout, depsStderr, runErr = runRootWithDeps(t, nil, tc.args...)
			})
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
			}
			if stdout+depsStdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout+depsStdout)
			}
			allStderr := stderr + depsStderr
			assertUsageErrorStderr(t, allStderr, tc.wantErr)
			if !strings.Contains(allStderr, "\nUSAGE\n") {
				t.Fatalf("stderr = %q, want the usage page", allStderr)
			}
			if !wantConfirmDiagnostic {
				return
			}
			diagnostic, ok := shared.DiagnosticFromError(runErr)
			if !ok || diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "--confirm" {
				t.Fatalf("diagnostic = %+v (ok=%t), want %q for --confirm", diagnostic, ok, shared.DiagnosticRequiredInputMissing)
			}
		})
	}
}

// TestDestructiveCommandsAskForConfirmWithOneText locks the line a destructive
// command prints when every other required flag is given and --confirm is
// not: "--confirm is required", or "--confirm is required unless --dry-run is
// set" where the command has --dry-run. It lists the commands whose own tests
// do not already pin that line exactly.
func TestDestructiveCommandsAskForConfirmWithOneText(t *testing.T) {
	runConfirmRequiredCases(t, []confirmRequiredCase{
		{args: []string{"app-clips", "default-experiences", "localizations", "delete", "--localization-id", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"app-clips", "header-images", "delete", "--id", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"app-clips", "invocations", "localizations", "delete", "--localization-id", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"app-events", "localizations", "placements", "delete", "--id", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"app-events", "localizations", "placements", "swap", "--localization-id", "1", "--image-id", "1", "--placement-type", "EVENT_CARD_ASSET", "--placement-id", "1"}, wantErr: "--confirm is required"},
		{args: []string{"app-events", "submit", "--event-id", "123456789", "--app", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"asset-library", "images", "archive", "--id", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"asset-library", "images", "delete", "--id", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"asset-library", "videos", "archive", "--id", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"asset-library", "videos", "delete", "--id", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"localizations", "placements", "delete", "--id", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"localizations", "placements", "swap", "--localization-id", "1", "--image-id", "1", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET", "--placement-id", "1"}, wantErr: "--confirm is required"},
		{args: []string{"notarization", "staple", "--file", "missing.json"}, wantErr: "--confirm is required"},
		{args: []string{"product-pages", "custom-pages", "localizations", "placements", "delete", "--id", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"product-pages", "custom-pages", "localizations", "placements", "swap", "--localization-id", "1", "--image-id", "1", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET", "--placement-id", "1"}, wantErr: "--confirm is required"},
		{args: []string{"product-pages", "experiments", "treatments", "localizations", "placements", "delete", "--id", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"product-pages", "experiments", "treatments", "localizations", "placements", "swap", "--localization-id", "1", "--image-id", "1", "--placement-type", "PRODUCT_PAGE_HEADER_ASSET", "--placement-id", "1"}, wantErr: "--confirm is required"},
		{args: []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "sync", "--localization-id", "123456789", "--path", "missing.json", "--device-type", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"profiles", "local", "clean", "--expired"}, wantErr: "--confirm is required unless --dry-run is set"},
		{args: []string{"signing", "keychain", "delete", "--keychain", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"signing", "keychain", "install", "--identity", "123456789", "--identity-password-file", "missing.json", "--keychain", "123456789", "--keychain-password-file", "missing.json"}, wantErr: "--confirm is required"},
		{args: []string{"submit", "cancel", "--id", "123456789"}, wantErr: "--confirm is required"},
		{args: []string{"web", "service-ids", "domains", "set", "--service-id", "1", "--domain", "example.com", "--return-url", "https://example.com/x"}, wantErr: "--confirm is required"},
		{args: []string{"web", "subscriptions", "pricing", "monthly-commitment", "bootstrap", "--subscription-id", "1", "--territory", "FRA", "--upfront-price", "1", "--monthly-price", "1"}, wantErr: "--confirm is required unless --dry-run is set"},
	}, true)
}

// TestDestructiveCommandsCheckConfirmAfterRequiredFlags locks the order: a
// destructive command missing a required flag and --confirm reports the
// required flag first.
func TestDestructiveCommandsCheckConfirmAfterRequiredFlags(t *testing.T) {
	runConfirmRequiredCases(t, []confirmRequiredCase{
		{args: []string{"ads", "ad-groups", "delete"}, wantErr: "--ad-group is required"},
		{args: []string{"ads", "ads", "delete"}, wantErr: "--ad is required"},
		{args: []string{"ads", "assets", "delete"}, wantErr: "--asset is required"},
		{args: []string{"ads", "budget-orders", "delete"}, wantErr: "--budget-order is required"},
		{args: []string{"ads", "campaigns", "delete"}, wantErr: "--campaign is required"},
		{args: []string{"ads", "creatives", "delete"}, wantErr: "--creative is required"},
		{args: []string{"ads", "location-groups", "delete"}, wantErr: "--location-group is required"},
		{args: []string{"ads", "negative-keywords", "delete"}, wantErr: "--negative-keyword is required"},
		{args: []string{"ads", "targeting-keywords", "delete"}, wantErr: "--keyword is required"},
		{args: []string{"app-events", "submit"}, wantErr: "--event-id is required"},
		{args: []string{"apps", "search-keywords", "set", "--app", "123456789", "--version", "1.0", "--locale", "en-US"}, wantErr: "--keywords is required"},
		{args: []string{"bundle-ids", "capabilities", "reconcile", "apply"}, wantErr: "--bundle is required"},
		{args: []string{"iap", "promoted-purchases", "delete"}, wantErr: "--promoted-purchase-id is required"},
		{args: []string{"localizations", "search-keywords", "delete", "--localization-id", "123456789"}, wantErr: "--keywords is required"},
		{args: []string{"pricing", "availability", "remove-from-sale"}, wantErr: "--app is required (or set ASC_APP_ID)"},
		{args: []string{"product-pages", "custom-pages", "localizations", "preview-sets", "sync"}, wantErr: "--localization-id is required"},
		{args: []string{"product-pages", "custom-pages", "localizations", "screenshot-sets", "sync"}, wantErr: "--localization-id is required"},
		{args: []string{"product-pages", "custom-pages", "localizations", "search-keywords", "delete", "--localization-id", "123456789"}, wantErr: "--keywords is required"},
		{args: []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "sync"}, wantErr: "--localization-id is required"},
		{args: []string{"profiles", "local", "clean"}, wantErr: "at least one clean mode is required (e.g. --expired)"},
		{args: []string{"release", "stage"}, wantErr: "--app is required (or set ASC_APP_ID)"},
		{args: []string{"review", "items", "remove"}, wantErr: "--id is required"},
		{args: []string{"review", "items-remove"}, wantErr: "--id is required"},
		{args: []string{"review", "submissions-cancel"}, wantErr: "--id is required"},
		{args: []string{"review", "submissions-submit"}, wantErr: "--id is required"},
		{args: []string{"sandbox", "clear-history"}, wantErr: "--id or --email is required"},
		{args: []string{"screenshots", "apply"}, wantErr: "--app is required (or set ASC_APP_ID)"},
		{args: []string{"storekit", "auth", "logout"}, wantErr: "exactly one of --name or --all is required"},
		{args: []string{"storekit", "retention-messaging", "defaults", "delete"}, wantErr: "--product-id is required"},
		{args: []string{"storekit", "retention-messaging", "images", "delete"}, wantErr: "--image-id is required"},
		{args: []string{"storekit", "retention-messaging", "messages", "delete"}, wantErr: "--message-id is required"},
		{args: []string{"submit", "cancel"}, wantErr: "--id or --version-id is required"},
		{args: []string{"subscriptions", "promoted-purchases", "delete"}, wantErr: "--promoted-purchase-id is required"},
		{args: []string{"users", "delete"}, wantErr: "--id is required"},
		{args: []string{"users", "invites", "revoke"}, wantErr: "--id is required"},
		{args: []string{"webhooks", "delete"}, wantErr: "--webhook-id is required"},
	}, false)
}

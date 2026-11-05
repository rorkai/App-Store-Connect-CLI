package cmdtest

import (
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// TestIAPProductScopedCommandsAreUnknown locks the 5.0.0 removal of the App
// Store Connect API 4.4.1 product-scoped surfaces. The former `asc iap images`,
// `asc iap localizations`, and `asc iap submit` commands must behave like any
// other unknown command: a generic usage error, exit code 2, no stub text, and
// no deprecation warning. Their replacements live under `asc iap versions` and
// `asc review items add --item-type inAppPurchaseVersions`.
func TestIAPProductScopedCommandsAreUnknown(t *testing.T) {
	setupUsageExitCodeEnv(t)

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "images list",
			args:       []string{"iap", "images", "list", "--iap-id", "IAP_ID"},
			wantStderr: "Error: unknown command `asc iap images`\n",
		},
		{
			name:       "images create",
			args:       []string{"iap", "images", "create", "--iap-id", "IAP_ID", "--file", "./image.png"},
			wantStderr: "Error: unknown command `asc iap images`\n",
		},
		{
			name:       "localizations list",
			args:       []string{"iap", "localizations", "list", "--iap-id", "IAP_ID"},
			wantStderr: "Error: unknown command `asc iap localizations`\n",
		},
		{
			name:       "localizations update",
			args:       []string{"iap", "localizations", "update", "--localization-id", "LOC_ID", "--name", "Name"},
			wantStderr: "Error: unknown command `asc iap localizations`\n",
		},
		{
			name:       "submit",
			args:       []string{"iap", "submit", "--iap-id", "IAP_ID", "--confirm"},
			wantStderr: "Error: unknown command `asc iap submit`\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "1.2.3"); code != cmd.ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, cmd.ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.HasPrefix(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want prefix %q", stderr, test.wantStderr)
			}
			for _, forbidden := range []string{"DEPRECATED", "deprecated", "Warning:", "removed in"} {
				if strings.Contains(stderr, forbidden) {
					t.Fatalf("stderr must be a generic unknown-command diagnostic, got %q", stderr)
				}
			}
		})
	}
}

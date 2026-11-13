package cmdtest

import (
	"path/filepath"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// TestSubscriptionsInputValidationReturnsUsageExitCode locks the usage-error contract
// for subscriptions pre-request flag validation: every check must print
// "Error: <message>" to stderr, naming the command path the operator invoked,
// and exit with code 2 rather than the generic runtime failure code.
//
// Part of #518.
func TestSubscriptionsInputValidationReturnsUsageExitCode(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "groups list app 123 limit 201",
			args:    []string{"subscriptions", "groups", "list", "--app", "123", "--limit", "201"},
			wantErr: "subscriptions groups list: --limit must be between 1 and 200",
		},
		{
			name:    "groups list app 123 next non apple url",
			args:    []string{"subscriptions", "groups", "list", "--app", "123", "--next", "http://example.com/x"},
			wantErr: "subscriptions groups list: --next must be an App Store Connect URL",
		},
		{
			name:    "list group-id g limit 201",
			args:    []string{"subscriptions", "list", "--group-id", "g", "--limit", "201"},
			wantErr: "subscriptions list: --limit must be between 1 and 200",
		},
		{
			name:    "list group-id g next non apple url",
			args:    []string{"subscriptions", "list", "--group-id", "g", "--next", "http://example.com/x"},
			wantErr: "subscriptions list: --next must be an App Store Connect URL",
		},
		{
			name:    "pricing prices list subscription-id s limit 201",
			args:    []string{"subscriptions", "pricing", "prices", "list", "--subscription-id", "s", "--limit", "201"},
			wantErr: "subscriptions pricing prices list: --limit must be between 1 and 200",
		},
		{
			name:    "pricing prices list subscription-id s next non apple url",
			args:    []string{"subscriptions", "pricing", "prices", "list", "--subscription-id", "s", "--next", "http://example.com/x"},
			wantErr: "subscriptions pricing prices list: --next must be an App Store Connect URL",
		},
		{
			name:    "pricing price-points list subscription-id s limit 201",
			args:    []string{"subscriptions", "pricing", "price-points", "list", "--subscription-id", "s", "--limit", "201"},
			wantErr: "subscriptions pricing price-points list: --limit must be between 1 and 200",
		},
		{
			name:    "pricing price-points list subscription-id s next non apple url",
			args:    []string{"subscriptions", "pricing", "price-points", "list", "--subscription-id", "s", "--next", "http://example.com/x"},
			wantErr: "subscriptions pricing price-points list: --next must be an App Store Connect URL",
		},
		{
			name:    "offers introductory list subscription-id s limit 201",
			args:    []string{"subscriptions", "offers", "introductory", "list", "--subscription-id", "s", "--limit", "201"},
			wantErr: "subscriptions offers introductory list: --limit must be between 1 and 200",
		},
		{
			name:    "offers introductory list subscription-id s next non apple url",
			args:    []string{"subscriptions", "offers", "introductory", "list", "--subscription-id", "s", "--next", "http://example.com/x"},
			wantErr: "subscriptions offers introductory list: --next must be an App Store Connect URL",
		},
		{
			name:    "offers promotional list subscription-id s limit 201",
			args:    []string{"subscriptions", "offers", "promotional", "list", "--subscription-id", "s", "--limit", "201"},
			wantErr: "subscriptions offers promotional list: --limit must be between 1 and 200",
		},
		{
			name:    "offers promotional list subscription-id s next non apple url",
			args:    []string{"subscriptions", "offers", "promotional", "list", "--subscription-id", "s", "--next", "http://example.com/x"},
			wantErr: "subscriptions offers promotional list: --next must be an App Store Connect URL",
		},
		{
			name:    "offers promotional prices id o limit 201",
			args:    []string{"subscriptions", "offers", "promotional", "prices", "--id", "o", "--limit", "201"},
			wantErr: "subscriptions offers promotional prices: --limit must be between 1 and 200",
		},
		{
			name:    "offers promotional prices id o next non apple url",
			args:    []string{"subscriptions", "offers", "promotional", "prices", "--id", "o", "--next", "http://example.com/x"},
			wantErr: "subscriptions offers promotional prices: --next must be an App Store Connect URL",
		},
		{
			name:    "offers offer-codes list subscription-id s limit 201",
			args:    []string{"subscriptions", "offers", "offer-codes", "list", "--subscription-id", "s", "--limit", "201"},
			wantErr: "subscriptions offers offer-codes list: --limit must be between 1 and 200",
		},
		{
			name:    "offers offer-codes list subscription-id s next non apple url",
			args:    []string{"subscriptions", "offers", "offer-codes", "list", "--subscription-id", "s", "--next", "http://example.com/x"},
			wantErr: "subscriptions offers offer-codes list: --next must be an App Store Connect URL",
		},
		{
			name:    "offers offer-codes one-time-codes list offer-code-id o limit 201",
			args:    []string{"subscriptions", "offers", "offer-codes", "one-time-codes", "list", "--offer-code-id", "o", "--limit", "201"},
			wantErr: "subscriptions offers offer-codes one-time-codes list: --limit must be between 1 and 200",
		},
		{
			name:    "offers offer-codes one-time-codes list offer-code-id o next non apple url",
			args:    []string{"subscriptions", "offers", "offer-codes", "one-time-codes", "list", "--offer-code-id", "o", "--next", "http://example.com/x"},
			wantErr: "subscriptions offers offer-codes one-time-codes list: --next must be an App Store Connect URL",
		},
		{
			name:    "offers win-back list subscription-id s limit 201",
			args:    []string{"subscriptions", "offers", "win-back", "list", "--subscription-id", "s", "--limit", "201"},
			wantErr: "subscriptions offers win-back list: --limit must be between 1 and 200",
		},
		{
			name:    "offers win-back list subscription-id s prices-limit 51",
			args:    []string{"subscriptions", "offers", "win-back", "list", "--subscription-id", "s", "--prices-limit", "51"},
			wantErr: "subscriptions offers win-back list: --prices-limit must be between 1 and 50",
		},
		{
			name:    "offers win-back prices id o limit 201",
			args:    []string{"subscriptions", "offers", "win-back", "prices", "--id", "o", "--limit", "201"},
			wantErr: "subscriptions offers win-back prices: --limit must be between 1 and 200",
		},
		{
			name:    "offers win-back prices id o next non apple url",
			args:    []string{"subscriptions", "offers", "win-back", "prices", "--id", "o", "--next", "http://example.com/x"},
			wantErr: "subscriptions offers win-back prices: --next must be an App Store Connect URL",
		},
		{
			name:    "offers win-back links subscription-id s limit 201",
			args:    []string{"subscriptions", "offers", "win-back", "links", "--subscription-id", "s", "--limit", "201"},
			wantErr: "subscriptions offers win-back links: --limit must be between 1 and 200",
		},
		{
			name:    "offers win-back links subscription-id s next non apple url",
			args:    []string{"subscriptions", "offers", "win-back", "links", "--subscription-id", "s", "--next", "http://example.com/x"},
			wantErr: "subscriptions offers win-back links: --next must be an App Store Connect URL",
		},
		{
			name:    "offers offer-codes one-time-codes update missing batch id",
			args:    []string{"subscriptions", "offers", "offer-codes", "one-time-codes", "update", "--active", "false"},
			wantErr: "--batch-id is required",
		},
		{
			name:    "offers offer-codes one-time-codes update missing active",
			args:    []string{"subscriptions", "offers", "offer-codes", "one-time-codes", "update", "--batch-id", "b"},
			wantErr: "--active is required",
		},
		{
			name:    "offers offer-codes one-time-codes update invalid active",
			args:    []string{"subscriptions", "offers", "offer-codes", "one-time-codes", "update", "--batch-id", "b", "--active", "maybe"},
			wantErr: "subscriptions offers offer-codes one-time-codes update: --active must be true or false",
		},
		{
			name:    "offers offer-codes custom-codes update invalid active",
			args:    []string{"subscriptions", "offers", "offer-codes", "custom-codes", "update", "--custom-code-id", "c", "--active", "maybe"},
			wantErr: "subscriptions offers offer-codes custom-codes update: --active must be true or false",
		},
		{
			name:    "offers offer-codes prices offer-code-id o limit 201",
			args:    []string{"subscriptions", "offers", "offer-codes", "prices", "--offer-code-id", "o", "--limit", "201"},
			wantErr: "subscriptions offers offer-codes prices: --limit must be between 1 and 200",
		},
		{
			name:    "offers offer-codes prices offer-code-id o next non apple url",
			args:    []string{"subscriptions", "offers", "offer-codes", "prices", "--offer-code-id", "o", "--next", "http://example.com/x"},
			wantErr: "subscriptions offers offer-codes prices: --next must be an App Store Connect URL",
		},
		{
			name:    "pricing price-points view without price-point-id",
			args:    []string{"subscriptions", "pricing", "price-points", "view"},
			wantErr: "--price-point-id is required",
		},
		{
			name:    "offers promotional view without id",
			args:    []string{"subscriptions", "offers", "promotional", "view"},
			wantErr: "--id is required",
		},
		{
			name:    "versions images view without id",
			args:    []string{"subscriptions", "versions", "images", "view"},
			wantErr: "--id is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runCommand(t, test.args)

			if runErr == nil {
				t.Fatal("expected error, got nil")
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, rootcmd.ExitUsage, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			assertUsageErrorStderr(t, stderr, test.wantErr)
		})
	}
}

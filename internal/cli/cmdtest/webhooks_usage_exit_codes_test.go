package cmdtest

import (
	"path/filepath"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// TestWebhooksInputValidationReturnsUsageExitCode locks the usage-error contract
// for webhooks pre-request flag validation: every check must print
// "Error: <message>" to stderr, naming the command path the operator invoked,
// and exit with code 2 rather than the generic runtime failure code.
//
// Part of #518.
func TestWebhooksInputValidationReturnsUsageExitCode(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "list app 123 limit 201",
			args:    []string{"webhooks", "list", "--app", "123", "--limit", "201"},
			wantErr: "webhooks list: --limit must be between 1 and 200",
		},
		{
			name:    "list app 123 next non apple url",
			args:    []string{"webhooks", "list", "--app", "123", "--next", "http://example.com/x"},
			wantErr: "webhooks list: --next must be an App Store Connect URL",
		},
		{
			name:    "deliveries webhook-id w limit 201",
			args:    []string{"webhooks", "deliveries", "--webhook-id", "w", "--limit", "201"},
			wantErr: "webhooks deliveries: --limit must be between 1 and 200",
		},
		{
			name:    "deliveries webhook-id w next non apple url",
			args:    []string{"webhooks", "deliveries", "--webhook-id", "w", "--next", "http://example.com/x"},
			wantErr: "webhooks deliveries: --next must be an App Store Connect URL",
		},
		{
			name:    "deliveries links webhook-id w limit 201",
			args:    []string{"webhooks", "deliveries", "links", "--webhook-id", "w", "--limit", "201"},
			wantErr: "webhooks deliveries links: --limit must be between 1 and 200",
		},
		{
			name:    "deliveries links webhook-id w next non apple url",
			args:    []string{"webhooks", "deliveries", "links", "--webhook-id", "w", "--next", "http://example.com/x"},
			wantErr: "webhooks deliveries links: --next must be an App Store Connect URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, runErr := runRootWithDeps(t, nil, test.args...)

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

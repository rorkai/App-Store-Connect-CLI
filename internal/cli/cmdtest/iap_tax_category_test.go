package cmdtest

import (
	"path/filepath"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// TestWebIAPTaxCategoryInputValidationReturnsUsageExitCode locks the
// pre-session validation contract for each IAP tax-category leaf. Required
// inputs and positional arguments must produce one concise stderr diagnostic
// and usage exit code 2 before any web-session lookup can occur.
func TestWebIAPTaxCategoryInputValidationReturnsUsageExitCode(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "view requires iap",
			args:    []string{"web", "iap", "tax-category", "view"},
			wantErr: "--iap is required",
		},
		{
			name:    "set requires iap",
			args:    []string{"web", "iap", "tax-category", "set"},
			wantErr: "--iap is required",
		},
		{
			name:    "set requires category",
			args:    []string{"web", "iap", "tax-category", "set", "--iap", "IAP_ID"},
			wantErr: "--category is required",
		},
		{
			name:    "set requires confirmation",
			args:    []string{"web", "iap", "tax-category", "set", "--iap", "IAP_ID", "--category", "CATEGORY_ID"},
			wantErr: "--confirm is required",
		},
		{
			name:    "reset requires iap",
			args:    []string{"web", "iap", "tax-category", "reset"},
			wantErr: "--iap is required",
		},
		{
			name:    "reset requires confirmation",
			args:    []string{"web", "iap", "tax-category", "reset", "--iap", "IAP_ID"},
			wantErr: "--confirm is required",
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

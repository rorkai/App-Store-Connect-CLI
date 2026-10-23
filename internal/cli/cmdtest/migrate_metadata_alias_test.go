package cmdtest

import (
	"context"
	"io"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestMigrateMetadataAliasesWarnAndDelegateToMetadataCommand(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantDeprecate string
		wantErr       string
	}{
		{
			name:          "pull",
			args:          []string{"migrate", "metadata", "pull", "--version", "1.2.3", "--dir", "./metadata"},
			wantDeprecate: "Warning: `asc migrate metadata pull` is deprecated and will be removed in 6.0.0. Use `asc metadata pull`.\n",
			wantErr:       "Error: --app is required (or set ASC_APP_ID)\n",
		},
		{
			name:          "push",
			args:          []string{"migrate", "metadata", "push", "--version", "1.2.3", "--dir", "./metadata", "--dry-run"},
			wantDeprecate: "Warning: `asc migrate metadata push` is deprecated and will be removed in 6.0.0. Use `asc metadata push`.\n",
			wantErr:       "Error: --app is required (or set ASC_APP_ID)\n",
		},
		{
			name:          "validate",
			args:          []string{"migrate", "metadata", "validate"},
			wantDeprecate: "Warning: `asc migrate metadata validate` is deprecated and will be removed in 6.0.0. Use `asc metadata validate`.\n",
			wantErr:       "Error: --dir is required\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !shared.IsReportedUsageError(runErr) {
				t.Fatalf("expected reported usage error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if want := test.wantDeprecate + test.wantErr; stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
		})
	}
}

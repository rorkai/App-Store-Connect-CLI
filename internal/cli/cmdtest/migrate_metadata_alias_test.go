package cmdtest

import (
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestMigrateMetadataAliasesAreRemovedWithMigrationGuidance(t *testing.T) {
	removed := findSubcommand(RootCommand("6.0.0"), "migrate", "metadata")
	if removed == nil {
		t.Fatal("asc migrate metadata is not registered")
	}
	usage := removed.UsageFunc(removed) + "\n"

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "pull",
			args:    []string{"migrate", "metadata", "pull", "--app", "APP_ID", "--version", "1.2.3", "--dir", "./metadata"},
			wantErr: "Error: `asc migrate metadata pull` was removed in 6.0.0; use `asc metadata pull` instead\n",
		},
		{
			name:    "push",
			args:    []string{"migrate", "metadata", "push", "--app", "APP_ID", "--version", "1.2.3", "--dir", "./metadata", "--dry-run"},
			wantErr: "Error: `asc migrate metadata push` was removed in 6.0.0; use `asc metadata push` instead\n",
		},
		{
			name:    "validate",
			args:    []string{"migrate", "metadata", "validate", "--dir", "./metadata"},
			wantErr: "Error: `asc migrate metadata validate` was removed in 6.0.0; use `asc metadata validate` instead\n",
		},
		{
			name:    "group",
			args:    []string{"migrate", "metadata"},
			wantErr: "Error: `asc migrate metadata` was removed in 6.0.0; use `asc metadata` instead\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "6.0.0"); code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if want := test.wantErr + usage; stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
		})
	}
}

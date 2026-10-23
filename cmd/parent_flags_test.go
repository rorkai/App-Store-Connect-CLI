package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRejectsGroupFlagsBeforeSubcommand(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "reviews filter before list",
			args:    []string{"reviews", "--stars", "1", "list", "--app", "123"},
			wantErr: "Error: --stars must be passed after the subcommand name (asc reviews list [flags])",
		},
		{
			name:    "xcode-cloud sort before list",
			args:    []string{"xcode-cloud", "build-runs", "--sort", "-number", "list", "--workflow-id", "W1"},
			wantErr: "Error: --sort must be passed after the subcommand name (asc xcode-cloud build-runs list [flags])",
		},
		{
			name:    "validate flags before testflight",
			args:    []string{"validate", "--deep", "--strict", "testflight", "--app", "app-1", "--build-id", "build-1"},
			wantErr: "Error: --strict must be passed after the subcommand name (asc validate testflight [flags]); --deep is only valid for asc validate",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.2.3"); code != ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantErr)
			}
		})
	}
}

func TestRunKeepsRootFlagsBeforeSubcommand(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "root flag before group",
			args:    []string{"--strict-auth", "reviews", "list"},
			wantErr: "--app is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.2.3"); code != ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, ExitUsage)
				}
			})
			if strings.Contains(stderr, "subcommand name") || !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantErr)
			}
		})
	}
}

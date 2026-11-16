package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestSigningSyncRejectsRemovedPasswordFlagAsUnknown(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "push",
			args: []string{
				"signing", "sync", "push",
				"--bundle-id", "com.example.app",
				"--profile-type", "IOS_APP_STORE",
				"--repo", "git@example.com:team/signing.git",
				"--password", "legacy-password",
			},
		},
		{
			name: "pull",
			args: []string{
				"signing", "sync", "pull",
				"--repo", "git@example.com:team/signing.git",
				"--password", "legacy-password",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ASC_SIGNING_SYNC_PASSWORD", "")
			stdout, stderr, code := runWithDeps(t, nil, tt.args...)
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "`--password` was removed in 5.0.0") {
				t.Fatalf("stderr = %q, want removed-flag diagnostic", stderr)
			}
			if !strings.Contains(stderr, "--password-file") {
				t.Fatalf("stderr = %q, want a --password-file suggestion", stderr)
			}
			if strings.Contains(stderr, "ASC_MATCH_PASSWORD") || strings.Contains(stderr, "deprecated") {
				t.Fatalf("stderr mentions removed legacy sources: %q", stderr)
			}
		})
	}
}

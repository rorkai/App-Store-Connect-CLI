package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestSigningSyncLifecycleUsageErrorsExitTwoBeforeSideEffects(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "nuke without confirm",
			args: []string{
				"signing", "sync", "nuke",
				"--profile-type", "IOS_APP_DEVELOPMENT",
				"--repo", "git@example.com:team/signing.git",
			},
			want: "--confirm is required to delete profiles and revoke certificates (or pass --dry-run to preview)",
		},
		{
			name: "force for new devices with App Store profile",
			args: []string{
				"signing", "sync", "push",
				"--bundle-id", "com.example.app",
				"--profile-type", "IOS_APP_STORE",
				"--repo", "git@example.com:team/signing.git",
				"--force-for-new-devices",
			},
			want: "--force-for-new-devices requires a development or ad hoc profile type; IOS_APP_STORE profiles have no device list",
		},
		{
			name: "include mac without force",
			args: []string{
				"signing", "sync", "push",
				"--bundle-id", "com.example.app",
				"--profile-type", "IOS_APP_DEVELOPMENT",
				"--repo", "git@example.com:team/signing.git",
				"--include-mac-in-profiles",
			},
			want: "--include-mac-in-profiles requires --force-for-new-devices",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ASC_SIGNING_SYNC_PASSWORD", "repository-password")
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(tt.args, "test")
			})
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tt.want) {
				t.Fatalf("stderr = %q, want %q", stderr, tt.want)
			}
			if strings.Contains(stderr, "Cloning signing repo") || strings.Contains(stderr, "Listing signing assets") {
				t.Fatalf("stderr shows side effects: %q", stderr)
			}
		})
	}
}

func TestSigningSyncNukeRejectsConfirmWithDryRunBeforeRequests(t *testing.T) {
	setupSubmitCreateAuth(t)
	t.Setenv("ASC_SIGNING_SYNC_PASSWORD", "repository-password")

	reg := asctest.NewRegistry(t)
	reg.Install()

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"signing", "sync", "nuke",
			"--profile-type", "IOS_APP_DEVELOPMENT",
			"--repo", "git@example.com:team/signing.git",
			"--confirm",
			"--dry-run",
		}, "test")
	})
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitUsage, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	assertUsageDiagnosticFirstLine(t, stderr, "--confirm and --dry-run are mutually exclusive")
	if strings.Contains(stderr, "Cloning signing repo") || strings.Contains(stderr, "Listing signing assets") || strings.Contains(stderr, "Dry run:") {
		t.Fatalf("stderr shows side effects: %q", stderr)
	}
	if got := len(reg.Requests()); got != 0 {
		t.Fatalf("HTTP requests = %d, want 0", got)
	}
}

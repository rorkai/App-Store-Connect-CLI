package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestWebICloudContainersCreateRunRejectsUnprefixedIdentifier(t *testing.T) {
	setCmdtestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"web", "icloud-containers", "create",
			"--identifier", "com.example.app",
			"--name", "Example", "--confirm",
		}, "1.2.3")
	})
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if count := strings.Count(stderr, `Error: --identifier must start with "iCloud."`); count != 1 {
		t.Fatalf("prefix diagnostic count = %d, want 1; stderr = %q", count, stderr)
	}
}

func TestWebICloudContainersCreateUsageErrors(t *testing.T) {
	setCmdtestHome(t)

	runUsageExitCases(t, []usageExitCase{
		{
			name:    "requires confirm",
			args:    []string{"web", "icloud-containers", "create", "--identifier", "iCloud.com.example.app", "--name", "Example"},
			wantErr: "--confirm is required",
		},
		{
			name:    "rejects hidden",
			args:    []string{"web", "icloud-containers", "create", "--identifier", "iCloud.com.example.app", "--name", "Example", "--confirm", "--hidden"},
			wantErr: "unknown flag",
		},
		{
			name:    "rejects paginate",
			args:    []string{"web", "icloud-containers", "create", "--identifier", "iCloud.com.example.app", "--name", "Example", "--confirm", "--paginate"},
			wantErr: "unknown flag",
		},
		{
			name:    "rejects container id",
			args:    []string{"web", "icloud-containers", "create", "--identifier", "iCloud.com.example.app", "--name", "Example", "--confirm", "--container-id=cloud-1"},
			wantErr: "unknown flag",
		},
	})
}

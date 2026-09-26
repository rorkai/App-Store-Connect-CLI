package cmdtest

import (
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestWebICloudContainersCreateRunReportsRefusalOnce(t *testing.T) {
	setCmdtestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{
			"web", "icloud-containers", "create",
			"--identifier", "iCloud.com.example.app",
			"--name", "Example", "--confirm",
		}, "1.2.3")
	})
	if code != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitError)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if count := strings.Count(stderr, "no accepted write request has been captured"); count != 1 {
		t.Fatalf("refusal count = %d, want 1; stderr = %q", count, stderr)
	}
}

func TestWebICloudContainersCreateRunRejectsUnsupportedFlags(t *testing.T) {
	setCmdtestHome(t)
	t.Setenv("ASC_TELEMETRY_DISABLED", "1")
	for _, flag := range []string{
		"--apple-id=user@example.com", "--two-factor-code-command=unused",
		"--provider-id=123", "--public-provider-id=team", "--developer-team=team",
		"--output=invalid", "--pretty",
	} {
		t.Run(flag, func(t *testing.T) {
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run([]string{
					"web", "icloud-containers", "create",
					"--identifier", "iCloud.com.example.app",
					"--name", "Example", "--confirm", flag,
				}, "1.2.3")
			})
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" || !strings.Contains(stderr, "unknown flag") {
				t.Fatalf("stdout = %q, stderr = %q; want only unknown-flag diagnostic", stdout, stderr)
			}
		})
	}
}

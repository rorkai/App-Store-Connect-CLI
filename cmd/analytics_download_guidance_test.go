package cmd

import (
	"strings"
	"testing"
)

func TestRunAnalyticsDownloadDateGuidanceUsesInstanceID(t *testing.T) {
	resetReportFlags(t)

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"analytics", "download", "--date", "2024-01-01"}, "1.0.0"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := "Error: unknown flag `--date` for `asc analytics download`\n" +
		"Find the report instance for a date first:\n" +
		"  asc analytics view --request-id REQUEST_ID --processing-date YYYY-MM-DD --paginate\n" +
		"Then download it with:\n" +
		"  asc analytics download --request-id REQUEST_ID --instance-id INSTANCE_ID\n" +
		"For help:\n" +
		"  asc analytics download --help\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if strings.Contains(stderr, "--date is accepted") || strings.Contains(stderr, "--date=") {
		t.Fatalf("stderr teaches a --date alias: %q", stderr)
	}
}

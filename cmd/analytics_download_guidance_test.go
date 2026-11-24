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
	for _, want := range []string{
		"Error: unknown flag `--date` for `asc analytics download`",
		"asc analytics view --request-id REQUEST_ID --processing-date YYYY-MM-DD --paginate --output json",
		"asc analytics download --request-id REQUEST_ID --instance-id INSTANCE_ID",
		"asc analytics download --request-id REQUEST_ID --instance-id INSTANCE_ID --all-segments --output-dir OUTPUT_DIR",
		"asc analytics download --help",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want it to contain %q", stderr, want)
		}
	}
	if strings.Contains(stderr, "--date is accepted") || strings.Contains(stderr, "--date=") {
		t.Fatalf("stderr teaches a --date alias: %q", stderr)
	}
}

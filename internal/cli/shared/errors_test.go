package shared

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestUsageErrorPreservesValidationMessage(t *testing.T) {
	var usageErr error
	_, stderr := captureOutput(t, func() {
		usageErr = UsageError("--app is required")
	})

	if usageErr.Error() != "--app is required" {
		t.Fatalf("UsageError().Error() = %q, want %q", usageErr.Error(), "--app is required")
	}
	if !errors.Is(usageErr, flag.ErrHelp) {
		t.Fatalf("UsageError() should unwrap to flag.ErrHelp, got %v", usageErr)
	}
	if got := ClassifyUsageError(usageErr); got != UsageErrorMissingRequired {
		t.Fatalf("ClassifyUsageError() = %q, want %q", got, UsageErrorMissingRequired)
	}
	if !strings.Contains(stderr, "Error: --app is required") {
		t.Fatalf("UsageError() stderr = %q", stderr)
	}
}

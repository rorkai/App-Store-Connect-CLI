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

func TestNewReportedUsageErrorPreservesUsageClassificationWithoutHelp(t *testing.T) {
	err := NewReportedUsageError(UsageErrorInvalidValue, "invalid value for --territory")

	if err.Error() != "invalid value for --territory" {
		t.Fatalf("NewReportedUsageError().Error() = %q", err.Error())
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("reported usage error must not unwrap to flag.ErrHelp: %v", err)
	}
	if !IsReportedUsageError(err) {
		t.Fatalf("IsReportedUsageError() = false for %T", err)
	}
	var reported ReportedError
	if !errors.As(err, &reported) || !reported.Reported() {
		t.Fatalf("expected ReportedError marker, got %T", err)
	}
	if got := ClassifyUsageError(err); got != UsageErrorInvalidValue {
		t.Fatalf("ClassifyUsageError() = %q, want %q", got, UsageErrorInvalidValue)
	}
}

func TestNewReportedUsageErrorNormalizesUnknownKind(t *testing.T) {
	err := NewReportedUsageError(UsageErrorKind("unknown"), "--app is required")
	if got := ClassifyUsageError(err); got != UsageErrorMissingRequired {
		t.Fatalf("ClassifyUsageError() = %q, want %q", got, UsageErrorMissingRequired)
	}
}

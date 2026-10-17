package shared

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
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

func TestUsageErrorSanitizesTerminalControls(t *testing.T) {
	var usageErr error
	_, stderr := captureOutput(t, func() {
		usageErr = UsageError("bad\x1b[31m\r\ncommand")
	})

	const wantMessage = "bad[31m  command"
	if got := usageErr.Error(); got != wantMessage {
		t.Fatalf("UsageError().Error() = %q, want %q", got, wantMessage)
	}
	if !errors.Is(usageErr, flag.ErrHelp) {
		t.Fatalf("UsageError() should unwrap to flag.ErrHelp, got %v", usageErr)
	}
	if got, want := stderr, "Error: "+wantMessage+"\n"; got != want {
		t.Fatalf("UsageError() stderr = %q, want %q", got, want)
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

func TestWithDiagnosticPreservesExistingErrorContract(t *testing.T) {
	cause := errors.New("credential source")
	base := NewErrorWithCause(
		NewReportedUsageError(UsageErrorMissingRequired, "--name is required"),
		cause,
	)
	err := WithDiagnostic(base, DiagnosticRequiredInputMissing, "--name")

	if got, want := err.Error(), "--name is required"; got != want {
		t.Fatalf("WithDiagnostic().Error() = %q, want %q", got, want)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("WithDiagnostic() should preserve the cause chain: %v", err)
	}
	if !IsReportedUsageError(err) {
		t.Fatalf("WithDiagnostic() should preserve reported usage classification: %T", err)
	}
	if got := ClassifyUsageError(err); got != UsageErrorMissingRequired {
		t.Fatalf("ClassifyUsageError() = %q, want %q", got, UsageErrorMissingRequired)
	}

	diagnostic, ok := DiagnosticFromError(err)
	if !ok {
		t.Fatal("DiagnosticFromError() did not find structured metadata")
	}
	if diagnostic.Code != DiagnosticRequiredInputMissing || diagnostic.Parameter != "--name" {
		t.Fatalf("DiagnosticFromError() = %+v", diagnostic)
	}
}

func TestMissingRequiredUsageErrorCarriesStructuredDiagnostic(t *testing.T) {
	for _, tt := range []struct {
		name          string
		parameter     string
		wantMessage   string
		wantParameter string
	}{
		{name: "without parameter", wantMessage: flag.ErrHelp.Error()},
		{name: "with parameter", parameter: "--app", wantMessage: "--app", wantParameter: "--app"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := MissingRequiredUsageError(tt.parameter)
			if got := err.Error(); got != tt.wantMessage {
				t.Fatalf("MissingRequiredUsageError().Error() = %q, want %q", got, tt.wantMessage)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("MissingRequiredUsageError() should preserve flag.ErrHelp: %v", err)
			}
			if got := ClassifyUsageError(err); got != UsageErrorMissingRequired {
				t.Fatalf("ClassifyUsageError() = %q, want %q", got, UsageErrorMissingRequired)
			}
			diagnostic, ok := DiagnosticFromError(err)
			if !ok {
				t.Fatal("DiagnosticFromError() did not find required-input metadata")
			}
			if diagnostic.Code != DiagnosticRequiredInputMissing || diagnostic.Parameter != tt.wantParameter {
				t.Fatalf("DiagnosticFromError() = %+v", diagnostic)
			}
		})
	}
}

func TestInvalidValueUsageErrorCarriesStructuredDiagnostic(t *testing.T) {
	err := InvalidValueUsageError("--number-of-periods")
	if got := err.Error(); got != flag.ErrHelp.Error() {
		t.Fatalf("InvalidValueUsageError().Error() = %q, want %q", got, flag.ErrHelp.Error())
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("InvalidValueUsageError() should preserve flag.ErrHelp: %v", err)
	}
	if got := ClassifyUsageError(err); got != UsageErrorInvalidValue {
		t.Fatalf("ClassifyUsageError() = %q, want %q", got, UsageErrorInvalidValue)
	}
	diagnostic, ok := DiagnosticFromError(err)
	if !ok {
		t.Fatal("DiagnosticFromError() did not find invalid-input metadata")
	}
	if diagnostic.Code != DiagnosticInvalidInput || diagnostic.Parameter != "--number-of-periods" {
		t.Fatalf("DiagnosticFromError() = %+v", diagnostic)
	}
}

func TestDiagnosticFromErrorUsesOutermostAnnotation(t *testing.T) {
	err := WithDiagnostic(
		WithDiagnostic(errors.New("unchanged"), DiagnosticInvalidInput, "--issuer-id"),
		DiagnosticConflictingInput,
		"--key-type",
	)

	diagnostic, ok := DiagnosticFromError(err)
	if !ok {
		t.Fatal("DiagnosticFromError() did not find structured metadata")
	}
	if diagnostic.Code != DiagnosticConflictingInput || diagnostic.Parameter != "--key-type" {
		t.Fatalf("DiagnosticFromError() = %+v", diagnostic)
	}
}

func TestWithDiagnosticRejectsUnboundedCode(t *testing.T) {
	base := errors.New("unchanged")
	err := WithDiagnostic(base, DiagnosticCode("user-controlled-value"), "--name")

	if !errors.Is(err, base) {
		t.Fatalf("WithDiagnostic() should preserve an error with an unknown code: %T", err)
	}
	if _, ok := DiagnosticFromError(err); ok {
		t.Fatal("DiagnosticFromError() accepted an unknown code")
	}
}

func TestWithLocalFileDiagnosticClassifiesOnlyFilesystemAccessFailures(t *testing.T) {
	for _, tt := range []struct {
		name           string
		cause          error
		wantValidation bool
		wantCode       DiagnosticCode
	}{
		{name: "missing", cause: fs.ErrNotExist, wantValidation: true, wantCode: DiagnosticFileNotFound},
		{name: "unreadable", cause: fs.ErrPermission, wantValidation: true, wantCode: DiagnosticFilePermissionDenied},
		{name: "other", cause: errors.New("too many levels of symbolic links")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rendered := fmt.Errorf("command: open input: %w", tt.cause)
			err := WithLocalFileDiagnostic(rendered, "--path")

			if got, want := err.Error(), rendered.Error(); got != want {
				t.Fatalf("WithLocalFileDiagnostic().Error() = %q, want %q", got, want)
			}
			if !errors.Is(err, tt.cause) {
				t.Fatalf("WithLocalFileDiagnostic() should preserve the cause chain: %v", err)
			}
			if got := IsValidationError(err); got != tt.wantValidation {
				t.Fatalf("IsValidationError() = %t, want %t", got, tt.wantValidation)
			}
			diagnostic, ok := DiagnosticFromError(err)
			if tt.wantCode == "" {
				if ok {
					t.Fatalf("DiagnosticFromError() = %+v, want none", diagnostic)
				}
				return
			}
			if !ok || diagnostic.Code != tt.wantCode || diagnostic.Parameter != "--path" {
				t.Fatalf("DiagnosticFromError() = %+v, %t; want %q for --path", diagnostic, ok, tt.wantCode)
			}
		})
	}
	if err := WithLocalFileDiagnostic(nil, "--path"); err != nil {
		t.Fatalf("WithLocalFileDiagnostic(nil) = %v, want nil", err)
	}
}

func TestNewProcessExitErrorWithCausePreservesExitCodeAndCause(t *testing.T) {
	cause := errors.New("local child failure")
	err := NewProcessExitErrorWithCause(65, cause)

	if !errors.Is(err, cause) {
		t.Fatalf("NewProcessExitErrorWithCause() = %v, want preserved cause", err)
	}
	if code, ok := ProcessExitCode(err); !ok || code != 65 {
		t.Fatalf("ProcessExitCode() = %d/%v, want 65/true", code, ok)
	}
	if !IsLocalProcessFailure(err) {
		t.Fatalf("IsLocalProcessFailure() = false for %T", err)
	}
}

package shared

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

// ReportedError marks an error as already reported to the user.
// The main entrypoint should exit non-zero without duplicating output.
type ReportedError interface {
	error
	Reported() bool
}

// ReportedUsageError marks an already-printed usage failure without wrapping
// flag.ErrHelp. This lets commands provide concise corrective guidance while
// preserving usage exit and telemetry semantics without triggering full help.
type ReportedUsageError interface {
	ReportedError
	UsageErrorKind() UsageErrorKind
}

// ValidationFailure marks an error as a command/domain validation result rather
// than a generic runtime failure. It intentionally carries no command-specific
// value so telemetry can classify the stage without increasing cardinality.
type ValidationFailure interface {
	error
	ValidationFailure() bool
}

type reportedError struct {
	err error
}

type reportedUsageError struct {
	kind    UsageErrorKind
	message string
}

type validationError struct {
	err error
}

type errorWithCause struct {
	err   error
	cause error
}

type UsageErrorKind string

const (
	UsageErrorMissingRequired UsageErrorKind = "missing_required"
	UsageErrorInvalidValue    UsageErrorKind = "invalid_value"
	UsageErrorOther           UsageErrorKind = "other"
)

type classifiedUsageError struct {
	kind    UsageErrorKind
	message string
}

func (e classifiedUsageError) Error() string {
	if e.message == "" {
		return flag.ErrHelp.Error()
	}
	return e.message
}
func (e classifiedUsageError) Unwrap() error { return flag.ErrHelp }
func (e classifiedUsageError) UsageErrorKind() UsageErrorKind {
	return e.kind
}

func (e reportedUsageError) Error() string  { return e.message }
func (e reportedUsageError) Reported() bool { return true }
func (e reportedUsageError) UsageErrorKind() UsageErrorKind {
	return e.kind
}

func (e reportedError) Error() string {
	return e.err.Error()
}

func (e reportedError) Unwrap() error {
	return e.err
}

func (e reportedError) Reported() bool {
	return true
}

func (e validationError) Error() string {
	return e.err.Error()
}

func (e validationError) Unwrap() error {
	return e.err
}

func (e validationError) ValidationFailure() bool {
	return true
}

func (e errorWithCause) Error() string {
	return e.err.Error()
}

func (e errorWithCause) Unwrap() []error {
	return []error{e.err, e.cause}
}

// NewReportedError wraps an error that has already been printed.
func NewReportedError(err error) error {
	if err == nil {
		return nil
	}
	return reportedError{err: err}
}

// NewReportedUsageError classifies an already-printed usage failure without
// wrapping flag.ErrHelp, which would cause ffcli to print the command's full
// usage page. The returned error maps to usage exit code 2.
func NewReportedUsageError(kind UsageErrorKind, message string) error {
	trimmed := strings.TrimSpace(message)
	if kind != UsageErrorMissingRequired && kind != UsageErrorInvalidValue && kind != UsageErrorOther {
		kind = classifyUsageMessage(trimmed)
	}
	return reportedUsageError{kind: kind, message: trimmed}
}

// IsReportedUsageError reports whether err is an already-printed usage
// failure that must not trigger ffcli's full help renderer.
func IsReportedUsageError(err error) bool {
	var reportedUsage ReportedUsageError
	return errors.As(err, &reportedUsage)
}

// NewValidationError wraps an error that represents local/domain validation.
func NewValidationError(err error) error {
	if err == nil {
		return nil
	}
	return validationError{err: err}
}

// NewValidationReportedError wraps an already printed validation result.
func NewValidationReportedError(err error) error {
	if err == nil {
		return nil
	}
	return NewReportedError(NewValidationError(err))
}

// NewErrorWithCause preserves err's rendered message and classification while
// retaining an additional cause for errors.Is/errors.As and telemetry.
func NewErrorWithCause(err, cause error) error {
	if err == nil {
		return cause
	}
	if cause == nil {
		return err
	}
	return errorWithCause{err: err, cause: cause}
}

func IsValidationError(err error) bool {
	var validationErr ValidationFailure
	return errors.As(err, &validationErr) && validationErr.ValidationFailure()
}

// UsageError prints a CLI validation error and returns flag.ErrHelp so callers
// map the failure to usage exit code semantics.
func UsageError(message string) error {
	trimmed := strings.TrimSpace(message)
	if trimmed != "" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", trimmed)
	}
	return classifiedUsageError{kind: classifyUsageMessage(trimmed), message: trimmed}
}

// UsageErrorf formats and returns a usage-class validation error.
func UsageErrorf(format string, args ...any) error {
	return UsageError(fmt.Sprintf(format, args...))
}

// MissingRequiredUsageError classifies a required-input failure after the
// command has already written its diagnostic to stderr.
func MissingRequiredUsageError(parameters ...string) error {
	parameter := ""
	if len(parameters) > 0 {
		parameter = strings.TrimSpace(parameters[0])
	}
	return classifiedUsageError{kind: UsageErrorMissingRequired, message: parameter}
}

func ClassifyUsageError(err error) UsageErrorKind {
	var classified interface{ UsageErrorKind() UsageErrorKind }
	if errors.As(err, &classified) {
		return classified.UsageErrorKind()
	}
	return ""
}

func classifyUsageMessage(message string) UsageErrorKind {
	lower := strings.ToLower(message)
	if strings.Contains(lower, "required") {
		return UsageErrorMissingRequired
	}
	if strings.Contains(lower, "invalid") || strings.Contains(lower, "unsupported") || strings.Contains(lower, "must be") {
		return UsageErrorInvalidValue
	}
	return UsageErrorOther
}

package shared

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

// ReportedError marks an error whose result the command already reported,
// usually as structured output on stdout. The root still prints one stderr
// error line unless the error is also marked as printed to stderr.
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

// processExitCoder is deliberately private so ordinary errors with an
// ExitCode method, including os/exec.ExitError, cannot opt into raw root-process
// exit propagation by structural accident.
type processExitCoder interface {
	error
	ascProcessExitCode() int
	isASCProcessExit()
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

type processExitError struct {
	code  int
	cause error
}

func (e processExitError) Error() string {
	return fmt.Sprintf("child command exited with status %d", e.code)
}
func (e processExitError) Unwrap() error           { return e.cause }
func (e processExitError) Reported() bool          { return true }
func (e processExitError) ascProcessExitCode() int { return e.code }
func (e processExitError) isASCProcessExit()       {}
func (e processExitError) isLocalProcessFailure()  {}
func (e processExitError) printedToStderr()        {}

// NewProcessExitError preserves a child process exit code without duplicating
// the child's stderr through the root error renderer.
func NewProcessExitError(code int) error {
	return NewProcessExitErrorWithCause(code, nil)
}

// NewProcessExitErrorWithCause preserves a local child exit status while
// retaining the typed child failure for errors.Is/errors.As and telemetry.
func NewProcessExitErrorWithCause(code int, cause error) error {
	if code <= 0 || code > 255 {
		code = 1
	}
	return processExitError{code: code, cause: cause}
}

// ProcessExitCode reports an exact child-process exit code only for errors
// created by NewProcessExitError. The private marker prevents collisions with
// ordinary command execution errors elsewhere in asc.
func ProcessExitCode(err error) (int, bool) {
	var exitErr processExitCoder
	if !errors.As(err, &exitErr) {
		return 0, false
	}
	return exitErr.ascProcessExitCode(), true
}

// IsLocalProcessFailure reports whether err contains a process-exit marker
// created by this package. The private marker prevents arbitrary errors from
// being mistaken for local child failures.
func IsLocalProcessFailure(err error) bool {
	var marker interface{ isLocalProcessFailure() }
	return errors.As(err, &marker)
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

func (e reportedUsageError) Error() string    { return e.message }
func (e reportedUsageError) Reported() bool   { return true }
func (e reportedUsageError) printedToStderr() {}
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

type stderrReportedError struct {
	reportedError
}

func (e stderrReportedError) printedToStderr() {}

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

// NewReportedError wraps an error whose result the command already printed
// to stdout. The root prints err once to stderr as an "Error:" line.
func NewReportedError(err error) error {
	if err == nil {
		return nil
	}
	return reportedError{err: err}
}

// NewStderrReportedError wraps an error the command already printed to
// stderr, so the root prints nothing more for it.
func NewStderrReportedError(err error) error {
	if err == nil {
		return nil
	}
	return stderrReportedError{reportedError{err: err}}
}

// IsPrintedToStderr reports whether err carries a marker for a failure that
// was already written to stderr by the command or a child process.
func IsPrintedToStderr(err error) bool {
	var printed interface{ printedToStderr() }
	return errors.As(err, &printed)
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

// ErrPending marks a bounded wait that stopped before its target reached a
// terminal state, after the command reported the pending state itself. Commands
// return it only when the caller opted in to a pending outcome, and it maps to
// the dedicated pending exit code instead of a failure.
var ErrPending = errors.New("still pending")

type pendingError struct {
	message string
}

func (e pendingError) Error() string        { return e.message }
func (e pendingError) Is(target error) bool { return target == ErrPending }
func (e pendingError) Reported() bool       { return true }
func (e pendingError) printedToStderr()     {}
func (e pendingError) Diagnostic() Diagnostic {
	return Diagnostic{Code: DiagnosticStateNotReady}
}

// NewPendingError returns an already-reported ErrPending outcome. The root
// renderer prints nothing for it; the command must have written its pending
// result and any stderr notice before returning it.
func NewPendingError(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = ErrPending.Error()
	}
	return pendingError{message: message}
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

// NewNotConfiguredReportedError classifies an already-printed expected
// negative for a resource that has not been configured yet, such as an app
// without an availability record. It maps to the validation exit code and the
// state_not_ready diagnostic instead of a not-found API failure.
func NewNotConfiguredReportedError(err error) error {
	if err == nil {
		return nil
	}
	return WithDiagnostic(NewStderrReportedError(NewValidationError(err)), DiagnosticStateNotReady, "")
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

// HintedError carries a next step that the root error renderer prints on its
// own line after the message.
type HintedError struct {
	Err  error
	Hint string
}

func (e *HintedError) Error() string { return e.Err.Error() }

func (e *HintedError) Unwrap() error { return e.Err }

// KeepReadOnlyRefusal returns kept, or err when kept is nil and err is a
// read-only refusal. Batch commands that summarize per-item failures pass the
// result to NewErrorWithCause so the summary keeps the read-only exit code.
func KeepReadOnlyRefusal(kept, err error) error {
	if kept == nil && errors.Is(err, readonly.ErrRefused) {
		return err
	}
	return kept
}

func IsValidationError(err error) bool {
	var validationErr ValidationFailure
	return errors.As(err, &validationErr) && validationErr.ValidationFailure()
}

// UsageError prints a CLI validation error and returns flag.ErrHelp so callers
// map the failure to usage exit code semantics.
func UsageError(message string) error {
	trimmed := strings.TrimSpace(SanitizeTerminal(message))
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
	return WithDiagnostic(
		classifiedUsageError{kind: UsageErrorMissingRequired, message: parameter},
		DiagnosticRequiredInputMissing,
		parameter,
	)
}

// RequiredFlag reports a missing required flag: it prints
// "Error: <name> is required" to stderr and returns
// MissingRequiredUsageError(name).
func RequiredFlag(name string) error {
	fmt.Fprintln(os.Stderr, "Error: "+name+" is required")
	return MissingRequiredUsageError(name)
}

// InvalidValueUsageError classifies an invalid or conflicting input after the
// command has already written its diagnostic to stderr. It preserves the
// flag.ErrHelp contract so ffcli continues to render the command usage page.
func InvalidValueUsageError(parameters ...string) error {
	parameter := ""
	if len(parameters) > 0 {
		parameter = strings.TrimSpace(parameters[0])
	}
	return WithDiagnostic(
		classifiedUsageError{kind: UsageErrorInvalidValue},
		DiagnosticInvalidInput,
		parameter,
	)
}

// reportedUsageErrHelp preserves the flag.ErrHelp usage contract for a
// validation failure whose message has already been written to stderr, while
// forwarding any structured diagnostic the validator attached so telemetry
// keeps the failing parameter.
func reportedUsageErrHelp(err error) error {
	diagnostic, ok := DiagnosticFromError(err)
	if !ok {
		return flag.ErrHelp
	}
	return WithDiagnostic(flag.ErrHelp, diagnostic.Code, diagnostic.Parameter)
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

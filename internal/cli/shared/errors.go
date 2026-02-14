package shared

import (
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

type reportedError struct {
	err error
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

// NewReportedError wraps an error that has already been printed.
func NewReportedError(err error) error {
	if err == nil {
		return nil
	}
	return reportedError{err: err}
}

// UsageError prints a CLI validation error and returns flag.ErrHelp so callers
// map the failure to usage exit code semantics.
func UsageError(message string) error {
	trimmed := strings.TrimSpace(message)
	if trimmed != "" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", trimmed)
	}
	return flag.ErrHelp
}

// UsageErrorf formats and returns a usage-class validation error.
func UsageErrorf(format string, args ...any) error {
	return UsageError(fmt.Sprintf(format, args...))
}

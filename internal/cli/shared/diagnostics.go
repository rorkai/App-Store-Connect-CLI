package shared

import (
	"errors"
	"strings"
)

// DiagnosticCode is a privacy-safe, low-cardinality reason for a command
// failure. Codes describe the failure independently from the rendered error
// message; pair them with Parameter when the failure concerns a public flag.
type DiagnosticCode string

const (
	DiagnosticRequiredInputMissing    DiagnosticCode = "required_input_missing"
	DiagnosticInvalidInput            DiagnosticCode = "invalid_input"
	DiagnosticConflictingInput        DiagnosticCode = "conflicting_input"
	DiagnosticFileNotFound            DiagnosticCode = "file_not_found"
	DiagnosticFilePermissionDenied    DiagnosticCode = "file_permission_denied"
	DiagnosticFilePermissionsInsecure DiagnosticCode = "file_permissions_insecure"
	DiagnosticFileInvalidFormat       DiagnosticCode = "file_invalid_format"
	DiagnosticKeyAlgorithmUnsupported DiagnosticCode = "key_algorithm_unsupported"
	DiagnosticAuthenticationRejected  DiagnosticCode = "authentication_rejected"
	DiagnosticResourceNotFound        DiagnosticCode = "resource_not_found"
	DiagnosticResourceConflict        DiagnosticCode = "resource_conflict"
	DiagnosticStateNotReady           DiagnosticCode = "state_not_ready"
	DiagnosticDSYMNotReady            DiagnosticCode = "dsym_not_ready"
	DiagnosticDependencyFailed        DiagnosticCode = "dependency_failed"
	DiagnosticRequestFailed           DiagnosticCode = "request_failed"
	DiagnosticInternalError           DiagnosticCode = "internal_error"
)

// Diagnostic carries structured failure metadata without changing the error's
// user-facing message, classification, cause chain, or exit-code behavior.
type Diagnostic struct {
	Code      DiagnosticCode
	Parameter string
}

type diagnosticCarrier interface {
	Diagnostic() Diagnostic
}

type diagnosticError struct {
	err        error
	diagnostic Diagnostic
}

func (e diagnosticError) Error() string          { return e.err.Error() }
func (e diagnosticError) Unwrap() error          { return e.err }
func (e diagnosticError) Diagnostic() Diagnostic { return e.diagnostic }

// WithDiagnostic attaches allowlisted structured metadata to err. Unknown
// codes are deliberately ignored so arbitrary strings cannot become a
// telemetry dimension.
func WithDiagnostic(err error, code DiagnosticCode, parameter string) error {
	if err == nil {
		return nil
	}
	if !IsKnownDiagnosticCode(code) {
		return err
	}
	return diagnosticError{
		err: err,
		diagnostic: Diagnostic{
			Code:      code,
			Parameter: strings.TrimSpace(parameter),
		},
	}
}

// DiagnosticFromError returns the outermost allowlisted diagnostic annotation
// in err's chain.
func DiagnosticFromError(err error) (Diagnostic, bool) {
	var carrier diagnosticCarrier
	if !errors.As(err, &carrier) {
		return Diagnostic{}, false
	}
	diagnostic := carrier.Diagnostic()
	if !IsKnownDiagnosticCode(diagnostic.Code) {
		return Diagnostic{}, false
	}
	diagnostic.Parameter = strings.TrimSpace(diagnostic.Parameter)
	return diagnostic, true
}

// IsKnownDiagnosticCode reports whether code belongs to the bounded public
// diagnostic taxonomy.
func IsKnownDiagnosticCode(code DiagnosticCode) bool {
	switch code {
	case DiagnosticRequiredInputMissing,
		DiagnosticInvalidInput,
		DiagnosticConflictingInput,
		DiagnosticFileNotFound,
		DiagnosticFilePermissionDenied,
		DiagnosticFilePermissionsInsecure,
		DiagnosticFileInvalidFormat,
		DiagnosticKeyAlgorithmUnsupported,
		DiagnosticAuthenticationRejected,
		DiagnosticResourceNotFound,
		DiagnosticResourceConflict,
		DiagnosticStateNotReady,
		DiagnosticDSYMNotReady,
		DiagnosticDependencyFailed,
		DiagnosticRequestFailed,
		DiagnosticInternalError:
		return true
	default:
		return false
	}
}

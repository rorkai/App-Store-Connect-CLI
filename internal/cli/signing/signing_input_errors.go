package signing

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var (
	// errSigningInputInsecure matches a local signing input whose ownership,
	// link count, or mode is unsafe for secret material.
	errSigningInputInsecure = errors.New("signing input file is insecure")
	// errSigningInputUnsupported matches a local signing input that is not a
	// regular file or exceeds its size limit.
	errSigningInputUnsupported = errors.New("signing input file is unsupported")
)

// signingInputCheckError renders message while matching kind.
type signingInputCheckError struct {
	kind    error
	message string
}

func (e signingInputCheckError) Error() string        { return e.message }
func (e signingInputCheckError) Is(target error) bool { return target == e.kind }

func signingInputCheckErrorf(kind error, format string, args ...any) error {
	return signingInputCheckError{kind: kind, message: fmt.Sprintf(format, args...)}
}

// signingInputError records the public flag and diagnostic for a local
// signing input that could not be read or decoded. Its message is unchanged.
// Only command entry points turn it into a classified failure through
// classifySigningInputError, so library callers that resolve paths from a
// configuration file never report a flag they did not receive.
type signingInputError struct {
	parameter string
	code      shared.DiagnosticCode
	err       error
}

func (e *signingInputError) Error() string { return e.err.Error() }
func (e *signingInputError) Unwrap() error { return e.err }

// signingInputReadError annotates rendered, a failure to read the file named
// by parameter, when the file itself caused the failure. Other failures are
// returned unchanged.
func signingInputReadError(parameter string, rendered error) error {
	var code shared.DiagnosticCode
	switch {
	case errors.Is(rendered, fs.ErrNotExist):
		code = shared.DiagnosticFileNotFound
	case errors.Is(rendered, fs.ErrPermission):
		code = shared.DiagnosticFilePermissionDenied
	case errors.Is(rendered, errSigningInputInsecure):
		code = shared.DiagnosticFilePermissionsInsecure
	case errors.Is(rendered, errSigningInputUnsupported):
		code = shared.DiagnosticFileInvalidFormat
	default:
		return rendered
	}
	return &signingInputError{parameter: parameter, code: code, err: rendered}
}

// signingInputDecodeError annotates rendered, a failure to decode the contents
// of the file named by parameter.
func signingInputDecodeError(parameter string, rendered error) error {
	return &signingInputError{parameter: parameter, code: shared.DiagnosticFileInvalidFormat, err: rendered}
}

// signingIdentityPasswordError annotates rendered, a PKCS#12 identity that
// the supplied password does not open.
func signingIdentityPasswordError(rendered error) error {
	return &signingInputError{parameter: "--identity-password-file", code: shared.DiagnosticInvalidInput, err: rendered}
}

// classifySigningInputError marks a local signing input failure as an
// expected-negative validation failure for its flag without changing the
// message or exit code. Other errors are returned unchanged.
func classifySigningInputError(err error) error {
	var inputErr *signingInputError
	if !errors.As(err, &inputErr) {
		return err
	}
	return shared.WithDiagnostic(shared.NewValidationError(err), inputErr.code, inputErr.parameter)
}

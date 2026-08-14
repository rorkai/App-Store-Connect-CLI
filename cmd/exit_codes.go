package cmd

import (
	"errors"
	"flag"
	"net/http"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// Exit codes following the CI/CD specification.
const (
	ExitSuccess  = 0 // Successful execution
	ExitError    = 1 // Generic/unclassified error
	ExitUsage    = 2 // Invalid usage / flags / command invocation
	ExitAuth     = 3 // Authentication failure (missing, unauthorized, forbidden)
	ExitNotFound = 4 // Resource not found
	ExitConflict = 5 // Conflict / resource already exists

	// HTTP 4xx range: 10 + (status - 400)
	// Note: 404 and 409 are mapped to ExitNotFound and ExitConflict above.
	ExitHTTPBadRequest    = 10       // 400
	ExitHTTPUnauthorized  = ExitAuth // 401 (special case)
	ExitHTTPForbidden     = ExitAuth // 403 (special case)
	ExitHTTPUnprocessable = 32       // 422

	// HTTP 5xx range: 60 + (status - 500)
	ExitHTTPInternalServer     = 60 // 500
	ExitHTTPBadGateway         = 62 // 502
	ExitHTTPServiceUnavailable = 63 // 503
)

// ExitCodeFromError maps an error to the appropriate exit code.
// This is the single source of truth for exit code determination.
func ExitCodeFromError(err error) int {
	if err == nil {
		return ExitSuccess
	}

	// Usage errors
	if errors.Is(err, flag.ErrHelp) || shared.IsReportedUsageError(err) {
		return ExitUsage
	}

	// Well-known error types
	if errors.Is(err, shared.ErrMissingAuth) ||
		errors.Is(err, asc.ErrUnauthorized) ||
		errors.Is(err, asc.ErrForbidden) ||
		errors.Is(err, webcore.ErrInvalidAppleAccountCredentials) {
		return ExitAuth
	}
	if errors.Is(err, asc.ErrNotFound) {
		return ExitNotFound
	}
	if errors.Is(err, asc.ErrConflict) {
		return ExitConflict
	}

	// Check for APIError with status code or known code
	if apiErr, ok := errors.AsType[*asc.APIError](err); ok {
		// Prefer HTTP status code if available
		if apiErr.StatusCode > 0 {
			return HTTPStatusToExitCode(apiErr.StatusCode)
		}
		// Fall back to API error code mapping
		return APIErrorCodeToExitCode(apiErr.Code)
	}
	if webErr, ok := errors.AsType[*webcore.APIError](err); ok {
		return HTTPStatusToExitCode(webErr.HTTPStatusCode())
	}

	// Generic error
	return ExitError
}

// APIErrorCodeToExitCode maps an API error code string to the appropriate exit code.
func APIErrorCodeToExitCode(code string) int {
	switch code {
	case "NOT_FOUND":
		return ExitNotFound
	case "CONFLICT":
		return ExitConflict
	case "UNAUTHORIZED", "FORBIDDEN":
		return ExitAuth
	case "BAD_REQUEST":
		return ExitHTTPBadRequest
	default:
		return ExitError
	}
}

// HTTPStatusToExitCode maps an HTTP status code to the appropriate exit code.
func HTTPStatusToExitCode(status int) int {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return ExitAuth
	case status == http.StatusNotFound:
		return ExitNotFound
	case status == http.StatusConflict:
		return ExitConflict
	case status >= 400 && status < 500:
		// 4xx: 10 + (status - 400), clamped to 10-59
		code := min(10+(status-400), 59)
		return code
	case status >= 500 && status < 600:
		// 5xx: 60 + (status - 500), clamped to 60-99
		code := min(60+(status-500), 99)
		return code
	default:
		return ExitError
	}
}

package cmd

import (
	"errors"
	"flag"
	"net/http"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
	ExitHTTPBadRequest    = 10 // 400
	ExitHTTPUnauthorized  = 11 // 401
	ExitHTTPForbidden     = 12 // 403
	ExitHTTPNotFound      = 14 // 404
	ExitHTTPConflict      = 19 // 409
	ExitHTTPUnprocessable = 22 // 422

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
	if errors.Is(err, flag.ErrHelp) {
		return ExitUsage
	}

	// Authentication errors
	if errors.Is(err, shared.ErrMissingAuth) {
		return ExitAuth
	}
	if errors.Is(err, asc.ErrUnauthorized) {
		return ExitAuth
	}
	if errors.Is(err, asc.ErrForbidden) {
		return ExitAuth
	}

	// Not found errors
	if errors.Is(err, asc.ErrNotFound) {
		return ExitNotFound
	}

	// Check for APIError with specific error codes
	var apiErr *asc.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case "NOT_FOUND":
			return ExitNotFound
		case "CONFLICT":
			return ExitConflict
		case "UNAUTHORIZED":
			return ExitAuth
		case "FORBIDDEN":
			return ExitAuth
		case "BAD_REQUEST":
			return ExitHTTPBadRequest
		}
	}

	// Check for HTTP status code in APIError
	if apiErr != nil && apiErr.StatusCode > 0 {
		return HTTPStatusToExitCode(apiErr.StatusCode)
	}

	// Generic error
	return ExitError
}

// HTTPStatusToExitCode maps an HTTP status code to the appropriate exit code.
func HTTPStatusToExitCode(status int) int {
	switch {
	case status == http.StatusNotFound:
		return ExitNotFound
	case status == http.StatusConflict:
		return ExitConflict
	case status >= 400 && status < 500:
		// 4xx: 10 + (status - 400)
		return 10 + (status - 400)
	case status >= 500 && status < 600:
		// 5xx: 60 + (status - 500)
		return 60 + (status - 500)
	default:
		return ExitError
	}
}

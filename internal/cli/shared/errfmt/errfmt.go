package errfmt

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type ClassifiedError struct {
	Message string
	Hint    string
}

func Classify(err error) ClassifiedError {
	if err == nil {
		return ClassifiedError{}
	}

	if errors.Is(err, shared.ErrMissingAuth) {
		return ClassifiedError{
			Message: err.Error(),
			Hint:    "Run `asc auth login` or `asc auth init` (or set ASC_KEY_ID/ASC_ISSUER_ID/ASC_PRIVATE_KEY_PATH). Try `asc auth doctor` if you're unsure what's misconfigured.",
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return ClassifiedError{
			Message: err.Error(),
			Hint:    "Increase the request timeout (e.g. set `ASC_TIMEOUT=90s`).",
		}
	}

	if containsPrivacyError(err) {
		return ClassifiedError{
			Message: err.Error(),
			Hint:    "App privacy declarations (data usages) must be configured in the App Store Connect web UI — the API does not support this. Visit https://appstoreconnect.apple.com and complete the App Privacy section before submitting.",
		}
	}

	if errors.Is(err, asc.ErrForbidden) {
		return ClassifiedError{
			Message: err.Error(),
			Hint:    "Check that your API key has the right role/permissions for this operation in App Store Connect.",
		}
	}

	if errors.Is(err, asc.ErrUnauthorized) {
		return ClassifiedError{
			Message: err.Error(),
			Hint:    "Your credentials may be invalid or expired. Try `asc auth status` and re-login if needed.",
		}
	}

	return ClassifiedError{
		Message: err.Error(),
		Hint:    "",
	}
}

// containsPrivacyError checks whether the error references app data usage /
// privacy declaration resources that are not manageable via the API.
func containsPrivacyError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "appdatausages") || strings.Contains(msg, "appdatausagespublications")
}

func FormatStderr(err error) string {
	ce := Classify(err)
	if ce.Message == "" {
		return ""
	}
	if ce.Hint == "" {
		return fmt.Sprintf("Error: %s\n", ce.Message)
	}
	return fmt.Sprintf("Error: %s\nHint: %s\n", ce.Message, ce.Hint)
}

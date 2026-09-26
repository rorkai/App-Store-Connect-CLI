package asc

import (
	"errors"
	"strings"
)

type submittedSecretRedactedError struct {
	message      string
	cause        error
	safeAPIError *APIError
}

func (e *submittedSecretRedactedError) Error() string {
	return e.message
}

func (e *submittedSecretRedactedError) Unwrap() error {
	return e.cause
}

func (e *submittedSecretRedactedError) As(target any) bool {
	apiErrorTarget, ok := target.(**APIError)
	if !ok || e.safeAPIError == nil {
		return false
	}
	*apiErrorTarget = e.safeAPIError
	return true
}

// redactSubmittedSecretFromError is the credential-mutation error boundary.
// App Store Connect may echo an invalid submitted password in its structured
// error title, detail, or associated errors. Preserve error classification
// through safe cloned causes while exposing only a redacted message.
func redactSubmittedSecretFromError(err error, secret *string) error {
	if err == nil || secret == nil || *secret == "" {
		return err
	}

	values := []string{*secret, SanitizeTerminalText(*secret)}
	redact := func(value string) string {
		for _, secretValue := range values {
			if secretValue != "" {
				value = strings.ReplaceAll(value, secretValue, RedactedValuePlaceholder)
			}
		}
		return value
	}

	message := redact(err.Error())

	var safeAPIError *APIError
	var apiError *APIError
	if errors.As(err, &apiError) {
		safe := *apiError
		// Code is a protocol enum used by APIError.Is. Preserve it verbatim so a
		// short submitted password cannot accidentally erase classification.
		safe.Title = redact(safe.Title)
		safe.Detail = redact(safe.Detail)
		if apiError.AllDetails != nil {
			safe.AllDetails = make([]string, len(apiError.AllDetails))
			for index, detail := range apiError.AllDetails {
				safe.AllDetails[index] = redact(detail)
			}
		}
		if apiError.AssociatedErrors != nil {
			safe.AssociatedErrors = make(map[string][]APIAssociatedError, len(apiError.AssociatedErrors))
			for resource, entries := range apiError.AssociatedErrors {
				safeEntries := make([]APIAssociatedError, len(entries))
				for index, entry := range entries {
					entry.Code = redact(entry.Code)
					entry.Detail = redact(entry.Detail)
					safeEntries[index] = entry
				}
				safe.AssociatedErrors[redact(resource)] = safeEntries
			}
		}
		safeAPIError = &safe
	}

	safeCause := err
	if safeAPIError != nil {
		safeCause = safeAPIError
		var retryable *RetryableError
		if errors.As(err, &retryable) {
			safeRetryable := *retryable
			safeRetryable.Err = &submittedSecretRedactedError{
				message:      redact(retryable.Err.Error()),
				cause:        safeAPIError,
				safeAPIError: safeAPIError,
			}
			safeCause = &safeRetryable
		}
	}
	return &submittedSecretRedactedError{
		message:      message,
		cause:        safeCause,
		safeAPIError: safeAPIError,
	}
}

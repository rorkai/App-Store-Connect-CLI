package asc

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

var (
	ErrNotFound              = errors.New("resource not found")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrForbidden             = errors.New("forbidden")
	ErrBadRequest            = errors.New("bad request")
	ErrConflict              = errors.New("resource conflict")
	ErrMissingKeyID          = errors.New("key ID is required")
	ErrRepeatedPaginationURL = errors.New("detected repeated pagination URL")
)

var (
	ErrMissingPaginationFetcher = errors.New("pagination fetch function is required")
	ErrNilPaginationPage        = errors.New("pagination page is nil")
)

type responseBodyReadError struct {
	err error
}

func (e *responseBodyReadError) Error() string {
	return fmt.Sprintf("failed to read response body: %v", e.err)
}

func (e *responseBodyReadError) Unwrap() error {
	return e.err
}

func isResponseBodyReadError(err error) bool {
	var readErr *responseBodyReadError
	return errors.As(err, &readErr)
}

type buildUploadFileCommitResponseError struct {
	err error
}

func (e *buildUploadFileCommitResponseError) Error() string {
	return e.err.Error()
}

func (e *buildUploadFileCommitResponseError) Unwrap() error {
	return e.err
}

func newBuildUploadFileCommitResponseError(err error) error {
	return &buildUploadFileCommitResponseError{err: err}
}

// IsBuildUploadFileCommitResponseError reports whether a successful
// build-upload-file commit response could not be read or decoded.
func IsBuildUploadFileCommitResponseError(err error) bool {
	var responseErr *buildUploadFileCommitResponseError
	return errors.As(err, &responseErr)
}

// APIError represents a parsed App Store Connect error response.
type APIError struct {
	Code             string
	Title            string
	Detail           string
	StatusCode       int // HTTP status code that triggered this error (0 if unknown)
	AssociatedErrors map[string][]APIAssociatedError
	// AllCodes lists the code of every entry in Apple's errors[] array, in
	// response order, with Code as the first element. Apple can report several
	// causes for one status: a duplicate versionString on
	// POST /v1/appStoreVersions arrives as ENTITY_ERROR.RELATIONSHIP.INVALID
	// followed by ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE. Callers that
	// classify a response by code must consult this slice so a cause reported
	// after the first is not lost. It is empty when the body carried no
	// parsable errors[] array.
	AllCodes []string
	// AllDetails lists every detail from Apple's errors[] array in response
	// order, including empty details. Callers that classify a response by its
	// diagnostic text must consult this slice so a later cause is not lost.
	AllDetails []string
	// Remediation is operator guidance for error codes whose cause is an
	// account-level state that no API key permission can satisfy. It is
	// appended to Error() so the guidance travels with the error itself.
	Remediation string
}

// requiredAgreementRemediation explains a 403 that no key permission can fix.
// An unaccepted or expired agreement blocks the whole team, and only the
// Account Holder can clear it.
const requiredAgreementRemediation = "Your team has an unaccepted or expired agreement, which blocks App Store Connect API access account-wide. " +
	"An Account Holder must accept it at https://appstoreconnect.apple.com/agreements (App Store Connect may show the prompt as a banner on its home page instead). " +
	"Access can take a few minutes to return after acceptance."

// remediationForAPIError returns operator guidance for an App Store Connect
// error code, or an empty string when the code has no account-level cause.
//
// Apple returns the same cause under the FORBIDDEN and FORBIDDEN_ERROR
// prefixes, so accept either known prefix while matching the final segment.
func remediationForAPIError(code string) string {
	code = strings.TrimSpace(code)
	index := strings.LastIndex(code, ".")
	if index < 0 {
		return ""
	}
	prefix := strings.ToUpper(strings.TrimSpace(code[:index]))
	if prefix != "FORBIDDEN" && prefix != "FORBIDDEN_ERROR" {
		return ""
	}
	segment := strings.ToUpper(strings.TrimSpace(code[index+1:]))

	switch segment {
	case "REQUIRED_AGREEMENTS_MISSING_OR_EXPIRED", "PLA_NOT_VALID":
		return requiredAgreementRemediation
	default:
		return ""
	}
}

// IsRequiredAgreementError reports whether App Store Connect rejected API
// access because the team's required agreements are missing or expired.
func IsRequiredAgreementError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr == nil {
		return false
	}
	return remediationForAPIError(apiErr.Code) != ""
}

// APIAssociatedError represents an additional actionable error returned
// under errors[].meta.associatedErrors in App Store Connect responses.
type APIAssociatedError struct {
	Code   string
	Detail string
}

func (e *APIError) Error() string {
	title := strings.TrimSpace(SanitizeTerminalText(e.Title))
	detail := strings.TrimSpace(SanitizeTerminalText(e.Detail))
	code := strings.TrimSpace(SanitizeTerminalText(e.Code))
	baseMessage := ""
	switch {
	case title != "" && detail != "":
		baseMessage = fmt.Sprintf("%s: %s", title, detail)
	case title != "":
		baseMessage = title
	case detail != "":
		baseMessage = detail
	case code != "":
		baseMessage = code
	default:
		baseMessage = "API error"
	}

	sections := []string{baseMessage}
	if associated := formatAssociatedErrors(e.AssociatedErrors); associated != "" {
		sections = append(sections, associated)
	}
	if remediation := strings.TrimSpace(SanitizeTerminalText(e.Remediation)); remediation != "" {
		sections = append(sections, remediation)
	}
	return strings.Join(sections, "\n\n")
}

func (e *APIError) HTTPStatusCode() int {
	if e == nil {
		return 0
	}
	return e.StatusCode
}

func formatAssociatedErrors(values map[string][]APIAssociatedError) string {
	if len(values) == 0 {
		return ""
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	sections := make([]string, 0, len(keys))
	for _, key := range keys {
		resource := strings.TrimSpace(SanitizeTerminalText(key))
		if resource == "" {
			resource = "(unknown resource)"
		}

		entries := values[key]
		lines := make([]string, 0, len(entries)+1)
		lines = append(lines, fmt.Sprintf("Associated errors for %s:", resource))

		for _, entry := range entries {
			entryDetail := strings.TrimSpace(SanitizeTerminalText(entry.Detail))
			entryCode := strings.TrimSpace(SanitizeTerminalText(entry.Code))
			switch {
			case entryDetail != "":
				lines = append(lines, fmt.Sprintf("  - %s", entryDetail))
			case entryCode != "":
				lines = append(lines, fmt.Sprintf("  - %s", entryCode))
			}
		}

		if len(lines) > 1 {
			sections = append(sections, strings.Join(lines, "\n"))
		}
	}

	if len(sections) == 0 {
		return ""
	}
	return strings.Join(sections, "\n\n")
}

func (e *APIError) Is(target error) bool {
	switch target {
	case ErrNotFound:
		return strings.EqualFold(e.Code, "NOT_FOUND") || e.StatusCode == 404
	case ErrUnauthorized:
		// Apple returns 401 with code NOT_AUTHORIZED, so match on the
		// status code as well as the canonical code string.
		return strings.EqualFold(e.Code, "UNAUTHORIZED") || e.StatusCode == 401
	case ErrForbidden:
		return hasAPIErrorCodePrefix(e.Code, "FORBIDDEN", "FORBIDDEN_ERROR") || e.StatusCode == 403
	case ErrBadRequest:
		return strings.EqualFold(e.Code, "BAD_REQUEST")
	case ErrConflict:
		return strings.EqualFold(e.Code, "CONFLICT") || e.StatusCode == http.StatusConflict
	default:
		return false
	}
}

func hasAPIErrorCodePrefix(code string, prefixes ...string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	for _, prefix := range prefixes {
		if normalized == prefix || strings.HasPrefix(normalized, prefix+".") {
			return true
		}
	}
	return false
}

// IsMissingResourceOfType reports whether err is App Store Connect's 404 for a
// related resource that has not been created yet, such as an app without an
// availability record or price schedule. It keys on the HTTP status and the
// resource type named in Apple's detail so a 404 for the parent resource (for
// example an unknown app ID) is not mistaken for an unconfigured child.
func IsMissingResourceOfType(err error, resourceType string) bool {
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	if err == nil || resourceType == "" {
		return false
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok || apiErr == nil || apiErr.StatusCode != http.StatusNotFound {
		return false
	}
	detail := strings.ToLower(strings.TrimSpace(apiErr.Detail))
	if detail == "" {
		return false
	}
	// "There is no resource of type 'appAvailabilities' with id '123'"
	if strings.Contains(detail, "no resource of type '"+resourceType+"'") {
		return true
	}
	// "No appAvailabilities resource exists"
	return strings.Contains(detail, "no "+resourceType+" resource")
}

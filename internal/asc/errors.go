package asc

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrNotFound              = errors.New("resource not found")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrForbidden             = errors.New("forbidden")
	ErrBadRequest            = errors.New("bad request")
	ErrConflict              = errors.New("resource conflict")
	ErrRepeatedPaginationURL = errors.New("detected repeated pagination URL")
)

// APIError represents a parsed App Store Connect error response.
type APIError struct {
	Code             string
	Title            string
	Detail           string
	StatusCode       int // HTTP status code that triggered this error (0 if unknown)
	AssociatedErrors map[string][]APIAssociatedError
}

// APIAssociatedError represents an additional actionable error returned
// under errors[].meta.associatedErrors in App Store Connect responses.
type APIAssociatedError struct {
	Code   string
	Detail string
}

func (e *APIError) Error() string {
	title := strings.TrimSpace(sanitizeTerminal(e.Title))
	detail := strings.TrimSpace(sanitizeTerminal(e.Detail))
	code := strings.TrimSpace(sanitizeTerminal(e.Code))
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

	associated := formatAssociatedErrors(e.AssociatedErrors)
	if associated == "" {
		return baseMessage
	}
	return fmt.Sprintf("%s\n\n%s", baseMessage, associated)
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
		resource := strings.TrimSpace(sanitizeTerminal(key))
		if resource == "" {
			resource = "(unknown resource)"
		}

		entries := values[key]
		lines := make([]string, 0, len(entries)+1)
		lines = append(lines, fmt.Sprintf("Associated errors for %s:", resource))

		for _, entry := range entries {
			entryDetail := strings.TrimSpace(sanitizeTerminal(entry.Detail))
			entryCode := strings.TrimSpace(sanitizeTerminal(entry.Code))
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
		return strings.EqualFold(e.Code, "UNAUTHORIZED")
	case ErrForbidden:
		return strings.EqualFold(e.Code, "FORBIDDEN")
	case ErrBadRequest:
		return strings.EqualFold(e.Code, "BAD_REQUEST")
	case ErrConflict:
		return strings.EqualFold(e.Code, "CONFLICT")
	default:
		return false
	}
}

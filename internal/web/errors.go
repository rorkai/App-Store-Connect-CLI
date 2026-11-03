package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// IsSessionAuthFailure reports whether a web API or session-info request
// failed with 401 or 403, meaning the cached web session no longer works.
func IsSessionAuthFailure(err error) bool {
	if apiErr, ok := errors.AsType[*APIError](err); ok && (apiErr.Status == http.StatusUnauthorized || apiErr.Status == http.StatusForbidden) {
		return true
	}
	return isSessionInfoAuthExpired(err)
}

// IsDeveloperPortalSessionError reports whether err is a Developer Portal 401
// or 403 that already explains the session or permission failure.
func IsDeveloperPortalSessionError(err error) bool {
	_, ok := errors.AsType[*developerPortalSessionStatusError](err)
	return ok
}

// IsDuplicateAppNameError reports whether an internal API error means app name is taken.
func IsDuplicateAppNameError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr == nil || len(apiErr.rawResponseBody()) == 0 {
		return false
	}

	var payload struct {
		Errors []struct {
			Code   string `json:"code"`
			Detail string `json:"detail"`
			Title  string `json:"title"`
		} `json:"errors"`
	}
	if json.Unmarshal(apiErr.rawResponseBody(), &payload) != nil {
		body := strings.ToLower(string(apiErr.rawResponseBody()))
		return strings.Contains(body, "app name") && strings.Contains(body, "already")
	}

	for _, e := range payload.Errors {
		code := strings.ToUpper(strings.TrimSpace(e.Code))
		detail := strings.ToLower(strings.TrimSpace(e.Detail))
		title := strings.ToLower(strings.TrimSpace(e.Title))

		if strings.Contains(code, "DUPLICATE") && (strings.Contains(detail, "app name") || strings.Contains(title, "app name")) {
			return true
		}
		if strings.Contains(detail, "app name you entered is already being used") {
			return true
		}
	}
	return false
}

// AllCodes returns every errors[].code in the response body, in order, like
// asc.APIError.AllCodes on the public API. Apple can report several causes for
// one response and the one a caller keys on is not always first: re-creating
// an existing app answers POST /iris/v1/apps with three entries (captured live
// on 2026-09-29, see docs/API_NOTES.md). Blank codes are dropped; a body that
// is not a JSON:API error document yields nil.
func (e *APIError) AllCodes() []string {
	if e == nil || len(e.rawResponseBody()) == 0 {
		return nil
	}
	var payload struct {
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if json.Unmarshal(e.rawResponseBody(), &payload) != nil {
		return nil
	}
	codes := make([]string, 0, len(payload.Errors))
	for _, responseError := range payload.Errors {
		if code := strings.TrimSpace(responseError.Code); code != "" {
			codes = append(codes, code)
		}
	}
	if len(codes) == 0 {
		return nil
	}
	return codes
}

// IsMissingCompanyNameError reports whether an internal API error means Apple
// requires a company name for the app-creation request. The response body is
// only used for this package-internal classification; APIError.Error keeps it
// out of user-facing messages.
func IsMissingCompanyNameError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr == nil || len(apiErr.rawResponseBody()) == 0 {
		return false
	}

	var payload struct {
		Errors []struct {
			AttributeName string `json:"attributeName"`
			Detail        string `json:"detail"`
			Title         string `json:"title"`
		} `json:"errors"`
	}
	if json.Unmarshal(apiErr.rawResponseBody(), &payload) != nil {
		return false
	}

	for _, responseError := range payload.Errors {
		attributeName := strings.ToLower(strings.TrimSpace(responseError.AttributeName))
		text := strings.ToLower(strings.TrimSpace(responseError.Title + " " + responseError.Detail))
		if attributeName == "companyname" &&
			(strings.Contains(text, "is required") || strings.Contains(text, "required attribute") ||
				strings.Contains(text, "missing") || strings.Contains(text, "must provide")) {
			return true
		}
		if strings.Contains(text, "missing a required attribute") &&
			strings.Contains(text, "attribute 'companyname'") {
			return true
		}
	}
	return false
}

// IsAlreadyExistsConflict reports whether an internal API error is a 409 caused
// by an exact already-exists response. It intentionally avoids treating broader
// "already attached/submitted" wording as idempotent success.
func IsAlreadyExistsConflict(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr == nil || apiErr.Status != http.StatusConflict {
		return false
	}

	var payload struct {
		Errors []struct {
			Code   string `json:"code"`
			Detail string `json:"detail"`
			Title  string `json:"title"`
		} `json:"errors"`
	}
	if json.Unmarshal(apiErr.rawResponseBody(), &payload) != nil {
		body := strings.ToLower(string(apiErr.rawResponseBody()))
		return strings.Contains(body, "already exists") && !conflictTextMentionsDifferentTarget(body)
	}

	if len(payload.Errors) == 0 {
		return false
	}
	for _, e := range payload.Errors {
		code := strings.ToUpper(strings.TrimSpace(e.Code))
		detail := strings.ToLower(strings.TrimSpace(e.Detail))
		title := strings.ToLower(strings.TrimSpace(e.Title))
		text := detail + " " + title
		if !strings.Contains(code, "ALREADY_EXISTS") || conflictTextMentionsDifferentTarget(text) {
			return false
		}
	}
	return true
}

func conflictTextMentionsDifferentTarget(text string) bool {
	return strings.Contains(text, "another") || strings.Contains(text, "different")
}

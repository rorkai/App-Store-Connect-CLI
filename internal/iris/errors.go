package iris

import (
	"encoding/json"
	"errors"
	"strings"
)

// IsDuplicateAppNameError reports whether an IRIS API error indicates the app
// name is already taken (often globally across accounts).
func IsDuplicateAppNameError(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr == nil || len(apiErr.Body) == 0 {
		return false
	}

	var payload struct {
		Errors []struct {
			Code   string `json:"code"`
			Detail string `json:"detail"`
			Title  string `json:"title"`
		} `json:"errors"`
	}
	if json.Unmarshal(apiErr.Body, &payload) != nil {
		// Fallback to substring match if body isn't in the expected shape.
		body := strings.ToLower(string(apiErr.Body))
		return strings.Contains(body, "app name") && strings.Contains(body, "already")
	}

	for _, e := range payload.Errors {
		code := strings.TrimSpace(e.Code)
		detail := strings.ToLower(strings.TrimSpace(e.Detail))
		title := strings.ToLower(strings.TrimSpace(e.Title))

		// Observed: ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.DIFFERENT_ACCOUNT
		if strings.Contains(code, "DUPLICATE") && (strings.Contains(detail, "app name") || strings.Contains(title, "app name")) {
			return true
		}
		if strings.Contains(detail, "app name you entered is already being used") {
			return true
		}
	}

	return false
}

package web

import (
	"encoding/json"
	"errors"
	"strings"
)

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

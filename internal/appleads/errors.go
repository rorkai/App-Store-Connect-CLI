package appleads

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// APIError describes an Apple Ads API error response.
type APIError struct {
	StatusCode  int
	Field       string
	Message     string
	MessageCode string
	Detail      string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{}
	if e.StatusCode > 0 {
		parts = append(parts, fmt.Sprintf("HTTP %d", e.StatusCode))
	}
	if strings.TrimSpace(e.MessageCode) != "" {
		parts = append(parts, e.MessageCode)
	}
	if strings.TrimSpace(e.Field) != "" {
		parts = append(parts, e.Field)
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = strings.TrimSpace(e.Detail)
	}
	if message == "" {
		message = http.StatusText(e.StatusCode)
	}
	if message != "" {
		parts = append(parts, message)
	}
	if len(parts) == 0 {
		return "Apple Ads API request failed"
	}
	return strings.Join(parts, ": ")
}

func (e *APIError) HTTPStatusCode() int {
	if e == nil {
		return 0
	}
	return e.StatusCode
}

func parseError(body []byte, statusCode int) error {
	var errResp struct {
		Error struct {
			Errors []struct {
				Field       string `json:"field"`
				Message     string `json:"message"`
				MessageCode string `json:"messageCode"`
			} `json:"errors"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && len(errResp.Error.Errors) > 0 {
		item := errResp.Error.Errors[0]
		return &APIError{
			StatusCode:  statusCode,
			Field:       item.Field,
			Message:     item.Message,
			MessageCode: item.MessageCode,
		}
	}

	detail := sanitizeErrorBody(body)
	if detail == "" {
		detail = http.StatusText(statusCode)
	}
	return &APIError{
		StatusCode: statusCode,
		Detail:     detail,
	}
}

func sanitizeErrorBody(body []byte) string {
	sanitized := strings.TrimSpace(string(body))
	if sanitized == "" {
		return ""
	}
	if len(sanitized) > 4096 {
		sanitized = sanitized[:4096]
	}
	return sanitized
}

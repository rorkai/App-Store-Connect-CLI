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
	Version     APIVersion
	Code        string
	Details     []APIErrorDetail
	RateLimit   RateLimit
}

// APIErrorDetail describes one Apple Ads Platform API error detail.
type APIErrorDetail struct {
	Code    string         `json:"code,omitempty"`
	Message string         `json:"message,omitempty"`
	Info    map[string]any `json:"info,omitempty"`
}

type platformError struct {
	Code    string           `json:"code"`
	Message string           `json:"message"`
	Details []APIErrorDetail `json:"details"`
}

// RateLimit preserves the Apple Ads Platform API rate-limit response headers.
type RateLimit struct {
	Limit     string `json:"limit,omitempty"`
	Remaining string `json:"remaining,omitempty"`
	Reset     string `json:"reset,omitempty"`
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
	} else if strings.TrimSpace(e.Code) != "" {
		parts = append(parts, e.Code)
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
	surfacedCode := strings.TrimSpace(e.MessageCode)
	if surfacedCode == "" {
		surfacedCode = strings.TrimSpace(e.Code)
	}
	if surfacedCode == "" {
		surfacedCode = strings.TrimSpace(e.Field)
	}
	if details := formatAPIErrorDetails(e.Details, surfacedCode, message); details != "" {
		parts = append(parts, details)
	}
	if len(parts) == 0 {
		return "Apple Ads API request failed"
	}
	return strings.Join(parts, ": ")
}

func formatAPIErrorDetails(details []APIErrorDetail, surfacedCode, surfacedMessage string) string {
	formatted := make([]string, 0, len(details))
	for _, detail := range details {
		parts := make([]string, 0, 3)
		code := strings.TrimSpace(detail.Code)
		message := strings.TrimSpace(detail.Message)
		alreadySurfaced := code == strings.TrimSpace(surfacedCode) && message == strings.TrimSpace(surfacedMessage)
		if !alreadySurfaced {
			if code != "" {
				parts = append(parts, code)
			}
			if message != "" {
				parts = append(parts, message)
			}
		}
		if len(detail.Info) > 0 {
			if info, err := json.Marshal(detail.Info); err == nil {
				parts = append(parts, string(info))
			}
		}
		if len(parts) > 0 {
			formatted = append(formatted, strings.Join(parts, ": "))
		}
	}
	return strings.Join(formatted, "; ")
}

func (e *APIError) HTTPStatusCode() int {
	if e == nil {
		return 0
	}
	return e.StatusCode
}

func parseError(body []byte, statusCode int) error {
	return parseErrorForVersion(body, statusCode, nil, "")
}

func parseErrorForVersion(body []byte, statusCode int, headers http.Header, version APIVersion) error {
	if version == APIVersionPlatformV1 {
		if parsed := parsePlatformError(body, statusCode, headers); parsed != nil {
			return parsed
		}
	}

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
			Version:     version,
			RateLimit:   rateLimitFromHeaders(headers),
		}
	}

	detail := sanitizeErrorBody(body)
	if detail == "" {
		detail = http.StatusText(statusCode)
	}
	return &APIError{
		StatusCode: statusCode,
		Detail:     detail,
		Version:    version,
		RateLimit:  rateLimitFromHeaders(headers),
	}
}

func parsePlatformError(body []byte, statusCode int, headers http.Header) *APIError {
	var wrapped struct {
		Error platformError `json:"error"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && platformErrorPresent(wrapped.Error) {
		return platformAPIError(wrapped.Error, statusCode, headers)
	}
	var direct platformError
	if err := json.Unmarshal(body, &direct); err == nil && platformErrorPresent(direct) {
		return platformAPIError(direct, statusCode, headers)
	}
	return nil
}

func platformErrorPresent(err platformError) bool {
	return strings.TrimSpace(err.Code) != "" || strings.TrimSpace(err.Message) != "" || len(err.Details) > 0
}

func platformAPIError(err platformError, statusCode int, headers http.Header) *APIError {
	apiErr := &APIError{
		StatusCode: statusCode,
		Version:    APIVersionPlatformV1,
		Code:       strings.TrimSpace(err.Code),
		Message:    strings.TrimSpace(err.Message),
		Details:    err.Details,
		RateLimit:  rateLimitFromHeaders(headers),
	}
	if apiErr.Message == "" && len(apiErr.Details) > 0 {
		apiErr.Field = strings.TrimSpace(apiErr.Details[0].Code)
		apiErr.Detail = strings.TrimSpace(apiErr.Details[0].Message)
	}
	return apiErr
}

func rateLimitFromHeaders(headers http.Header) RateLimit {
	return RateLimit{
		Limit:     strings.TrimSpace(headerValue(headers, "RateLimit-Limit")),
		Remaining: strings.TrimSpace(headerValue(headers, "RateLimit-Remaining")),
		Reset:     strings.TrimSpace(headerValue(headers, "RateLimit-Reset")),
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

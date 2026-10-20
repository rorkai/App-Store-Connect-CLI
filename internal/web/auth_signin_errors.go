package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

const (
	signinBodyExcerptMaxBytes = 1024
	signinRedactedValue       = "[REDACTED]"

	// appleAccountLockedCode is the serviceErrors code Apple's IdMSA sends with
	// a 403 from signin/complete once it has locked the account for API
	// sign-in. A browser sign-in to App Store Connect can still succeed, so
	// only the account owner can clear it, at iforgot.apple.com.
	appleAccountLockedCode = "-20209"
)

// ErrAppleAccountLocked reports that Apple has locked the Apple Account for
// the API sign-in path asc uses. A *SigninServiceError matches it through
// errors.Is when its stage, status, and code say so.
var ErrAppleAccountLocked = errors.New("this Apple Account is locked for API sign-in")

// SigninServiceError reports an unexpected response from an Apple IdMSA
// sign-in stage. It carries only Apple's public service error codes and
// messages plus response metadata, never request data.
type SigninServiceError struct {
	// Stage is the debug stage name, for example "signin_init".
	Stage  string
	Status int
	// ServiceErrors holds Apple's serviceErrors entries as "code: message".
	ServiceErrors []string
	// Codes holds the bare serviceErrors codes, in Apple's order.
	Codes      []string
	RetryAfter string
	RequestID  string
}

// AccountLocked reports a 403 from signin/complete carrying Apple's locked
// account code. Other 403 refusals, and the same code under a 5xx, are not
// locks.
func (e *SigninServiceError) AccountLocked() bool {
	if e == nil || e.Stage != "signin_complete" || e.Status != http.StatusForbidden {
		return false
	}
	for _, code := range e.Codes {
		if code == appleAccountLockedCode {
			return true
		}
	}
	return false
}

// Is lets errors.Is(err, ErrAppleAccountLocked) recognize a locked account
// while callers keep Apple's status, message, and request ID.
func (e *SigninServiceError) Is(target error) bool {
	return target == ErrAppleAccountLocked && e.AccountLocked()
}

func (e *SigninServiceError) Error() string {
	if e == nil {
		return "sign-in failed"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s failed with status %d at %s", strings.ReplaceAll(e.Stage, "_", " "), e.Status, e.Stage)
	details := make([]string, 0, len(e.ServiceErrors)+2)
	for _, serviceError := range e.ServiceErrors {
		details = append(details, "Apple service error "+serviceError)
	}
	if e.RetryAfter != "" {
		details = append(details, "Retry-After: "+e.RetryAfter)
	}
	if e.RequestID != "" {
		details = append(details, "request_id="+e.RequestID)
	}
	if len(details) > 0 {
		b.WriteString(": ")
		b.WriteString(strings.Join(details, "; "))
	}
	return b.String()
}

func (e *SigninServiceError) HTTPStatusCode() int {
	if e == nil {
		return 0
	}
	return e.Status
}

// IsServerError reports whether Apple's sign-in service failed with a 5xx
// status, which is Apple's answer to both outages and sign-in throttling.
func (e *SigninServiceError) IsServerError() bool {
	return e != nil && e.Status >= 500 && e.Status <= 599
}

func newSigninServiceError(stage string, resp *http.Response, body []byte, sensitive []string) *SigninServiceError {
	err := &SigninServiceError{Stage: stage}
	if resp == nil {
		return err
	}
	err.Status = resp.StatusCode
	err.RetryAfter = sanitizeWebAuthDiagnosticValue(scrubSigninSecrets(resp.Header.Get("Retry-After"), sensitive))
	err.RequestID = sanitizeWebAuthDiagnosticValue(extractAppleRequestID(resp.Header))
	err.ServiceErrors = signinServiceErrorSummaries(body, sensitive)
	err.Codes = boundedWebAuthDiagnosticCodes(extractServiceErrorCodes(body))
	return err
}

func signinServiceErrorSummaries(body []byte, sensitive []string) []string {
	var payload struct {
		ServiceErrors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Title   string `json:"title"`
		} `json:"serviceErrors"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	summaries := make([]string, 0, min(len(payload.ServiceErrors), webAuthDiagnosticCodeLimit))
	for _, serviceError := range payload.ServiceErrors {
		if len(summaries) == webAuthDiagnosticCodeLimit {
			break
		}
		code := sanitizeWebAuthDiagnosticValue(scrubSigninSecrets(serviceError.Code, sensitive))
		message := strings.TrimSpace(serviceError.Message)
		if message == "" {
			message = strings.TrimSpace(serviceError.Title)
		}
		message = sanitizeWebPortalErrorText(scrubSigninSecrets(message, sensitive))
		switch {
		case code != "" && message != "":
			summaries = append(summaries, fmt.Sprintf("%s: %q", code, message))
		case code != "":
			summaries = append(summaries, code)
		case message != "":
			summaries = append(summaries, fmt.Sprintf("%q", message))
		}
	}
	return summaries
}

// signinRequestSecrets lists request values that a response body must never
// echo into diagnostics: the account name, SRP values, and every cookie value
// the client could have sent.
func signinRequestSecrets(client *http.Client, req *http.Request, values ...string) []string {
	secrets := make([]string, 0, len(values)+8)
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			secrets = append(secrets, value)
		}
	}
	if req != nil {
		for _, cookie := range req.Cookies() {
			if cookie.Value != "" {
				secrets = append(secrets, cookie.Value)
			}
		}
		if client != nil && client.Jar != nil && req.URL != nil {
			for _, cookie := range client.Jar.Cookies(req.URL) {
				if cookie != nil && cookie.Value != "" {
					secrets = append(secrets, cookie.Value)
				}
			}
		}
	}
	// Replace longer values first so a secret containing another is fully removed.
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	return secrets
}

func scrubSigninSecrets(value string, sensitive []string) string {
	for _, secret := range sensitive {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, signinRedactedValue)
		}
	}
	return value
}

// signinBodyAllowedKeys are the only response fields whose values reach a
// debug excerpt. Everything else keeps its key but loses its value, so the
// excerpt shows Apple's structure without any account or session data.
var signinBodyAllowedKeys = map[string]bool{
	"code":    true,
	"message": true,
	"title":   true,
}

var htmlTitlePattern = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// redactedSigninBodyExcerpt renders a bounded, redacted view of a failing
// IdMSA response body for --api-debug.
func redactedSigninBodyExcerpt(resp *http.Response, body []byte, sensitive []string) string {
	if len(bytes.TrimSpace(body)) == 0 {
		return "empty"
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err == nil {
		redacted, err := json.Marshal(redactSigninJSONValue("", decoded, sensitive))
		if err == nil {
			return boundSigninBodyExcerpt(sanitizeWebPortalErrorText(string(redacted)))
		}
	}

	contentType := ""
	if resp != nil {
		contentType = sanitizeWebAuthDiagnosticValue(resp.Header.Get("Content-Type"))
	}
	summary := fmt.Sprintf("non-JSON body (%d bytes", len(body))
	if contentType != "" {
		summary += ", content-type " + contentType
	}
	summary += ")"
	if match := htmlTitlePattern.FindSubmatch(body); len(match) == 2 {
		if title := sanitizeWebAuthDiagnosticValue(scrubSigninSecrets(string(match[1]), sensitive)); title != "" {
			summary += fmt.Sprintf(" title=%q", title)
		}
	}
	return summary
}

func redactSigninJSONValue(key string, value any, sensitive []string) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for childKey, child := range typed {
			out[childKey] = redactSigninJSONValue(childKey, child, sensitive)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, child := range typed {
			out[index] = redactSigninJSONValue(key, child, sensitive)
		}
		return out
	case nil:
		return nil
	case string:
		if signinBodyAllowedKeys[key] {
			return scrubSigninSecrets(typed, sensitive)
		}
		return signinRedactedValue
	default:
		if signinBodyAllowedKeys[key] {
			return typed
		}
		return signinRedactedValue
	}
}

func boundSigninBodyExcerpt(value string) string {
	if len(value) <= signinBodyExcerptMaxBytes {
		return value
	}
	limit := signinBodyExcerptMaxBytes - len(webAuthDiagnosticMarker)
	for limit > 0 && !isUTF8Boundary(value, limit) {
		limit--
	}
	return value[:limit] + webAuthDiagnosticMarker
}

func isUTF8Boundary(value string, index int) bool {
	return index >= len(value) || value[index]&0xC0 != 0x80
}

// logWebAuthHTTPFailure logs a failing IdMSA stage with Retry-After and a
// redacted body excerpt in addition to the usual status diagnostics.
func logWebAuthHTTPFailure(stage string, req *http.Request, resp *http.Response, body []byte, sensitive []string) {
	if !webDebugEnabledFn() {
		return
	}
	fields := webAuthHTTPLogFields(stage, req, resp, body, nil)
	if resp != nil {
		if retryAfter := sanitizeWebAuthDiagnosticValue(scrubSigninSecrets(resp.Header.Get("Retry-After"), sensitive)); retryAfter != "" {
			fields = append(fields, "retry_after", retryAfter)
		}
	}
	fields = append(fields, "body", redactedSigninBodyExcerpt(resp, body, sensitive))
	webDebugLogger.Info("web auth http", fields...)
}

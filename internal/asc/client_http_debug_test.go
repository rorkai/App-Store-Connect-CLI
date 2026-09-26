package asc

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func captureHTTPDebugLog(t *testing.T, enabled bool) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	originalLogger := debugLogger
	debugLogger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return attr
		},
	}))
	t.Cleanup(func() { debugLogger = originalLogger })

	SetDebugOverride(&enabled)
	SetDebugHTTPOverride(&enabled)
	t.Cleanup(func() {
		SetDebugOverride(nil)
		SetDebugHTTPOverride(nil)
	})

	return &buf
}

func TestSanitizeAuthHeader(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty", value: "", want: ""},
		{name: "bearer", value: "Bearer token", want: "Bearer [REDACTED]"},
		{name: "basic", value: "Basic abc123", want: "[REDACTED]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sanitizeAuthHeader(test.value); got != test.want {
				t.Fatalf("sanitizeAuthHeader(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestSanitizeURLForLog_RedactsSignedQuery(t *testing.T) {
	rawURL := "https://example.com/path?X-Amz-Signature=abc&foo=bar"
	got := sanitizeURLForLog(rawURL)

	if strings.Contains(got, "X-Amz-Signature=abc") {
		t.Fatalf("expected signature to be redacted, got %q", got)
	}
	if strings.Contains(got, "foo=bar") {
		t.Fatalf("expected non-sensitive values to be redacted for signed URLs, got %q", got)
	}
	if !strings.Contains(got, "REDACTED") {
		t.Fatalf("expected redacted placeholder in %q", got)
	}
}

func TestSanitizeURLForLog_RedactsTokenQuery(t *testing.T) {
	rawURL := "https://example.com/path?token=abc&foo=bar"
	got := sanitizeURLForLog(rawURL)

	if strings.Contains(got, "token=abc") {
		t.Fatalf("expected token to be redacted, got %q", got)
	}
	if !strings.Contains(got, "foo=bar") {
		t.Fatalf("expected non-sensitive values to remain, got %q", got)
	}
	if !strings.Contains(got, "REDACTED") {
		t.Fatalf("expected redacted placeholder in %q", got)
	}
}

func TestSanitizeURLForLog_EmptySignatureDoesNotRedactAll(t *testing.T) {
	rawURL := "https://example.com/path?X-Amz-Signature=&foo=bar"
	got := sanitizeURLForLog(rawURL)

	if !strings.Contains(got, "foo=bar") {
		t.Fatalf("expected non-sensitive values to remain, got %q", got)
	}
	if !strings.Contains(got, "REDACTED") {
		t.Fatalf("expected signature to be redacted, got %q", got)
	}
}

func TestSanitizeURLForLog_RedactsUserInfo(t *testing.T) {
	rawURL := "https://user:pass@example.com/path"
	got := sanitizeURLForLog(rawURL)

	if strings.Contains(got, "user:pass") {
		t.Fatalf("expected userinfo to be redacted, got %q", got)
	}
	if !strings.Contains(got, "REDACTED") {
		t.Fatalf("expected redacted placeholder in %q", got)
	}
}

func TestDebugLoggingRedactsSignedQuery(t *testing.T) {
	debugEnabled := true
	buf := captureHTTPDebugLog(t, debugEnabled)

	client := newTestClient(t, nil, jsonResponse(http.StatusOK, `{"data":[]}`))
	_, err := client.doOnce(context.Background(), http.MethodGet, "https://example.com/path?X-Amz-Signature=abc&foo=bar", nil, client.httpClient)
	if err != nil {
		t.Fatalf("doOnce() error: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "X-Amz-Signature=abc") {
		t.Fatalf("expected signature to be redacted, got %q", output)
	}
	if strings.Contains(output, "foo=bar") {
		t.Fatalf("expected signed query values to be redacted, got %q", output)
	}
	if !strings.Contains(output, "REDACTED") {
		t.Fatalf("expected redacted placeholder in %q", output)
	}
}

func TestDebugLoggingIncludesRateLimitHeader(t *testing.T) {
	buf := captureHTTPDebugLog(t, true)

	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	response.Header.Set("X-Rate-Limit", "user-hour-lim:3500;user-hour-rem:500;")
	client := newTestClient(t, nil, response)

	data, err := client.doOnce(context.Background(), http.MethodGet, "/v1/apps", nil, client.httpClient)
	if err != nil {
		t.Fatalf("doOnce() error: %v", err)
	}
	if string(data) != `{"data":[]}` {
		t.Fatalf("doOnce() data = %q, want unchanged response body", data)
	}

	output := buf.String()
	if !strings.Contains(output, `x-rate-limit=user-hour-lim:3500;user-hour-rem:500;`) {
		t.Fatalf("expected rate limit header in HTTP debug output, got %q", output)
	}
}

func TestDebugLoggingIncludesRateLimitHeaderOnError(t *testing.T) {
	buf := captureHTTPDebugLog(t, true)

	response := jsonResponse(http.StatusTooManyRequests, `{"errors":[{"code":"RATE_LIMIT_EXCEEDED"}]}`)
	response.Header.Set("X-Rate-Limit", "user-hour-lim:3500;user-hour-rem:0;")
	client := newTestClient(t, nil, response)

	_, err := client.doOnce(context.Background(), http.MethodGet, "/v1/apps", nil, client.httpClient)
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	var retryableErr *RetryableError
	if !errors.As(err, &retryableErr) {
		t.Fatalf("expected RetryableError, got %T: %v", err, err)
	}

	output := buf.String()
	if !strings.Contains(output, `status=429`) {
		t.Fatalf("expected HTTP status in debug output, got %q", output)
	}
	if !strings.Contains(output, `x-rate-limit=user-hour-lim:3500;user-hour-rem:0;`) {
		t.Fatalf("expected rate limit header in HTTP error debug output, got %q", output)
	}
}

func TestDebugLoggingDisabledDoesNotExposeRateLimitHeader(t *testing.T) {
	buf := captureHTTPDebugLog(t, false)

	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	response.Header.Set("X-Rate-Limit", "user-hour-lim:3500;user-hour-rem:500;")
	client := newTestClient(t, nil, response)

	if _, err := client.doOnce(context.Background(), http.MethodGet, "/v1/apps", nil, client.httpClient); err != nil {
		t.Fatalf("doOnce() error: %v", err)
	}
	if output := buf.String(); output != "" {
		t.Fatalf("expected no HTTP debug output when disabled, got %q", output)
	}
}

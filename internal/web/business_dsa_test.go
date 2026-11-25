package web

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"testing/iotest"
)

func TestDSAStatusTransportFailureKeepsIdentifiersPrivate(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	originalLogger, originalDebug := webDebugLogger, webDebugEnabledFn
	t.Cleanup(func() { webDebugLogger, webDebugEnabledFn = originalLogger, originalDebug })
	var logs bytes.Buffer
	webDebugLogger = slog.New(slog.NewTextHandler(&logs, nil))
	webDebugEnabledFn = func() bool { return true }
	for _, tc := range []struct {
		name       string
		cause      error
		bodyRead   bool
		wantCause  error
		wantStatus int
	}{
		{name: "broker refusal", cause: &BrokerError{Status: http.StatusForbidden, Code: "private-code", Message: "private-contact"}, wantStatus: http.StatusForbidden},
		{name: "client deadline", cause: fmt.Errorf("private-contact: %w", context.DeadlineExceeded), wantCause: context.DeadlineExceeded},
		{name: "transport timeout", cause: &net.DNSError{Name: "private-host", IsTimeout: true}, wantCause: os.ErrDeadlineExceeded},
		{name: "canceled body read", bodyRead: true, cause: fmt.Errorf("private-contact: %w", context.Canceled), wantCause: context.Canceled},
		{name: "unclassified failure", cause: errors.New("private-contact")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			client := NewClient(&AuthSession{Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				if tc.bodyRead {
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(iotest.ErrReader(tc.cause))}, nil
				}
				return nil, tc.cause
			})}})
			_, err := client.GetDSAStatus(context.Background(), "private-account")
			if err == nil || strings.Contains(err.Error(), "private-") || strings.Contains(logs.String(), "private-") {
				t.Fatalf("DSA request exposed private values: error = %v, logs = %q", err, logs.String())
			}
			if !strings.Contains(logs.String(), "/sellerInfo") {
				t.Fatalf("missing safe request path in logs: %q", logs.String())
			}
			if tc.wantStatus != 0 {
				broker, ok := errors.AsType[*BrokerError](err)
				if !ok || broker.HTTPStatusCode() != tc.wantStatus || broker.Code != "" || broker.Message != "" {
					t.Fatalf("expected a status-only broker refusal, got %v", err)
				}
			} else if tc.wantCause != nil {
				if !errors.Is(err, tc.wantCause) {
					t.Fatalf("error = %v, want cause %v", err, tc.wantCause)
				}
			} else if err.Error() != "DSA status request failed" {
				t.Fatalf("unexpected diagnostic: %v", err)
			}
		})
	}
}

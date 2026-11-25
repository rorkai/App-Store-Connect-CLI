package web

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func TestDSAStatusTransportFailureKeepsIdentifiersPrivate(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	originalLogger, originalDebug := webDebugLogger, webDebugEnabledFn
	t.Cleanup(func() { webDebugLogger, webDebugEnabledFn = originalLogger, originalDebug })
	var logs bytes.Buffer
	webDebugLogger = slog.New(slog.NewTextHandler(&logs, nil))
	webDebugEnabledFn = func() bool { return true }
	client := NewClient(&AuthSession{Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("private-contact")
	})}})
	_, err := client.GetDSAStatus(context.Background(), "private-account")
	if err == nil || strings.Contains(err.Error(), "private-") || strings.Contains(logs.String(), "private-") {
		t.Fatalf("DSA request exposed private values: error = %v, logs = %q", err, logs.String())
	}
	if !strings.Contains(err.Error(), "DSA status request failed") || !strings.Contains(logs.String(), "/sellerInfo") {
		t.Fatalf("missing safe diagnostics: error = %v, logs = %q", err, logs.String())
	}
}

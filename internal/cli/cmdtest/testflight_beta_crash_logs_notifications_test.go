package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestFlightBetaNotificationsCreateOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds/build-1/buildBetaDetail":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			body := `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/buildBetaNotifications":
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read body error: %v", err)
			}
			if !strings.Contains(string(payload), `"id":"build-1"`) {
				t.Fatalf("expected build id in body, got %s", string(payload))
			}
			body := `{"data":{"type":"buildBetaNotifications","id":"notif-1"}}`
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request path %s", req.URL.Path)
		}
		return nil, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "notifications", "send", "--build-id", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"notif-1"`) {
		t.Fatalf("expected notification id in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"notificationAction":`) {
		t.Fatalf("expected notificationAction to remain omitted for manual success, got %q", stdout)
	}
}

func TestTestFlightBetaNotificationsCreateReportsPreflightAutoNotify(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-1/buildBetaDetail" {
			t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
		}
		body := `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":true}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "notifications", "send", "--build-id", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stdout, `"notificationAction":"auto_notify_enabled"`) {
		t.Fatalf("expected auto-notify action in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"data":`) || strings.Contains(stdout, `"links":`) {
		t.Fatalf("expected action-only output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 1 {
		t.Fatalf("expected one build beta detail request, got %d", requestCount)
	}
}

func TestTestFlightBetaNotificationsCreateReportsRaceAutoNotify(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch req.URL.Path {
		case "/v1/builds/build-1/buildBetaDetail":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			body := `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/buildBetaNotifications":
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			body := `{"errors":[{"code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"There is a problem with the request entity","detail":"Auto-notify already enabled"}]}`
			return &http.Response{
				StatusCode: http.StatusConflict,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "notifications", "send", "--build-id", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if !strings.Contains(stdout, `"notificationAction":"auto_notify_enabled"`) {
		t.Fatalf("expected auto-notify action in output, got %q", stdout)
	}
	if strings.Contains(stdout, `"data":`) || strings.Contains(stdout, `"links":`) {
		t.Fatalf("expected action-only output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 2 {
		t.Fatalf("expected detail preflight and notification create requests, got %d", requestCount)
	}
}

func TestTestFlightBetaNotificationsCreatePreservesUnrelatedConflict(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch req.URL.Path {
		case "/v1/builds/build-1/buildBetaDetail":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			body := `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "/v1/buildBetaNotifications":
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", req.Method)
			}
			body := `{"errors":[{"code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"There is a problem with the request entity","detail":"Build is not eligible for notification"}]}`
			return &http.Response{
				StatusCode: http.StatusConflict,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "notifications", "send", "--build-id", "build-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected unrelated conflict error")
	}
	if !strings.Contains(runErr.Error(), "notifications send: failed to send:") {
		t.Fatalf("expected send error prefix, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "Build is not eligible for notification") {
		t.Fatalf("expected unrelated conflict detail, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount != 2 {
		t.Fatalf("expected detail preflight and notification create requests, got %d", requestCount)
	}
}

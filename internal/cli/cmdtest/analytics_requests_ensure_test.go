package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyticsRequestsEnsureRejectsInvalidAccessType(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "requests", "ensure",
			"--app", "app-1",
			"--access-type", "DAILY",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--access-type must be ONGOING or ONE_TIME_SNAPSHOT") {
		t.Fatalf("expected invalid access type error, got %q", stderr)
	}
}

func TestAnalyticsRequestsEnsureUsesExistingRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	var requestCount lockedCounter
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		count := requestCount.Inc()
		if count != 1 {
			t.Fatalf("unexpected extra request %d: %s %s", count, req.Method, req.URL.String())
		}
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/analyticsReportRequests" {
			t.Fatalf("expected analytics requests path, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "200" {
			t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
		}

		body := `{"data":[` +
			`{"type":"analyticsReportRequests","id":"req-snapshot","attributes":{"accessType":"ONE_TIME_SNAPSHOT","state":"COMPLETED"}},` +
			`{"type":"analyticsReportRequests","id":"req-existing","attributes":{"accessType":"ONGOING","state":"PROCESSING","createdDate":"2026-05-01T00:00:00Z"}}` +
			`],"links":{"next":""}}`
		return analyticsEnsureJSONResponse(http.StatusOK, body), nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "requests", "ensure",
			"--app", "app-1",
			"--access-type", "ONGOING",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
	}
	if result["requestId"] != "req-existing" {
		t.Fatalf("expected existing request id, got %#v", result["requestId"])
	}
	if result["created"] != false {
		t.Fatalf("expected created=false, got %#v", result["created"])
	}
}

func TestAnalyticsRequestsEnsureCreatesMissingRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	var requestCount lockedCounter
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch count := requestCount.Inc(); count {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected first request GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/apps/app-1/analyticsReportRequests" {
				t.Fatalf("expected analytics requests path, got %s", req.URL.Path)
			}
			return analyticsEnsureJSONResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case 2:
			if req.Method != http.MethodPost {
				t.Fatalf("expected second request POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/analyticsReportRequests" {
				t.Fatalf("expected create path, got %s", req.URL.Path)
			}
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			body := string(bodyBytes)
			if !strings.Contains(body, `"accessType":"ONGOING"`) || !strings.Contains(body, `"id":"app-1"`) {
				t.Fatalf("unexpected create payload: %s", body)
			}
			response := `{"data":{"type":"analyticsReportRequests","id":"req-created","attributes":{"accessType":"ONGOING","state":"PROCESSING","createdDate":"2026-05-02T00:00:00Z"}}}`
			return analyticsEnsureJSONResponse(http.StatusCreated, response), nil
		default:
			t.Fatalf("unexpected extra request %d: %s %s", count, req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "requests", "ensure",
			"--app", "app-1",
			"--access-type", "ONGOING",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("failed to parse JSON output %q: %v", stdout, err)
	}
	if result["requestId"] != "req-created" {
		t.Fatalf("expected created request id, got %#v", result["requestId"])
	}
	if result["created"] != true {
		t.Fatalf("expected created=true, got %#v", result["created"])
	}
}

func analyticsEnsureJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

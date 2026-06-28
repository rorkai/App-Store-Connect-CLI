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

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestAnalyticsRequestReuseExistingRejectsInvalidAccessType(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "request",
			"--app", "app-1",
			"--access-type", "DAILY",
			"--reuse-existing",
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

func TestAnalyticsRequestReuseExistingRejectsInvalidBoolExitCode(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "invalid bool",
			args: []string{
				"analytics", "request",
				"--app", "app-1",
				"--access-type", "ONGOING",
				"--reuse-existing=maybe",
			},
			wantErr: `invalid boolean value "maybe" for -reuse-existing`,
		},
		{
			name: "invalid bool mixed flag order",
			args: []string{
				"analytics", "request",
				"--access-type=ONGOING",
				"--reuse-existing=maybe",
				"--app", "app-1",
			},
			wantErr: `invalid boolean value "maybe" for -reuse-existing`,
		},
		{
			name: "invalid bool before subcommand",
			args: []string{
				"--reuse-existing=maybe",
				"analytics", "request",
				"--app", "app-1",
				"--access-type", "ONGOING",
			},
			wantErr: "flag provided but not defined: -reuse-existing",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "1.2.3"); code != rootcmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", rootcmd.ExitUsage, code)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected invalid bool error, got %q", stderr)
			}
		})
	}
}

func TestAnalyticsRequestReuseExistingUsesExistingRequest(t *testing.T) {
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
		return analyticsReuseExistingJSONResponse(http.StatusOK, body), nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "request",
			"--app", "app-1",
			"--access-type", "ONGOING",
			"--reuse-existing",
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

func TestAnalyticsRequestReuseExistingUsesExistingRequestFromLaterPage(t *testing.T) {
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
			body := `{"data":[` +
				`{"type":"analyticsReportRequests","id":"req-snapshot","attributes":{"accessType":"ONE_TIME_SNAPSHOT","state":"COMPLETED"}}` +
				`],"links":{"next":"/v1/apps/app-1/analyticsReportRequests?cursor=2"}}`
			return analyticsReuseExistingJSONResponse(http.StatusOK, body), nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected second request GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/apps/app-1/analyticsReportRequests" {
				t.Fatalf("expected analytics requests path, got %s", req.URL.Path)
			}
			if req.URL.Query().Get("cursor") != "2" {
				t.Fatalf("expected cursor=2, got %q", req.URL.Query().Get("cursor"))
			}
			body := `{"data":[` +
				`{"type":"analyticsReportRequests","id":"req-existing-page-2","attributes":{"accessType":"ONGOING","state":"PROCESSING","createdDate":"2026-05-01T00:00:00Z"}}` +
				`],"links":{"next":""}}`
			return analyticsReuseExistingJSONResponse(http.StatusOK, body), nil
		default:
			t.Fatalf("unexpected extra request %d: %s %s", count, req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "request",
			"--app", "app-1",
			"--access-type", "ONGOING",
			"--reuse-existing",
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
	if result["requestId"] != "req-existing-page-2" {
		t.Fatalf("expected existing request id from second page, got %#v", result["requestId"])
	}
	if result["created"] != false {
		t.Fatalf("expected created=false, got %#v", result["created"])
	}
}

func TestAnalyticsRequestReuseExistingUsesExistingRequestFromFirstPageWithMultiplePages(t *testing.T) {
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
			body := `{"data":[` +
				`{"type":"analyticsReportRequests","id":"req-existing-page-1","attributes":{"accessType":"ONGOING","state":"PROCESSING","createdDate":"2026-05-01T00:00:00Z"}}` +
				`],"links":{"next":"/v1/apps/app-1/analyticsReportRequests?cursor=2"}}`
			return analyticsReuseExistingJSONResponse(http.StatusOK, body), nil
		case 2:
			if req.Method != http.MethodGet {
				t.Fatalf("expected second request GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/apps/app-1/analyticsReportRequests" {
				t.Fatalf("expected analytics requests path, got %s", req.URL.Path)
			}
			if req.URL.Query().Get("cursor") != "2" {
				t.Fatalf("expected cursor=2, got %q", req.URL.Query().Get("cursor"))
			}
			body := `{"data":[` +
				`{"type":"analyticsReportRequests","id":"req-snapshot","attributes":{"accessType":"ONE_TIME_SNAPSHOT","state":"COMPLETED"}}` +
				`],"links":{"next":""}}`
			return analyticsReuseExistingJSONResponse(http.StatusOK, body), nil
		default:
			t.Fatalf("unexpected extra request %d: %s %s", count, req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "request",
			"--app", "app-1",
			"--access-type", "ONGOING",
			"--reuse-existing",
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
	if result["requestId"] != "req-existing-page-1" {
		t.Fatalf("expected existing request id from first page, got %#v", result["requestId"])
	}
	if result["created"] != false {
		t.Fatalf("expected created=false, got %#v", result["created"])
	}
}

func TestAnalyticsRequestReuseExistingCreatesMissingRequest(t *testing.T) {
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
			return analyticsReuseExistingJSONResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
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
			var payload struct {
				Data struct {
					Attributes struct {
						AccessType string `json:"accessType"`
					} `json:"attributes"`
					Relationships struct {
						App struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"app"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.Unmarshal(bodyBytes, &payload); err != nil {
				t.Fatalf("failed to parse create payload: %v", err)
			}
			if payload.Data.Attributes.AccessType != "ONGOING" || payload.Data.Relationships.App.Data.ID != "app-1" {
				t.Fatalf("unexpected create payload: %#v", payload)
			}
			response := `{"data":{"type":"analyticsReportRequests","id":"req-created","attributes":{"accessType":"ONGOING","state":"PROCESSING","createdDate":"2026-05-02T00:00:00Z"}}}`
			return analyticsReuseExistingJSONResponse(http.StatusCreated, response), nil
		default:
			t.Fatalf("unexpected extra request %d: %s %s", count, req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "request",
			"--app", "app-1",
			"--access-type", "ONGOING",
			"--reuse-existing",
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

func TestAnalyticsRequestReuseExistingRefetchesAfterCreateConflict(t *testing.T) {
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
			return analyticsReuseExistingJSONResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`), nil
		case 2:
			if req.Method != http.MethodPost {
				t.Fatalf("expected second request POST, got %s", req.Method)
			}
			if req.URL.Path != "/v1/analyticsReportRequests" {
				t.Fatalf("expected create path, got %s", req.URL.Path)
			}
			body := `{"errors":[{"status":"409","code":"ENTITY_ERROR","title":"Conflict","detail":"duplicate request"}]}`
			return analyticsReuseExistingJSONResponse(http.StatusConflict, body), nil
		case 3:
			if req.Method != http.MethodGet {
				t.Fatalf("expected third request GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/apps/app-1/analyticsReportRequests" {
				t.Fatalf("expected analytics requests path, got %s", req.URL.Path)
			}
			body := `{"data":[` +
				`{"type":"analyticsReportRequests","id":"req-created-by-race","attributes":{"accessType":"ONGOING","state":"PROCESSING","createdDate":"2026-05-03T00:00:00Z"}}` +
				`],"links":{"next":""}}`
			return analyticsReuseExistingJSONResponse(http.StatusOK, body), nil
		default:
			t.Fatalf("unexpected extra request %d: %s %s", count, req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"analytics", "request",
			"--app", "app-1",
			"--access-type", "ONGOING",
			"--reuse-existing",
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
	if result["requestId"] != "req-created-by-race" {
		t.Fatalf("expected raced request id, got %#v", result["requestId"])
	}
	if result["created"] != false {
		t.Fatalf("expected created=false, got %#v", result["created"])
	}
}

func analyticsReuseExistingJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

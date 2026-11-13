package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestXcodeCloudIssuesListAggregatesIssuesFromRunID(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildRuns/run-1/actions" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"ciBuildActions","id":"act-1","attributes":{"name":"Archive","actionType":"ARCHIVE"}},{"type":"ciBuildActions","id":"act-2","attributes":{"name":"Test","actionType":"TEST"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildActions/act-1/issues" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildActions/act-2/issues" {
				t.Fatalf("unexpected third request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"ciIssues","id":"issue-1"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "issues", "list", "--run-id", "run-1", "--paginate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"issue-1"`) {
		t.Fatalf("expected issue output, got %q", stdout)
	}
	if callCount != 3 {
		t.Fatalf("expected 3 requests, got %d", callCount)
	}
}

func TestXcodeCloudArtifactsListAggregatesAllActionsFromRunID(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildRuns/run-1/actions" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"ciBuildActions","id":"act-1","attributes":{"name":"Archive iOS","actionType":"ARCHIVE"}},{"type":"ciBuildActions","id":"act-2","attributes":{"name":"Test iOS","actionType":"TEST"}},{"type":"ciBuildActions","id":"act-3","attributes":{"name":"Archive macOS","actionType":"ARCHIVE"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildActions/act-1/artifacts" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"ciArtifacts","id":"artifact-1"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildActions/act-2/artifacts" {
				t.Fatalf("unexpected third request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"ciArtifacts","id":"test-log-bundle","attributes":{"fileType":"LOG_BUNDLE"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 4:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildActions/act-3/artifacts" {
				t.Fatalf("unexpected fourth request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"ciArtifacts","id":"artifact-2"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "artifacts", "list", "--run-id", "run-1", "--paginate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"artifact-1"`) {
		t.Fatalf("expected artifact output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"id":"artifact-2"`) {
		t.Fatalf("expected second artifact output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"id":"test-log-bundle"`) {
		t.Fatalf("expected test action log bundle output, got %q", stdout)
	}
	if callCount != 4 {
		t.Fatalf("expected 4 requests, got %d", callCount)
	}
}

func TestXcodeCloudTestResultsListAggregatesTestActionsFromRunID(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildRuns/run-1/actions" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"ciBuildActions","id":"act-1","attributes":{"name":"Archive","actionType":"ARCHIVE"}},{"type":"ciBuildActions","id":"act-2","attributes":{"name":"Test iOS","actionType":"TEST"}},{"type":"ciBuildActions","id":"act-3","attributes":{"name":"Test macOS","actionType":"TEST"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildActions/act-2/testResults" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"ciTestResults","id":"result-1"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildActions/act-3/testResults" {
				t.Fatalf("unexpected third request: %s %s", req.Method, req.URL.String())
			}
			if req.URL.Query().Get("limit") != "200" {
				t.Fatalf("expected limit=200, got %q", req.URL.Query().Get("limit"))
			}
			body := `{"data":[{"type":"ciTestResults","id":"result-2"}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "test-results", "list", "--run-id", "run-1", "--paginate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"result-1"`) || !strings.Contains(stdout, `"id":"result-2"`) {
		t.Fatalf("expected aggregated test results, got %q", stdout)
	}
	if callCount != 3 {
		t.Fatalf("expected 3 requests, got %d", callCount)
	}
}

func TestXcodeCloudTestResultsListRunIDEmptyResultsStayArray(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	callCount := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		switch callCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildRuns/run-1/actions" {
				t.Fatalf("unexpected first request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[{"type":"ciBuildActions","id":"act-1","attributes":{"name":"Test iOS","actionType":"TEST"}}]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/ciBuildActions/act-1/testResults" {
				t.Fatalf("unexpected second request: %s %s", req.Method, req.URL.String())
			}
			body := `{"data":[]}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		default:
			t.Fatalf("unexpected request count %d", callCount)
			return nil, nil
		}
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "test-results", "list", "--run-id", "run-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"data":[]`) {
		t.Fatalf("expected empty data array, got %q", stdout)
	}
}

func TestXcodeCloudActionResourceUsageErrors(t *testing.T) {
	setupAuth(t)

	runUsageExitCases(t, []usageExitCase{
		{
			name:    "issues list rejects next with run id",
			args:    []string{"xcode-cloud", "issues", "list", "--run-id", "run-1", "--next", "https://api.appstoreconnect.apple.com/v1/ciBuildActions/act-1/issues?cursor=abc"},
			wantErr: "--next is not supported with --run-id",
		},
		{
			name:    "issues list rejects conflicting selectors",
			args:    []string{"xcode-cloud", "issues", "list", "--action-id", "act-1", "--run-id", "run-1"},
			wantErr: "--action-id and --run-id are mutually exclusive",
		},
		{
			name:    "artifacts list rejects conflicting selectors",
			args:    []string{"xcode-cloud", "artifacts", "list", "--action-id", "act-1", "--run-id", "run-1"},
			wantErr: "--action-id and --run-id are mutually exclusive",
		},
	})
}

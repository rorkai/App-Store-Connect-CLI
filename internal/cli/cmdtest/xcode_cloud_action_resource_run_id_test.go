package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestXcodeCloudActionResourcesAggregateFromRunID(t *testing.T) {
	type step struct {
		path string
		body string
	}
	tests := []struct {
		name       string
		args       []string
		steps      []step
		wantStdout []string
	}{
		{
			name: "issues",
			args: []string{"xcode-cloud", "issues", "list", "--run-id", "run-1", "--paginate"},
			steps: []step{
				{"/v1/ciBuildRuns/run-1/actions", `{"data":[{"type":"ciBuildActions","id":"act-1","attributes":{"name":"Archive","actionType":"ARCHIVE"}},{"type":"ciBuildActions","id":"act-2","attributes":{"name":"Test","actionType":"TEST"}}]}`},
				{"/v1/ciBuildActions/act-1/issues", `{"data":[]}`},
				{"/v1/ciBuildActions/act-2/issues", `{"data":[{"type":"ciIssues","id":"issue-1"}]}`},
			},
			wantStdout: []string{`"id":"issue-1"`},
		},
		{
			name: "artifacts from all actions",
			args: []string{"xcode-cloud", "artifacts", "list", "--run-id", "run-1", "--paginate"},
			steps: []step{
				{"/v1/ciBuildRuns/run-1/actions", `{"data":[{"type":"ciBuildActions","id":"act-1","attributes":{"name":"Archive iOS","actionType":"ARCHIVE"}},{"type":"ciBuildActions","id":"act-2","attributes":{"name":"Test iOS","actionType":"TEST"}},{"type":"ciBuildActions","id":"act-3","attributes":{"name":"Archive macOS","actionType":"ARCHIVE"}}]}`},
				{"/v1/ciBuildActions/act-1/artifacts", `{"data":[{"type":"ciArtifacts","id":"artifact-1"}]}`},
				{"/v1/ciBuildActions/act-2/artifacts", `{"data":[{"type":"ciArtifacts","id":"test-log-bundle","attributes":{"fileType":"LOG_BUNDLE"}}]}`},
				{"/v1/ciBuildActions/act-3/artifacts", `{"data":[{"type":"ciArtifacts","id":"artifact-2"}]}`},
			},
			wantStdout: []string{`"id":"artifact-1"`, `"id":"artifact-2"`, `"id":"test-log-bundle"`},
		},
		{
			name: "test results from test actions only",
			args: []string{"xcode-cloud", "test-results", "list", "--run-id", "run-1", "--paginate"},
			steps: []step{
				{"/v1/ciBuildRuns/run-1/actions", `{"data":[{"type":"ciBuildActions","id":"act-1","attributes":{"name":"Archive","actionType":"ARCHIVE"}},{"type":"ciBuildActions","id":"act-2","attributes":{"name":"Test iOS","actionType":"TEST"}},{"type":"ciBuildActions","id":"act-3","attributes":{"name":"Test macOS","actionType":"TEST"}}]}`},
				{"/v1/ciBuildActions/act-2/testResults", `{"data":[{"type":"ciTestResults","id":"result-1"}]}`},
				{"/v1/ciBuildActions/act-3/testResults", `{"data":[{"type":"ciTestResults","id":"result-2"}]}`},
			},
			wantStdout: []string{`"id":"result-1"`, `"id":"result-2"`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)

			callCount := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				callCount++
				if callCount > len(test.steps) {
					t.Fatalf("unexpected request %d: %s %s", callCount, req.Method, req.URL.String())
				}
				want := test.steps[callCount-1]
				if req.Method != http.MethodGet || req.URL.Path != want.path {
					t.Fatalf("request %d = %s %s, want GET %s", callCount, req.Method, req.URL.String(), want.path)
				}
				if req.URL.Query().Get("limit") != "200" {
					t.Fatalf("request %d limit = %q, want 200", callCount, req.URL.Query().Get("limit"))
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(want.body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			for _, want := range test.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected %s in output, got %q", want, stdout)
				}
			}
			if callCount != len(test.steps) {
				t.Fatalf("expected %d requests, got %d", len(test.steps), callCount)
			}
		})
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

package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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

			reg := asctest.NewRegistry(t)
			reg.Install()
			for _, step := range test.steps {
				reg.Register(asctest.GET(step.path, "limit=200"), asctest.JSON(http.StatusOK, step.body))
			}

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
			requests := reg.Requests()
			if callCount := len(requests); callCount != len(test.steps) {
				t.Fatalf("expected %d requests, got %d", len(test.steps), callCount)
			}
			for i, req := range requests {
				if want := test.steps[i]; req.URL.Path != want.path {
					t.Fatalf("request %d = %s %s, want GET %s", i+1, req.Method, req.URL.String(), want.path)
				}
			}
		})
	}
}

func TestXcodeCloudTestResultsListRunIDEmptyResultsStayArray(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/ciBuildRuns/run-1/actions", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"ciBuildActions","id":"act-1","attributes":{"name":"Test iOS","actionType":"TEST"}}]}`))
	reg.Register(asctest.GET("/v1/ciBuildActions/act-1/testResults"), asctest.JSON(http.StatusOK, `{"data":[]}`))

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

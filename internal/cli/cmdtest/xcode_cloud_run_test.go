package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestXcodeCloudRunWithPullRequestIDPostsExpectedPayload(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "configured-app")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.POST("/v1/ciBuildRuns").JSONBody(`{"data":{"type":"ciBuildRuns","relationships":{"workflow":{"data":{"type":"ciWorkflows","id":"wf-1"}},"pullRequest":{"data":{"type":"scmPullRequests","id":"pr-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"ciBuildRuns","id":"run-pr-1","attributes":{"number":1,"executionProgress":"PENDING","startReason":"MANUAL","createdDate":"2026-03-05T10:00:00Z"}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "xcode-cloud", "run", "--workflow-id", "wf-1", "--pull-request-id", "pr-1")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"buildRunId":"run-pr-1"`) {
		t.Fatalf("expected build run ID in output, got %q", stdout)
	}
	if requestCount := len(reg.Requests()); requestCount != 1 {
		t.Fatalf("expected exactly one request, got %d", requestCount)
	}
}

func TestXcodeCloudRunWithSourceRunAndCleanPostsExpectedPayload(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/ciBuildRuns/run-1", "fields[ciBuildRuns]=workflow", "include=workflow"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"ciBuildRuns","id":"run-1","relationships":{"workflow":{"data":{"type":"ciWorkflows","id":"wf-1"}}}}}`))
	reg.Register(asctest.POST("/v1/ciBuildRuns").JSONBody(`{"data":{"type":"ciBuildRuns","attributes":{"clean":true},"relationships":{"buildRun":{"data":{"type":"ciBuildRuns","id":"run-1"}},"workflow":{"data":{"type":"ciWorkflows","id":"wf-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"ciBuildRuns","id":"run-rerun-1","attributes":{"number":2,"executionProgress":"PENDING","startReason":"MANUAL","createdDate":"2026-03-05T10:00:00Z"}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "xcode-cloud", "run", "--source-run-id", "run-1", "--clean")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"buildRunId":"run-rerun-1"`) {
		t.Fatalf("expected build run ID in output, got %q", stdout)
	}
	if requestCount := len(reg.Requests()); requestCount != 2 {
		t.Fatalf("expected exactly two requests, got %d", requestCount)
	}
}

func TestXcodeCloudRunRejectsAppWithDirectIDSelectorsBeforeRequests(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "workflow ID",
			args:    []string{"xcode-cloud", "run", "--app", "APP_ID", "--workflow-id", "wf-1", "--pull-request-id", "pr-1"},
			wantErr: "--app cannot be combined with --workflow-id",
		},
		{
			name:    "source run ID",
			args:    []string{"xcode-cloud", "run", "--app", "APP_ID", "--source-run-id", "run-1", "--clean"},
			wantErr: "--app cannot be combined with --source-run-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			clientFactory := stubClient(t, reg)
			clientCalls := 0
			stdout, stderr, code := runWithDeps(t, func() (*asc.Client, error) {
				clientCalls++
				return clientFactory()
			}, tt.args...)

			if code != 2 {
				t.Fatalf("expected usage exit code 2, got %d (stderr %q)", code, stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", tt.wantErr, stderr)
			}
			if clientCalls != 0 {
				t.Fatalf("expected client creation to be skipped, got %d calls", clientCalls)
			}
			if requestCount := len(reg.Requests()); requestCount != 0 {
				t.Fatalf("expected zero requests, got %d", requestCount)
			}
		})
	}
}

func TestXcodeCloudBuildRunsGetFetchesBuildRun(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/ciBuildRuns/run-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":42,"executionProgress":"RUNNING"}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "xcode-cloud", "build-runs", "view", "--id", "run-1")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"run-1"`) {
		t.Fatalf("expected run ID in output, got %q", stdout)
	}
	if requestCount := len(reg.Requests()); requestCount != 1 {
		t.Fatalf("expected exactly one request, got %d", requestCount)
	}
}

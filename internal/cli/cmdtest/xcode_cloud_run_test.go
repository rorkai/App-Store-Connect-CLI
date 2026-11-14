package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestXcodeCloudRunWithPullRequestIDPostsExpectedPayload(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/ciBuildRuns").JSONBody(`{"data":{"type":"ciBuildRuns","relationships":{"workflow":{"data":{"type":"ciWorkflows","id":"wf-1"}},"pullRequest":{"data":{"type":"scmPullRequests","id":"pr-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"ciBuildRuns","id":"run-pr-1","attributes":{"number":1,"executionProgress":"PENDING","startReason":"MANUAL","createdDate":"2026-03-05T10:00:00Z"}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "run", "--workflow-id", "wf-1", "--pull-request-id", "pr-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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
	reg.Install()
	reg.Register(asctest.GET("/v1/ciBuildRuns/run-1", "fields[ciBuildRuns]=workflow", "include=workflow"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"ciBuildRuns","id":"run-1","relationships":{"workflow":{"data":{"type":"ciWorkflows","id":"wf-1"}}}}}`))
	reg.Register(asctest.POST("/v1/ciBuildRuns").JSONBody(`{"data":{"type":"ciBuildRuns","attributes":{"clean":true},"relationships":{"buildRun":{"data":{"type":"ciBuildRuns","id":"run-1"}},"workflow":{"data":{"type":"ciWorkflows","id":"wf-1"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"ciBuildRuns","id":"run-rerun-1","attributes":{"number":2,"executionProgress":"PENDING","startReason":"MANUAL","createdDate":"2026-03-05T10:00:00Z"}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "run", "--source-run-id", "run-1", "--clean"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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

func TestXcodeCloudBuildRunsGetFetchesBuildRun(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/ciBuildRuns/run-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":42,"executionProgress":"RUNNING"}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "build-runs", "view", "--id", "run-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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

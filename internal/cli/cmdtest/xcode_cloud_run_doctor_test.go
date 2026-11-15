package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestXcodeCloudRunDoctorRequiresWait(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	_, _, err := runRootWithDeps(t, nil, "xcode-cloud", "run", "--workflow-id", "wf-1", "--git-reference-id", "ref-1", "--doctor")
	if err == nil || !strings.Contains(err.Error(), "--doctor requires --wait") {
		t.Fatalf("run error = %v, want --doctor/--wait validation", err)
	}
}

func TestXcodeCloudRunWaitDoctorReturnsDiagnosticReportForFailedRun(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.POST("/v1/ciBuildRuns").JSONBody(`{"data":{"type":"ciBuildRuns","relationships":{"workflow":{"data":{"type":"ciWorkflows","id":"wf-1"}},"sourceBranchOrTag":{"data":{"type":"scmGitReferences","id":"ref-1"}}}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"executionProgress":"PENDING"}}}`))
	reg.Register(asctest.GET("/v1/ciBuildRuns/run-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":93,"executionProgress":"COMPLETE","completionStatus":"FAILED"}}}`))
	reg.Register(asctest.GET("/v1/ciBuildRuns/run-1/actions", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"ciBuildActions","id":"action-1","attributes":{"name":"Build - iOS","actionType":"BUILD","executionProgress":"COMPLETE","completionStatus":"FAILED"}}]}`))
	reg.Register(asctest.GET("/v1/ciBuildActions/action-1/issues", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"ciIssues","id":"issue-1","attributes":{"issueType":"ERROR","category":"Xcodebuild","message":"Swift compilation failed"}}]}`))
	reg.Register(asctest.GET("/v1/ciBuildActions/action-1/artifacts", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[]}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg),
		"xcode-cloud", "run",
		"--workflow-id", "wf-1",
		"--git-reference-id", "ref-1",
		"--wait",
		"--doctor",
		"--poll-interval", "1ms",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result struct {
		Run struct {
			BuildRunID       string `json:"buildRunId"`
			CompletionStatus string `json:"completionStatus"`
		} `json:"run"`
		Conclusion string `json:"conclusion"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if result.Run.BuildRunID != "run-1" || result.Run.CompletionStatus != "FAILED" {
		t.Fatalf("unexpected run: %+v", result.Run)
	}
	if !strings.Contains(result.Conclusion, "actionable issues") {
		t.Fatalf("unexpected conclusion %q", result.Conclusion)
	}
	requests := make(map[string]int)
	for _, req := range reg.Requests() {
		requests[req.Method+" "+req.URL.Path]++
	}
	for _, request := range []string{
		"POST /v1/ciBuildRuns",
		"GET /v1/ciBuildRuns/run-1",
		"GET /v1/ciBuildRuns/run-1/actions",
		"GET /v1/ciBuildActions/action-1/issues",
		"GET /v1/ciBuildActions/action-1/artifacts",
	} {
		if requests[request] != 1 {
			t.Fatalf("request count for %s = %d, want 1", request, requests[request])
		}
	}
}

func TestXcodeCloudRunWaitWithoutDoctorKeepsFailureExit(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.POST("/v1/ciBuildRuns").JSONBody(`{"data":{"type":"ciBuildRuns","relationships":{"workflow":{"data":{"type":"ciWorkflows","id":"wf-1"}},"sourceBranchOrTag":{"data":{"type":"scmGitReferences","id":"ref-1"}}}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"executionProgress":"PENDING"}}}`))
	reg.Register(asctest.GET("/v1/ciBuildRuns/run-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"executionProgress":"COMPLETE","completionStatus":"FAILED"}}}`))

	stdout, _, runErr := runRootWithDeps(t, stubClient(t, reg),
		"xcode-cloud", "run",
		"--workflow-id", "wf-1",
		"--git-reference-id", "ref-1",
		"--wait",
		"--poll-interval", "1ms",
	)
	if runErr == nil || !strings.Contains(runErr.Error(), "completed with status: FAILED") {
		t.Fatalf("run error = %v, want failed-run exit", runErr)
	}
	if !strings.Contains(stdout, `"completionStatus":"FAILED"`) {
		t.Fatalf("legacy wait output changed: %q", stdout)
	}
}

package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestXcodeCloudStatusRunIDReturnsJSONWithoutWarning(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/ciBuildRuns/run-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":42,"executionProgress":"COMPLETE","completionStatus":"SUCCEEDED"}}}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "xcode-cloud", "status", "--run-id", "run-1", "--output", "json")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	var result struct {
		BuildRunID string `json:"buildRunId"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if result.BuildRunID != "run-1" {
		t.Fatalf("buildRunId = %q, want run-1", result.BuildRunID)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
}

func TestXcodeCloudScmRepositoriesRelationshipsIsUnknownCommand(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "group", args: []string{"xcode-cloud", "scm", "repositories", "relationships"}},
		{name: "git-references", args: []string{"xcode-cloud", "scm", "repositories", "relationships", "git-references", "--repo-id", "REPO_ID"}},
		{name: "pull-requests", args: []string{"xcode-cloud", "scm", "repositories", "relationships", "pull-requests", "--repo-id", "REPO_ID"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := runWithDeps(t, nil, tt.args...)
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitUsage, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "relationships") {
				t.Fatalf("stderr = %q, want unknown-command diagnostic naming relationships", stderr)
			}
			if strings.Contains(stderr, "deprecated") || strings.Contains(stderr, "Warning") {
				t.Fatalf("stderr still treats relationships as a deprecated alias: %q", stderr)
			}
		})
	}
}

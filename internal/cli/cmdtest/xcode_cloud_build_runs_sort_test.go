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

func TestXcodeCloudBuildRunsListWithSortPassesQuery(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/ciWorkflows/wf-1/buildRuns", "limit=2", "sort=-number"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"ciBuildRuns","id":"run-1","attributes":{"number":42}}]}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "build-runs", "--workflow-id", "wf-1", "--sort", "-number", "--limit", "2"}); err != nil {
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

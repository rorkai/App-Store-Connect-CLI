package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildSelectorLatestExcludeExpiredFiltersResolvedBuild(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[expired]=false", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-active","attributes":{"version":"42","uploadedDate":"2026-05-31T00:00:00Z"}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-active/app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"123456789","attributes":{"name":"Example"}}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "app", "view", "--app", "123456789", "--latest", "--exclude-expired"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"123456789"`) {
		t.Fatalf("expected app output, got %q", stdout)
	}
	if requestCount := len(reg.Requests()); requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}
}

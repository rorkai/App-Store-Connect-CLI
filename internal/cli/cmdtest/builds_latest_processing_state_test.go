package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsInfoLatestProcessingStateFilterUsesTopLevelBuildsEndpoint(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[processingState]=PROCESSING,VALID", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-processing","attributes":{"processingState":"PROCESSING"}}]}`))
	reg.Register(asctest.GET("/v1/builds/build-processing/preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"prv-1"}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "info", "--app", "123456789", "--latest", "--processing-state", "processing,valid"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-processing"`) {
		t.Fatalf("expected build output, got %q", stdout)
	}
}

func TestBuildsInfoLatestProcessingStateAllExpandsToAllStates(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[processingState]=PROCESSING,FAILED,INVALID,VALID", "limit=200", "sort=-uploadedDate"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-any-state"}]}`))
	reg.Register(asctest.GET("/v1/builds/build-any-state/preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"prv-1"}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "info", "--app", "123456789", "--latest", "--processing-state", "all"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-any-state"`) {
		t.Fatalf("expected build output, got %q", stdout)
	}
}

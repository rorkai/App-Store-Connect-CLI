package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsListProcessingStateFilterUsesTopLevelBuildsEndpoint(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[processingState]=PROCESSING", "include=preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-processing","attributes":{"processingState":"PROCESSING","version":"42"}}]}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "list", "--app", "123456789", "--processing-state", "processing"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-processing"`) {
		t.Fatalf("expected build output, got %q", stdout)
	}
	if requestCount := len(reg.Requests()); requestCount != 1 {
		t.Fatalf("expected 1 request, got %d", requestCount)
	}
}

func TestBuildsListProcessingStateAllExpandsToAllStates(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[processingState]=PROCESSING,FAILED,INVALID,VALID", "include=preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-any-state"}]}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "list", "--app", "123456789", "--processing-state", "all"})
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

func TestBuildsListExcludeExpiredSendsExpiredFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[expired]=false", "include=preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-active","attributes":{"expired":false,"version":"42"}}]}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "list", "--app", "123456789", "--exclude-expired"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-active"`) {
		t.Fatalf("expected build output, got %q", stdout)
	}
}

func TestBuildsCountNotExpiredSendsExpiredFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[expired]=false", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[],"meta":{"paging":{"total":7,"limit":1}}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "count", "--app", "123456789", "--not-expired"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"total":7`) {
		t.Fatalf("expected total output, got %q", stdout)
	}
}

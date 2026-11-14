package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsTestNotesUpdateByBuildLocaleNotFound(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds/build-1"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"VALID"}}}`))
	reg.Register(asctest.GET("/v1/betaBuildLocalizations", "filter[build]=build-1", "filter[locale]=en-US", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))

	stdout, _, runErr := runCommand(t, []string{
		"builds", "test-notes", "update",
		"--build-id", "build-1",
		"--locale", "en-US",
		"--whats-new", "Updated notes",
	})

	if runErr == nil {
		t.Fatal("expected localization lookup error, got nil")
	}
	if !strings.Contains(runErr.Error(), `no localization found for build "build-1" and locale "en-US"`) {
		t.Fatalf("expected not-found localization error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on lookup failure, got %q", stdout)
	}
}

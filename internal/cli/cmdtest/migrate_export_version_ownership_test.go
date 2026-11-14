package cmdtest

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// TestMigrateExportRejectsVersionOwnedByAnotherAppBeforeOutput proves that an
// explicit --version-id is checked against --app before migrate export creates
// its output directory or fetches any metadata to write.
func TestMigrateExportRejectsVersionOwnedByAnotherAppBeforeOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	outputDir := filepath.Join(t.TempDir(), "fastlane")
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_B", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_B","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_B"}}}}}`))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "export",
			"--app", "APP_A",
			"--version-id", "VERSION_B",
			"--output-dir", outputDir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected migrate export to fail for a version owned by another app")
	}
	if !strings.Contains(runErr.Error(), "belongs to app") {
		t.Fatalf("expected ownership error, got %v", runErr)
	}
	localizationRequests := 0
	for _, req := range reg.Requests() {
		if req.URL.Path == "/v1/appStoreVersions/VERSION_B/appStoreVersionLocalizations" {
			localizationRequests++
		}
	}
	if localizationRequests != 0 {
		t.Fatalf("expected ownership rejection before localization fetch, got %d requests", localizationRequests)
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("expected no output directory side effect, stat error = %v", err)
	}
}

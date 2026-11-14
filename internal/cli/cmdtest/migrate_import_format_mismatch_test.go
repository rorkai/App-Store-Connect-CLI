package cmdtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestMigrateImportRejectsFormatExtensionMismatchBeforeAnyMutation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	fastlaneDir := filepath.Join(t.TempDir(), "fastlane")
	metadataDir := filepath.Join(fastlaneDir, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")

	screenshotsDir := filepath.Join(fastlaneDir, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writeCmdtestScreenshotJPEG(t, screenshotsDir, "iphone_65_screen.png")

	// Discovery runs before the client exists, so a mismatch must not create
	// the localization or the screenshot set first.
	reg := asctest.NewRegistry(t)
	reg.Install()

	stdout, _, runErr := runMigrateImportWithOptions(t, fastlaneDir)

	if runErr == nil {
		t.Fatal("expected migrate import to fail for JPEG data named .png")
	}
	if got := len(reg.Requests()); got != 0 {
		t.Fatalf("expected zero requests, got %d", got)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{"iphone_65_screen.png", "JPEG", "iphone_65_screen.jpg", "PNG"} {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected error to mention %q, got %v", want, runErr)
		}
	}
}

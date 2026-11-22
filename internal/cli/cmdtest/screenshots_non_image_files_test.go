package cmdtest

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// writeScreenshotSourceFile writes a non-image file of the kind screenshot
// generators leave next to their PNGs, such as the HTML a mockup renders from.
func writeScreenshotSourceFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("<html></html>"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// A screenshot directory often holds the HTML each PNG was rendered from.
// validate and upload select the .png/.jpg/.jpeg files, say which files they
// skipped, and only fail when the directory holds no screenshot at all.
func TestScreenshotsDirectorySkipsNonImageFiles(t *testing.T) {
	setupAuth(t)

	t.Run("validate reports skipped files as warnings", func(t *testing.T) {
		dir := t.TempDir()
		writePNG(t, filepath.Join(dir, "01-home.png"), 1242, 2688)
		writeScreenshotSourceFile(t, dir, "01-home.html")

		stdout, stderr, runErr := runRootWithDeps(t, nil, "screenshots", "validate", "--path", dir, "--device-type", "IPHONE_65", "--output", "json")
		if runErr != nil {
			t.Fatalf("validate error: %v; stderr=%s", runErr, stderr)
		}
		var result screenshotValidateOutput
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("decode output: %v; stdout=%s", err, stdout)
		}
		if result.TotalFiles != 1 || result.ReadyFiles != 1 || result.ErrorCount != 0 || result.WarningCount != 1 {
			t.Fatalf("counts = total %d ready %d errors %d warnings %d, want 1/1/0/1", result.TotalFiles, result.ReadyFiles, result.ErrorCount, result.WarningCount)
		}
		if !hasScreenshotValidateIssue(result.Issues, "skipped_file", "warning", "01-home.html") {
			t.Fatalf("expected skipped_file warning for 01-home.html, got %+v", result.Issues)
		}
	})

	t.Run("upload previews only the images and warns", func(t *testing.T) {
		dir := t.TempDir()
		writePNG(t, filepath.Join(dir, "01-home.png"), 1242, 2688)
		writeScreenshotSourceFile(t, dir, "01-home.html")

		reg := asctest.NewRegistry(t)
		reg.Register(asctest.GET("/v1/appStoreVersionLocalizations/LOC/appScreenshotSets"),
			asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))

		stdout, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "screenshots", "upload", "--version-localization", "LOC", "--path", dir, "--device-type", "IPHONE_65", "--dry-run", "--output", "json")
		if runErr != nil {
			t.Fatalf("upload error: %v; stderr=%s", runErr, stderr)
		}
		if !strings.Contains(stdout, `"fileName":"01-home.png"`) || strings.Contains(stdout, "01-home.html") {
			t.Fatalf("expected only 01-home.png in the preview, got %s", stdout)
		}
		wantWarning := `Warning: screenshots upload: skipped 1 file(s) in ` + `"` + dir + `"` + ` that are not .png, .jpg, or .jpeg screenshots: 01-home.html`
		if !strings.Contains(stderr, wantWarning) {
			t.Fatalf("expected skipped-file warning %q, got %q", wantWarning, stderr)
		}
	})

	t.Run("upload of a directory without images fails before any request", func(t *testing.T) {
		dir := t.TempDir()
		writeScreenshotSourceFile(t, dir, "01-home.html")

		reg := asctest.NewRegistry(t)
		_, stderr, runErr := runRootWithDeps(t, stubClient(t, reg), "screenshots", "upload", "--version-localization", "LOC", "--path", dir, "--device-type", "IPHONE_65")
		want := `no .png, .jpg, or .jpeg screenshots found in "` + dir + `" (skipped: 01-home.html)`
		if runErr == nil || !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected %q, got %v; stderr=%s", want, runErr, stderr)
		}
		if calls := len(reg.Requests()); calls != 0 {
			t.Fatalf("made %d network calls for a directory without screenshots", calls)
		}
	})
}

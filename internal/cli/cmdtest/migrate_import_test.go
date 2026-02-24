package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/migrate"
)

func TestMigrateImportDryRunPlan(t *testing.T) {
	root := t.TempDir()
	metadataDir := filepath.Join(root, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")
	writeFile(t, filepath.Join(metadataDir, "name.txt"), "App Name")
	writeFile(t, filepath.Join(metadataDir, "privacy_url.txt"), "https://example.com/privacy")

	reviewDir := filepath.Join(root, "metadata", "review_information")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review_information: %v", err)
	}
	writeFile(t, filepath.Join(reviewDir, "first_name.txt"), "Rita")
	writeFile(t, filepath.Join(reviewDir, "email_address.txt"), "rita@example.com")
	writeFile(t, filepath.Join(reviewDir, "demo_required.txt"), "false")

	screenshotsDir := filepath.Join(root, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_screen.png"), 1242, 2688)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.DryRun {
		t.Fatalf("expected dry run true")
	}
	if result.VersionID != "VERSION_ID" {
		t.Fatalf("expected version id VERSION_ID, got %q", result.VersionID)
	}
	if len(result.ScreenshotPlan) != 1 {
		t.Fatalf("expected 1 screenshot plan, got %d", len(result.ScreenshotPlan))
	}
	if result.ReviewInformation == nil || result.ReviewInformation.ContactFirstName == nil {
		t.Fatalf("expected review info to be included")
	}
	if len(result.MetadataFiles) == 0 {
		t.Fatalf("expected metadata files plan")
	}
}

func TestMigrateImportDryRunReportsSkippedNonLocaleMetadataDirs(t *testing.T) {
	root := t.TempDir()
	metadataDir := filepath.Join(root, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")

	// Mimic a common fastlane deliver subdirectory that is not a locale.
	nonLocaleDir := filepath.Join(root, "metadata", "trade_representative_contact_information")
	if err := os.MkdirAll(nonLocaleDir, 0o755); err != nil {
		t.Fatalf("mkdir non-locale dir: %v", err)
	}
	writeFile(t, filepath.Join(nonLocaleDir, "first_name.txt"), "Rita")
	nonLocaleDirResolved, err := filepath.EvalSymlinks(nonLocaleDir)
	if err != nil {
		t.Fatalf("eval symlinks non-locale dir: %v", err)
	}

	// Ensure default screenshots directory exists so it isn't reported as skipped.
	if err := os.MkdirAll(filepath.Join(root, "screenshots"), 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	found := false
	for _, item := range result.Skipped {
		itemPathResolved, err := filepath.EvalSymlinks(item.Path)
		if err != nil {
			itemPathResolved = item.Path
		}
		if itemPathResolved == nonLocaleDirResolved && strings.Contains(item.Reason, "skipped non-locale directory") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected skipped to include %q (got %+v)", nonLocaleDir, result.Skipped)
	}
}

func TestMigrateImportDryRunSupportsIPhone69AliasAsAppIPhone67(t *testing.T) {
	root := t.TempDir()
	metadataDir := filepath.Join(root, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")

	screenshotsDir := filepath.Join(root, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_69_screen.png"), 1320, 2868)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.ScreenshotPlan) != 1 {
		t.Fatalf("expected 1 screenshot plan, got %d", len(result.ScreenshotPlan))
	}
	if result.ScreenshotPlan[0].DisplayType != "APP_IPHONE_69" {
		t.Fatalf("expected APP_IPHONE_69, got %q", result.ScreenshotPlan[0].DisplayType)
	}
}

func TestMigrateImportUploadsAndSkipsExistingScreenshots(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	metadataDir := filepath.Join(fastlaneDir, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")
	writeFile(t, filepath.Join(metadataDir, "name.txt"), "App Name")

	reviewDir := filepath.Join(fastlaneDir, "metadata", "review_information")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review_information: %v", err)
	}
	writeFile(t, filepath.Join(reviewDir, "first_name.txt"), "Rita")
	writeFile(t, filepath.Join(reviewDir, "email_address.txt"), "rita@example.com")

	screenshotsDir := filepath.Join(fastlaneDir, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_existing.png"), 1242, 2688)
	writePNGForMigrate(t, filepath.Join(screenshotsDir, "iphone_65_new.png"), 1242, 2688)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requestedUploads := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "upload.example.com" {
			requestedUploads++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
			}, nil
		}

		switch {
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appStoreVersions/") && strings.HasSuffix(req.URL.Path, "/appStoreVersionLocalizations"):
			body := `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`
			return migrateJSONResponse(http.StatusOK, body), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-1":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_ID/appInfos":
			body := `{"data":[{"type":"appInfos","id":"appinfo-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`
			return migrateJSONResponse(http.StatusOK, body), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appInfos/appinfo-1/appInfoLocalizations":
			return migrateJSONResponse(http.StatusOK, `{"data":[]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appInfoLocalizations":
			return migrateJSONResponse(http.StatusCreated, `{"data":{"type":"appInfoLocalizations","id":"appinfo-loc-1","attributes":{"locale":"en-US"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_ID/appStoreReviewDetail":
			return migrateJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","title":"not found"}]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreReviewDetails":
			return migrateJSONResponse(http.StatusCreated, `{"data":{"type":"appStoreReviewDetails","id":"review-1"}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets":
			return migrateJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			body := `{"data":[{"type":"appScreenshots","id":"shot-existing","attributes":{"fileName":"iphone_65_existing.png"}}]}`
			return migrateJSONResponse(http.StatusOK, body), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			resp := `{"data":{"type":"appScreenshots","id":"shot-new","attributes":{"fileName":"iphone_65_new.png","fileSize":1234,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/upload/shot-new","length":1234,"offset":0}]}}}`
			return migrateJSONResponse(http.StatusCreated, resp), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/shot-new":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"shot-new","attributes":{"fileName":"iphone_65_new.png"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/shot-new":
			body := `{"data":{"type":"appScreenshots","id":"shot-new","attributes":{"fileName":"iphone_65_new.png","assetDeliveryState":{"state":"COMPLETE"}}}}`
			return migrateJSONResponse(http.StatusOK, body), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestedUploads != 1 {
		t.Fatalf("expected 1 upload request, got %d", requestedUploads)
	}

	var result migrate.MigrateImportResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.ScreenshotResults) != 1 {
		t.Fatalf("expected 1 screenshot result, got %d", len(result.ScreenshotResults))
	}
	if len(result.ScreenshotResults[0].Uploaded) != 1 {
		t.Fatalf("expected 1 uploaded screenshot, got %d", len(result.ScreenshotResults[0].Uploaded))
	}
	if len(result.ScreenshotResults[0].Skipped) != 1 {
		t.Fatalf("expected 1 skipped screenshot, got %d", len(result.ScreenshotResults[0].Skipped))
	}
}

func TestMigrateImportRejectsInvalidScreenshot(t *testing.T) {
	root := t.TempDir()
	metadataDir := filepath.Join(root, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")

	screenshotsDir := filepath.Join(root, "screenshots", "en-US")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	badPath := filepath.Join(screenshotsDir, "bad.png")
	writeFile(t, badPath, "not an image")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(runErr.Error(), badPath) {
		t.Fatalf("expected error to mention %q, got %v", badPath, runErr)
	}
}

func migrateJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func writePNGForMigrate(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer func() { _ = file.Close() }()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}

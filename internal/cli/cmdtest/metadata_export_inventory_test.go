package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestMetadataRepeatedAssetExportRemovesOnlyOwnedUnchangedFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("owned cleanup uses strict filesystem primitives unavailable on Windows")
	}
	setupAuth(t)
	t.Chdir(t.TempDir())
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	dir := t.TempDir()
	empty := false
	installMetadataAssetExportTransport(t, &empty)
	run := func(scopes string) string {
		t.Helper()
		var err error
		stdout, stderr := captureOutput(t, func() {
			err = RootCommand("test").ParseAndRun(context.Background(), []string{"metadata", "pull", "--app", "123456789", "--version", "1.0", "--dir", dir, "--include", scopes, "--force", "--output", "json"})
		})
		if err != nil {
			t.Fatalf("pull: %v; stdout=%s stderr=%s", err, stdout, stderr)
		}
		return stderr
	}
	run("app-clip,previews")
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(path)), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("en-US/app_clip/subtitle.txt", "Local edit")
	write("en-US/app_clip/notes.txt", "Unrelated")
	write("app_previews/en-US/iphone_65/legacy.mp4", "Unrecorded legacy media")
	empty = true
	stderr := run("app-clip")
	if !strings.Contains(strings.ToLower(stderr), "preserv") {
		t.Errorf("edited owned file needs a preservation diagnostic: %s", stderr)
	}
	for _, path := range []string{"app_clip/action.txt", "en-US/app_clip/header_image.png"} {
		if _, err := os.Stat(filepath.Join(dir, path)); !os.IsNotExist(err) {
			t.Errorf("stale owned %s remains after empty export: %v", path, err)
		}
	}
	preview := "app_previews/en-US/iphone_65/"
	if _, err := os.Stat(filepath.Join(dir, preview+"demo.mp4")); err != nil {
		t.Fatalf("unselected previews must remain: %v", err)
	}
	stderr = run("previews")
	for _, name := range []string{"demo.mp4", "demo.poster_frame.txt", "order.json"} {
		if _, err := os.Stat(filepath.Join(dir, preview+name)); !os.IsNotExist(err) {
			t.Errorf("stale owned %s remains: %v", name, err)
		}
	}
	for path, want := range map[string]string{"en-US/app_clip/subtitle.txt": "Local edit", "en-US/app_clip/notes.txt": "Unrelated", preview + "legacy.mp4": "Unrecorded legacy media"} {
		got, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil || string(got) != want {
			t.Errorf("preserved %s = %q, %v", path, got, err)
		}
	}
	if !strings.Contains(strings.ToLower(stderr), "unrecorded") {
		t.Errorf("legacy file needs a diagnostic: %s", stderr)
	}
	// Remove the explicitly preserved local subtitle to leave only empty directories
	// and unrelated notes, then prove a confirmed import does not recreate the Clip.
	if err := os.Remove(filepath.Join(dir, "en-US/app_clip/subtitle.txt")); err != nil {
		t.Fatal(err)
	}
	var pushErr error
	stdout, stderr := captureOutput(t, func() {
		pushErr = RootCommand("test").ParseAndRun(context.Background(), []string{"metadata", "push", "--app", "123456789", "--version", "1.0", "--dir", dir, "--include", "app-clip", "--confirm", "--output", "json"})
	})
	if pushErr != nil {
		t.Fatalf("empty exported tree import: %v; %s %s", pushErr, stdout, stderr)
	}
	var result struct{ Adds, Updates, Deletes, AssetResults []json.RawMessage }
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Adds)+len(result.Updates)+len(result.Deletes)+len(result.AssetResults) != 0 {
		t.Fatalf("empty tree recreated assets: %s", stdout)
	}
}

// installMetadataAssetExportTransport serves one App Clip experience and one
// preview set; *empty switches both to "nothing on App Store Connect".
func installMetadataAssetExportTransport(t *testing.T, empty *bool) {
	t.Helper()
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("unexpected mutation %s", req.Method)
		}
		switch req.URL.Path {
		case "/v1/apps/123456789/appClips":
			return migrateJSONResponse(200, `{"data":[{"type":"appClips","id":"CLIP"}]}`), nil
		case "/v1/apps/123456789/appStoreVersions":
			return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
			if *empty {
				return migrateJSONResponse(404, `{}`), nil
			}
			return migrateJSONResponse(200, `{"data":{"type":"appClipDefaultExperiences","id":"EXP","attributes":{"action":"PLAY"}}}`), nil
		case "/v1/appClipDefaultExperiences/EXP/appClipDefaultExperienceLocalizations":
			return migrateJSONResponse(200, `{"data":[{"type":"appClipDefaultExperienceLocalizations","id":"CLIP_LOC","attributes":{"locale":"en-US","subtitle":"Original subtitle"}}]}`), nil
		case "/v1/appClipDefaultExperienceLocalizations/CLIP_LOC/appClipHeaderImage":
			return migrateJSONResponse(200, `{"data":{"type":"appClipHeaderImages","id":"HEADER","attributes":{"imageAsset":{"templateUrl":"https://media.example/header.png","width":1800,"height":1200}}}}`), nil
		case "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
			if *empty {
				return migrateJSONResponse(200, `{"data":[]}`), nil
			}
			return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersionLocalizations","id":"LOC","attributes":{"locale":"en-US"}}]}`), nil
		case "/v1/appStoreVersionLocalizations/LOC/appPreviewSets":
			return migrateJSONResponse(200, `{"data":[{"type":"appPreviewSets","id":"SET","attributes":{"previewType":"IPHONE_65"}}]}`), nil
		case "/v1/appPreviewSets/SET/appPreviews":
			return migrateJSONResponse(200, `{"data":[{"type":"appPreviews","id":"PREVIEW","attributes":{"fileName":"demo.mp4","videoUrl":"https://media.example/demo.mp4","previewFrameTimeCode":"00:00:01.000"}}]}`), nil
		case "/v1/appPreviewSets/SET/relationships/appPreviews":
			return migrateJSONResponse(200, `{"data":[{"type":"appPreviews","id":"PREVIEW"}]}`), nil
		case "/v1/apps/123456789/appInfos":
			return migrateJSONResponse(200, `{"data":[{"type":"appInfos","id":"APP_INFO","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/appInfos/APP_INFO/appInfoLocalizations":
			return migrateJSONResponse(200, `{"data":[{"type":"appInfoLocalizations","id":"INFO_LOC","attributes":{"locale":"en-US","name":"App"}}]}`), nil
		case "/header.png", "/demo.mp4":
			return migrateJSONResponse(200, `delivered media`), nil
		default:
			return nil, fmt.Errorf("unexpected request %s", req.URL.Path)
		}
	}))
}

func TestMetadataRepullWithMediaRefreshesUnchangedFilesAndRefusesEdits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("owned refresh uses strict filesystem primitives unavailable on Windows")
	}
	setupAuth(t)
	t.Chdir(t.TempDir())
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	dir := t.TempDir()
	empty := false
	installMetadataAssetExportTransport(t, &empty)
	pull := func() (string, string, error) {
		t.Helper()
		var err error
		stdout, stderr := captureOutput(t, func() {
			err = RootCommand("test").ParseAndRun(context.Background(), []string{"metadata", "pull", "--app", "123456789", "--version", "1.0", "--dir", dir, "--include", "localizations,app-clip,previews", "--output", "json"})
		})
		return stdout, stderr, err
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if stdout, stderr, err := pull(); err != nil {
			t.Fatalf("pull %d without --force: %v; stdout=%s stderr=%s", attempt, err, stdout, stderr)
		}
	}

	header := filepath.Join(dir, "en-US", "app_clip", "header_image.png")
	if err := os.WriteFile(header, []byte("local header"), 0o600); err != nil {
		t.Fatal(err)
	}
	localization := filepath.Join(dir, "app-info", "en-US.json")
	if err := os.Remove(localization); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := pull()
	if err == nil || !shared.IsReportedUsageError(err) {
		t.Fatalf("pull over an edited header image: want reported usage error, got %v", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := "Error: refusing to overwrite 1 file that differs from App Store Connect: " + header + "\n"
	if !strings.HasPrefix(stderr, want) {
		t.Fatalf("stderr = %q, want prefix %q", stderr, want)
	}
	if got, _ := os.ReadFile(header); string(got) != "local header" {
		t.Fatalf("edited header image replaced: %q", got)
	}
	if _, err := os.Stat(localization); !os.IsNotExist(err) {
		t.Fatalf("refused pull wrote %s: %v", localization, err)
	}
}

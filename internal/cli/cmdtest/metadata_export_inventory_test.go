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
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("unexpected mutation %s", req.Method)
		}
		switch req.URL.Path {
		case "/v1/apps/APP_ID/appClips":
			return migrateJSONResponse(200, `{"data":[{"type":"appClips","id":"CLIP"}]}`), nil
		case "/v1/apps/APP_ID/appStoreVersions":
			return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "/v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
			if empty {
				return migrateJSONResponse(404, `{}`), nil
			}
			return migrateJSONResponse(200, `{"data":{"type":"appClipDefaultExperiences","id":"EXP","attributes":{"action":"PLAY"}}}`), nil
		case "/v1/appClipDefaultExperiences/EXP/appClipDefaultExperienceLocalizations":
			return migrateJSONResponse(200, `{"data":[{"type":"appClipDefaultExperienceLocalizations","id":"CLIP_LOC","attributes":{"locale":"en-US","subtitle":"Original subtitle"}}]}`), nil
		case "/v1/appClipDefaultExperienceLocalizations/CLIP_LOC/appClipHeaderImage":
			return migrateJSONResponse(200, `{"data":{"type":"appClipHeaderImages","id":"HEADER","attributes":{"imageAsset":{"templateUrl":"https://media.example/header.png","width":1800,"height":1200}}}}`), nil
		case "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
			if empty {
				return migrateJSONResponse(200, `{"data":[]}`), nil
			}
			return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersionLocalizations","id":"LOC","attributes":{"locale":"en-US"}}]}`), nil
		case "/v1/appStoreVersionLocalizations/LOC/appPreviewSets":
			return migrateJSONResponse(200, `{"data":[{"type":"appPreviewSets","id":"SET","attributes":{"previewType":"IPHONE_65"}}]}`), nil
		case "/v1/appPreviewSets/SET/appPreviews":
			return migrateJSONResponse(200, `{"data":[{"type":"appPreviews","id":"PREVIEW","attributes":{"fileName":"demo.mp4","videoUrl":"https://media.example/demo.mp4","previewFrameTimeCode":"00:00:01.000"}}]}`), nil
		case "/v1/appPreviewSets/SET/relationships/appPreviews":
			return migrateJSONResponse(200, `{"data":[{"type":"appPreviews","id":"PREVIEW"}]}`), nil
		case "/header.png", "/demo.mp4":
			return migrateJSONResponse(200, `delivered media`), nil
		default:
			return nil, fmt.Errorf("unexpected request %s", req.URL.Path)
		}
	}))
	run := func(scopes string) string {
		t.Helper()
		var err error
		stdout, stderr := captureOutput(t, func() {
			err = RootCommand("test").ParseAndRun(context.Background(), []string{"metadata", "pull", "--app", "APP_ID", "--version", "1.0", "--dir", dir, "--include", scopes, "--force", "--output", "json"})
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
		pushErr = RootCommand("test").ParseAndRun(context.Background(), []string{"metadata", "push", "--app", "APP_ID", "--version", "1.0", "--dir", dir, "--include", "app-clip", "--confirm", "--output", "json"})
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

package cmdtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMigrateImportAppliesAppClipAction(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "metadata", "app_clip"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "metadata", "app_clip", "action.txt"), "PLAY")
	applied := false
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method + " " + req.URL.Path {
		case "GET /v1/appStoreVersions/VERSION_ID":
			return migrateJSONResponse(200, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`), nil
		case "GET /v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
			return migrateJSONResponse(200, `{"data":[]}`), nil
		case "GET /v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
			return migrateJSONResponse(200, `{"data":{"type":"appClipDefaultExperiences","id":"CLIP_EXPERIENCE","attributes":{"action":"OPEN"}}}`), nil
		case "GET /v1/appClipDefaultExperiences/CLIP_EXPERIENCE/appClipDefaultExperienceLocalizations":
			return migrateJSONResponse(200, `{"data":[]}`), nil
		case "PATCH /v1/appClipDefaultExperiences/CLIP_EXPERIENCE":
			body, _ := io.ReadAll(req.Body)
			var payload struct {
				Data struct {
					Attributes struct {
						Action string `json:"action"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &payload); err != nil || payload.Data.Attributes.Action != "PLAY" {
				t.Errorf("action payload = %s", body)
			}
			applied = true
			return migrateJSONResponse(200, `{"data":{"type":"appClipDefaultExperiences","id":"CLIP_EXPERIENCE","attributes":{"action":"PLAY"}}}`), nil
		default:
			return nil, fmt.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
		}
	}))
	cmd := RootCommand("test")
	var runErr error
	captureOutput(t, func() {
		runErr = cmd.ParseAndRun(context.Background(), []string{"migrate", "import", "--app", "APP_ID", "--version-id", "VERSION_ID", "--fastlane-dir", root, "--skip-screenshots", "--confirm", "--output", "json"})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if !applied {
		t.Fatal("confirmed import succeeded without applying App Clip action")
	}
}

func TestMigrateExportFetchesAppClipMetadata(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	root := t.TempDir()
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method + " " + req.URL.Path {
		case "GET /v1/appStoreVersions/VERSION_ID":
			return migrateJSONResponse(200, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`), nil
		case "GET /v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "GET /v1/apps/APP_ID/appInfos":
			return migrateJSONResponse(200, `{"data":[]}`), nil
		case "GET /v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
			return migrateJSONResponse(200, `{"data":{"type":"appClipDefaultExperiences","id":"CLIP_EXPERIENCE","attributes":{"action":"PLAY"}}}`), nil
		case "GET /v1/appClipDefaultExperiences/CLIP_EXPERIENCE/appClipDefaultExperienceLocalizations":
			return migrateJSONResponse(200, `{"data":[{"type":"appClipDefaultExperienceLocalizations","id":"CLIP_LOC","attributes":{"locale":"en-US","subtitle":"Play now"}}]}`), nil
		case "GET /v1/appClipDefaultExperienceLocalizations/CLIP_LOC/appClipHeaderImage":
			return migrateJSONResponse(200, `{"data":null}`), nil
		default:
			return nil, fmt.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
		}
	}))
	cmd := RootCommand("test")
	var runErr error
	captureOutput(t, func() {
		runErr = cmd.ParseAndRun(context.Background(), []string{"migrate", "export", "--app", "APP_ID", "--version-id", "VERSION_ID", "--output-dir", root, "--output", "json"})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	for path, want := range map[string]string{"metadata/app_clip/action.txt": "PLAY\n", "metadata/en-US/app_clip/subtitle.txt": "Play now\n"} {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || string(got) != want {
			t.Errorf("export %s = %q, %v; want %q", path, got, err, want)
		}
	}
}

func TestMetadataPullExportsAppClip(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	root := t.TempDir()
	installStoreAssetMetadataFixture(t)
	cmd := RootCommand("test")
	var runErr error
	captureOutput(t, func() {
		runErr = cmd.ParseAndRun(context.Background(), []string{"metadata", "pull", "--app", "APP_ID", "--version", "1.0", "--dir", root, "--include", "localizations,app-clip", "--output", "json"})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	got, err := os.ReadFile(filepath.Join(root, "app_clip", "action.txt"))
	if err != nil || string(got) != "OPEN\n" {
		t.Fatalf("action=%q err=%v", got, err)
	}
}

func TestMetadataPushAppliesAppClipAction(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	root := t.TempDir()
	for _, dir := range []string{"app_clip", "app-info", "version/1.0"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(root, "app_clip", "action.txt"), "PLAY")
	applied := installStoreAssetMetadataFixture(t)
	cmd := RootCommand("test")
	var runErr error
	captureOutput(t, func() {
		runErr = cmd.ParseAndRun(context.Background(), []string{"metadata", "push", "--app", "APP_ID", "--version", "1.0", "--dir", root, "--include", "localizations,app-clip", "--confirm", "--output", "json"})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if !*applied {
		t.Fatal("metadata push ignored App Clip action")
	}
}

func installStoreAssetMetadataFixture(t *testing.T) *bool {
	t.Helper()
	applied := false
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method + " " + req.URL.Path {
		case "GET /v1/apps/APP_ID/appInfos":
			return migrateJSONResponse(200, `{"data":[{"type":"appInfos","id":"INFO_ID","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "GET /v1/apps/APP_ID/appStoreVersions":
			return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`), nil
		case "GET /v1/appStoreVersions/VERSION_ID":
			return migrateJSONResponse(200, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`), nil
		case "GET /v1/appInfos/INFO_ID/appInfoLocalizations", "GET /v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "GET /v1/appClipDefaultExperiences/CLIP_EXPERIENCE/appClipDefaultExperienceLocalizations":
			return migrateJSONResponse(200, `{"data":[]}`), nil
		case "GET /v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
			return migrateJSONResponse(200, `{"data":{"type":"appClipDefaultExperiences","id":"CLIP_EXPERIENCE","attributes":{"action":"OPEN"}}}`), nil
		case "PATCH /v1/appClipDefaultExperiences/CLIP_EXPERIENCE":
			applied = true
			return migrateJSONResponse(200, `{"data":{"type":"appClipDefaultExperiences","id":"CLIP_EXPERIENCE","attributes":{"action":"PLAY"}}}`), nil
		default:
			return nil, fmt.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
		}
	}))
	return &applied
}

func TestMigrateValidateRejectsInvalidAppClipHeader(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "metadata", "en-US", "app_clip")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePNGForMigrate(t, filepath.Join(dir, "header_image.png"), 1, 1)
	cmd := RootCommand("test")
	var runErr error
	captureOutput(t, func() {
		runErr = cmd.ParseAndRun(context.Background(), []string{"migrate", "validate", "--fastlane-dir", root, "--output", "json"})
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "1800 x 1200") {
		t.Fatalf("expected header dimensions error; got %v", runErr)
	}
}

func TestMigrateValidateRejectsInvalidPreviewDuration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	root := t.TempDir()
	for _, dir := range []string{"metadata/en-US", "app_previews/en-US/iphone_65"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(root, "app_previews/en-US/iphone_65/preview.mp4"), "fixture-video")
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ffprobe"), []byte("#!/bin/sh\nprintf '%s' '{\"streams\":[{\"width\":886,\"height\":1920}],\"format\":{\"duration\":\"2.0\"}}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	cmd := RootCommand("test")
	var runErr error
	captureOutput(t, func() {
		runErr = cmd.ParseAndRun(context.Background(), []string{"migrate", "validate", "--fastlane-dir", root, "--output", "json"})
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "between 15 and 30") {
		t.Fatalf("expected duration error; got %v", runErr)
	}
}

// The fixture exercises the public export -> import path, including download,
// reservation, transfer, commit, poster frame, and remote order reconciliation.
func TestMigrateStoreAssetsProductionRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	root := t.TempDir()
	headerFile := filepath.Join(t.TempDir(), "header.png")
	writePNGForMigrate(t, headerFile, 1800, 1200)
	header, err := os.ReadFile(headerFile)
	if err != nil {
		t.Fatal(err)
	}
	video := []byte("fixture-video")
	installValidStoreAssetProbe(t)
	phase := "export"
	uploaded := map[string][]byte{}
	posterApplied := false
	headerCommitted := false
	previewCommitted := false
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		key := req.Method + " " + req.URL.Path
		switch key {
		case "GET /v1/appStoreVersions/VERSION_ID":
			return migrateJSONResponse(200, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`), nil
		case "GET /v1/apps/APP_ID/appInfos":
			return migrateJSONResponse(200, `{"data":[]}`), nil
		case "GET /v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
			return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersionLocalizations","id":"LOC","attributes":{"locale":"en-US"}}]}`), nil
		case "GET /v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
			return migrateJSONResponse(200, `{"data":{"type":"appClipDefaultExperiences","id":"EXP","attributes":{"action":"PLAY"}}}`), nil
		case "GET /v1/appClipDefaultExperiences/EXP/appClipDefaultExperienceLocalizations":
			return migrateJSONResponse(200, `{"data":[{"type":"appClipDefaultExperienceLocalizations","id":"CLIP_LOC","attributes":{"locale":"en-US","subtitle":"Play now"}}]}`), nil
		case "GET /v1/appClipDefaultExperienceLocalizations/CLIP_LOC/appClipHeaderImage":
			if phase == "import" {
				return migrateJSONResponse(200, `{"data":null}`), nil
			}
			return migrateJSONResponse(200, `{"data":{"type":"appClipHeaderImages","id":"HEADER","attributes":{"imageAsset":{"templateUrl":"https://media.example/header.png","width":1800,"height":1200}}}}`), nil
		case "GET /v1/appStoreVersionLocalizations/LOC/appPreviewSets":
			return migrateJSONResponse(200, `{"data":[{"type":"appPreviewSets","id":"SET","attributes":{"previewType":"IPHONE_65"}}]}`), nil
		case "GET /v1/appPreviewSets/SET/appPreviews":
			if phase == "import" {
				return migrateJSONResponse(200, `{"data":[]}`), nil
			}
			return migrateJSONResponse(200, `{"data":[{"type":"appPreviews","id":"VIDEO","attributes":{"fileName":"preview.mp4","videoUrl":"https://media.example/preview.mp4","previewFrameTimeCode":"00:00:01.000"}}]}`), nil
		case "GET /v1/appPreviewSets/SET/relationships/appPreviews":
			if phase == "import" && !previewCommitted {
				return migrateJSONResponse(200, `{"data":[]}`), nil
			}
			return migrateJSONResponse(200, `{"data":[{"type":"appPreviews","id":"VIDEO"}]}`), nil
		case "GET /header.png":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(header)), Header: http.Header{}}, nil
		case "GET /preview.mp4":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(video)), Header: http.Header{}}, nil
		case "POST /v1/appClipHeaderImages":
			return migrateJSONResponse(201, fmt.Sprintf(`{"data":{"type":"appClipHeaderImages","id":"HEADER","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/header","length":%d,"offset":0}]}}}`, len(header))), nil
		case "POST /v1/appPreviews":
			return migrateJSONResponse(201, fmt.Sprintf(`{"data":{"type":"appPreviews","id":"VIDEO","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/video","length":%d,"offset":0}]}}}`, len(video))), nil
		case "PUT /header", "PUT /video":
			uploaded[req.URL.Path], err = io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			return migrateJSONResponse(200, `{}`), nil
		case "PATCH /v1/appClipHeaderImages/HEADER":
			headerCommitted = true
			return migrateJSONResponse(200, `{"data":{"type":"appClipHeaderImages","id":"HEADER"}}`), nil
		case "PATCH /v1/appPreviews/VIDEO":
			body, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				return nil, readErr
			}
			if strings.Contains(string(body), "00:00:01.000") {
				posterApplied = true
			} else {
				previewCommitted = true
			}
			return migrateJSONResponse(200, `{"data":{"type":"appPreviews","id":"VIDEO","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`), nil
		case "GET /v1/appPreviews/VIDEO":
			return migrateJSONResponse(200, `{"data":{"type":"appPreviews","id":"VIDEO","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`), nil
		default:
			return nil, fmt.Errorf("unexpected %s during %s", key, phase)
		}
	}))
	var runErr error
	captureOutput(t, func() {
		runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"migrate", "export", "--app", "APP_ID", "--version-id", "VERSION_ID", "--output-dir", root, "--output", "json"})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	for path, want := range map[string][]byte{"metadata/en-US/app_clip/header_image.png": header, "app_previews/en-US/iphone_65/preview.mp4": video} {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("download %s: %v, byte equality=%v", path, err, bytes.Equal(got, want))
		}
	}
	phase = "import"
	stdout, _ := captureOutput(t, func() {
		runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"migrate", "import", "--app", "APP_ID", "--version-id", "VERSION_ID", "--fastlane-dir", root, "--skip-screenshots", "--confirm", "--output", "json"})
	})
	if runErr != nil {
		t.Fatalf("import: %v; receipt=%s", runErr, stdout)
	}
	if !bytes.Equal(uploaded["/header"], header) || !bytes.Equal(uploaded["/video"], video) {
		t.Fatal("uploaded bytes differ from exported bytes")
	}
	if !headerCommitted || !previewCommitted || !posterApplied {
		t.Fatalf("header=%v preview=%v poster=%v", headerCommitted, previewCommitted, posterApplied)
	}
}

func installValidStoreAssetProbe(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "ffprobe"), []byte("#!/bin/sh\nprintf '%s' '{\"streams\":[{\"width\":886,\"height\":1920}],\"format\":{\"duration\":\"20.0\"}}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
}

func TestMetadataAssetsOnlyNeverMutatesLocalizations(t *testing.T) {
	for _, allowDeletes := range []bool{false, true} {
		t.Run(fmt.Sprint(allowDeletes), func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "app_clip"), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "app_clip/action.txt"), "PLAY")
			applied := false
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.Method + " " + req.URL.Path {
				case "GET /v1/apps/APP_ID/appStoreVersions":
					return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`), nil
				case "GET /v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
					return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersionLocalizations","id":"LOC","attributes":{"locale":"en-US","description":"keep"}}]}`), nil
				case "GET /v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
					return migrateJSONResponse(200, `{"data":{"type":"appClipDefaultExperiences","id":"EXP","attributes":{"action":"OPEN"}}}`), nil
				case "GET /v1/appClipDefaultExperiences/EXP/appClipDefaultExperienceLocalizations":
					return migrateJSONResponse(200, `{"data":[]}`), nil
				case "PATCH /v1/appClipDefaultExperiences/EXP":
					applied = true
					return migrateJSONResponse(200, `{"data":{"type":"appClipDefaultExperiences","id":"EXP","attributes":{"action":"PLAY"}}}`), nil
				default:
					t.Errorf("out-of-scope request %s %s", req.Method, req.URL.Path)
					return nil, fmt.Errorf("unexpected request")
				}
			}))
			args := []string{"metadata", "push", "--app", "APP_ID", "--version", "1.0", "--dir", root, "--include", "app-clip", "--confirm", "--output", "json"}
			if allowDeletes {
				args = append(args, "--allow-deletes")
			}
			var runErr error
			captureOutput(t, func() { runErr = RootCommand("test").ParseAndRun(context.Background(), args) })
			if runErr != nil || !applied {
				t.Fatalf("err=%v action applied=%v", runErr, applied)
			}
		})
	}
}

func TestMigrateHeaderReplacementFailureKeepsDeletionReceipt(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	root := t.TempDir()
	dir := filepath.Join(root, "metadata/en-US/app_clip")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePNGForMigrate(t, filepath.Join(dir, "header_image.png"), 1800, 1200)
	deleted := false
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method + " " + req.URL.Path {
		case "GET /v1/appStoreVersions/VERSION_ID":
			return migrateJSONResponse(200, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`), nil
		case "GET /v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
			return migrateJSONResponse(200, `{"data":[]}`), nil
		case "GET /v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
			return migrateJSONResponse(200, `{"data":{"type":"appClipDefaultExperiences","id":"EXP","attributes":{"action":"OPEN"}}}`), nil
		case "GET /v1/appClipDefaultExperiences/EXP/appClipDefaultExperienceLocalizations":
			return migrateJSONResponse(200, `{"data":[{"type":"appClipDefaultExperienceLocalizations","id":"LOC","attributes":{"locale":"en-US"}}]}`), nil
		case "GET /v1/appClipDefaultExperienceLocalizations/LOC/appClipHeaderImage":
			return migrateJSONResponse(200, `{"data":{"type":"appClipHeaderImages","id":"OLD_HEADER","attributes":{"sourceFileChecksum":"old"}}}`), nil
		case "DELETE /v1/appClipHeaderImages/OLD_HEADER":
			deleted = true
			return migrateJSONResponse(204, ``), nil
		case "POST /v1/appClipHeaderImages":
			return migrateJSONResponse(400, `{"errors":[{"status":"400","code":"INVALID","title":"reservation rejected"}]}`), nil
		default:
			return nil, fmt.Errorf("unexpected %s %s", req.Method, req.URL.Path)
		}
	}))
	var runErr error
	stdout, _ := captureOutput(t, func() {
		runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"migrate", "import", "--app", "APP_ID", "--version-id", "VERSION_ID", "--fastlane-dir", root, "--skip-screenshots", "--confirm", "--output", "json"})
	})
	if runErr == nil || !deleted {
		t.Fatalf("err=%v deleted=%v", runErr, deleted)
	}
	var receipt struct {
		Status       string `json:"status"`
		AssetResults []struct {
			PreviousID      string `json:"previousId"`
			PreviousDeleted bool   `json:"previousDeleted"`
			Status          string `json:"status"`
		} `json:"assetResults"`
	}
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("decode receipt: %v; %s", err, stdout)
	}
	if receipt.Status != "partial" || len(receipt.AssetResults) != 1 || receipt.AssetResults[0].PreviousID != "OLD_HEADER" || !receipt.AssetResults[0].PreviousDeleted || receipt.AssetResults[0].Status != "failed" {
		t.Fatalf("lost partial replacement receipt: %s", stdout)
	}
}

func TestMigrateSkippedAssetsAreNotReadOrUploaded(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	root := t.TempDir()
	for _, dir := range []string{"metadata/app_clip", "app_previews/en-US/iphone_65"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(root, "metadata/app_clip/action.txt"), "INVALID")
	writeFile(t, filepath.Join(root, "app_previews/en-US/iphone_65/preview.mp4"), "not a video")
	t.Setenv("PATH", t.TempDir())
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Errorf("dry-run made request %s %s", req.Method, req.URL.Path)
		return nil, fmt.Errorf("unexpected request")
	}))
	var runErr error
	stdout, _ := captureOutput(t, func() {
		runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"migrate", "import", "--app", "APP_ID", "--version-id", "VERSION_ID", "--fastlane-dir", root, "--skip-screenshots", "--skip-app-clip", "--skip-previews", "--dry-run", "--output", "json"})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	var receipt map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatal(err)
	}
	if _, exists := receipt["appClip"]; exists {
		t.Fatal("skipped App Clip present in plan")
	}
	if _, exists := receipt["previews"]; exists {
		t.Fatal("skipped previews present in plan")
	}
	var skipped []struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(receipt["skipped"], &skipped); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range skipped {
		seen[item.Reason] = true
	}
	if !seen["skipped by --skip-app-clip"] || !seen["skipped by --skip-previews"] {
		t.Fatalf("missing skip provenance: %s", stdout)
	}
}

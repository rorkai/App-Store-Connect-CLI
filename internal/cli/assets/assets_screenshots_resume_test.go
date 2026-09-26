package assets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestExecuteAppScreenshotUploadDryRunValidatesSourceRootBeforePreview(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()
	writeAssetsTestPNG(t, outsideDir, "01-home.png")
	linkDir := filepath.Join(rootDir, "linked")
	if err := os.Symlink(outsideDir, linkDir); err != nil {
		t.Fatalf("create source symlink: %v", err)
	}
	filePath := filepath.Join(linkDir, "01-home.png")

	requests := 0
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
	}))

	_, err := executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: "LOC_123",
		DisplayType:    "APP_IPHONE_65",
		RootPath:       rootDir,
		Files:          []string{filePath},
		DryRun:         true,
		RequestContext: contextWithAssetUploadTimeout,
		UploadContext:  contextWithAssetUploadTimeout,
		Access:         appStoreVersionScreenshotSetAccess,
	}, "")
	if !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("executeAppScreenshotUpload() error = %v, want rootfs.ErrSymlink", err)
	}
	if requests != 0 {
		t.Fatalf("expected source validation before API lookup, got %d requests", requests)
	}
}

func TestExecuteAppScreenshotUploadDryRunRejectsSymlinkAboveSelectedFileRoot(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := filepath.Join(t.TempDir(), "nested")
	if err := os.Mkdir(outsideDir, 0o700); err != nil {
		t.Fatalf("create outside directory: %v", err)
	}
	writeAssetsTestPNG(t, outsideDir, "01-home.png")
	linkDir := filepath.Join(rootDir, "linked")
	if err := os.Symlink(filepath.Dir(outsideDir), linkDir); err != nil {
		t.Fatalf("create source symlink: %v", err)
	}
	filePath := filepath.Join(linkDir, filepath.Base(outsideDir), "01-home.png")

	requests := 0
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
	}))

	_, err := executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: "LOC_123",
		DisplayType:    "APP_IPHONE_65",
		RootPath:       filePath,
		Files:          []string{filePath},
		DryRun:         true,
		RequestContext: contextWithAssetUploadTimeout,
		UploadContext:  contextWithAssetUploadTimeout,
		Access:         appStoreVersionScreenshotSetAccess,
	}, "")
	if !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("executeAppScreenshotUpload() error = %v, want rootfs.ErrSymlink", err)
	}
	if requests != 0 {
		t.Fatalf("expected source validation before API lookup, got %d requests", requests)
	}
}

func TestExecuteAppScreenshotUploadDryRunAllowsSystemAliasedSourceDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("system directory aliases are Unix-only")
	}

	// macOS reaches /tmp, /var and /etc through symlinks into /private, so a
	// source path under any of them must not be reported as an untrusted
	// symlink the caller never wrote.
	for _, base := range []string{"/tmp", "/var/tmp"} {
		t.Run(base, func(t *testing.T) {
			workDir, err := os.MkdirTemp(base, "asc-screenshot-source-*")
			if err != nil {
				t.Fatalf("create temporary screenshot directory: %v", err)
			}
			t.Cleanup(func() {
				if err := os.RemoveAll(workDir); err != nil {
					t.Errorf("remove temporary screenshot directory: %v", err)
				}
			})
			filePath := writeAssetsTestPNG(t, workDir, "01-home.png")

			requests := 0
			client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests++
				writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
			}))

			_, err = executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
				Client:         client,
				LocalizationID: "LOC_123",
				DisplayType:    "APP_IPHONE_65",
				Files:          []string{filePath},
				DryRun:         true,
				RequestContext: contextWithAssetUploadTimeout,
				UploadContext:  contextWithAssetUploadTimeout,
				Access:         appStoreVersionScreenshotSetAccess,
			}, "")
			if err != nil {
				t.Fatalf("executeAppScreenshotUpload() error: %v", err)
			}
			if requests == 0 {
				t.Fatal("expected dry-run API lookup after source validation")
			}
		})
	}
}

func TestResolveScreenshotUploadRootRejectsSymlinkedSourceFileOutsideTrustedRoots(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("system directory aliases are Unix-only")
	}

	targetPath := writeAssetsTestPNG(t, t.TempDir(), "real.png")
	workDir, err := os.MkdirTemp("/var/tmp", "asc-screenshot-source-*")
	if err != nil {
		t.Fatalf("create temporary screenshot directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(workDir); err != nil {
			t.Errorf("remove temporary screenshot directory: %v", err)
		}
	})
	linkPath := filepath.Join(workDir, "01-home.png")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Fatalf("create screenshot symlink: %v", err)
	}

	if _, err := resolveScreenshotUploadRoot("", []string{linkPath}); !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("resolveScreenshotUploadRoot() error = %v, want rootfs.ErrSymlink", err)
	}
}

func TestExecuteAppScreenshotUploadSkipExistingDoesNotPatchOrderingWhenAlreadyMatched(t *testing.T) {
	filePath := writeAssetsTestPNG(t, t.TempDir(), "01-home.png")
	checksum, err := computeFileChecksum(filePath)
	if err != nil {
		t.Fatalf("compute checksum: %v", err)
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":"existing-1","attributes":{"fileName":"01-home.png","fileSize":100,"sourceFileChecksum":"%s"}}],"links":{}}`, checksum))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-1"},{"type":"appScreenshots","id":"unrelated-1"}],"links":{}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			t.Fatalf("unexpected remote order patch when skip-existing order already matches")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	result, err := executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: "LOC_123",
		DisplayType:    "APP_IPHONE_65",
		Files:          []string{filePath},
		SkipExisting:   true,
		RequestContext: contextWithAssetUploadTimeout,
		UploadContext:  contextWithAssetUploadTimeout,
		Access:         appStoreVersionScreenshotSetAccess,
	}, "")
	if err != nil {
		t.Fatalf("executeAppScreenshotUpload() error: %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if result.Results[0].State != "skipped" {
		t.Fatalf("expected skipped result, got %#v", result.Results[0])
	}
	if result.Uploaded != 0 {
		t.Fatalf("expected uploaded=0, got %d", result.Uploaded)
	}
	if result.Skipped != 1 {
		t.Fatalf("expected skipped=1, got %d", result.Skipped)
	}
}

func TestResumeAppScreenshotUploadReplacesResolvedFailures(t *testing.T) {
	workDir := t.TempDir()
	fileB := writeAssetsTestPNG(t, workDir, "02-settings.png")
	fileC := writeAssetsTestPNG(t, workDir, "03-profile.png")

	artifactPath := filepath.Join(workDir, "resume-artifact.json")
	_, err := persistScreenshotUploadFailureArtifact(artifactPath, screenshotUploadFailureArtifact{
		VersionLocalizationID: "LOC_123",
		DisplayType:           "APP_IPHONE_65",
		SetID:                 "set-1",
		OrderedIDs:            []string{"new-1"},
		PendingFiles:          []string{fileB, fileC},
		Results: []asc.AssetUploadResultItem{
			{FileName: "01-home.png", FilePath: filepath.Join(workDir, "01-home.png"), AssetID: "new-1", State: "COMPLETE"},
		},
		Failures: []asc.AssetUploadFailureItem{
			{FileName: filepath.Base(fileB), FilePath: fileB, Error: "previous create failed"},
		},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("persistScreenshotUploadFailureArtifact() error: %v", err)
	}

	fileBSize := fileSize(t, fileB)

	origTransport := http.DefaultTransport
	createCount := 0
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			createCount++
			if createCount == 1 {
				return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-2","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-2","length":%d,"offset":0}]}}}`, fileBSize))
			}
			return assetsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"upload create failed"}]}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-2":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-2","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-2":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-2","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			t.Fatalf("unexpected relationship patch after mid-resume upload failure")
			return nil, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	result, err := resumeAppScreenshotUpload(context.Background(), client, artifactPath)
	if err == nil {
		t.Fatal("expected resumeAppScreenshotUpload() error")
	}

	if len(result.Results) != 2 {
		t.Fatalf("expected 2 successful results carried forward, got %#v", result.Results)
	}
	if result.Results[1].FilePath != fileB {
		t.Fatalf("expected resumed success for %q, got %#v", fileB, result.Results[1])
	}
	if result.Pending != 1 {
		t.Fatalf("expected pending=1, got %d", result.Pending)
	}
	if result.Failed != 1 {
		t.Fatalf("expected failed=1, got %d", result.Failed)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("expected 1 current failure, got %#v", result.Failures)
	}
	if result.Failures[0].FilePath != fileC {
		t.Fatalf("expected only %q to remain failed, got %#v", fileC, result.Failures)
	}

	artifactData, err := loadScreenshotUploadFailureArtifact(artifactPath)
	if err != nil {
		t.Fatalf("loadScreenshotUploadFailureArtifact() error: %v", err)
	}
	if len(artifactData.Failures) != 1 || artifactData.Failures[0].FilePath != fileC {
		serialized, _ := json.Marshal(artifactData.Failures)
		t.Fatalf("expected rewritten artifact failures to only include %q, got %s", fileC, string(serialized))
	}
	if len(artifactData.PendingFiles) != 1 || artifactData.PendingFiles[0] != fileC {
		t.Fatalf("expected rewritten artifact pending files to only include %q, got %#v", fileC, artifactData.PendingFiles)
	}
}

func TestResumeAppScreenshotUploadRetriesCleanupBeforeNewUploads(t *testing.T) {
	workDir := t.TempDir()
	artifactPath := filepath.Join(workDir, "resume-artifact.json")
	if _, err := persistScreenshotUploadFailureArtifact(artifactPath, screenshotUploadFailureArtifact{
		SetID:      "set-1",
		OrderedIDs: []string{"existing-1"},
		CleanupFailures: []screenshotPendingAsset{{
			FileName: "04-settings.png",
			FilePath: filepath.Join(workDir, "04-settings.png"),
			AssetID:  "leaked-1",
			State:    "COMPLETE",
		}},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("persistScreenshotUploadFailureArtifact() error: %v", err)
	}

	deleteAttempts := 0
	postAttempts := 0
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appScreenshots/leaked-1":
			deleteAttempts++
			if deleteAttempts == 1 {
				writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"cleanup unavailable"}]}`)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case req.Method == http.MethodPost:
			postAttempts++
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"unexpected new reservation"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	}))

	result, err := resumeAppScreenshotUpload(context.Background(), client, artifactPath)
	if err == nil || !strings.Contains(err.Error(), "cleanup unavailable") {
		t.Fatalf("first resume error = %v, want cleanup failure", err)
	}
	if postAttempts != 0 {
		t.Fatalf("new reservations after failed cleanup = %d, want none", postAttempts)
	}
	if result.FailureArtifactPath == "" {
		t.Fatal("expected updated failure artifact path")
	}
	artifact, err := loadScreenshotUploadFailureArtifact(artifactPath)
	if err != nil {
		t.Fatalf("load rewritten artifact: %v", err)
	}
	if len(artifact.CleanupFailures) != 1 || artifact.CleanupFailures[0].AssetID != "leaked-1" {
		t.Fatalf("rewritten cleanup failures = %#v, want leaked-1", artifact.CleanupFailures)
	}

	result, err = resumeAppScreenshotUpload(context.Background(), client, artifactPath)
	if err != nil {
		t.Fatalf("second resume error = %v, want 404 cleanup to be idempotent", err)
	}
	if deleteAttempts != 2 {
		t.Fatalf("cleanup attempts = %d, want retry", deleteAttempts)
	}
	if postAttempts != 0 {
		t.Fatalf("new reservations after successful cleanup = %d, want none", postAttempts)
	}
}

func TestExecuteAppScreenshotUploadSurfacesCleanupFailures(t *testing.T) {
	workDir := t.TempDir()
	files := []string{
		writeAssetsTestPNG(t, workDir, "01-home.png"),
		writeAssetsTestPNG(t, workDir, "02-settings.png"),
		writeAssetsTestPNG(t, workDir, "03-profile.png"),
	}
	artifactPath := filepath.Join(workDir, "failure-artifact.json")
	uploadErr := errors.New("upload failed")
	previous := uploadOneScreenshot
	t.Cleanup(func() { uploadOneScreenshot = previous })
	uploadOneScreenshot = func(_ context.Context, _ *asc.Client, _, filePath, _ string, _ openedScreenshotFiles) (asc.AssetUploadResultItem, screenshotPendingAsset, error) {
		if filepath.Base(filePath) == "01-home.png" {
			return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, uploadErr
		}
		return asc.AssetUploadResultItem{
			FileName: filepath.Base(filePath),
			FilePath: filePath,
			AssetID:  "asset-" + filepath.Base(filePath),
		}, screenshotPendingAsset{}, nil
	}

	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodDelete:
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"cleanup failed"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	}))

	result, err := executeAppScreenshotUpload(
		withScreenshotUploadConcurrency(context.Background(), 3),
		screenshotUploadConfig[asc.AppScreenshotUploadResult]{
			Client:         client,
			LocalizationID: "LOC_123",
			DisplayType:    "APP_IPHONE_65",
			Files:          files,
			RequestContext: contextWithAssetUploadTimeout,
			UploadContext:  contextWithAssetUploadTimeout,
			Access:         appStoreVersionScreenshotSetAccess,
		},
		artifactPath,
	)
	if !errors.Is(err, uploadErr) || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("executeAppScreenshotUpload() error = %v, want upload and cleanup failures", err)
	}
	if len(result.Failures) != 2 {
		t.Fatalf("result failures = %#v, want upload plus cleanup entries", result.Failures)
	}
	if result.Failures[0].FilePath != files[0] || result.Failures[1].FileName != "screenshot cleanup" {
		t.Fatalf("result failures = %#v, want failed file and cleanup entries", result.Failures)
	}
	artifact, err := loadScreenshotUploadFailureArtifact(artifactPath)
	if err != nil {
		t.Fatalf("load failure artifact: %v", err)
	}
	if len(artifact.CleanupFailures) != 2 {
		t.Fatalf("artifact cleanup failures = %#v, want unresolved siblings", artifact.CleanupFailures)
	}
}

func TestResumeAppScreenshotUploadReusesCreatedAssetAfterCommitFailure(t *testing.T) {
	workDir := t.TempDir()
	filePath := writeAssetsTestPNG(t, workDir, "01-home.png")
	fileSizeBytes := fileSize(t, filePath)
	artifactPath := filepath.Join(workDir, "resume-artifact.json")

	createCount := 0
	commitCount := 0
	committed := false
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			createCount++
			writeAssetsTestJSON(w, http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"pending-1","attributes":{"uploadOperations":[{"method":"PUT","url":"http://%s/uploads/pending-1","length":%d,"offset":0}]}}}`, req.Host, fileSizeBytes))
		case req.Method == http.MethodPut && req.URL.Path == "/uploads/pending-1":
			writeAssetsTestJSON(w, http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/pending-1":
			commitCount++
			if commitCount == 1 {
				writeAssetsTestJSON(w, http.StatusBadRequest, `{"errors":[{"status":"400","code":"ENTITY_ERROR","detail":"commit interrupted"}]}`)
				return
			}
			committed = true
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"pending-1","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/pending-1":
			if committed {
				writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"pending-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
				return
			}
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"pending-1","attributes":{"assetDeliveryState":{"state":"AWAITING_UPLOAD"}}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			writeAssetsTestJSON(w, http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	result, err := executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: "LOC_123",
		DisplayType:    "APP_IPHONE_65",
		Files:          []string{filePath},
		RequestContext: contextWithAssetUploadTimeout,
		UploadContext:  contextWithAssetUploadTimeout,
		Access:         appStoreVersionScreenshotSetAccess,
	}, artifactPath)
	if err == nil {
		t.Fatal("expected initial upload error")
	}
	if result.Pending != 1 || result.FailureArtifactPath == "" {
		t.Fatalf("expected resumable failure result, got %#v", result)
	}

	artifact, err := loadScreenshotUploadFailureArtifact(artifactPath)
	if err != nil {
		t.Fatalf("loadScreenshotUploadFailureArtifact() error: %v", err)
	}
	if len(artifact.PendingAssets) != 1 {
		t.Fatalf("expected one pending remote asset, got %#v", artifact.PendingAssets)
	}
	pending := artifact.PendingAssets[0]
	if pending.AssetID != "pending-1" || pending.FilePath != filePath || pending.State != "UPLOADED" || pending.Checksum == "" {
		t.Fatalf("unexpected pending asset: %#v", pending)
	}

	result, err = resumeAppScreenshotUpload(context.Background(), client, artifactPath)
	if err != nil {
		t.Fatalf("resumeAppScreenshotUpload() error: %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].AssetID != "pending-1" || result.Results[0].State != "COMPLETE" {
		t.Fatalf("expected resumed result to reuse pending-1, got %#v", result.Results)
	}
	if createCount != 1 {
		t.Fatalf("expected exactly one screenshot reservation, got %d", createCount)
	}
	if commitCount != 2 {
		t.Fatalf("expected resume to retry the commit once, got %d attempts", commitCount)
	}
}

func TestReconcilePendingScreenshotWaitsForCompletedAssetChecksum(t *testing.T) {
	workDir := t.TempDir()
	filePath := writeAssetsTestPNG(t, workDir, "01-home.png")
	checksum, err := computeFileChecksum(filePath)
	if err != nil {
		t.Fatalf("computeFileChecksum() error: %v", err)
	}

	detailCalls := 0
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appScreenshots/pending-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		detailCalls++
		if detailCalls == 1 {
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"pending-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
			return
		}
		writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"pending-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
	}))

	result, pending, retry, err := reconcilePendingScreenshotAsset(context.Background(), client, screenshotPendingAsset{
		FileName: filepath.Base(filePath),
		FilePath: filePath,
		AssetID:  "pending-1",
		Checksum: checksum,
		State:    "COMPLETE",
	}, workDir)
	if err != nil {
		t.Fatalf("reconcilePendingScreenshotAsset() error: %v", err)
	}
	if retry {
		t.Fatal("expected completed reservation to be reused")
	}
	if pending.AssetID != "" {
		t.Fatalf("expected settled pending asset to be cleared, got %#v", pending)
	}
	if result.AssetID != "pending-1" || result.State != "COMPLETE" {
		t.Fatalf("unexpected completed result: %#v", result)
	}
	if detailCalls != 2 {
		t.Fatalf("expected checksum settlement detail request, got %d calls", detailCalls)
	}
}

func TestResumeAppScreenshotUploadRejectsChangedFileForCompletedReservation(t *testing.T) {
	workDir := t.TempDir()
	filePath := writeAssetsTestPNG(t, workDir, "01-home.png")
	checksum, err := computeFileChecksum(filePath)
	if err != nil {
		t.Fatalf("computeFileChecksum() error: %v", err)
	}
	artifactPath := filepath.Join(workDir, "resume-artifact.json")
	_, err = persistScreenshotUploadFailureArtifact(artifactPath, screenshotUploadFailureArtifact{
		RootPath:     workDir,
		SetID:        "set-1",
		PendingFiles: []string{filePath},
		PendingAssets: []screenshotPendingAsset{{
			FileName: filepath.Base(filePath),
			FilePath: filePath,
			AssetID:  "pending-1",
			Checksum: checksum,
			State:    "UPLOAD_COMPLETE",
		}},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("persistScreenshotUploadFailureArtifact() error: %v", err)
	}
	writeAssetsTestPNGWithSize(t, workDir, filepath.Base(filePath), 9, 8)

	orderPatched := false
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/pending-1":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"pending-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			orderPatched = true
			writeAssetsTestJSON(w, http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	result, err := resumeAppScreenshotUpload(context.Background(), client, artifactPath)
	if err == nil {
		t.Fatal("expected changed-file error")
	}
	if len(result.Failures) != 1 || !strings.Contains(result.Failures[0].Error, "pending screenshot file changed after upload") {
		t.Fatalf("expected changed-file failure detail, got %#v", result.Failures)
	}
	if orderPatched {
		t.Fatal("did not expect ordering to be updated for a stale completed reservation")
	}
}

func TestResumeAppScreenshotUploadRejectsSymlinkedParentBelowRecordedRoot(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := writeAssetsTestPNG(t, outsideDir, "01-home.png")
	linkDir := filepath.Join(rootDir, "linked")
	if err := os.Symlink(outsideDir, linkDir); err != nil {
		t.Fatalf("create parent symlink: %v", err)
	}
	linkedPath := filepath.Join(linkDir, filepath.Base(outsidePath))
	checksum, err := computeFileChecksum(linkedPath)
	if err != nil {
		t.Fatalf("computeFileChecksum() error: %v", err)
	}
	artifactPath := filepath.Join(rootDir, "resume-artifact.json")
	artifactData, err := json.MarshalIndent(screenshotUploadFailureArtifact{
		RootPath:     rootDir,
		SetID:        "set-1",
		PendingFiles: []string{linkedPath},
		PendingAssets: []screenshotPendingAsset{{
			FileName: filepath.Base(linkedPath),
			FilePath: linkedPath,
			AssetID:  "pending-1",
			Checksum: checksum,
			State:    "UPLOAD_COMPLETE",
		}},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		t.Fatalf("marshal screenshot artifact: %v", err)
	}
	root, err := rootfs.New(rootDir)
	if err != nil {
		t.Fatalf("rootfs.New() error: %v", err)
	}
	if err := root.WriteFile(filepath.Base(artifactPath), append(artifactData, '\n'), 0o600); err != nil {
		t.Fatalf("write screenshot artifact: %v", err)
	}

	requests := 0
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"pending-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
	}))

	_, err = resumeAppScreenshotUpload(context.Background(), client, artifactPath)
	if !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("resumeAppScreenshotUpload() error = %v, want rootfs.ErrSymlink", err)
	}
	if requests != 0 {
		t.Fatalf("expected source validation before API lookup, got %d requests", requests)
	}
}

func TestUploadScreenshotAssetRejectsSymlinkedParentBelowSourceRoot(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := writeAssetsTestPNG(t, outsideDir, "01-home.png")
	linkDir := filepath.Join(rootDir, "linked")
	if err := os.Symlink(outsideDir, linkDir); err != nil {
		t.Fatalf("create parent symlink: %v", err)
	}

	_, _, err := uploadScreenshotAsset(context.Background(), nil, "set-1", rootDir, filepath.Join(linkDir, filepath.Base(outsidePath)))
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "symlink") {
		t.Fatalf("expected rooted parent-symlink rejection, got %v", err)
	}
}

func TestResumeScreenshotsDeletesIncompleteReservationBeforeRecreating(t *testing.T) {
	filePath := writeAssetsTestPNG(t, t.TempDir(), "01-home.png")
	fileSizeBytes := fileSize(t, filePath)
	checksum, err := computeFileChecksum(filePath)
	if err != nil {
		t.Fatalf("computeFileChecksum() error: %v", err)
	}

	deletedIncomplete := false
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/incomplete-1":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"incomplete-1","attributes":{"assetDeliveryState":{"state":"AWAITING_UPLOAD"}}}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appScreenshots/incomplete-1":
			deletedIncomplete = true
			writeAssetsTestJSON(w, http.StatusNoContent, "")
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			if !deletedIncomplete {
				t.Fatal("replacement reservation was created before the incomplete one was deleted")
			}
			writeAssetsTestJSON(w, http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"replacement-1","attributes":{"uploadOperations":[{"method":"PUT","url":"http://%s/uploads/replacement-1","length":%d,"offset":0}]}}}`, req.Host, fileSizeBytes))
		case req.Method == http.MethodPut && req.URL.Path == "/uploads/replacement-1":
			writeAssetsTestJSON(w, http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/replacement-1":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"replacement-1","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/replacement-1":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"replacement-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			writeAssetsTestJSON(w, http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	progress, err := resumeScreenshotsWithOrderState(
		context.Background(),
		client,
		"set-1",
		nil,
		[]string{filePath},
		[]screenshotPendingAsset{{
			FileName: filepath.Base(filePath),
			FilePath: filePath,
			AssetID:  "incomplete-1",
			Checksum: checksum,
			State:    "AWAITING_UPLOAD",
		}},
		filepath.Dir(filePath),
		true,
		true,
	)
	if err != nil {
		t.Fatalf("resumeScreenshotsWithOrderState() error: %v", err)
	}
	if len(progress.Results) != 1 || progress.Results[0].AssetID != "replacement-1" {
		t.Fatalf("expected replacement upload result, got %#v", progress.Results)
	}
}

func TestResumeScreenshotsDoesNotCreateWhenPendingLookupFails(t *testing.T) {
	filePath := writeAssetsTestPNG(t, t.TempDir(), "01-home.png")
	createCalled := false
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/pending-1":
			writeAssetsTestJSON(w, http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","detail":"lookup unavailable"}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			createCalled = true
			writeAssetsTestJSON(w, http.StatusInternalServerError, `{}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	progress, err := resumeScreenshotsWithOrderState(
		context.Background(),
		client,
		"set-1",
		nil,
		[]string{filePath},
		[]screenshotPendingAsset{{FileName: filepath.Base(filePath), FilePath: filePath, AssetID: "pending-1", State: "UPLOAD_COMPLETE"}},
		filepath.Dir(filePath),
		true,
		true,
	)
	if err == nil {
		t.Fatal("expected pending lookup error")
	}
	if createCalled {
		t.Fatal("did not expect a new reservation after an inconclusive lookup")
	}
	if len(progress.PendingAssets) != 1 || progress.PendingAssets[0].AssetID != "pending-1" {
		t.Fatalf("expected pending asset to remain resumable, got %#v", progress.PendingAssets)
	}
}

func TestExecuteAppScreenshotUploadKeepsUploadErrorWhenArtifactWriteFails(t *testing.T) {
	workDir := t.TempDir()
	filePath := writeAssetsTestPNG(t, workDir, "01-home.png")

	// Make the artifact directory unusable by putting a regular file where the
	// artifact's parent directory has to be.
	blockedDir := filepath.Join(workDir, "reports")
	if err := os.WriteFile(blockedDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}
	artifactPath := filepath.Join(blockedDir, "failure-artifact.json")

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return assetsJSONResponse(http.StatusUnauthorized, `{"errors":[{"status":"401","code":"NOT_AUTHORIZED","detail":"authentication credentials are missing"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	_, err := executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: "LOC_123",
		DisplayType:    "APP_IPHONE_65",
		Files:          []string{filePath},
		RequestContext: contextWithAssetUploadTimeout,
		UploadContext:  contextWithAssetUploadTimeout,
		Access:         appStoreVersionScreenshotSetAccess,
	}, artifactPath)
	if err == nil {
		t.Fatal("expected executeAppScreenshotUpload() error")
	}
	if !strings.Contains(err.Error(), "write screenshot upload failure artifact") {
		t.Fatalf("expected artifact write failure in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "authentication credentials are missing") {
		t.Fatalf("expected the original upload failure to survive, got %v", err)
	}
}

func TestExecuteAppScreenshotUploadOrderSyncFailureSurfacesOrderingError(t *testing.T) {
	workDir := t.TempDir()
	filePath := writeAssetsTestPNG(t, workDir, "01-home.png")
	fileSizeBytes := fileSize(t, filePath)
	artifactPath := filepath.Join(workDir, "failure-artifact.json")

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, fileSizeBytes))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-1":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-1":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"reorder failed"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	result, err := executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: "LOC_123",
		DisplayType:    "APP_IPHONE_65",
		Files:          []string{filePath},
		RequestContext: contextWithAssetUploadTimeout,
		UploadContext:  contextWithAssetUploadTimeout,
		Access:         appStoreVersionScreenshotSetAccess,
	}, artifactPath)
	if err == nil {
		t.Fatal("expected executeAppScreenshotUpload() error")
	}

	var reported shared.ReportedError
	if !errors.As(err, &reported) {
		t.Fatalf("expected ReportedError, got %T: %v", err, err)
	}
	if err.Error() != "screenshots upload: retry needed to sync screenshot ordering" {
		t.Fatalf("unexpected retry message: %v", err)
	}
	if result.Pending != 0 {
		t.Fatalf("expected pending=0 for order-only retry, got %d", result.Pending)
	}
	if result.Failed != 1 {
		t.Fatalf("expected failed=1, got %d", result.Failed)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("expected 1 failure entry, got %#v", result.Failures)
	}
	if result.Failures[0].FileName != "screenshot ordering" {
		t.Fatalf("expected ordering failure row, got %#v", result.Failures[0])
	}
	if !strings.Contains(result.Failures[0].Error, "reorder failed") {
		t.Fatalf("expected ordering failure detail, got %#v", result.Failures[0])
	}
	if result.FailureArtifactPath == "" {
		t.Fatalf("expected failure artifact path, got %#v", result)
	}

	artifactData, err := loadScreenshotUploadFailureArtifact(artifactPath)
	if err != nil {
		t.Fatalf("loadScreenshotUploadFailureArtifact() error: %v", err)
	}
	if len(artifactData.PendingFiles) != 0 {
		t.Fatalf("expected no pending files for order-only retry, got %#v", artifactData.PendingFiles)
	}
	if len(artifactData.OrderedIDs) != 1 || artifactData.OrderedIDs[0] != "new-1" {
		t.Fatalf("expected artifact to preserve uploaded ordering, got %#v", artifactData.OrderedIDs)
	}
	if len(artifactData.Failures) != 1 || artifactData.Failures[0].FileName != "screenshot ordering" {
		t.Fatalf("expected ordering failure in artifact, got %#v", artifactData.Failures)
	}
}

func TestResumeAppScreenshotUploadOrderingOnlyDoesNotRequireSourceRoot(t *testing.T) {
	workDir := t.TempDir()
	missingRoot := filepath.Join(workDir, "removed-screenshots")
	artifactPath := filepath.Join(workDir, "failure-artifact.json")
	_, err := persistScreenshotUploadFailureArtifact(artifactPath, screenshotUploadFailureArtifact{
		RootPath:   missingRoot,
		SetID:      "set-1",
		Files:      []string{filepath.Join(missingRoot, "01-home.png")},
		OrderedIDs: []string{"new-1"},
		Results: []asc.AssetUploadResultItem{{
			FileName: "01-home.png",
			FilePath: filepath.Join(missingRoot, "01-home.png"),
			AssetID:  "new-1",
			State:    "COMPLETE",
		}},
		Error:       "reorder failed",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("persistScreenshotUploadFailureArtifact() error: %v", err)
	}

	patched := false
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/appScreenshotSets/set-1/relationships/appScreenshots" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		patched = true
		writeAssetsTestJSON(w, http.StatusNoContent, "")
	}))

	result, err := resumeAppScreenshotUpload(context.Background(), client, artifactPath)
	if err != nil {
		t.Fatalf("resumeAppScreenshotUpload() error: %v", err)
	}
	if !patched {
		t.Fatal("expected ordering retry")
	}
	if len(result.Results) != 1 || result.Results[0].AssetID != "new-1" {
		t.Fatalf("expected preserved completed result, got %#v", result.Results)
	}
}

func TestExecuteAppScreenshotUploadSkipExistingPatchFailurePersistsLocalOrder(t *testing.T) {
	workDir := t.TempDir()
	newPath := writeAssetsTestPNGWithSize(t, workDir, "01-new.png", 9, 8)
	existingPath := writeAssetsTestPNG(t, workDir, "02-existing.png")
	newSizeBytes := fileSize(t, newPath)
	existingChecksum, err := computeFileChecksum(existingPath)
	if err != nil {
		t.Fatalf("compute existing checksum: %v", err)
	}
	artifactPath := filepath.Join(workDir, "failure-artifact.json")

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":"existing-1","attributes":{"fileName":"02-existing.png","fileSize":100,"sourceFileChecksum":"%s"}}],"links":{}}`, existingChecksum))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-1"},{"type":"appScreenshots","id":"unrelated-1"}],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, newSizeBytes))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-1":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-1":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"reorder failed"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	_, err = executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: "LOC_123",
		DisplayType:    "APP_IPHONE_65",
		Files:          []string{newPath, existingPath},
		SkipExisting:   true,
		RequestContext: contextWithAssetUploadTimeout,
		UploadContext:  contextWithAssetUploadTimeout,
		Access:         appStoreVersionScreenshotSetAccess,
	}, artifactPath)
	if err == nil {
		t.Fatal("expected executeAppScreenshotUpload() error")
	}

	artifactData, err := loadScreenshotUploadFailureArtifact(artifactPath)
	if err != nil {
		t.Fatalf("loadScreenshotUploadFailureArtifact() error: %v", err)
	}
	if got, want := strings.Join(artifactData.OrderedIDs, ","), "new-1,existing-1,unrelated-1"; got != want {
		t.Fatalf("expected artifact to preserve local file order %q, got %q", want, got)
	}
}

func TestExecuteAppScreenshotUploadSkipExistingSyncFailurePersistsLocalOrder(t *testing.T) {
	workDir := t.TempDir()
	newPath := writeAssetsTestPNGWithSize(t, workDir, "01-new.png", 9, 8)
	existingPath := writeAssetsTestPNG(t, workDir, "02-existing.png")
	newSizeBytes := fileSize(t, newPath)
	existingChecksum, err := computeFileChecksum(existingPath)
	if err != nil {
		t.Fatalf("compute existing checksum: %v", err)
	}
	artifactPath := filepath.Join(workDir, "failure-artifact.json")

	origTransport := http.DefaultTransport
	relationshipPatchCount := 0
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":"existing-1","attributes":{"fileName":"02-existing.png","fileSize":100,"sourceFileChecksum":"%s"}}],"links":{}}`, existingChecksum))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-1"},{"type":"appScreenshots","id":"new-1"}],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, newSizeBytes))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-1":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-1":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			relationshipPatchCount++
			if relationshipPatchCount == 1 {
				return assetsJSONResponse(http.StatusOK, `{}`)
			}
			return assetsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"reorder failed"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	result, err := executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: "LOC_123",
		DisplayType:    "APP_IPHONE_65",
		Files:          []string{newPath, existingPath},
		SkipExisting:   true,
		RequestContext: contextWithAssetUploadTimeout,
		UploadContext:  contextWithAssetUploadTimeout,
		Access:         appStoreVersionScreenshotSetAccess,
	}, artifactPath)
	if err == nil {
		t.Fatal("expected executeAppScreenshotUpload() error")
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected skipped and uploaded results to be preserved, got %#v", result.Results)
	}
	if result.FailureArtifactPath == "" {
		t.Fatalf("expected failure artifact path, got %#v", result)
	}

	artifactData, err := loadScreenshotUploadFailureArtifact(artifactPath)
	if err != nil {
		t.Fatalf("loadScreenshotUploadFailureArtifact() error: %v", err)
	}
	if got, want := strings.Join(artifactData.OrderedIDs, ","), "new-1,existing-1"; got != want {
		t.Fatalf("expected artifact to preserve local file order %q, got %q", want, got)
	}
	if len(artifactData.Failures) != 1 || artifactData.Failures[0].FileName != "screenshot ordering" {
		t.Fatalf("expected ordering failure in artifact, got %#v", artifactData.Failures)
	}
}

func TestResumeAppScreenshotUploadSkipExistingPreservesPendingLocalOrder(t *testing.T) {
	workDir := t.TempDir()
	newPath := writeAssetsTestPNGWithSize(t, workDir, "01-new.png", 9, 8)
	pendingPath := writeAssetsTestPNGWithSize(t, workDir, "02-pending.png", 10, 8)
	existingPath := writeAssetsTestPNG(t, workDir, "03-existing.png")
	pendingSizeBytes := fileSize(t, pendingPath)
	artifactPath := filepath.Join(workDir, "failure-artifact.json")

	_, err := persistScreenshotUploadFailureArtifact(artifactPath, screenshotUploadFailureArtifact{
		VersionLocalizationID: "LOC_123",
		Path:                  artifactPath,
		DisplayType:           "APP_IPHONE_65",
		SkipExisting:          true,
		SetID:                 "set-1",
		Files:                 []string{newPath, pendingPath, existingPath},
		OrderedIDs:            []string{"new-1", "existing-1"},
		PendingFiles:          []string{pendingPath},
		Results: []asc.AssetUploadResultItem{
			{FileName: filepath.Base(existingPath), FilePath: existingPath, AssetID: "existing-1", State: "skipped", Skipped: true},
			{FileName: filepath.Base(newPath), FilePath: newPath, AssetID: "new-1", State: "COMPLETE"},
		},
		Error:       "screenshots upload: 1 file(s) pending retry",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("persistScreenshotUploadFailureArtifact() error: %v", err)
	}

	relationshipPatches := make([][]string, 0, 2)
	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"pending-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/pending-1","length":%d,"offset":0}]}}}`, pendingSizeBytes))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/pending-1":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"pending-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/pending-1":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"pending-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"unrelated-1"},{"type":"appScreenshots","id":"new-1"},{"type":"appScreenshots","id":"existing-1"},{"type":"appScreenshots","id":"pending-1"}],"links":{}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read relationship patch body: %v", err)
			}
			var payload asc.RelationshipRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode relationship patch body: %v", err)
			}
			gotIDs := make([]string, 0, len(payload.Data))
			for _, item := range payload.Data {
				gotIDs = append(gotIDs, item.ID)
			}
			relationshipPatches = append(relationshipPatches, gotIDs)
			return assetsJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	result, err := resumeAppScreenshotUpload(context.Background(), client, artifactPath)
	if err != nil {
		t.Fatalf("resumeAppScreenshotUpload() error: %v", err)
	}
	if len(result.Results) != 3 {
		t.Fatalf("expected previous and resumed results, got %#v", result.Results)
	}
	if len(relationshipPatches) == 0 {
		t.Fatal("expected relationship patch during resume")
	}
	wantFinalOrder := []string{"new-1", "pending-1", "existing-1", "unrelated-1"}
	if got := relationshipPatches[len(relationshipPatches)-1]; !reflect.DeepEqual(got, wantFinalOrder) {
		t.Fatalf("final relationship order = %v, want %v", got, wantFinalOrder)
	}
}

func TestResumeAppScreenshotUploadSkipExistingRetriesOrderingWithoutIDs(t *testing.T) {
	workDir := t.TempDir()
	existingPath := writeAssetsTestPNG(t, workDir, "01-existing.png")
	artifactPath := filepath.Join(workDir, "failure-artifact.json")

	_, err := persistScreenshotUploadFailureArtifact(artifactPath, screenshotUploadFailureArtifact{
		VersionLocalizationID: "LOC_123",
		Path:                  artifactPath,
		DisplayType:           "APP_IPHONE_65",
		SkipExisting:          true,
		SetID:                 "set-1",
		Files:                 []string{existingPath},
		Results: []asc.AssetUploadResultItem{
			{FileName: filepath.Base(existingPath), FilePath: existingPath, AssetID: "existing-1", State: "skipped", Skipped: true},
		},
		Error:       "relationship lookup failed",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("persistScreenshotUploadFailureArtifact() error: %v", err)
	}

	relationshipPatchCalled := false
	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"unrelated-1"},{"type":"appScreenshots","id":"existing-1"}],"links":{}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read relationship patch body: %v", err)
			}
			var payload asc.RelationshipRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode relationship patch body: %v", err)
			}
			gotIDs := make([]string, 0, len(payload.Data))
			for _, item := range payload.Data {
				gotIDs = append(gotIDs, item.ID)
			}
			wantIDs := []string{"existing-1", "unrelated-1"}
			if !reflect.DeepEqual(gotIDs, wantIDs) {
				t.Fatalf("relationship order = %v, want %v", gotIDs, wantIDs)
			}
			relationshipPatchCalled = true
			return assetsJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	result, err := resumeAppScreenshotUpload(context.Background(), client, artifactPath)
	if err != nil {
		t.Fatalf("resumeAppScreenshotUpload() error: %v", err)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected preserved skipped result, got %#v", result.Results)
	}
	if !relationshipPatchCalled {
		t.Fatal("expected relationship patch during resume")
	}
}

func TestPersistScreenshotUploadFailureArtifactNormalizesPendingPathsForResume(t *testing.T) {
	workDir := t.TempDir()
	otherDir := t.TempDir()
	screenshotsDir := filepath.Join(workDir, "screenshots")
	if err := os.MkdirAll(screenshotsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir(%q) error: %v", workDir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousDir)
	})

	relativeFile := filepath.Join("screenshots", "02-settings.png")
	absoluteFile := writeAssetsTestPNG(t, screenshotsDir, "02-settings.png")
	artifactPath := filepath.Join(workDir, "resume-artifact.json")
	expectedPendingPath, err := filepath.Abs(relativeFile)
	if err != nil {
		t.Fatalf("Abs(%q) error: %v", relativeFile, err)
	}

	_, err = persistScreenshotUploadFailureArtifact(artifactPath, screenshotUploadFailureArtifact{
		VersionLocalizationID: "LOC_123",
		DisplayType:           "APP_IPHONE_65",
		SetID:                 "set-1",
		OrderedIDs:            []string{"new-1"},
		PendingFiles:          []string{relativeFile},
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("persistScreenshotUploadFailureArtifact() error: %v", err)
	}

	artifactData, err := loadScreenshotUploadFailureArtifact(artifactPath)
	if err != nil {
		t.Fatalf("loadScreenshotUploadFailureArtifact() error: %v", err)
	}
	if len(artifactData.PendingFiles) != 1 || artifactData.PendingFiles[0] != expectedPendingPath {
		t.Fatalf("expected absolute pending file path %q, got %#v", expectedPendingPath, artifactData.PendingFiles)
	}

	if err := os.Chdir(otherDir); err != nil {
		t.Fatalf("Chdir(%q) error: %v", otherDir, err)
	}

	fileSizeBytes := fileSize(t, absoluteFile)
	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-2","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-2","length":%d,"offset":0}]}}}`, fileSizeBytes))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-2":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-2","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-2":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-2","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	result, err := resumeAppScreenshotUpload(context.Background(), client, artifactPath)
	if err != nil {
		t.Fatalf("resumeAppScreenshotUpload() error: %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 resumed result, got %#v", result.Results)
	}
	if result.Results[0].FilePath != expectedPendingPath {
		t.Fatalf("expected resumed upload to use absolute file path %q, got %#v", expectedPendingPath, result.Results[0])
	}
}

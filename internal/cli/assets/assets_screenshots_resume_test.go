package assets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestExecuteAppScreenshotUploadSkipExistingDoesNotFetchOrderingWhenNoFilesRemain(t *testing.T) {
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
			t.Fatalf("unexpected remote order lookup when skip-existing leaves no files to upload")
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
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-2","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
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
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-1","length":%d,"offset":0}]}}}`, fileSizeBytes))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-1":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-1":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
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
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-2","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
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

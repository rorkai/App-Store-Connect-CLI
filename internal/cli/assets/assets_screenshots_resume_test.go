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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

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
	fileBComplete := make(chan struct{})
	var fileBCompleteOnce sync.Once
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			if strings.Contains(string(body), "02-settings.png") {
				return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-2","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-2","length":%d,"offset":0}]}}}`, fileBSize))
			}
			// Fail 03-profile.png only after 02-settings.png fully
			// completed so the concurrent resume outcome is
			// deterministic: one resolved file, one failed file.
			<-fileBComplete
			return assetsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"upload create failed"}]}`)
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-2":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-2","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-2":
			fileBCompleteOnce.Do(func() { close(fileBComplete) })
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
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
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
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"pending-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
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

func TestResumeAppScreenshotUploadReordersResumedFilesIntoFileOrder(t *testing.T) {
	workDir := t.TempDir()
	fileA := writeAssetsTestPNG(t, workDir, "01-home.png")
	fileB := writeAssetsTestPNG(t, workDir, "02-settings.png")
	fileC := writeAssetsTestPNG(t, workDir, "03-profile.png")
	fileBSize := fileSize(t, fileB)

	// A concurrent upload can fail a middle file while later files
	// succeeded: the artifact then records completed uploads out of local
	// file order. Resume must slot the retried file back into place.
	artifactPath := filepath.Join(workDir, "resume-artifact.json")
	_, err := persistScreenshotUploadFailureArtifact(artifactPath, screenshotUploadFailureArtifact{
		VersionLocalizationID: "LOC_123",
		DisplayType:           "APP_IPHONE_65",
		SetID:                 "set-1",
		Files:                 []string{fileA, fileB, fileC},
		OrderedIDs:            []string{"pre-1", "new-1", "new-3"},
		PendingFiles:          []string{fileB},
		Results: []asc.AssetUploadResultItem{
			{FileName: filepath.Base(fileA), FilePath: fileA, AssetID: "new-1", State: "COMPLETE"},
			{FileName: filepath.Base(fileC), FilePath: fileC, AssetID: "new-3", State: "COMPLETE"},
		},
		Failures: []asc.AssetUploadFailureItem{
			{FileName: filepath.Base(fileB), FilePath: fileB, Error: "previous create failed"},
		},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("persistScreenshotUploadFailureArtifact() error: %v", err)
	}

	relationshipPatches := make([][]string, 0, 1)
	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-2","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-2","length":%d,"offset":0}]}}}`, fileBSize))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-2":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-2","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-2":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-2","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
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
	if len(relationshipPatches) != 1 {
		t.Fatalf("expected exactly one relationship patch during resume, got %#v", relationshipPatches)
	}
	wantOrder := []string{"pre-1", "new-1", "new-2", "new-3"}
	if !reflect.DeepEqual(relationshipPatches[0], wantOrder) {
		t.Fatalf("relationship order = %v, want %v", relationshipPatches[0], wantOrder)
	}
}

func TestMergeResumedScreenshotOrderKeepsUnmappedIDsFirst(t *testing.T) {
	files := []string{"/tmp/01.png", "/tmp/02.png", "/tmp/03.png"}
	results := []asc.AssetUploadResultItem{
		{FilePath: "/tmp/01.png", AssetID: "new-1"},
		{FilePath: "/tmp/03.png", AssetID: "new-3"},
		{FilePath: "/tmp/02.png", AssetID: "new-2"},
	}

	got := mergeResumedScreenshotOrder([]string{"pre-1", "new-1", "new-3", "new-2"}, files, results)
	want := []string{"pre-1", "new-1", "new-2", "new-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mergeResumedScreenshotOrder() = %v, want %v", got, want)
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

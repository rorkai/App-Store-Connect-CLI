package assets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestUploadScreenshotsReplaceValidatesOpenedFileBeforeDeletingExistingScreenshots(t *testing.T) {
	dir := t.TempDir()
	filePath := writeAssetsTestPNGWithSize(t, dir, "01-home.png", 1242, 2688)
	replacementPath := filepath.Join(dir, "replacement.jpg")
	replacement, err := os.Create(replacementPath)
	if err != nil {
		t.Fatalf("create replacement: %v", err)
	}
	if err := jpeg.Encode(replacement, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		replacement.Close()
		t.Fatalf("encode replacement: %v", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatalf("close replacement: %v", err)
	}

	if err := validateScreenshotDimensions([]string{filePath}, "APP_IPHONE_65"); err != nil {
		t.Fatalf("initial screenshot preflight failed: %v", err)
	}

	deleted := false
	reserved := false
	var replacementErr error
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			replacementErr = os.Rename(replacementPath, filePath)
			if replacementErr != nil {
				http.Error(w, replacementErr.Error(), http.StatusInternalServerError)
				return
			}
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-1","attributes":{"fileName":"old.png"}}],"links":{}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appScreenshots/existing-1":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			reserved = true
			http.Error(w, "replacement reached reservation", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))

	_, err = uploadScreenshots(context.Background(), client, "LOC_123", "APP_IPHONE_65", []string{filePath}, false, true, false)
	if replacementErr != nil {
		t.Fatalf("replace validated screenshot: %v", replacementErr)
	}
	if err == nil {
		t.Fatal("uploadScreenshots() error = nil, want format mismatch")
	}
	for _, want := range []string{"01-home.png", "JPEG", ".png"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("uploadScreenshots() error = %q, want %q", err, want)
		}
	}
	if deleted {
		t.Fatal("existing screenshot was deleted before the replacement mismatch was rejected")
	}
	if reserved {
		t.Fatal("replacement screenshot reached asset reservation")
	}
}

func TestUploadScreenshotsReplaceValidatesRootBeforeDeletingExistingScreenshots(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	writeAssetsTestPNG(t, outside, "01-home.png")
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatalf("create source symlink: %v", err)
	}
	filePath := filepath.Join(root, "linked", "01-home.png")

	deleted := false
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-1","attributes":{"fileName":"old.png"}}],"links":{}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appScreenshots/existing-1":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	_, err := uploadScreenshots(context.Background(), client, "LOC_123", "APP_IPHONE_65", []string{filePath}, false, true, false)
	if !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("uploadScreenshots() error = %v, want rootfs.ErrSymlink", err)
	}
	if deleted {
		t.Fatal("existing screenshot was deleted before the upload source root was validated")
	}
}

func TestUploadScreenshotsDryRunValidatesSourceRootBeforePreview(t *testing.T) {
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

	_, err := uploadScreenshots(context.Background(), client, "LOC_123", "APP_IPHONE_65", []string{filePath}, false, false, true)
	if !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("uploadScreenshots() error = %v, want rootfs.ErrSymlink", err)
	}
	if requests != 0 {
		t.Fatalf("expected source validation before API lookup, got %d requests", requests)
	}
}

func TestUploadScreenshotsSkipExistingStartsUploadTimeoutAfterChecksumFiltering(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "200ms")
	t.Setenv("ASC_UPLOAD_TIMEOUT", "30s")
	t.Cleanup(asc.SetMediaPollIntervalsForTest(time.Millisecond, time.Millisecond))

	filePath := writeAssetsTestPNG(t, t.TempDir(), "01-home.png")
	fileSizeBytes := fileSize(t, filePath)

	origChecksumFunc := screenshotFileChecksumFunc
	screenshotFileChecksumFunc = func(path string) (string, error) {
		time.Sleep(250 * time.Millisecond)
		return computeFileChecksum(path)
	}
	t.Cleanup(func() {
		screenshotFileChecksumFunc = origChecksumFunc
	})

	deliveryCalls := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if err := req.Context().Err(); err != nil {
			t.Fatalf("request context error: %v", err)
		}

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			uploadURL := "http://" + req.Host + "/new-1"
			body := fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploadOperations":[{"method":"PUT","url":%q,"length":%d,"offset":0}]}}}`, uploadURL, fileSizeBytes)
			writeAssetsTestJSON(w, http.StatusCreated, body)
		case req.Method == http.MethodPut && req.URL.Path == "/new-1":
			writeAssetsTestJSON(w, http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-1":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-1":
			deliveryCalls++
			if deliveryCalls == 1 {
				writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
				return
			}
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"sourceFileChecksum":"settled-checksum","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})
	client := newAssetsUploadTestHandlerClient(t, handler)
	originalTransport := http.DefaultTransport
	http.DefaultTransport = assetsHandlerTransport(handler)
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	var result asc.AppScreenshotUploadResult
	var err error
	synctest.Test(t, func(*testing.T) {
		result, err = uploadScreenshots(context.Background(), client, "LOC_123", "APP_IPHONE_65", []string{filePath}, true, false, false)
	})
	if err != nil {
		t.Fatalf("uploadScreenshots() error: %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 upload result, got %d", len(result.Results))
	}
	if result.Results[0].AssetID != "new-1" {
		t.Fatalf("expected uploaded asset ID new-1, got %#v", result.Results[0])
	}
	if deliveryCalls != 2 {
		t.Fatalf("expected delivery polling to wait for the checksum, got %d request(s)", deliveryCalls)
	}
}

func TestUploadScreenshotsSkipExistingSettlesMissingRemoteChecksum(t *testing.T) {
	filePath := writeAssetsTestPNG(t, t.TempDir(), "01-home.png")
	checksum, err := computeFileChecksum(filePath)
	if err != nil {
		t.Fatalf("compute checksum: %v", err)
	}

	createCalls := 0
	detailCalls := 0
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-1","attributes":{"fileName":"01-home.png","assetDeliveryState":{"state":"COMPLETE"}}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/existing-1":
			detailCalls++
			writeAssetsTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"existing-1","attributes":{"fileName":"01-home.png","sourceFileChecksum":%q,"assetDeliveryState":{"state":"COMPLETE"}}}}`, checksum))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-1"}],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			createCalls++
			writeAssetsTestJSON(w, http.StatusConflict, `{"errors":[{"status":"409","detail":"duplicate reservation"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	result, err := uploadScreenshots(context.Background(), client, "LOC_123", "APP_IPHONE_65", []string{filePath}, true, false, false)
	if err != nil {
		t.Fatalf("uploadScreenshots() error: %v", err)
	}
	if createCalls != 0 {
		t.Fatalf("expected no screenshot reservation, got %d", createCalls)
	}
	if detailCalls != 1 {
		t.Fatalf("expected one checksum settlement request, got %d", detailCalls)
	}
	if len(result.Results) != 1 || !result.Results[0].Skipped || result.Results[0].AssetID != "existing-1" {
		t.Fatalf("expected existing screenshot to be skipped with its asset ID, got %#v", result.Results)
	}
}

func TestExecuteAppScreenshotUploadSkipExistingSettlesMissingRemoteChecksum(t *testing.T) {
	filePath := writeAssetsTestPNG(t, t.TempDir(), "01-home.png")
	checksum, err := computeFileChecksum(filePath)
	if err != nil {
		t.Fatalf("compute checksum: %v", err)
	}

	createCalls := 0
	detailCalls := 0
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-1","attributes":{"fileName":"01-home.png","assetDeliveryState":{"state":"COMPLETE"}}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/existing-1":
			detailCalls++
			writeAssetsTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"existing-1","attributes":{"fileName":"01-home.png","sourceFileChecksum":%q,"assetDeliveryState":{"state":"COMPLETE"}}}}`, checksum))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-1"}],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			createCalls++
			writeAssetsTestJSON(w, http.StatusConflict, `{"errors":[{"status":"409","detail":"duplicate reservation"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	result, err := executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: "LOC_123",
		DisplayType:    "APP_IPHONE_65",
		Files:          []string{filePath},
		SkipExisting:   true,
		Access:         appStoreVersionScreenshotSetAccess,
		BuildResult: func(localizationID string, set asc.Resource[asc.AppScreenshotSetAttributes], dryRun bool, results []asc.AssetUploadResultItem) asc.AppScreenshotUploadResult {
			return buildAppScreenshotUploadResult(localizationID, set, dryRun, results)
		},
	}, "")
	if err != nil {
		t.Fatalf("executeAppScreenshotUpload() error: %v", err)
	}
	if createCalls != 0 {
		t.Fatalf("expected no screenshot reservation, got %d", createCalls)
	}
	if detailCalls != 1 {
		t.Fatalf("expected one checksum settlement request, got %d", detailCalls)
	}
	if len(result.Results) != 1 || !result.Results[0].Skipped || result.Results[0].AssetID != "existing-1" {
		t.Fatalf("expected existing screenshot to be skipped with its asset ID, got %#v", result.Results)
	}
}

func TestExecuteAppScreenshotUploadSkipExistingDoesNotWaitForUndeliveredReservation(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{name: "awaiting upload", state: "AWAITING_UPLOAD"},
		{name: "failed", state: "FAILED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := writeAssetsTestPNG(t, t.TempDir(), "01-home.png")
			detailCalls := 0
			client := newAssetsUploadTestHandlerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
					writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
					writeAssetsTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":"pending-1","attributes":{"fileName":"old.png","assetDeliveryState":{"state":%q}}}],"links":{}}`, tt.state))
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/pending-1":
					detailCalls++
					writeAssetsTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"pending-1","attributes":{"fileName":"old.png","assetDeliveryState":{"state":%q}}}}`, tt.state))
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				}
			}))

			var result asc.AppScreenshotUploadResult
			var err error
			synctest.Test(t, func(*testing.T) {
				result, err = executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
					Client:         client,
					LocalizationID: "LOC_123",
					DisplayType:    "APP_IPHONE_65",
					Files:          []string{filePath},
					SkipExisting:   true,
					DryRun:         true,
					RequestContext: func(ctx context.Context) (context.Context, context.CancelFunc) {
						return context.WithTimeout(ctx, 20*time.Millisecond)
					},
					Access: appStoreVersionScreenshotSetAccess,
					BuildResult: func(localizationID string, set asc.Resource[asc.AppScreenshotSetAttributes], dryRun bool, results []asc.AssetUploadResultItem) asc.AppScreenshotUploadResult {
						return buildAppScreenshotUploadResult(localizationID, set, dryRun, results)
					},
				}, "")
			})
			if err != nil {
				t.Fatalf("executeAppScreenshotUpload() error: %v", err)
			}
			if detailCalls != 0 {
				t.Fatalf("expected no settlement request for %s reservation, got %d", tt.state, detailCalls)
			}
			if len(result.Results) != 1 || result.Results[0].State != "would-upload" {
				t.Fatalf("expected unrelated local screenshot to remain uploadable, got %#v", result.Results)
			}
		})
	}
}

func TestUploadScreenshotsSkipExistingChecksumTimeoutIncludesAssetID(t *testing.T) {
	filePath := writeAssetsTestPNG(t, t.TempDir(), "01-home.png")

	createCalls := 0
	client := newAssetsUploadTestHandlerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"unsettled-1","attributes":{"fileName":"01-home.png","assetDeliveryState":{"state":"COMPLETE"}}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/unsettled-1":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"unsettled-1","attributes":{"fileName":"01-home.png","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			createCalls++
			writeAssetsTestJSON(w, http.StatusConflict, `{"errors":[{"status":"409","detail":"duplicate reservation"}]}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	var err error
	synctest.Test(t, func(*testing.T) {
		_, err = uploadScreenshotsWithConfig(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
			Client:         client,
			LocalizationID: "LOC_123",
			DisplayType:    "APP_IPHONE_65",
			Files:          []string{filePath},
			SkipExisting:   true,
			DryRun:         true,
			RequestContext: func(ctx context.Context) (context.Context, context.CancelFunc) {
				return context.WithTimeout(ctx, 20*time.Millisecond)
			},
			Access: appStoreVersionScreenshotSetAccess,
			BuildResult: func(localizationID string, set asc.Resource[asc.AppScreenshotSetAttributes], dryRun bool, results []asc.AssetUploadResultItem) asc.AppScreenshotUploadResult {
				return buildAppScreenshotUploadResult(localizationID, set, dryRun, results)
			},
		})
	})
	if err == nil {
		t.Fatal("expected checksum settlement timeout")
	}
	if !strings.Contains(err.Error(), "unsettled-1") || !strings.Contains(strings.ToLower(err.Error()), "checksum") {
		t.Fatalf("expected checksum timeout to preserve the asset ID, got %v", err)
	}
	if createCalls != 0 {
		t.Fatalf("expected no screenshot reservation after checksum timeout, got %d", createCalls)
	}
}

func TestWaitForScreenshotSettlementReportsCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(asc.SetMediaPollIntervalsForTest(time.Millisecond, time.Millisecond))
	t.Cleanup(cancel)

	deliveryCalls := 0
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appScreenshots/canceled-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		deliveryCalls++
		if deliveryCalls == 2 {
			cancel()
		}
		writeAssetsTestJSON(w, http.StatusOK, `{"data":{"type":"appScreenshots","id":"canceled-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
	}))

	_, err := waitForScreenshotSettlement(ctx, client, "canceled-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForScreenshotSettlement() error = %v, want context.Canceled", err)
	}
	if !strings.Contains(err.Error(), "canceled-1") || !strings.Contains(err.Error(), "canceled waiting") || !strings.Contains(err.Error(), "state COMPLETE") {
		t.Fatalf("expected cancellation error to preserve the screenshot ID, got %v", err)
	}
	if strings.Contains(err.Error(), "timed out") {
		t.Fatalf("caller cancellation must not be reported as a timeout: %v", err)
	}
}

func TestUploadScreenshotsDryRunReportsWouldUpload(t *testing.T) {
	filePath := writeAssetsTestPNG(t, t.TempDir(), "01-home.png")

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		default:
			t.Fatalf("unexpected request in dry-run: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	result, err := uploadScreenshots(context.Background(), client, "LOC_123", "APP_IPHONE_65", []string{filePath}, false, false, true)
	if err != nil {
		t.Fatalf("uploadScreenshots() error: %v", err)
	}

	if !result.DryRun {
		t.Fatal("expected DryRun=true")
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if result.Results[0].State != "would-upload" {
		t.Fatalf("expected state would-upload, got %q", result.Results[0].State)
	}
	if result.Results[0].AssetID != "" {
		t.Fatalf("expected empty asset ID in dry-run, got %q", result.Results[0].AssetID)
	}
}

func TestUploadScreenshotsDryRunDoesNotCreateSet(t *testing.T) {
	filePath := writeAssetsTestPNG(t, t.TempDir(), "01-home.png")

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		default:
			t.Fatalf("dry-run must not issue mutating requests: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	result, err := uploadScreenshots(context.Background(), client, "LOC_123", "APP_IPHONE_65", []string{filePath}, false, false, true)
	if err != nil {
		t.Fatalf("uploadScreenshots() error: %v", err)
	}

	if !result.DryRun {
		t.Fatal("expected DryRun=true")
	}
	if result.SetID != "" {
		t.Fatalf("expected empty set ID when no set exists, got %q", result.SetID)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if result.Results[0].State != "would-upload" {
		t.Fatalf("expected state would-upload, got %q", result.Results[0].State)
	}
}

func TestUploadScreenshotsDryRunWithReplaceReportsWouldDelete(t *testing.T) {
	filePath := writeAssetsTestPNG(t, t.TempDir(), "01-home.png")

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-1","attributes":{"fileName":"old.png","fileSize":100}}],"links":{}}`)
		default:
			t.Fatalf("unexpected request in dry-run --replace: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	result, err := uploadScreenshots(context.Background(), client, "LOC_123", "APP_IPHONE_65", []string{filePath}, false, true, true)
	if err != nil {
		t.Fatalf("uploadScreenshots() error: %v", err)
	}

	if !result.DryRun {
		t.Fatal("expected DryRun=true")
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected 2 results (1 delete + 1 upload), got %d", len(result.Results))
	}
	if result.Results[0].State != "would-delete" {
		t.Fatalf("expected first result state would-delete, got %q", result.Results[0].State)
	}
	if result.Results[0].AssetID != "existing-1" {
		t.Fatalf("expected would-delete asset ID existing-1, got %q", result.Results[0].AssetID)
	}
	if result.Results[1].State != "would-upload" {
		t.Fatalf("expected second result state would-upload, got %q", result.Results[1].State)
	}
	if result.Uploaded != 0 {
		t.Fatalf("expected uploaded=0 for dry-run replace preview, got %d", result.Uploaded)
	}
}

func TestUploadScreenshotsDryRunWithSkipExistingReportsSkipped(t *testing.T) {
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
		default:
			t.Fatalf("unexpected request in dry-run --skip-existing: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	result, err := uploadScreenshots(context.Background(), client, "LOC_123", "APP_IPHONE_65", []string{filePath}, true, false, true)
	if err != nil {
		t.Fatalf("uploadScreenshots() error: %v", err)
	}

	if !result.DryRun {
		t.Fatal("expected DryRun=true")
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if result.Results[0].State != "skipped" {
		t.Fatalf("expected state skipped, got %q", result.Results[0].State)
	}
	if !result.Results[0].Skipped {
		t.Fatal("expected Skipped=true")
	}
}

func TestUploadScreenshotsSkipExistingSyncsLocalFileOrder(t *testing.T) {
	dir := t.TempDir()
	fileA := writeAssetsTestPNG(t, dir, "01-home.png")
	fileB := writeAssetsTestPNG(t, dir, "02-settings.png")
	const (
		checksumA = "checksum-a"
		checksumB = "checksum-b"
	)
	origChecksumFunc := screenshotFileChecksumFunc
	screenshotFileChecksumFunc = func(path string) (string, error) {
		switch path {
		case fileA:
			return checksumA, nil
		case fileB:
			return checksumB, nil
		default:
			return computeFileChecksum(path)
		}
	}
	t.Cleanup(func() {
		screenshotFileChecksumFunc = origChecksumFunc
	})

	relationshipPatchCalled := false
	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":"existing-b","attributes":{"fileName":"02-settings.png","fileSize":100,"sourceFileChecksum":"%s"}},{"type":"appScreenshots","id":"existing-a","attributes":{"fileName":"01-home.png","fileSize":100,"sourceFileChecksum":"%s"}}],"links":{}}`, checksumB, checksumA))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-b"},{"type":"appScreenshots","id":"existing-a"}],"links":{}}`)
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
			wantIDs := []string{"existing-a", "existing-b"}
			if !reflect.DeepEqual(gotIDs, wantIDs) {
				t.Fatalf("relationship order = %v, want %v", gotIDs, wantIDs)
			}
			relationshipPatchCalled = true
			return assetsJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request in --skip-existing reorder: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	result, err := uploadScreenshots(context.Background(), client, "LOC_123", "APP_IPHONE_65", []string{fileA, fileB}, true, false, false)
	if err != nil {
		t.Fatalf("uploadScreenshots() error: %v", err)
	}

	if len(result.Results) != 2 {
		t.Fatalf("expected 2 skipped results, got %d", len(result.Results))
	}
	if result.Results[0].AssetID != "existing-a" || result.Results[1].AssetID != "existing-b" {
		t.Fatalf("expected skipped results to keep existing IDs in local order, got %#v", result.Results)
	}
	if !relationshipPatchCalled {
		t.Fatal("expected --skip-existing to sync screenshot relationship order")
	}
}

func TestUploadScreenshotsSkipExistingSyncFailurePreservesResults(t *testing.T) {
	dir := t.TempDir()
	fileA := writeAssetsTestPNG(t, dir, "01-home.png")
	fileB := writeAssetsTestPNG(t, dir, "02-settings.png")
	const (
		checksumA = "checksum-a"
		checksumB = "checksum-b"
	)
	origChecksumFunc := screenshotFileChecksumFunc
	screenshotFileChecksumFunc = func(path string) (string, error) {
		switch path {
		case fileA:
			return checksumA, nil
		case fileB:
			return checksumB, nil
		default:
			return computeFileChecksum(path)
		}
	}
	t.Cleanup(func() {
		screenshotFileChecksumFunc = origChecksumFunc
	})

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":"existing-b","attributes":{"fileName":"02-settings.png","fileSize":100,"sourceFileChecksum":"%s"}},{"type":"appScreenshots","id":"existing-a","attributes":{"fileName":"01-home.png","fileSize":100,"sourceFileChecksum":"%s"}}],"links":{}}`, checksumB, checksumA))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-b"},{"type":"appScreenshots","id":"existing-a"}],"links":{}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","detail":"reorder failed"}]}`)
		default:
			t.Fatalf("unexpected request in --skip-existing reorder failure: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	result, err := uploadScreenshots(context.Background(), client, "LOC_123", "APP_IPHONE_65", []string{fileA, fileB}, true, false, false)
	if err == nil {
		t.Fatal("expected uploadScreenshots() error")
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected skipped results to be preserved, got %#v", result.Results)
	}
	if result.Results[0].AssetID != "existing-a" || result.Results[1].AssetID != "existing-b" {
		t.Fatalf("expected skipped results in local order, got %#v", result.Results)
	}
}

func TestExecuteAppScreenshotUploadSkipExistingSyncsLocalFileOrder(t *testing.T) {
	dir := t.TempDir()
	fileA := writeAssetsTestPNG(t, dir, "01-home.png")
	fileB := writeAssetsTestPNG(t, dir, "02-settings.png")
	const (
		checksumA = "checksum-a"
		checksumB = "checksum-b"
	)
	origChecksumFunc := screenshotFileChecksumFunc
	screenshotFileChecksumFunc = func(path string) (string, error) {
		switch path {
		case fileA:
			return checksumA, nil
		case fileB:
			return checksumB, nil
		default:
			return computeFileChecksum(path)
		}
	}
	t.Cleanup(func() {
		screenshotFileChecksumFunc = origChecksumFunc
	})

	relationshipPatchCalled := false
	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":"existing-b","attributes":{"fileName":"02-settings.png","fileSize":100,"sourceFileChecksum":"%s"}},{"type":"appScreenshots","id":"existing-a","attributes":{"fileName":"01-home.png","fileSize":100,"sourceFileChecksum":"%s"}}],"links":{}}`, checksumB, checksumA))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"existing-b"},{"type":"appScreenshots","id":"existing-a"}],"links":{}}`)
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
			wantIDs := []string{"existing-a", "existing-b"}
			if !reflect.DeepEqual(gotIDs, wantIDs) {
				t.Fatalf("relationship order = %v, want %v", gotIDs, wantIDs)
			}
			relationshipPatchCalled = true
			return assetsJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request in execute --skip-existing reorder: %s %s", req.Method, req.URL.String())
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
		Files:          []string{fileA, fileB},
		SkipExisting:   true,
		Access:         appStoreVersionScreenshotSetAccess,
		BuildResult: func(localizationID string, set asc.Resource[asc.AppScreenshotSetAttributes], dryRun bool, results []asc.AssetUploadResultItem) asc.AppScreenshotUploadResult {
			return buildAppScreenshotUploadResult(localizationID, set, dryRun, results)
		},
	}, "")
	if err != nil {
		t.Fatalf("executeAppScreenshotUpload() error: %v", err)
	}

	if len(result.Results) != 2 {
		t.Fatalf("expected 2 skipped results, got %d", len(result.Results))
	}
	if result.Results[0].AssetID != "existing-a" || result.Results[1].AssetID != "existing-b" {
		t.Fatalf("expected skipped results to keep existing IDs in local order, got %#v", result.Results)
	}
	if !relationshipPatchCalled {
		t.Fatal("expected execute --skip-existing to sync screenshot relationship order")
	}
}

func TestExecuteAppScreenshotUploadMaxScreenshotsAccountsForExistingRemoteScreenshots(t *testing.T) {
	dir := t.TempDir()
	files := make([]string, 0, appScreenshotSetMaxScreenshots)
	sizes := make(map[string]int64, appScreenshotSetMaxScreenshots)
	for i := 1; i <= appScreenshotSetMaxScreenshots; i++ {
		filePath := writeAssetsTestPNG(t, dir, fmt.Sprintf("%02d-home.png", i))
		files = append(files, filePath)
		sizes[filepath.Base(filePath)] = fileSize(t, filePath)
	}

	createCount := 0
	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"old-1","attributes":{"fileName":"old.png","fileSize":100}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"old-1"}],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			createCount++
			if createCount > appScreenshotSetMaxScreenshots-1 {
				t.Fatalf("max-screenshots with one existing remote screenshot must upload at most 9 new files; got create %d", createCount)
			}
			name := fmt.Sprintf("%02d-home.png", createCount)
			return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-%d","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-%d","length":%d,"offset":0}]}}}`, createCount, createCount, sizes[name]))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`, id))
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
	result, err := executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: "LOC_123",
		DisplayType:    "APP_IPHONE_65",
		Files:          files,
		MaxScreenshots: appScreenshotSetMaxScreenshots,
		Access:         appStoreVersionScreenshotSetAccess,
		BuildResult: func(localizationID string, set asc.Resource[asc.AppScreenshotSetAttributes], dryRun bool, results []asc.AssetUploadResultItem) asc.AppScreenshotUploadResult {
			return buildAppScreenshotUploadResult(localizationID, set, dryRun, results)
		},
	}, "")
	if err != nil {
		t.Fatalf("executeAppScreenshotUpload() error: %v", err)
	}
	if createCount != appScreenshotSetMaxScreenshots-1 {
		t.Fatalf("expected 9 uploaded screenshots, got %d", createCount)
	}
	if result.Uploaded != appScreenshotSetMaxScreenshots-1 {
		t.Fatalf("expected uploaded count 9, got %d", result.Uploaded)
	}
}

func TestExecuteAppScreenshotUploadRejectsAppendAboveScreenshotLimit(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		writeAssetsTestPNG(t, dir, "01-home.png"),
		writeAssetsTestPNG(t, dir, "02-home.png"),
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"old-1"},{"type":"appScreenshots","id":"old-2"},{"type":"appScreenshots","id":"old-3"},{"type":"appScreenshots","id":"old-4"},{"type":"appScreenshots","id":"old-5"},{"type":"appScreenshots","id":"old-6"},{"type":"appScreenshots","id":"old-7"},{"type":"appScreenshots","id":"old-8"},{"type":"appScreenshots","id":"old-9"}],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			t.Fatal("must reject before creating screenshots when append would exceed the set limit")
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
	_, err := executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: "LOC_123",
		DisplayType:    "APP_IPHONE_65",
		Files:          files,
		Access:         appStoreVersionScreenshotSetAccess,
	}, "")
	if err == nil {
		t.Fatal("expected screenshot set limit error")
	}
	if !strings.Contains(err.Error(), "would exceed App Store screenshot set limit 10") {
		t.Fatalf("expected screenshot set limit error, got %v", err)
	}
}

func TestExecuteAppScreenshotUploadFullSetProvidesUsableRemediation(t *testing.T) {
	dir := t.TempDir()
	filePath := writeAssetsTestPNG(t, dir, "01-home.png")

	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshots","id":"old-1"},{"type":"appScreenshots","id":"old-2"},{"type":"appScreenshots","id":"old-3"},{"type":"appScreenshots","id":"old-4"},{"type":"appScreenshots","id":"old-5"},{"type":"appScreenshots","id":"old-6"},{"type":"appScreenshots","id":"old-7"},{"type":"appScreenshots","id":"old-8"},{"type":"appScreenshots","id":"old-9"},{"type":"appScreenshots","id":"old-10"}],"links":{}}`)
		default:
			t.Fatalf("full-set remediation must not mutate remote assets: %s %s", req.Method, req.URL.String())
		}
	}))

	_, err := executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: "LOC_123",
		DisplayType:    "APP_IPHONE_65",
		Files:          []string{filePath},
		Access:         appStoreVersionScreenshotSetAccess,
	}, "")
	if err == nil {
		t.Fatal("expected full screenshot set error")
	}
	errText := err.Error()
	if strings.Contains(errText, "--max-screenshots 0") {
		t.Fatalf("remediation recommends an unusable zero limit: %v", err)
	}
	for _, want := range []string{
		"no upload slots remain",
		"no remote assets were changed for this screenshot set",
		`asc screenshots list --version-localization "LOC_123" --output json`,
		`asc screenshots delete --id "SCREENSHOT_ID" --confirm`,
		"--replace --confirm",
	} {
		if !strings.Contains(errText, want) {
			t.Fatalf("full-set remediation missing %q: %v", want, err)
		}
	}
}

func TestExecuteAppScreenshotUploadSkipExistingRejectsAmbiguousRemoteChecksum(t *testing.T) {
	dir := t.TempDir()
	filePath := writeAssetsTestPNG(t, dir, "01-home.png")
	checksum, err := computeFileChecksum(filePath)
	if err != nil {
		t.Fatalf("computeFileChecksum() error: %v", err)
	}

	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/LOC_123/appScreenshotSets":
			writeAssetsTestJSON(w, http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			writeAssetsTestJSON(w, http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":"duplicate-c","attributes":{"fileName":"third.png","sourceFileChecksum":%q}},{"type":"appScreenshots","id":"duplicate-b","attributes":{"fileName":"copy.png","sourceFileChecksum":%q}},{"type":"appScreenshots","id":"duplicate-a","attributes":{"fileName":"original.png","sourceFileChecksum":%q}}],"links":{}}`, checksum, checksum, checksum))
		default:
			t.Fatalf("ambiguous checksum handling must not mutate remote assets: %s %s", req.Method, req.URL.String())
		}
	}))

	_, err = executeAppScreenshotUpload(context.Background(), screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: "LOC_123",
		DisplayType:    "APP_IPHONE_65",
		Files:          []string{filePath},
		SkipExisting:   true,
		Access:         appStoreVersionScreenshotSetAccess,
	}, "")
	if err == nil {
		t.Fatal("expected ambiguous remote checksum error")
	}
	errText := err.Error()
	for _, want := range []string{
		`local screenshot "01-home.png" matches multiple remote screenshots by checksum`,
		`asset IDs: "duplicate-a", "duplicate-b", "duplicate-c"`,
		"no remote assets were changed for this screenshot set",
		`asc screenshots list --version-localization "LOC_123" --output json`,
		"retain one matching screenshot and delete every other duplicate",
		`asc screenshots delete --id "duplicate-a" --confirm`,
		`asc screenshots delete --id "duplicate-b" --confirm`,
		`asc screenshots delete --id "duplicate-c" --confirm`,
		"retry --skip-existing",
	} {
		if !strings.Contains(errText, want) {
			t.Fatalf("duplicate-checksum remediation missing %q: %v", want, err)
		}
	}
	if strings.Index(errText, "duplicate-a") > strings.Index(errText, "duplicate-b") {
		t.Fatalf("expected stable asset ID ordering, got %v", err)
	}
}

func TestFilterExistingScreenshotFilesDoesNotTreatRepeatedResourceAsAmbiguous(t *testing.T) {
	filePath := writeAssetsTestPNG(t, t.TempDir(), "01-home.png")
	checksum, err := computeFileChecksum(filePath)
	if err != nil {
		t.Fatalf("computeFileChecksum() error: %v", err)
	}
	resource := asc.Resource[asc.AppScreenshotAttributes]{
		ID: "existing-1",
		Attributes: asc.AppScreenshotAttributes{
			SourceFileChecksum: checksum,
		},
	}

	files, skipped, err := filterExistingScreenshotFiles(
		[]string{filePath},
		[]asc.Resource[asc.AppScreenshotAttributes]{resource, resource},
		screenshotInspectionCommand("LOC_123"),
	)
	if err != nil {
		t.Fatalf("filterExistingScreenshotFiles() error: %v", err)
	}
	if len(files) != 0 || len(skipped) != 1 || skipped[0].AssetID != "existing-1" {
		t.Fatalf("files = %v, skipped = %#v", files, skipped)
	}
}

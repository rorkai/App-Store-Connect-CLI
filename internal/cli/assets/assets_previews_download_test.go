package assets

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePreviewDownloadURLsFillsMissingURLsAndKeepsExistingOnes(t *testing.T) {
	installAssetsTestTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviews/preview-2":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appPreviews","id":"preview-2","attributes":{"fileName":"b.mov","videoUrl":"https://download.example/b.mov"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviews/preview-3":
			return assetsJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","detail":"preview missing"}]}`)
		default:
			return assetsJSONResponse(http.StatusNotFound, fmt.Sprintf(`{"errors":[{"status":"404","code":"UNEXPECTED","detail":"unexpected request %s %s"}]}`, req.Method, req.URL.String()))
		}
	})

	items := []previewDownloadItem{
		{ID: "preview-1", URL: "https://download.example/a.mov"},
		{ID: "preview-2"},
		{ID: "preview-3"},
	}

	client := newAssetsUploadTestClient(t)
	resolvePreviewDownloadURLs(context.Background(), client, items)

	if items[0].URL != "https://download.example/a.mov" {
		t.Fatalf("expected existing URL to be preserved, got %#v", items[0])
	}
	if items[1].URL != "https://download.example/b.mov" {
		t.Fatalf("expected missing URL to be resolved from preview details, got %#v", items[1])
	}
	if items[2].URL != "" {
		t.Fatalf("expected fetch failure to leave URL empty, got %#v", items[2])
	}
}

func TestDownloadPreviewItemsPreservesItemOrderAndRecordsFailuresInOrder(t *testing.T) {
	installAssetsTestTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Host == "download.example" && req.URL.Path == "/ok.mov":
			return assetsJSONResponse(http.StatusOK, "video-bytes")
		case req.Method == http.MethodGet && req.URL.Host == "download.example" && req.URL.Path == "/missing.mov":
			return assetsJSONResponse(http.StatusNotFound, "not found")
		case req.Method == http.MethodGet && req.URL.Host == "download.example" && req.URL.Path == "/ok2.mov":
			return assetsJSONResponse(http.StatusOK, "more-video-bytes")
		default:
			return assetsJSONResponse(http.StatusNotFound, fmt.Sprintf("unexpected request %s %s", req.Method, req.URL.String()))
		}
	})

	dir := t.TempDir()
	items := []previewDownloadItem{
		{ID: "preview-1", PreviewType: "IPHONE_65", URL: "https://download.example/ok.mov", OutputPath: filepath.Join(dir, "IPHONE_65", "01.mov")},
		{ID: "preview-2", PreviewType: "IPHONE_65", OutputPath: filepath.Join(dir, "IPHONE_65", "02.mov")},
		{ID: "preview-3", PreviewType: "IPAD_PRO_129", URL: "https://download.example/missing.mov", OutputPath: filepath.Join(dir, "IPAD_PRO_129", "03.mov")},
		{ID: "preview-4", PreviewType: "IPAD_PRO_129", URL: "https://download.example/ok2.mov", OutputPath: filepath.Join(dir, "IPAD_PRO_129", "04.mov")},
	}

	downloaded, failures := downloadPreviewItems(context.Background(), items, false)

	if downloaded != 2 {
		t.Fatalf("expected 2 downloads, got %d", downloaded)
	}
	if len(failures) != 1 || failures[0].ID != "preview-3" {
		t.Fatalf("expected one failure for preview-3, got %#v", failures)
	}
	if items[0].BytesWritten != int64(len("video-bytes")) {
		t.Fatalf("expected bytes recorded for first item, got %#v", items[0])
	}
	if items[1].BytesWritten != 0 {
		t.Fatalf("expected item without URL to be skipped, got %#v", items[1])
	}
	if items[3].BytesWritten != int64(len("more-video-bytes")) {
		t.Fatalf("expected bytes recorded for last item, got %#v", items[3])
	}

	data, err := os.ReadFile(items[0].OutputPath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(data) != "video-bytes" {
		t.Fatalf("unexpected downloaded contents %q", string(data))
	}
	if _, err := os.Stat(items[2].OutputPath); !os.IsNotExist(err) {
		t.Fatalf("expected no file for failed download, got err=%v", err)
	}
}

func TestDownloadPreviewItemsReportsCallerCancellationForQueuedItems(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	items := make([]previewDownloadItem, assetTransferWorkerLimit+1)
	for idx := range items {
		items[idx] = previewDownloadItem{
			ID:          fmt.Sprintf("preview-%d", idx),
			PreviewType: "IPHONE_65",
			URL:         fmt.Sprintf("https://download.example/%d.mov", idx),
			OutputPath:  filepath.Join(t.TempDir(), fmt.Sprintf("%d.mov", idx)),
		}
	}

	downloaded, failures := downloadPreviewItems(ctx, items, false)
	if downloaded != 0 {
		t.Fatalf("expected no downloads after caller cancellation, got %d", downloaded)
	}
	if len(failures) != len(items) {
		t.Fatalf("expected one cancellation failure per item, got %#v", failures)
	}
	for idx, failure := range failures {
		if failure.ID != items[idx].ID || failure.Error != context.Canceled.Error() {
			t.Fatalf("failures[%d] = %#v, want cancellation for %s", idx, failure, items[idx].ID)
		}
	}
}

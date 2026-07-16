package assets

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestResolvePreviewDownloadURLsPreservesIndexAlignedLookupResults(t *testing.T) {
	installAssetsTestTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviews/preview-2":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appPreviews","id":"preview-2","attributes":{"fileName":"b.mov","videoUrl":"https://download.example/b.mov"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviews/preview-3":
			return assetsJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","detail":"preview missing"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appPreviews/preview-4":
			return assetsJSONResponse(http.StatusOK, `{"data":{"type":"appPreviews","id":"preview-4","attributes":{"fileName":"d.mov"}}}`)
		default:
			return assetsJSONResponse(http.StatusNotFound, fmt.Sprintf(`{"errors":[{"status":"404","code":"UNEXPECTED","detail":"unexpected request %s %s"}]}`, req.Method, req.URL.String()))
		}
	})

	items := []previewDownloadItem{
		{ID: "preview-1", URL: "https://download.example/a.mov"},
		{ID: "preview-2"},
		{ID: "preview-3"},
		{ID: "preview-4"},
	}

	client := newAssetsUploadTestClient(t)
	lookupErrs := resolvePreviewDownloadURLs(context.Background(), client, items)

	if items[0].URL != "https://download.example/a.mov" {
		t.Fatalf("expected existing URL to be preserved, got %#v", items[0])
	}
	if items[1].URL != "https://download.example/b.mov" {
		t.Fatalf("expected missing URL to be resolved from preview details, got %#v", items[1])
	}
	if items[2].URL != "" {
		t.Fatalf("expected fetch failure to leave URL empty, got %#v", items[2])
	}
	if items[3].URL != "" {
		t.Fatalf("expected successful empty response to leave URL empty, got %#v", items[3])
	}
	if len(lookupErrs) != len(items) {
		t.Fatalf("lookup errors = %#v, want %d index-aligned slots", lookupErrs, len(items))
	}
	if lookupErrs[0] != nil || lookupErrs[1] != nil || lookupErrs[3] != nil {
		t.Fatalf("unexpected lookup errors: %#v", lookupErrs)
	}
	if !errors.Is(lookupErrs[2], asc.ErrNotFound) {
		t.Fatalf("lookupErrs[2] = %v, want not-found identity", lookupErrs[2])
	}
	var statusErr interface{ HTTPStatusCode() int }
	if !errors.As(lookupErrs[2], &statusErr) || statusErr.HTTPStatusCode() != http.StatusNotFound {
		t.Fatalf("lookupErrs[2] = %v, want HTTP status %d", lookupErrs[2], http.StatusNotFound)
	}
}

func TestResolvePreviewDownloadURLsPreservesRetryableAndAuthErrorIdentity(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")
	installAssetsTestTransport(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/appPreviews/preview-retryable":
			return nil, &asc.RetryableError{Err: &asc.APIError{
				Code:       "SERVICE_UNAVAILABLE",
				Detail:     "try again later",
				StatusCode: http.StatusServiceUnavailable,
			}}
		case "/v1/appPreviews/preview-auth":
			return assetsJSONResponse(http.StatusUnauthorized, `{"errors":[{"status":"401","code":"UNAUTHORIZED","detail":"token expired"}]}`)
		default:
			return assetsJSONResponse(http.StatusNotFound, `{}`)
		}
	})

	items := []previewDownloadItem{{ID: "preview-retryable"}, {ID: "preview-auth"}}
	lookupErrs := resolvePreviewDownloadURLs(context.Background(), newAssetsUploadTestClient(t), items)

	if len(lookupErrs) != len(items) {
		t.Fatalf("lookup errors = %#v, want %d index-aligned slots", lookupErrs, len(items))
	}
	if !asc.IsRetryable(lookupErrs[0]) {
		t.Fatalf("lookupErrs[0] = %v, want retryable identity", lookupErrs[0])
	}
	if !errors.Is(lookupErrs[1], asc.ErrUnauthorized) {
		t.Fatalf("lookupErrs[1] = %v, want unauthorized identity", lookupErrs[1])
	}

	for idx, wantStatus := range []int{http.StatusServiceUnavailable, http.StatusUnauthorized} {
		var statusErr interface{ HTTPStatusCode() int }
		if !errors.As(lookupErrs[idx], &statusErr) || statusErr.HTTPStatusCode() != wantStatus {
			t.Fatalf("lookupErrs[%d] = %v, want HTTP status %d", idx, lookupErrs[idx], wantStatus)
		}
	}

	reportedErr := previewDownloadReportedError(len(items), orderPreviewDownloadFailures(items, lookupErrs, nil))
	if !asc.IsRetryable(reportedErr) || !errors.Is(reportedErr, asc.ErrUnauthorized) {
		t.Fatalf("reported error = %v, want retryable and unauthorized identities", reportedErr)
	}
	var statusErr interface{ HTTPStatusCode() int }
	if !errors.As(reportedErr, &statusErr) || statusErr.HTTPStatusCode() != http.StatusServiceUnavailable {
		t.Fatalf("reported error = %v, want first HTTP status %d for exit-code telemetry", reportedErr, http.StatusServiceUnavailable)
	}
}

func TestResolvePreviewDownloadURLsPreservesCallerCancellationByItemIndex(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	items := []previewDownloadItem{
		{ID: "existing", URL: "https://download.example/existing.mov"},
		{ID: "missing-1"},
		{ID: "missing-2"},
	}
	lookupErrs := resolvePreviewDownloadURLs(ctx, newAssetsUploadTestClient(t), items)

	if len(lookupErrs) != len(items) {
		t.Fatalf("lookup errors = %#v, want %d index-aligned slots", lookupErrs, len(items))
	}
	if lookupErrs[0] != nil {
		t.Fatalf("existing URL lookup error = %v, want nil", lookupErrs[0])
	}
	for idx := 1; idx < len(items); idx++ {
		if !errors.Is(lookupErrs[idx], context.Canceled) {
			t.Fatalf("lookupErrs[%d] = %v, want caller cancellation", idx, lookupErrs[idx])
		}
	}
	reportedErr := previewDownloadReportedError(2, orderPreviewDownloadFailures(items, lookupErrs, nil))
	if !errors.Is(reportedErr, context.Canceled) {
		t.Fatalf("reported error = %v, want caller cancellation identity", reportedErr)
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

func TestOrderPreviewDownloadFailuresPreservesItemOrderAcrossFailureTypes(t *testing.T) {
	items := []previewDownloadItem{
		{ID: "lookup-1", OutputPath: "/tmp/lookup-1.mov"},
		{ID: "download-1", URL: "https://download.example/1.mov", OutputPath: "/tmp/download-1.mov"},
		{ID: "missing-2", OutputPath: "/tmp/missing-2.mov"},
		{ID: "download-2", URL: "https://download.example/2.mov", OutputPath: "/tmp/download-2.mov"},
	}
	lookupErr := &asc.APIError{Code: "NOT_FOUND", Detail: "preview missing", StatusCode: http.StatusNotFound}
	lookupErrs := []error{lookupErr, nil, nil, nil}
	downloadFailures := []previewDownloadFailure{
		{ID: "download-1", OutputPath: "/tmp/download-1.mov", Error: "download one failed", cause: errors.New("download one failed")},
		{ID: "download-2", OutputPath: "/tmp/download-2.mov", Error: "download two failed", cause: errors.New("download two failed")},
	}

	ordered := orderPreviewDownloadFailures(items, lookupErrs, downloadFailures)
	if len(ordered) != len(items) {
		t.Fatalf("ordered failures = %#v, want one per item", ordered)
	}
	for idx, item := range items {
		if ordered[idx].ID != item.ID {
			t.Fatalf("ordered[%d].ID = %q, want %q", idx, ordered[idx].ID, item.ID)
		}
	}
	if got, want := ordered[0].Error, "failed to fetch preview: preview missing"; got != want {
		t.Fatalf("lookup failure message = %q, want %q", got, want)
	}
	if !errors.Is(ordered[0].cause, lookupErr) {
		t.Fatalf("lookup failure cause = %v, want original lookup error", ordered[0].cause)
	}
	if got, want := ordered[2].Error, "preview has no videoUrl"; got != want {
		t.Fatalf("successful empty lookup message = %q, want %q", got, want)
	}
	if ordered[2].cause != nil {
		t.Fatalf("successful empty lookup cause = %v, want nil", ordered[2].cause)
	}

	reportedErr := previewDownloadReportedError(len(ordered), ordered)
	if !errors.Is(reportedErr, asc.ErrNotFound) {
		t.Fatalf("reported error = %v, want not-found identity", reportedErr)
	}
	var statusErr interface{ HTTPStatusCode() int }
	if !errors.As(reportedErr, &statusErr) || statusErr.HTTPStatusCode() != http.StatusNotFound {
		t.Fatalf("reported error = %v, want HTTP status %d for exit-code telemetry", reportedErr, http.StatusNotFound)
	}
}

package assets

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestConcurrentScreenshotUploadPreservesOrderAndSpeedsUp(t *testing.T) {
	files := make([]string, 40)
	for i := range files {
		files[i] = fmt.Sprintf("%02d.png", i)
	}
	previous := uploadOneScreenshot
	t.Cleanup(func() { uploadOneScreenshot = previous })
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	uploadOneScreenshot = func(_ context.Context, _ *asc.Client, _, filePath, _ string, _ openedScreenshotFiles) (asc.AssetUploadResultItem, screenshotPendingAsset, error) {
		current := inFlight.Add(1)
		for {
			seen := maxInFlight.Load()
			if current <= seen || maxInFlight.CompareAndSwap(seen, current) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		inFlight.Add(-1)
		return asc.AssetUploadResultItem{AssetID: filePath, FileName: filePath}, screenshotPendingAsset{}, nil
	}
	start := time.Now()
	progress, err := uploadScreenshotsWithOrderStateWithOpenedFiles(withScreenshotUploadConcurrency(context.Background(), 4), nil, "set", nil, files, "", false, false, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if len(progress.Results) != len(files) || len(progress.OrderedIDs) != len(files) {
		t.Fatalf("progress = %+v", progress)
	}
	for index, file := range files {
		if progress.OrderedIDs[index] != file {
			t.Fatalf("order[%d]=%s, want %s", index, progress.OrderedIDs[index], file)
		}
	}
	if max := maxInFlight.Load(); max < 2 || max > 4 {
		t.Fatalf("max in-flight = %d, want 2-4", max)
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("elapsed %s, want 40 delayed uploads under 250ms", elapsed)
	}
}

func TestConcurrentScreenshotUploadAggregatesCleanupFailures(t *testing.T) {
	uploadErr := errors.New("upload failed")
	files := []string{"01.png", "02.png", "03.png", "04.png"}
	previous := uploadOneScreenshot
	t.Cleanup(func() { uploadOneScreenshot = previous })
	uploadOneScreenshot = func(_ context.Context, _ *asc.Client, _, filePath, _ string, _ openedScreenshotFiles) (asc.AssetUploadResultItem, screenshotPendingAsset, error) {
		if filePath == files[0] {
			return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, uploadErr
		}
		return asc.AssetUploadResultItem{AssetID: "asset-" + filePath, FileName: filePath, FilePath: filePath}, screenshotPendingAsset{}, nil
	}

	var deleted []string
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
		deleted = append(deleted, id)
		if id == "asset-02.png" || id == "asset-04.png" {
			writeAssetsTestJSON(w, http.StatusInternalServerError, fmt.Sprintf(`{"errors":[{"status":"500","detail":"delete %s failed"}]}`, id))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	progress, err := uploadScreenshotsWithOrderStateWithOpenedFiles(
		withScreenshotUploadConcurrency(context.Background(), 4),
		client, "set", nil, files, "", false, false, nil,
	)
	if !errors.Is(err, uploadErr) {
		t.Fatalf("error = %v, want original upload error", err)
	}
	for _, detail := range []string{"delete asset-02.png failed", "delete asset-04.png failed"} {
		if !strings.Contains(err.Error(), detail) {
			t.Fatalf("error = %q, want cleanup detail %q", err, detail)
		}
	}
	if len(deleted) != 3 {
		t.Fatalf("deleted assets = %v, want all 3 completed siblings", deleted)
	}
	if len(progress.CleanupFailures) != 2 {
		t.Fatalf("cleanup failures = %#v, want 2 unresolved assets", progress.CleanupFailures)
	}
	if progress.CleanupFailures[0].AssetID != "asset-04.png" || progress.CleanupFailures[1].AssetID != "asset-02.png" {
		t.Fatalf("cleanup failures = %#v, want reverse cleanup order", progress.CleanupFailures)
	}
}

func TestCleanupScreenshotAssetsUsesFreshBoundedContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	requests := make(chan struct{}, 1)
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete || req.URL.Path != "/v1/appScreenshots/asset-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		requests <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))

	remaining, err := cleanupScreenshotAssets(ctx, client, []screenshotPendingAsset{{AssetID: "asset-1"}})
	if err != nil {
		t.Fatalf("cleanupScreenshotAssets() error: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("remaining assets = %#v, want none", remaining)
	}
	select {
	case <-requests:
	default:
		t.Fatal("expected cleanup DELETE after caller cancellation")
	}
}

func TestCleanupScreenshotAssetsUsesBoundedTimeout(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "20ms")
	requestCanceled := make(chan struct{}, 1)
	client := newAssetsUploadTestServerClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete || req.URL.Path != "/v1/appScreenshots/asset-1" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		<-req.Context().Done()
		requestCanceled <- struct{}{}
	}))

	started := time.Now()
	remaining, err := cleanupScreenshotAssets(context.Background(), client, []screenshotPendingAsset{{AssetID: "asset-1"}})
	if err == nil {
		t.Fatal("cleanupScreenshotAssets() error = nil, want bounded timeout error")
	}
	if len(remaining) != 1 || remaining[0].AssetID != "asset-1" {
		t.Fatalf("remaining assets = %#v, want asset-1", remaining)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cleanupScreenshotAssets() took %s, want bounded timeout", elapsed)
	}
	select {
	case <-requestCanceled:
	case <-time.After(time.Second):
		t.Fatal("server request did not observe cleanup cancellation")
	}
}

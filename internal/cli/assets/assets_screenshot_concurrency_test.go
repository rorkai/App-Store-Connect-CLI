package assets

import (
	"context"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestConcurrentScreenshotUploadPreservesOrderAndSpeedsUp(t *testing.T) {
	files := make([]string, 8)
	for i := range files {
		files[i] = string(rune('a' + i))
	}
	previous := uploadOneScreenshot
	t.Cleanup(func() { uploadOneScreenshot = previous })
	uploadOneScreenshot = func(_ context.Context, _ *asc.Client, _, filePath, _ string, _ openedScreenshotFiles) (asc.AssetUploadResultItem, screenshotPendingAsset, error) {
		time.Sleep(40 * time.Millisecond)
		return asc.AssetUploadResultItem{AssetID: filePath, FileName: filePath}, screenshotPendingAsset{}, nil
	}
	start := time.Now()
	progress, err := uploadScreenshotsWithOrderStateWithOpenedFiles(withScreenshotUploadConcurrency(context.Background(), 4), nil, "set", nil, files, "", false, false, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if len(progress.Results) != 8 || len(progress.OrderedIDs) != 8 {
		t.Fatalf("progress = %+v", progress)
	}
	for index, file := range files {
		if progress.OrderedIDs[index] != file {
			t.Fatalf("order[%d]=%s, want %s", index, progress.OrderedIDs[index], file)
		}
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("elapsed %s, want concurrency 4 to finish 8 delayed uploads under 200ms", elapsed)
	}
}

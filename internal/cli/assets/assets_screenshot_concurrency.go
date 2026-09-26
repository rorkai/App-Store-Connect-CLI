package assets

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type screenshotConcurrencyKey struct{}

const (
	defaultScreenshotUploadConcurrency = 4
	maxScreenshotUploadConcurrency     = 8
)

var uploadOneScreenshot = func(ctx context.Context, client *asc.Client, setID, filePath, sourceRootPath string, openedFiles openedScreenshotFiles) (asc.AssetUploadResultItem, screenshotPendingAsset, error) {
	if openedFile := openedScreenshotFileForPath(openedFiles, filePath); openedFile != nil {
		return uploadScreenshotAssetFromFile(ctx, client, setID, filePath, openedFile)
	}
	return uploadScreenshotAsset(ctx, client, setID, sourceRootPath, filePath)
}

func withScreenshotUploadConcurrency(ctx context.Context, concurrency int) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > maxScreenshotUploadConcurrency {
		concurrency = maxScreenshotUploadConcurrency
	}
	return context.WithValue(ctx, screenshotConcurrencyKey{}, concurrency)
}

func screenshotUploadConcurrencyFromContext(ctx context.Context) int {
	if ctx == nil {
		return 1
	}
	value, ok := ctx.Value(screenshotConcurrencyKey{}).(int)
	if !ok || value < 1 {
		return 1
	}
	if value > maxScreenshotUploadConcurrency {
		return maxScreenshotUploadConcurrency
	}
	return value
}

type screenshotUploadSlot struct {
	item    asc.AssetUploadResultItem
	pending screenshotPendingAsset
	err     error
}

func screenshotUploadItemPending(item asc.AssetUploadResultItem) screenshotPendingAsset {
	return screenshotPendingAsset{
		FileName: item.FileName,
		FilePath: item.FilePath,
		AssetID:  strings.TrimSpace(item.AssetID),
		State:    item.State,
	}
}

func cleanupScreenshotAssets(ctx context.Context, client *asc.Client, assets []screenshotPendingAsset) ([]screenshotPendingAsset, error) {
	if len(assets) == 0 {
		return nil, nil
	}
	cleanupBase := shared.ContextWithoutTimeout(ctx)
	cleanupCtx, cancel := shared.ContextWithTimeout(context.WithoutCancel(cleanupBase))
	defer cancel()

	remaining := make([]screenshotPendingAsset, 0)
	cleanupErrors := make([]error, 0)
	seen := make(map[string]struct{}, len(assets))
	for index := len(assets) - 1; index >= 0; index-- {
		asset := assets[index]
		asset.AssetID = strings.TrimSpace(asset.AssetID)
		if asset.AssetID == "" {
			continue
		}
		if _, ok := seen[asset.AssetID]; ok {
			continue
		}
		seen[asset.AssetID] = struct{}{}

		err := client.DeleteAppScreenshot(cleanupCtx, asset.AssetID)
		if err == nil || asc.IsNotFound(err) {
			continue
		}
		remaining = append(remaining, asset)
		cleanupErrors = append(cleanupErrors, fmt.Errorf("delete screenshot %q: %w", asset.AssetID, err))
	}
	return remaining, errors.Join(cleanupErrors...)
}

func uploadScreenshotsConcurrently(ctx context.Context, client *asc.Client, setID string, progress screenshotUploadProgress, files []string, sourceRootPath string, openedFiles openedScreenshotFiles, syncIfNoNew, syncAfterUpload bool, concurrency int) (screenshotUploadProgress, error) {
	slots := make([]screenshotUploadSlot, len(files))
	work := make(chan int)
	var wg sync.WaitGroup
	workerCount := concurrency
	if workerCount > len(files) {
		workerCount = len(files)
	}
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range work {
				if ctx.Err() != nil {
					slots[idx].err = ctx.Err()
					continue
				}
				filePath := files[idx]
				item, pending, err := uploadOneScreenshot(ctx, client, setID, filePath, sourceRootPath, openedFiles)
				slots[idx] = screenshotUploadSlot{item: item, pending: pending, err: err}
			}
		}()
	}
	for idx := range files {
		work <- idx
	}
	close(work)
	wg.Wait()

	failed := -1
	for idx, slot := range slots {
		if slot.err != nil {
			failed = idx
			break
		}
	}
	if failed >= 0 {
		cleanupAssets := make([]screenshotPendingAsset, 0, len(slots)-failed-1)
		for idx := failed + 1; idx < len(slots); idx++ {
			if id := strings.TrimSpace(slots[idx].item.AssetID); id != "" {
				cleanupAssets = append(cleanupAssets, screenshotUploadItemPending(slots[idx].item))
			}
			if id := strings.TrimSpace(slots[idx].pending.AssetID); id != "" {
				cleanupAssets = append(cleanupAssets, slots[idx].pending)
			}
		}
		cleanupFailures, cleanupErr := cleanupScreenshotAssets(ctx, client, cleanupAssets)
		for idx := 0; idx < failed; idx++ {
			progress.Results = append(progress.Results, slots[idx].item)
			progress.OrderedIDs = appendUniqueAssetID(progress.OrderedIDs, slots[idx].item.AssetID)
		}
		progress.PendingFiles = append([]string(nil), files[failed:]...)
		if strings.TrimSpace(slots[failed].pending.AssetID) != "" {
			progress.PendingAssets = []screenshotPendingAsset{slots[failed].pending}
		}
		progress.CleanupFailures = cleanupFailures
		progress.CleanupError = cleanupErr
		progress.UploadError = slots[failed].err
		progress.FailedFile = files[failed]
		return progress, errors.Join(slots[failed].err, cleanupErr)
	}

	for _, slot := range slots {
		progress.Results = append(progress.Results, slot.item)
		progress.OrderedIDs = appendUniqueAssetID(progress.OrderedIDs, slot.item.AssetID)
	}
	if len(progress.OrderedIDs) == 0 {
		return progress, nil
	}
	if len(progress.Results) == 0 && !syncIfNoNew {
		return progress, nil
	}
	if !syncAfterUpload {
		return progress, nil
	}
	if err := SetOrderedAppScreenshots(ctx, client, setID, progress.OrderedIDs); err != nil {
		return progress, err
	}
	return progress, nil
}

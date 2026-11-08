package assets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type screenshotConcurrencyKey struct{}

type screenshotNoWaitKey struct{}

// withScreenshotNoWait makes uploads return once each file is uploaded and
// committed (UPLOAD_COMPLETE) instead of polling until App Store Connect
// finishes processing it.
func withScreenshotNoWait(ctx context.Context, noWait bool) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, screenshotNoWaitKey{}, noWait)
}

func screenshotNoWaitFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	value, _ := ctx.Value(screenshotNoWaitKey{}).(bool)
	return value
}

const (
	// App Store Connect holds at most 10 screenshots per set, so 10 uploads a full set in one round.
	defaultScreenshotUploadConcurrency = 10
	maxScreenshotUploadConcurrency     = 10
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

type screenshotReservationTurnKey struct{}

// screenshotReservationTurn makes concurrent uploads create their screenshots
// in file order. App Store Connect appends each screenshot to its set when it
// is created, so the set keeps file order even if asc stops before the final
// reorder.
type screenshotReservationTurn struct {
	previous <-chan struct{}
	release  func()
}

func waitForScreenshotReservationTurn(ctx context.Context) error {
	turn, ok := ctx.Value(screenshotReservationTurnKey{}).(screenshotReservationTurn)
	if !ok || turn.previous == nil {
		return nil
	}
	select {
	case <-turn.previous:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseScreenshotReservationTurn(ctx context.Context) {
	if turn, ok := ctx.Value(screenshotReservationTurnKey{}).(screenshotReservationTurn); ok {
		turn.release()
	}
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
	slots, failed, cleanupFailures, cleanupErr := runScreenshotUploadPool(ctx, client, files, concurrency, func(ctx context.Context, filePath string) (asc.AssetUploadResultItem, screenshotPendingAsset, error) {
		return uploadOneScreenshot(ctx, client, setID, filePath, sourceRootPath, openedFiles)
	})
	if failed >= 0 {
		for idx := 0; idx < failed; idx++ {
			progress.Results = append(progress.Results, slots[idx].item)
			progress.OrderedIDs = appendUniqueAssetID(progress.OrderedIDs, slots[idx].item.AssetID)
		}
		progress.PendingFiles = append([]string(nil), files[failed:]...)
		if strings.TrimSpace(slots[failed].pending.AssetID) != "" {
			progress.PendingAssets = []screenshotPendingAsset{slots[failed].pending}
		}
		progress.CleanupFailures = cleanupFailures
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

// runScreenshotUploadPool uploads files with up to concurrency workers and
// keeps each slot at its file index. When an upload fails, it returns the
// first failed index and deletes the screenshots uploaded after it.
func runScreenshotUploadPool(ctx context.Context, client *asc.Client, files []string, concurrency int, upload func(context.Context, string) (asc.AssetUploadResultItem, screenshotPendingAsset, error)) ([]screenshotUploadSlot, int, []screenshotPendingAsset, error) {
	slots := make([]screenshotUploadSlot, len(files))
	reserved := make([]chan struct{}, len(files))
	for idx := range reserved {
		reserved[idx] = make(chan struct{})
	}
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
				release := sync.OnceFunc(func() { close(reserved[idx]) })
				if ctx.Err() != nil {
					slots[idx].err = ctx.Err()
					release()
					continue
				}
				turn := screenshotReservationTurn{release: release}
				if idx > 0 {
					turn.previous = reserved[idx-1]
				}
				item, pending, err := upload(context.WithValue(ctx, screenshotReservationTurnKey{}, turn), files[idx])
				release()
				slots[idx] = screenshotUploadSlot{item: item, pending: pending, err: err}
			}
		}()
	}
	for idx := range files {
		work <- idx
	}
	close(work)
	wg.Wait()

	for failed, slot := range slots {
		if slot.err == nil {
			continue
		}
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
		return slots, failed, cleanupFailures, cleanupErr
	}
	return slots, -1, nil, nil
}

// UploadScreenshotAssetsConcurrently uploads files into setID with the
// screenshot upload worker pool. Each upload runs under its own context from
// uploadContext and reads the handle from open, which it closes; a nil handle
// uploads from the path. Results keep file order. On failure it returns the
// results before the first failed file, that file, and the error, and deletes
// the screenshots uploaded after it.
func UploadScreenshotAssetsConcurrently(ctx context.Context, client *asc.Client, setID string, files []string, uploadContext func(context.Context) (context.Context, context.CancelFunc), open func(filePath string) (*os.File, error)) ([]asc.AssetUploadResultItem, string, error) {
	slots, failed, _, cleanupErr := runScreenshotUploadPool(ctx, client, files, defaultScreenshotUploadConcurrency, func(ctx context.Context, filePath string) (asc.AssetUploadResultItem, screenshotPendingAsset, error) {
		uploadCtx, cancel := uploadContext(ctx)
		defer cancel()
		// uploadContext may rebuild from an earlier parent, so carry the turn over.
		if turn, ok := ctx.Value(screenshotReservationTurnKey{}).(screenshotReservationTurn); ok {
			uploadCtx = context.WithValue(uploadCtx, screenshotReservationTurnKey{}, turn)
		}
		file, err := open(filePath)
		if err != nil {
			return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, err
		}
		if file == nil {
			sourceRootPath, err := resolveScreenshotUploadRoot("", []string{filePath})
			if err != nil {
				return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, err
			}
			return uploadScreenshotAsset(uploadCtx, client, setID, sourceRootPath, filePath)
		}
		item, pending, err := uploadScreenshotAssetFromFile(uploadCtx, client, setID, filePath, file)
		if closeErr := file.Close(); err == nil && closeErr != nil {
			return asc.AssetUploadResultItem{}, screenshotUploadItemPending(item), closeErr
		}
		return item, pending, err
	})
	results := make([]asc.AssetUploadResultItem, 0, len(files))
	end := len(slots)
	if failed >= 0 {
		end = failed
	}
	for _, slot := range slots[:end] {
		results = append(results, slot.item)
	}
	if failed >= 0 {
		return results, files[failed], errors.Join(slots[failed].err, cleanupErr)
	}
	return results, "", nil
}

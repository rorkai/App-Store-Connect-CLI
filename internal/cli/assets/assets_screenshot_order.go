package assets

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type screenshotUploadProgress struct {
	Results      []asc.AssetUploadResultItem
	OrderedIDs   []string
	PendingFiles []string
	FailedFile   string
}

// UploadScreenshotsToSet uploads screenshots concurrently while preserving the
// provided file order, and then applies that order to the remote screenshot set.
func UploadScreenshotsToSet(ctx context.Context, client *asc.Client, setID string, files []string, preserveExistingOrder bool) ([]asc.AssetUploadResultItem, error) {
	orderedIDs := make([]string, 0, len(files))
	if preserveExistingOrder {
		existingIDs, err := GetOrderedAppScreenshotIDs(ctx, client, setID)
		if err != nil {
			return nil, err
		}
		orderedIDs = append(orderedIDs, existingIDs...)
	}

	progress, err := uploadScreenshotsWithOrderState(ctx, client, setID, orderedIDs, files, false, true)
	if err != nil {
		return nil, err
	}
	return progress.Results, nil
}

func uploadScreenshotsWithOrderState(ctx context.Context, client *asc.Client, setID string, orderedIDs, files []string, syncIfNoNew, syncAfterUpload bool) (screenshotUploadProgress, error) {
	progress := screenshotUploadProgress{
		Results:    make([]asc.AssetUploadResultItem, 0, len(files)),
		OrderedIDs: append([]string(nil), orderedIDs...),
	}

	items := make([]asc.AssetUploadResultItem, len(files))
	taskErrs := forEachAssetTask(ctx, len(files), true, func(taskCtx context.Context, idx int) error {
		item, err := uploadScreenshotAsset(taskCtx, client, setID, files[idx])
		items[idx] = item
		if err != nil {
			return err
		}
		return nil
	})

	for idx := range files {
		if taskErrs[idx] != nil {
			continue
		}
		progress.Results = append(progress.Results, items[idx])
		progress.OrderedIDs = appendUniqueScreenshotID(progress.OrderedIDs, items[idx].AssetID)
	}

	if failedIdx, failedErr := firstAssetTaskError(taskErrs); failedErr != nil {
		cleanupCtx, cleanupCancel := contextWithAssetCleanupTimeout(ctx)
		defer cleanupCancel()

		var rollbackErrs []error
		for idx, taskErr := range taskErrs {
			if taskErr != nil {
				progress.PendingFiles = append(progress.PendingFiles, files[idx])
				if strings.TrimSpace(items[idx].AssetID) != "" {
					if err := client.DeleteAppScreenshot(cleanupCtx, items[idx].AssetID); err != nil {
						rollbackErrs = append(rollbackErrs, fmt.Errorf("delete partial screenshot %q: %w", items[idx].AssetID, err))
					}
				}
			}
		}
		progress.FailedFile = files[failedIdx]
		uploadErr := aggregateAssetTaskErrors(taskErrs)
		if len(rollbackErrs) > 0 {
			uploadErr = fmt.Errorf("screenshot upload failed and rollback was incomplete: %w", errors.Join(append([]error{uploadErr}, rollbackErrs...)...))
		}
		return progress, uploadErr
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

// GetOrderedAppScreenshotIDs returns screenshot IDs in the current remote order.
func GetOrderedAppScreenshotIDs(ctx context.Context, client *asc.Client, setID string) ([]string, error) {
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}

	firstPage, err := client.GetAppScreenshotSetAppScreenshotsRelationships(ctx, setID, asc.WithLinkagesLimit(200))
	if err != nil {
		return nil, err
	}

	orderedIDs := make([]string, 0, len(firstPage.Data))
	err = asc.PaginateEach(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppScreenshotSetAppScreenshotsRelationships(ctx, "", asc.WithLinkagesNextURL(nextURL))
	}, func(page asc.PaginatedResponse) error {
		linkages, ok := page.(*asc.LinkagesResponse)
		if !ok {
			return fmt.Errorf("unexpected screenshot relationship response type %T", page)
		}
		for _, item := range linkages.Data {
			orderedIDs = appendUniqueScreenshotID(orderedIDs, item.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return orderedIDs, nil
}

// SetOrderedAppScreenshots replaces the screenshot relationships for a set in the provided order.
func SetOrderedAppScreenshots(ctx context.Context, client *asc.Client, setID string, orderedIDs []string) error {
	if client == nil {
		return fmt.Errorf("client is required")
	}
	return client.UpdateAppScreenshotSetAppScreenshotsRelationship(ctx, setID, normalizeScreenshotIDs(orderedIDs))
}

func normalizeScreenshotIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(ids))
	normalized := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}

func appendUniqueScreenshotID(ids []string, id string) []string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ids
	}
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

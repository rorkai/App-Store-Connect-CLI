package assets

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type screenshotUploadProgress struct {
	Results         []asc.AssetUploadResultItem
	OrderedIDs      []string
	PendingFiles    []string
	PendingAssets   []screenshotPendingAsset
	CleanupFailures []screenshotPendingAsset
	FailedFile      string
}

func uploadScreenshotsToSetFromRootWithOpenedFiles(ctx context.Context, client *asc.Client, setID string, files []string, sourceRootPath string, preserveExistingOrder bool, openedFiles openedScreenshotFiles) ([]asc.AssetUploadResultItem, error) {
	orderedIDs := make([]string, 0, len(files))
	if preserveExistingOrder {
		existingIDs, err := GetOrderedAppScreenshotIDs(ctx, client, setID)
		if err != nil {
			return nil, err
		}
		orderedIDs = append(orderedIDs, existingIDs...)
	}

	progress, err := uploadScreenshotsWithOrderStateWithOpenedFiles(ctx, client, setID, orderedIDs, files, sourceRootPath, false, true, openedFiles)
	return progress.Results, err
}

func uploadScreenshotsWithOrderState(ctx context.Context, client *asc.Client, setID string, orderedIDs, files []string, sourceRootPath string, syncIfNoNew, syncAfterUpload bool) (screenshotUploadProgress, error) {
	return uploadScreenshotsWithOrderStateWithOpenedFiles(ctx, client, setID, orderedIDs, files, sourceRootPath, syncIfNoNew, syncAfterUpload, nil)
}

func uploadScreenshotsWithOrderStateWithOpenedFiles(ctx context.Context, client *asc.Client, setID string, orderedIDs, files []string, sourceRootPath string, syncIfNoNew, syncAfterUpload bool, openedFiles openedScreenshotFiles) (screenshotUploadProgress, error) {
	progress := screenshotUploadProgress{
		Results:    make([]asc.AssetUploadResultItem, 0, len(files)),
		OrderedIDs: append([]string(nil), orderedIDs...),
	}

	concurrency := screenshotUploadConcurrencyFromContext(ctx)
	if concurrency > 1 && len(files) > 1 {
		return uploadScreenshotsConcurrently(ctx, client, setID, progress, files, sourceRootPath, openedFiles, syncIfNoNew, syncAfterUpload, concurrency)
	}

	for idx, filePath := range files {
		var item asc.AssetUploadResultItem
		var pending screenshotPendingAsset
		var err error
		if openedFile := openedScreenshotFileForPath(openedFiles, filePath); openedFile != nil {
			item, pending, err = uploadScreenshotAssetFromFile(ctx, client, setID, filePath, openedFile)
		} else {
			item, pending, err = uploadScreenshotAsset(ctx, client, setID, sourceRootPath, filePath)
		}
		if err != nil {
			progress.PendingFiles = append([]string{filePath}, files[idx+1:]...)
			if strings.TrimSpace(pending.AssetID) != "" {
				progress.PendingAssets = []screenshotPendingAsset{pending}
			}
			progress.FailedFile = filePath
			return progress, err
		}
		progress.Results = append(progress.Results, item)
		progress.OrderedIDs = appendUniqueAssetID(progress.OrderedIDs, item.AssetID)
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

func resumeScreenshotsWithOrderState(ctx context.Context, client *asc.Client, setID string, orderedIDs, files []string, pendingAssets []screenshotPendingAsset, sourceRootPath string, syncIfNoNew, syncAfterUpload bool) (screenshotUploadProgress, error) {
	progress := screenshotUploadProgress{
		Results:    make([]asc.AssetUploadResultItem, 0, len(files)),
		OrderedIDs: append([]string(nil), orderedIDs...),
	}
	remainingFiles := append([]string(nil), files...)

	if len(pendingAssets) > 1 {
		progress.PendingFiles = remainingFiles
		progress.PendingAssets = append([]screenshotPendingAsset(nil), pendingAssets...)
		if len(remainingFiles) > 0 {
			progress.FailedFile = remainingFiles[0]
		}
		return progress, fmt.Errorf("resume artifact contains multiple in-flight screenshot assets")
	}
	if len(pendingAssets) == 1 {
		pending := pendingAssets[0]
		if len(remainingFiles) == 0 || filepath.Clean(pending.FilePath) != filepath.Clean(remainingFiles[0]) {
			progress.PendingFiles = remainingFiles
			progress.PendingAssets = []screenshotPendingAsset{pending}
			if len(remainingFiles) > 0 {
				progress.FailedFile = remainingFiles[0]
			}
			return progress, fmt.Errorf("pending screenshot asset does not match the first pending file")
		}

		result, updatedPending, retryUpload, err := reconcilePendingScreenshotAsset(ctx, client, pending, sourceRootPath)
		if err != nil {
			progress.PendingFiles = remainingFiles
			progress.PendingAssets = []screenshotPendingAsset{updatedPending}
			progress.FailedFile = remainingFiles[0]
			return progress, err
		}
		if !retryUpload {
			progress.Results = append(progress.Results, result)
			progress.OrderedIDs = appendUniqueAssetID(progress.OrderedIDs, result.AssetID)
			remainingFiles = remainingFiles[1:]
		}
	}

	remaining, err := uploadScreenshotsWithOrderState(ctx, client, setID, progress.OrderedIDs, remainingFiles, sourceRootPath, syncIfNoNew, syncAfterUpload)
	progress.Results = append(progress.Results, remaining.Results...)
	progress.OrderedIDs = remaining.OrderedIDs
	progress.PendingFiles = remaining.PendingFiles
	progress.PendingAssets = remaining.PendingAssets
	progress.CleanupFailures = remaining.CleanupFailures
	progress.FailedFile = remaining.FailedFile
	return progress, err
}

func reconcilePendingScreenshotAsset(ctx context.Context, client *asc.Client, pending screenshotPendingAsset, sourceRootPath string) (asc.AssetUploadResultItem, screenshotPendingAsset, bool, error) {
	remote, err := client.GetAppScreenshot(ctx, pending.AssetID)
	if err != nil {
		if asc.IsNotFound(err) {
			return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, true, nil
		}
		return asc.AssetUploadResultItem{}, pending, false, err
	}

	remoteState := ""
	if remote.Data.Attributes.AssetDeliveryState != nil {
		remoteState = strings.ToUpper(strings.TrimSpace(remote.Data.Attributes.AssetDeliveryState.State))
	}
	switch remoteState {
	case "COMPLETE":
		if err := validatePendingScreenshotChecksum(sourceRootPath, pending); err != nil {
			return asc.AssetUploadResultItem{}, pending, false, err
		}
		if checksum := strings.TrimSpace(remote.Data.Attributes.SourceFileChecksum); checksum != "" {
			if err := validatePendingScreenshotRemoteChecksum(pending.AssetID, checksum, pending.Checksum); err != nil {
				return asc.AssetUploadResultItem{}, pending, false, err
			}
			return completedPendingScreenshotResult(pending), screenshotPendingAsset{}, false, nil
		}
		return waitForPendingScreenshotDelivery(ctx, client, pending)
	case "FAILED":
		if err := client.DeleteAppScreenshot(ctx, pending.AssetID); err != nil {
			return asc.AssetUploadResultItem{}, pending, false, fmt.Errorf("delete failed screenshot reservation %s: %w", pending.AssetID, err)
		}
		return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, true, nil
	case "UPLOAD_COMPLETE":
		if err := validatePendingScreenshotChecksum(sourceRootPath, pending); err != nil {
			return asc.AssetUploadResultItem{}, pending, false, err
		}
		if err := validatePendingScreenshotRemoteChecksum(pending.AssetID, remote.Data.Attributes.SourceFileChecksum, pending.Checksum); err != nil {
			return asc.AssetUploadResultItem{}, pending, false, err
		}
		return waitForPendingScreenshotDelivery(ctx, client, pending)
	case "AWAITING_UPLOAD", "":
		pendingState := strings.ToUpper(strings.TrimSpace(pending.State))
		if pendingState == "UPLOADED" || pendingState == "UPLOAD_COMPLETE" || pendingState == "COMPLETE" {
			if err := validatePendingScreenshotChecksum(sourceRootPath, pending); err != nil {
				return asc.AssetUploadResultItem{}, pending, false, err
			}
			state, err := commitAndWaitForScreenshotDelivery(ctx, client, pending, true)
			return finishPendingScreenshotDelivery(ctx, pending, state, err)
		}

		if err := client.DeleteAppScreenshot(ctx, pending.AssetID); err != nil {
			return asc.AssetUploadResultItem{}, pending, false, fmt.Errorf("delete incomplete screenshot reservation %s: %w", pending.AssetID, err)
		}
		return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, true, nil
	default:
		pending.State = remoteState
		return asc.AssetUploadResultItem{}, pending, false, fmt.Errorf("screenshot %s has unrecognized delivery state %q", pending.AssetID, remoteState)
	}
}

func validatePendingScreenshotChecksum(sourceRootPath string, pending screenshotPendingAsset) error {
	checksum, err := computeFileChecksumInRoot(sourceRootPath, pending.FilePath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(checksum), strings.TrimSpace(pending.Checksum)) {
		return fmt.Errorf("pending screenshot file changed after upload: %q", pending.FilePath)
	}
	return nil
}

func validatePendingScreenshotRemoteChecksum(assetID, actual, expected string) error {
	actual, expected = strings.TrimSpace(actual), strings.TrimSpace(expected)
	if actual != "" && expected != "" && !strings.EqualFold(actual, expected) {
		return fmt.Errorf("pending screenshot source checksum does not match for asset %s", assetID)
	}
	return nil
}

func waitForPendingScreenshotDelivery(ctx context.Context, client *asc.Client, pending screenshotPendingAsset) (asc.AssetUploadResultItem, screenshotPendingAsset, bool, error) {
	state, err := waitForScreenshotDelivery(ctx, client, pending.AssetID, pending.Checksum)
	return finishPendingScreenshotDelivery(ctx, pending, state, err)
}

func finishPendingScreenshotDelivery(ctx context.Context, pending screenshotPendingAsset, state string, err error) (asc.AssetUploadResultItem, screenshotPendingAsset, bool, error) {
	if strings.TrimSpace(state) != "" {
		pending.State = strings.ToUpper(strings.TrimSpace(state))
	}
	if err != nil {
		return asc.AssetUploadResultItem{}, pending, false, err
	}
	result := completedPendingScreenshotResult(pending)
	if screenshotNoWaitFromContext(ctx) {
		// --no-wait only checked the state once; report it instead of claiming COMPLETE.
		result.State = pending.State
	}
	return result, screenshotPendingAsset{}, false, nil
}

func completedPendingScreenshotResult(pending screenshotPendingAsset) asc.AssetUploadResultItem {
	return asc.AssetUploadResultItem{
		FileName: pending.FileName,
		FilePath: pending.FilePath,
		AssetID:  pending.AssetID,
		State:    "COMPLETE",
	}
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

	return collectOrderedLinkageIDs(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppScreenshotSetAppScreenshotsRelationships(ctx, "", asc.WithLinkagesNextURL(nextURL))
	})
}

// SetOrderedAppScreenshots replaces the screenshot relationships for a set in the provided order.
func SetOrderedAppScreenshots(ctx context.Context, client *asc.Client, setID string, orderedIDs []string) error {
	if client == nil {
		return fmt.Errorf("client is required")
	}
	return client.UpdateAppScreenshotSetAppScreenshotsRelationship(ctx, setID, normalizeAssetIDs(orderedIDs))
}

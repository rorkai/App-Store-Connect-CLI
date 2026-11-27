package assets

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func syncScreenshotSetIncrementally[T any](ctx context.Context, cfg screenshotUploadConfig[T]) (T, error) {
	var zero T
	if cfg.Client == nil {
		return zero, fmt.Errorf("client is required")
	}
	if cfg.RequestContext == nil {
		cfg.RequestContext = cfg.Deps.Timeout
	}
	if len(cfg.Files) > appScreenshotSetMaxScreenshots {
		return zero, fmt.Errorf("screenshot set limit is %d files", appScreenshotSetMaxScreenshots)
	}
	root, err := resolveScreenshotUploadRoot(cfg.Deps, cfg.RootPath, cfg.Files)
	if err != nil {
		return zero, err
	}
	opened, err := openAndValidateScreenshotFiles(root, cfg.DisplayType, cfg.Files)
	if err != nil {
		return zero, err
	}
	defer closeOpenedScreenshotFiles(opened)
	checksums := map[string]string{}
	checksumSources := map[string]string{}
	for _, path := range cfg.Files {
		f := openedScreenshotFileForPath(opened, path)
		sum, err := asc.ComputeChecksumFromReader(f, asc.ChecksumAlgorithmMD5)
		if err != nil {
			return zero, err
		}
		if _, err = f.Seek(0, io.SeekStart); err != nil {
			return zero, err
		}
		if previous, ok := checksumSources[sum.Hash]; ok {
			return zero, fmt.Errorf("incremental sync requires distinct source checksums: %q and %q", previous, path)
		}
		checksumSources[sum.Hash] = path
		checksums[path] = sum.Hash
	}
	set, err := findScreenshotSetWithAccess(ctx, cfg.Client, cfg.LocalizationID, cfg.DisplayType, cfg.Access, cfg.RequestContext)
	if err != nil {
		return zero, err
	}
	if set.ID == "" {
		set.Attributes.ScreenshotDisplayType = cfg.DisplayType
	}
	var existing []asc.Resource[asc.AppScreenshotAttributes]
	var order []string
	if set.ID != "" {
		existing, order, err = replacementInventory(ctx, cfg, set.ID)
		if err != nil {
			return zero, err
		}
	}
	for i, item := range existing {
		checksum, valid := replacementChecksum(item.Attributes.SourceFileChecksum)
		if !valid || item.Attributes.AssetDeliveryState == nil || item.Attributes.AssetDeliveryState.State != "COMPLETE" {
			return zero, fmt.Errorf("cannot verify screenshot %s: incremental sync requires COMPLETE assets with source checksums; inspect the set before retrying", item.ID)
		}
		existing[i].Attributes.SourceFileChecksum = checksum
	}
	files, kept, err := filterScreenshotFilesByChecksum(cfg.Files, existing, cfg.InspectCommand, func(path string) (string, error) { return checksums[path], nil })
	if err != nil {
		return zero, err
	}
	retained := map[string]bool{}
	for _, item := range kept {
		retained[item.AssetID] = true
	}
	obsolete := []asc.Resource[asc.AppScreenshotAttributes]{}
	for _, item := range existing {
		if !retained[item.ID] {
			obsolete = append(obsolete, item)
		}
	}
	deleteBeforeUpload := max(0, len(existing)+len(files)-appScreenshotSetMaxScreenshots)
	results := append([]asc.AssetUploadResultItem{}, kept...)
	finish := func(err error) (T, error) { return cfg.BuildResult(cfg.LocalizationID, set, cfg.DryRun, results), err }
	if cfg.DryRun {
		if len(files) == 0 && len(obsolete) == 0 && !sameAssetIDOrder(order, orderAssetIDsForLocalFiles(nil, cfg.Files, kept, nil)) {
			for i := range results {
				results[i].State = "would-reorder"
			}
		}
		appendDeletions := func(items []asc.Resource[asc.AppScreenshotAttributes]) {
			for _, item := range items {
				results = append(results, asc.AssetUploadResultItem{AssetID: item.ID, FileName: item.Attributes.FileName, State: "would-delete"})
			}
		}
		appendDeletions(obsolete[:deleteBeforeUpload])
		for _, path := range files {
			results = append(results, asc.AssetUploadResultItem{FileName: filepath.Base(path), FilePath: path, State: "would-upload"})
		}
		appendDeletions(obsolete[deleteBeforeUpload:])
		return finish(nil)
	}
	if cfg.UploadContext == nil {
		cfg.UploadContext = assetUploadContext(cfg.Deps)
	}
	uploadCtx, cancel := cfg.UploadContext(withAssetUploadWork(ctx, files))
	defer cancel()
	if set.ID == "" {
		set, err = ensureScreenshotSetWithAccess(uploadCtx, cfg.Client, cfg.LocalizationID, cfg.DisplayType, cfg.Access, cfg.RequestContext)
		if err != nil {
			return zero, err
		}
		current, _, err := replacementInventory(uploadCtx, cfg, set.ID)
		if err != nil {
			return finish(err)
		}
		if len(current) != 0 {
			return finish(fmt.Errorf("new screenshot set is no longer empty; inspect it before syncing"))
		}
	}
	if len(obsolete) > 0 {
		current, currentOrder, err := replacementInventory(uploadCtx, cfg, set.ID)
		if err != nil {
			return finish(err)
		}
		if !sameReplacementInventory(existing, current) || !sameAssetIDOrder(order, currentOrder) {
			return finish(fmt.Errorf("screenshot set changed before incremental deletion; preview a fresh sync"))
		}
	}
	expected := append([]asc.Resource[asc.AppScreenshotAttributes]{}, existing...)
	deleteAssets := func(items []asc.Resource[asc.AppScreenshotAttributes]) error {
		for _, item := range items {
			if err := cfg.Client.DeleteAppScreenshot(uploadCtx, item.ID); err != nil {
				return err
			}
			results = append(results, asc.AssetUploadResultItem{AssetID: item.ID, FileName: item.Attributes.FileName, State: "deleted"})
			remaining := expected[:0]
			for _, current := range expected {
				if current.ID != item.ID {
					remaining = append(remaining, current)
				}
			}
			expected = remaining
		}
		return nil
	}
	if err := deleteAssets(obsolete[:deleteBeforeUpload]); err != nil {
		return finish(err)
	}
	progress, err := uploadScreenshotsWithOrderStateWithOpenedFiles(uploadCtx, cfg.Deps.IO.ErrOut, cfg.RequestContext, cfg.Client, set.ID, nil, files, root, false, false, opened)
	results = append(results, progress.Results...)
	if err != nil {
		for _, pending := range progress.PendingAssets {
			results = append(results, asc.AssetUploadResultItem{AssetID: pending.AssetID, FileName: pending.FileName, FilePath: pending.FilePath, State: pending.State})
		}
		return finish(err)
	}
	for _, item := range progress.Results {
		expected = append(expected, asc.Resource[asc.AppScreenshotAttributes]{Type: asc.ResourceTypeAppScreenshots, ID: item.AssetID, Attributes: asc.AppScreenshotAttributes{SourceFileChecksum: checksums[item.FilePath], AssetDeliveryState: &asc.AssetDeliveryState{State: "COMPLETE"}}})
	}
	if len(files) > 0 && len(obsolete) > deleteBeforeUpload {
		current, _, err := replacementInventory(uploadCtx, cfg, set.ID)
		if err != nil {
			return finish(err)
		}
		if !sameReplacementInventory(expected, current) {
			return finish(fmt.Errorf("screenshot set changed before incremental deletion; preview a fresh sync"))
		}
	}
	if err := deleteAssets(obsolete[deleteBeforeUpload:]); err != nil {
		return finish(err)
	}
	desired := orderAssetIDsForLocalFiles(nil, cfg.Files, kept, progress.Results)
	inventory, current, err := replacementInventory(uploadCtx, cfg, set.ID)
	if err != nil {
		return finish(err)
	}
	if !sameReplacementInventory(expected, inventory) {
		return finish(fmt.Errorf("screenshot set changed before incremental ordering; inspect the set"))
	}
	if !sameAssetIDOrder(current, desired) {
		if err := SetOrderedAppScreenshots(uploadCtx, cfg.Client, set.ID, desired); err != nil {
			return finish(err)
		}
		_, confirmed, err := replacementInventory(uploadCtx, cfg, set.ID)
		if err != nil {
			return finish(err)
		}
		if !sameAssetIDOrder(confirmed, desired) {
			return finish(fmt.Errorf("incremental screenshot order was not confirmed by App Store Connect"))
		}
	}
	return finish(nil)
}

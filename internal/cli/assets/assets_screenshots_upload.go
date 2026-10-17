package assets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

var screenshotSettlementPollInterval = assetPollInterval

func normalizeScreenshotDisplayType(input string) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(input))
	if value == "" {
		return "", fmt.Errorf("device type is required")
	}
	if !strings.HasPrefix(value, "APP_") && !strings.HasPrefix(value, "IMESSAGE_") {
		value = "APP_" + value
	}
	if !asc.IsValidScreenshotDisplayType(value) {
		return "", fmt.Errorf("unsupported screenshot display type %q", value)
	}
	return value, nil
}

func validateScreenshotDimensions(files []string, displayType string) error {
	for _, filePath := range files {
		file, err := rootfs.OpenFile(filePath)
		if err != nil {
			return markMediaInput(err, shared.DiagnosticFileInvalidFormat)
		}
		_, alphaErr := asc.ReadAppStoreImageFormatFrom(file)
		closeErr := file.Close()
		if alphaErr != nil {
			return markMediaInput(fmt.Errorf("%q: %w", filePath, alphaErr), shared.DiagnosticFileInvalidFormat)
		}
		if closeErr != nil {
			return closeErr
		}
		if err := asc.ValidateScreenshotDimensions(filePath, displayType); err != nil {
			return markMediaInput(err, shared.DiagnosticInvalidInput)
		}
	}
	return nil
}

func uploadScreenshots(ctx context.Context, client *asc.Client, localizationID, displayType string, files []string, skipExisting, replace, dryRun bool) (asc.AppScreenshotUploadResult, error) {
	result, err := uploadScreenshotsWithConfig(ctx, screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Client:         client,
		LocalizationID: localizationID,
		DisplayType:    displayType,
		Files:          files,
		SkipExisting:   skipExisting,
		Replace:        replace,
		DryRun:         dryRun,
		InspectCommand: screenshotInspectionCommand(localizationID),
		RequestContext: shared.ContextWithTimeout,
		UploadContext:  contextWithAssetUploadTimeout,
		Access:         appStoreVersionScreenshotSetAccess,
		BuildResult: func(localizationID string, set asc.Resource[asc.AppScreenshotSetAttributes], dryRun bool, results []asc.AssetUploadResultItem) asc.AppScreenshotUploadResult {
			return buildAppScreenshotUploadResult(localizationID, set, dryRun, results)
		},
	})
	return result, err
}

func findScreenshotSetWithAccess(ctx context.Context, client *asc.Client, localizationID, displayType string, access ScreenshotSetAccess, requestContext asc.RequestContextFunc) (asc.Resource[asc.AppScreenshotSetAttributes], error) {
	if access.List == nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, fmt.Errorf("screenshot set list function is required")
	}
	if requestContext == nil {
		requestContext = shared.ContextWithTimeout
	}

	resp, err := access.List(ctx, client, localizationID, requestContext)
	if err != nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, err
	}
	for _, set := range resp.Data {
		if strings.EqualFold(set.Attributes.ScreenshotDisplayType, displayType) {
			return set, nil
		}
	}
	return asc.Resource[asc.AppScreenshotSetAttributes]{
		Attributes: asc.AppScreenshotSetAttributes{ScreenshotDisplayType: displayType},
	}, nil
}

func ensureScreenshotSetWithAccess(ctx context.Context, client *asc.Client, localizationID, displayType string, access ScreenshotSetAccess, requestContext asc.RequestContextFunc) (asc.Resource[asc.AppScreenshotSetAttributes], error) {
	if access.Create == nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, fmt.Errorf("screenshot set create function is required")
	}
	if requestContext == nil {
		requestContext = shared.ContextWithTimeout
	}

	set, err := findScreenshotSetWithAccess(ctx, client, localizationID, displayType, access, requestContext)
	if err != nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, err
	}
	if set.ID != "" {
		return set, nil
	}

	createCtx, cancel := requestContext(ctx)
	created, err := access.Create(createCtx, client, localizationID, displayType)
	cancel()
	if err != nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, err
	}
	return created.Data, nil
}

func uploadScreenshotsWithConfig[T any](ctx context.Context, cfg screenshotUploadConfig[T]) (T, error) {
	var zero T

	if cfg.Client == nil {
		return zero, fmt.Errorf("client is required")
	}
	if cfg.BuildResult == nil {
		return zero, fmt.Errorf("build result function is required")
	}
	if cfg.RequestContext == nil {
		cfg.RequestContext = shared.ContextWithTimeout
	}
	if cfg.UploadContext == nil {
		cfg.UploadContext = contextWithAssetUploadTimeout
	}
	if strings.TrimSpace(cfg.InspectCommand) == "" {
		cfg.InspectCommand = screenshotInspectionCommand(cfg.LocalizationID)
	}
	if strings.TrimSpace(cfg.ReplaceCommand) == "" {
		cfg.ReplaceCommand = "--replace --confirm"
	}

	sourceRootPath := ""
	if len(cfg.Files) > 0 {
		var err error
		sourceRootPath, err = resolveScreenshotUploadRoot(cfg.RootPath, cfg.Files)
		if err != nil {
			return zero, fmt.Errorf("resolve screenshot source root: %w", err)
		}
	}

	var (
		set asc.Resource[asc.AppScreenshotSetAttributes]
		err error
	)
	if cfg.DryRun {
		set, err = findScreenshotSetWithAccess(ctx, cfg.Client, cfg.LocalizationID, cfg.DisplayType, cfg.Access, cfg.RequestContext)
	} else {
		set, err = ensureScreenshotSetWithAccess(ctx, cfg.Client, cfg.LocalizationID, cfg.DisplayType, cfg.Access, cfg.RequestContext)
	}
	if err != nil {
		return zero, err
	}

	existingScreenshots := make([]asc.Resource[asc.AppScreenshotAttributes], 0)
	if (cfg.SkipExisting || cfg.Replace || (!cfg.Replace && len(cfg.Files) > 0)) && set.ID != "" {
		existingResp, err := cfg.Client.GetAllAppScreenshots(ctx, set.ID, asc.WithAppScreenshotsRequestContext(cfg.RequestContext))
		if err != nil {
			return zero, err
		}
		existingScreenshots = existingResp.Data
	}
	if cfg.SkipExisting && len(existingScreenshots) > 0 {
		settleCtx, settleCancel := cfg.RequestContext(ctx)
		existingScreenshots, err = settleExistingScreenshotChecksums(settleCtx, cfg.Client, existingScreenshots)
		settleCancel()
		if err != nil {
			return zero, err
		}
	}

	skippedResults := make([]asc.AssetUploadResultItem, 0)
	files := cfg.Files
	if cfg.SkipExisting {
		var filterErr error
		files, skippedResults, filterErr = filterExistingScreenshotFiles(cfg.Files, existingScreenshots, cfg.InspectCommand)
		if filterErr != nil {
			return zero, filterErr
		}
	}
	files, err = limitScreenshotUploadFilesForExistingSet(files, cfg.MaxScreenshots, existingScreenshots, cfg.Replace, set.ID, cfg.InspectCommand, cfg.ReplaceCommand)
	if err != nil {
		return zero, err
	}

	if cfg.DryRun {
		results := make([]asc.AssetUploadResultItem, 0, len(skippedResults)+len(files)+len(existingScreenshots))
		if cfg.Replace {
			for _, screenshot := range existingScreenshots {
				results = append(results, asc.AssetUploadResultItem{
					FileName: screenshot.Attributes.FileName,
					AssetID:  screenshot.ID,
					State:    "would-delete",
				})
			}
		}
		for _, filePath := range files {
			results = append(results, asc.AssetUploadResultItem{
				FileName: filepath.Base(filePath),
				FilePath: filePath,
				State:    "would-upload",
			})
		}
		results = append(results, skippedResults...)
		return cfg.BuildResult(cfg.LocalizationID, set, true, results), nil
	}

	var openedFiles openedScreenshotFiles
	if cfg.Replace && len(files) > 0 {
		openedFiles, err = openAndValidateScreenshotFiles(sourceRootPath, files)
		if err != nil {
			return zero, err
		}
		defer closeOpenedScreenshotFiles(openedFiles)
	}

	uploadCtx, cancel := cfg.UploadContext(ctx)
	defer cancel()

	if cfg.Replace {
		if err := deleteExistingScreenshots(uploadCtx, cfg.Client, existingScreenshots); err != nil {
			return zero, err
		}
	}

	results := make([]asc.AssetUploadResultItem, 0, len(skippedResults)+len(files))
	if len(files) > 0 {
		uploadedResults, err := uploadScreenshotsToSetFromRootWithOpenedFiles(uploadCtx, cfg.Client, set.ID, files, sourceRootPath, !cfg.Replace, openedFiles)
		if err != nil {
			return zero, err
		}
		results = append(results, uploadedResults...)
	}
	if cfg.SkipExisting && len(skippedResults) > 0 {
		if _, err := syncSkippedScreenshotOrder(uploadCtx, cfg.Client, set.ID, cfg.Files, skippedResults, results); err != nil {
			results = append(skippedResults, results...)
			return cfg.BuildResult(cfg.LocalizationID, set, false, results), err
		}
	}
	results = append(skippedResults, results...)

	return cfg.BuildResult(cfg.LocalizationID, set, false, results), nil
}

func deleteExistingScreenshots(ctx context.Context, client *asc.Client, screenshots []asc.Resource[asc.AppScreenshotAttributes]) error {
	for _, screenshot := range screenshots {
		if err := client.DeleteAppScreenshot(ctx, screenshot.ID); err != nil {
			return err
		}
	}
	return nil
}

func filterExistingScreenshotFiles(files []string, screenshots []asc.Resource[asc.AppScreenshotAttributes], inspectCommand string) ([]string, []asc.AssetUploadResultItem, error) {
	existingByChecksum := make(map[string][]asc.Resource[asc.AppScreenshotAttributes], len(screenshots))
	for _, screenshot := range screenshots {
		checksum := strings.TrimSpace(screenshot.Attributes.SourceFileChecksum)
		if checksum == "" {
			continue
		}
		matches := existingByChecksum[checksum]
		seen := false
		for _, existing := range matches {
			if screenshot.ID != "" && screenshot.ID == existing.ID {
				seen = true
				break
			}
		}
		if !seen {
			existingByChecksum[checksum] = append(matches, screenshot)
		}
	}

	filtered := make([]string, 0, len(files))
	skipped := make([]asc.AssetUploadResultItem, 0)
	for _, filePath := range files {
		checksum, err := screenshotFileChecksumFunc(filePath)
		if err != nil {
			return nil, nil, err
		}
		matches := existingByChecksum[checksum]
		if len(matches) > 1 {
			ids := make([]string, 0, len(matches))
			for _, screenshot := range matches {
				id := strings.TrimSpace(screenshot.ID)
				if id == "" {
					id = "<missing asset ID>"
				}
				ids = append(ids, id)
			}
			sort.Strings(ids)
			quotedIDs := make([]string, 0, len(ids))
			deleteCommands := make([]string, 0, len(ids))
			for _, id := range ids {
				quotedIDs = append(quotedIDs, fmt.Sprintf("%q", id))
				if id != "<missing asset ID>" {
					deleteCommands = append(deleteCommands, fmt.Sprintf("asc screenshots delete --id %q --confirm", id))
				}
			}
			if strings.TrimSpace(inspectCommand) == "" {
				inspectCommand = screenshotInspectionCommand("")
			}
			deleteGuidance := fmt.Sprintf("delete only the unwanted duplicate with asc screenshots delete --id %q --confirm", "SCREENSHOT_ID")
			if len(deleteCommands) > 0 {
				deleteGuidance = fmt.Sprintf("retain one matching screenshot and delete every other duplicate by running each applicable command except the retained ID: %s", strings.Join(deleteCommands, "; "))
			}
			return nil, nil, fmt.Errorf(
				"local screenshot %q matches multiple remote screenshots by checksum (asset IDs: %s); ASC cannot choose one safely, so no remote assets were changed for this screenshot set. Inspect them with %s, then %s and retry --skip-existing",
				filepath.Base(filePath),
				strings.Join(quotedIDs, ", "),
				inspectCommand,
				deleteGuidance,
			)
		}
		if len(matches) == 1 {
			existing := matches[0]
			skipped = append(skipped, asc.AssetUploadResultItem{
				FileName: filepath.Base(filePath),
				FilePath: filePath,
				AssetID:  existing.ID,
				State:    "skipped",
				Skipped:  true,
			})
			continue
		}
		filtered = append(filtered, filePath)
	}

	return filtered, skipped, nil
}

func screenshotInspectionCommand(localizationID string) string {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		localizationID = "VERSION_LOCALIZATION_ID"
	}
	return fmt.Sprintf("asc screenshots list --version-localization %q --output json", localizationID)
}

func settleExistingScreenshotChecksums(ctx context.Context, client *asc.Client, screenshots []asc.Resource[asc.AppScreenshotAttributes]) ([]asc.Resource[asc.AppScreenshotAttributes], error) {
	settled := append([]asc.Resource[asc.AppScreenshotAttributes](nil), screenshots...)
	for index := range settled {
		if strings.TrimSpace(settled[index].Attributes.SourceFileChecksum) != "" {
			continue
		}
		deliveryState := settled[index].Attributes.AssetDeliveryState
		if deliveryState == nil || !strings.EqualFold(strings.TrimSpace(deliveryState.State), "COMPLETE") {
			continue
		}
		assetID := strings.TrimSpace(settled[index].ID)
		if assetID == "" {
			return nil, fmt.Errorf("cannot settle screenshot checksum: existing screenshot is missing its asset ID")
		}
		remote, err := waitForScreenshotSettlement(ctx, client, assetID)
		if err != nil {
			return nil, err
		}
		settled[index].Attributes.SourceFileChecksum = remote.Attributes.SourceFileChecksum
		settled[index].Attributes.AssetDeliveryState = remote.Attributes.AssetDeliveryState
	}
	return settled, nil
}

func syncSkippedScreenshotOrder(ctx context.Context, client *asc.Client, setID string, files []string, skippedResults, uploadedResults []asc.AssetUploadResultItem) ([]string, error) {
	currentOrder, err := GetOrderedAppScreenshotIDs(ctx, client, setID)
	if err != nil {
		return nil, err
	}

	orderedIDs := orderAssetIDsForLocalFiles(currentOrder, files, skippedResults, uploadedResults)
	if sameAssetIDOrder(currentOrder, orderedIDs) {
		return orderedIDs, nil
	}
	return orderedIDs, SetOrderedAppScreenshots(ctx, client, setID, orderedIDs)
}

func computeFileChecksum(filePath string) (string, error) {
	file, err := shared.OpenExistingNoFollow(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
	if err != nil {
		return "", err
	}
	return checksum.Hash, nil
}

func computeFileChecksumInRoot(rootPath, filePath string) (string, error) {
	root, err := rootfs.New(rootPath)
	if err != nil {
		return "", err
	}
	file, err := root.OpenFile(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
	if err != nil {
		return "", err
	}
	return checksum.Hash, nil
}

func uploadScreenshotAsset(ctx context.Context, client *asc.Client, setID, sourceRootPath, filePath string) (asc.AssetUploadResultItem, screenshotPendingAsset, error) {
	root, err := rootfs.New(sourceRootPath)
	if err != nil {
		return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, err
	}
	sourcePath, err := filepath.Abs(filePath)
	if err != nil {
		return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, err
	}
	file, err := root.OpenFile(sourcePath)
	if err != nil {
		return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, err
	}
	defer file.Close()
	return uploadScreenshotAssetFromFile(ctx, client, setID, filePath, file)
}

type openedScreenshotFiles map[string]*os.File

func openAndValidateScreenshotFiles(sourceRootPath string, filePaths []string) (openedScreenshotFiles, error) {
	if len(filePaths) == 0 {
		return nil, nil
	}
	root, err := rootfs.New(sourceRootPath)
	if err != nil {
		return nil, err
	}

	opened := make(openedScreenshotFiles, len(filePaths))
	closeOpened := func() {
		closeOpenedScreenshotFiles(opened)
	}
	for _, filePath := range filePaths {
		sourcePath, err := filepath.Abs(filePath)
		if err != nil {
			closeOpened()
			return nil, err
		}
		file, err := root.OpenFile(sourcePath)
		if err != nil {
			closeOpened()
			return nil, err
		}
		if err := validateOpenedScreenshotFileFormat(filePath, file); err != nil {
			_ = file.Close()
			closeOpened()
			return nil, err
		}

		key := filepath.Clean(filePath)
		if existing := opened[key]; existing != nil {
			_ = file.Close()
			continue
		}
		opened[key] = file
	}
	return opened, nil
}

func closeOpenedScreenshotFiles(files openedScreenshotFiles) {
	for _, file := range files {
		if file != nil {
			_ = file.Close()
		}
	}
}

func openedScreenshotFileForPath(files openedScreenshotFiles, filePath string) *os.File {
	if len(files) == 0 {
		return nil
	}
	if file := files[filepath.Clean(filePath)]; file != nil {
		return file
	}
	absolute, err := filepath.Abs(filePath)
	if err != nil {
		return nil
	}
	return files[filepath.Clean(absolute)]
}

func uploadScreenshotAssetFromFile(ctx context.Context, client *asc.Client, setID, filePath string, file *os.File) (asc.AssetUploadResultItem, screenshotPendingAsset, error) {
	if file == nil {
		return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, fmt.Errorf("screenshot file is required")
	}
	info, err := file.Stat()
	if err != nil {
		return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, err
	}
	if !info.Mode().IsRegular() {
		return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, fmt.Errorf("expected regular file: %q", filePath)
	}
	if info.Size() <= 0 {
		return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, fmt.Errorf("file is empty: %q", filePath)
	}
	const maxScreenshotUploadFileSize = int64(1024 * 1024 * 1024)
	if info.Size() > maxScreenshotUploadFileSize {
		return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, fmt.Errorf("file size exceeds %d bytes: %q", maxScreenshotUploadFileSize, filePath)
	}
	if err := validateOpenedScreenshotFileFormat(filePath, file); err != nil {
		return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, err
	}

	checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
	if err != nil {
		return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, err
	}

	created, err := client.CreateAppScreenshot(ctx, setID, info.Name(), info.Size())
	if err != nil {
		return asc.AssetUploadResultItem{}, screenshotPendingAsset{}, err
	}
	pending := screenshotPendingAsset{
		FileName: info.Name(),
		FilePath: filePath,
		AssetID:  created.Data.ID,
		Checksum: checksum.Hash,
		State:    "AWAITING_UPLOAD",
	}
	if len(created.Data.Attributes.UploadOperations) == 0 {
		return asc.AssetUploadResultItem{}, pending, fmt.Errorf("no upload operations returned for %q", info.Name())
	}

	if err := client.UploadAssetFromFile(ctx, file, info.Size(), created.Data.Attributes.UploadOperations); err != nil {
		return asc.AssetUploadResultItem{}, pending, err
	}
	pending.State = "UPLOADED"

	updated, err := client.UpdateAppScreenshot(ctx, created.Data.ID, true, checksum.Hash)
	if err != nil {
		return asc.AssetUploadResultItem{}, pending, err
	}
	pending.State = "UPLOAD_COMPLETE"
	if updated.Data.Attributes.AssetDeliveryState != nil && strings.TrimSpace(updated.Data.Attributes.AssetDeliveryState.State) != "" {
		pending.State = strings.ToUpper(strings.TrimSpace(updated.Data.Attributes.AssetDeliveryState.State))
	}

	state, err := waitForScreenshotDelivery(ctx, client, created.Data.ID)
	if strings.TrimSpace(state) != "" {
		pending.State = strings.ToUpper(strings.TrimSpace(state))
	}
	if err != nil {
		return asc.AssetUploadResultItem{}, pending, err
	}

	return asc.AssetUploadResultItem{
		FileName: info.Name(),
		FilePath: filePath,
		AssetID:  created.Data.ID,
		State:    state,
	}, screenshotPendingAsset{}, nil
}

func validateOpenedScreenshotFileFormat(filePath string, file *os.File) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	format, err := asc.ReadAppStoreImageFormatFrom(file)
	if err != nil {
		return fmt.Errorf("%q: %w", filePath, err)
	}
	if err := asc.ValidateImageFormatMatchesExtension(filePath, format); err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return nil
}

func waitForScreenshotDelivery(ctx context.Context, client *asc.Client, screenshotID string) (string, error) {
	settled, err := waitForScreenshotSettlement(ctx, client, screenshotID)
	state := ""
	if settled.Attributes.AssetDeliveryState != nil {
		state = settled.Attributes.AssetDeliveryState.State
	}
	return state, err
}

func waitForScreenshotSettlement(ctx context.Context, client *asc.Client, screenshotID string) (asc.Resource[asc.AppScreenshotAttributes], error) {
	assetID := strings.TrimSpace(screenshotID)
	if assetID == "" {
		return asc.Resource[asc.AppScreenshotAttributes]{}, fmt.Errorf("screenshot asset ID is required for checksum settlement")
	}

	lastState := ""
	lastRemote := asc.Resource[asc.AppScreenshotAttributes]{}
	settled, err := asc.PollUntil(ctx, screenshotSettlementPollInterval, func(ctx context.Context) (asc.Resource[asc.AppScreenshotAttributes], bool, error) {
		resp, err := client.GetAppScreenshot(ctx, assetID)
		if err != nil {
			return asc.Resource[asc.AppScreenshotAttributes]{}, false, fmt.Errorf("fetch screenshot %s during checksum settlement: %w", assetID, err)
		}
		lastRemote = resp.Data
		state := ""
		if resp.Data.Attributes.AssetDeliveryState != nil {
			state = strings.ToUpper(strings.TrimSpace(resp.Data.Attributes.AssetDeliveryState.State))
			lastState = state
		}
		switch state {
		case "FAILED":
			return asc.Resource[asc.AppScreenshotAttributes]{}, false, fmt.Errorf("screenshot %s delivery failed: %s", assetID, formatAssetErrors(resp.Data.Attributes.AssetDeliveryState.Errors))
		case "COMPLETE":
			if strings.TrimSpace(resp.Data.Attributes.SourceFileChecksum) != "" {
				return resp.Data, true, nil
			}
		}
		return resp.Data, false, nil
	})
	if err == nil {
		return settled, nil
	}
	stateDetail := ""
	if lastState != "" {
		stateDetail = fmt.Sprintf(" after delivery state %s", lastState)
	}
	if errors.Is(err, context.Canceled) {
		return lastRemote, fmt.Errorf("canceled waiting for screenshot %s checksum settlement%s: %w", assetID, stateDetail, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return lastRemote, fmt.Errorf("timed out waiting for screenshot %s checksum settlement%s: %w", assetID, stateDetail, err)
	}
	return lastRemote, err
}

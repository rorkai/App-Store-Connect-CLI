package assets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type screenshotUploadFailureArtifact struct {
	VersionLocalizationID string                       `json:"versionLocalizationId"`
	Path                  string                       `json:"path,omitempty"`
	RootPath              string                       `json:"rootPath,omitempty"`
	DeviceType            string                       `json:"deviceType,omitempty"`
	DisplayType           string                       `json:"displayType,omitempty"`
	SkipExisting          bool                         `json:"skipExisting,omitempty"`
	Replace               bool                         `json:"replace,omitempty"`
	SetID                 string                       `json:"setId,omitempty"`
	Files                 []string                     `json:"files,omitempty"`
	OrderedIDs            []string                     `json:"orderedIds,omitempty"`
	PendingFiles          []string                     `json:"pendingFiles,omitempty"`
	PendingAssets         []screenshotPendingAsset     `json:"pendingAssets,omitempty"`
	CleanupFailures       []screenshotPendingAsset     `json:"cleanupFailures,omitempty"`
	Results               []asc.AssetUploadResultItem  `json:"results,omitempty"`
	Failures              []asc.AssetUploadFailureItem `json:"failures,omitempty"`
	Error                 string                       `json:"error,omitempty"`
	GeneratedAt           string                       `json:"generatedAt"`
}

type screenshotPendingAsset struct {
	FileName string `json:"fileName"`
	FilePath string `json:"filePath"`
	AssetID  string `json:"assetId"`
	Checksum string `json:"checksum"`
	State    string `json:"state"`
}

type screenshotUploadPreparedState struct {
	Set                 asc.Resource[asc.AppScreenshotSetAttributes]
	ExistingScreenshots []asc.Resource[asc.AppScreenshotAttributes]
	Files               []string
	SkippedResults      []asc.AssetUploadResultItem
	OrderedIDs          []string
}

func buildAppScreenshotUploadResult(localizationID string, set asc.Resource[asc.AppScreenshotSetAttributes], dryRun bool, results []asc.AssetUploadResultItem) asc.AppScreenshotUploadResult {
	result := asc.AppScreenshotUploadResult{
		VersionLocalizationID: localizationID,
		SetID:                 set.ID,
		DisplayType:           set.Attributes.ScreenshotDisplayType,
		DryRun:                dryRun,
		Results:               results,
	}
	finalizeAppScreenshotUploadResult(&result)
	return result
}

func finalizeAppScreenshotUploadResult(result *asc.AppScreenshotUploadResult) {
	if result == nil {
		return
	}

	uploaded := 0
	skipped := 0
	for _, item := range result.Results {
		state := strings.ToLower(strings.TrimSpace(item.State))
		switch {
		case item.Skipped || state == "skipped":
			skipped++
		case state == "would-delete":
			continue
		case strings.TrimSpace(item.AssetID) != "":
			uploaded++
		}
	}

	result.Uploaded = uploaded
	result.Skipped = skipped
	if result.Failed == 0 {
		result.Failed = len(result.Failures)
	}
	if result.Total == 0 {
		result.Total = len(result.Results) + result.Pending
	}
}

func hasAppScreenshotUploadResultOutput(result asc.AppScreenshotUploadResult) bool {
	return strings.TrimSpace(result.VersionLocalizationID) != "" ||
		strings.TrimSpace(result.SetID) != "" ||
		strings.TrimSpace(result.DisplayType) != "" ||
		result.Pending > 0 ||
		result.Failed > 0 ||
		result.Total > 0 ||
		len(result.Results) > 0 ||
		len(result.Failures) > 0 ||
		strings.TrimSpace(result.FailureArtifactPath) != ""
}

func appendScreenshotUploadFailure(result *asc.AppScreenshotUploadResult, progress screenshotUploadProgress, uploadErr error) {
	if result == nil || uploadErr == nil {
		return
	}

	if strings.TrimSpace(progress.FailedFile) != "" {
		result.Failures = append(result.Failures, asc.AssetUploadFailureItem{
			FileName: filepath.Base(progress.FailedFile),
			FilePath: progress.FailedFile,
			Error:    uploadErr.Error(),
		})
	}
	if len(progress.CleanupFailures) > 0 {
		result.Failures = append(result.Failures, asc.AssetUploadFailureItem{
			FileName: "screenshot cleanup",
			Error:    uploadErr.Error(),
		})
		return
	}
	if strings.TrimSpace(progress.FailedFile) != "" {
		return
	}

	result.Failures = append(result.Failures, asc.AssetUploadFailureItem{
		FileName: "screenshot ordering",
		Error:    uploadErr.Error(),
	})
}

func screenshotUploadRetryError(progress screenshotUploadProgress) error {
	var retryErr error
	if len(progress.CleanupFailures) > 0 {
		retryErr = fmt.Errorf("screenshots upload: %d remote asset(s) pending cleanup", len(progress.CleanupFailures))
	} else if len(progress.PendingFiles) > 0 {
		retryErr = fmt.Errorf("screenshots upload: %d file(s) pending retry", len(progress.PendingFiles))
	} else {
		retryErr = fmt.Errorf("screenshots upload: retry needed to sync screenshot ordering")
	}
	if progress.CleanupError != nil {
		errList := []error{progress.CleanupError}
		if progress.UploadError != nil {
			errList = append(errList, progress.UploadError)
		}
		errList = append(errList, shared.NewReportedError(retryErr))
		return errors.Join(errList...)
	}
	return shared.NewReportedError(retryErr)
}

func prepareAppScreenshotUpload(ctx context.Context, cfg screenshotUploadConfig[asc.AppScreenshotUploadResult]) (screenshotUploadPreparedState, error) {
	if cfg.Client == nil {
		return screenshotUploadPreparedState{}, fmt.Errorf("client is required")
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
		return screenshotUploadPreparedState{}, err
	}

	existingScreenshots := make([]asc.Resource[asc.AppScreenshotAttributes], 0)
	if (cfg.SkipExisting || cfg.Replace || (!cfg.Replace && len(cfg.Files) > 0)) && set.ID != "" {
		existingResp, err := cfg.Client.GetAllAppScreenshots(ctx, set.ID, asc.WithAppScreenshotsRequestContext(cfg.RequestContext))
		if err != nil {
			return screenshotUploadPreparedState{}, err
		}
		existingScreenshots = existingResp.Data
	}
	if cfg.SkipExisting && len(existingScreenshots) > 0 {
		settleCtx, settleCancel := cfg.RequestContext(ctx)
		existingScreenshots, err = settleExistingScreenshotChecksums(settleCtx, cfg.Client, existingScreenshots)
		settleCancel()
		if err != nil {
			return screenshotUploadPreparedState{}, err
		}
	}

	skippedResults := make([]asc.AssetUploadResultItem, 0)
	files := cfg.Files
	if cfg.SkipExisting {
		var filterErr error
		files, skippedResults, filterErr = filterExistingScreenshotFiles(cfg.Files, existingScreenshots, cfg.InspectCommand)
		if filterErr != nil {
			return screenshotUploadPreparedState{}, filterErr
		}
	}
	files, err = limitScreenshotUploadFilesForExistingSet(files, cfg.MaxScreenshots, existingScreenshots, cfg.Replace, set.ID, cfg.InspectCommand, cfg.ReplaceCommand)
	if err != nil {
		return screenshotUploadPreparedState{}, err
	}

	orderedIDs := make([]string, 0)
	if !cfg.DryRun && !cfg.Replace && set.ID != "" && len(files) > 0 {
		orderCtx, orderCancel := cfg.UploadContext(ctx)
		orderedIDs, err = GetOrderedAppScreenshotIDs(orderCtx, cfg.Client, set.ID)
		orderCancel()
		if err != nil {
			return screenshotUploadPreparedState{}, err
		}
	}

	return screenshotUploadPreparedState{
		Set:                 set,
		ExistingScreenshots: existingScreenshots,
		Files:               files,
		SkippedResults:      skippedResults,
		OrderedIDs:          orderedIDs,
	}, nil
}

func executeAppScreenshotUpload(ctx context.Context, cfg screenshotUploadConfig[asc.AppScreenshotUploadResult], artifactPath string) (asc.AppScreenshotUploadResult, error) {
	if cfg.UploadContext == nil {
		cfg.UploadContext = contextWithAssetUploadTimeout
	}
	if len(cfg.Files) > 0 {
		sourceRootPath, err := resolveScreenshotUploadRoot(cfg.RootPath, cfg.Files)
		if err != nil {
			return asc.AppScreenshotUploadResult{}, fmt.Errorf("resolve screenshot source root: %w", err)
		}
		cfg.RootPath = sourceRootPath
	}
	prepared, err := prepareAppScreenshotUpload(ctx, cfg)
	if err != nil {
		return asc.AppScreenshotUploadResult{}, err
	}

	if cfg.DryRun {
		results := make([]asc.AssetUploadResultItem, 0, len(prepared.SkippedResults)+len(prepared.Files)+len(prepared.ExistingScreenshots))
		if cfg.Replace {
			for _, screenshot := range prepared.ExistingScreenshots {
				results = append(results, asc.AssetUploadResultItem{
					FileName: screenshot.Attributes.FileName,
					AssetID:  screenshot.ID,
					State:    "would-delete",
				})
			}
		}
		for _, filePath := range prepared.Files {
			results = append(results, asc.AssetUploadResultItem{
				FileName: filepath.Base(filePath),
				FilePath: filePath,
				State:    "would-upload",
			})
		}
		results = append(results, prepared.SkippedResults...)
		return buildAppScreenshotUploadResult(cfg.LocalizationID, prepared.Set, true, results), nil
	}

	uploadCtx, cancel := cfg.UploadContext(ctx)
	defer cancel()

	var openedFiles openedScreenshotFiles
	if cfg.Replace && len(prepared.Files) > 0 {
		openedFiles, err = openAndValidateScreenshotFiles(cfg.RootPath, prepared.Files)
		if err != nil {
			return asc.AppScreenshotUploadResult{}, err
		}
		defer closeOpenedScreenshotFiles(openedFiles)
		if err := deleteExistingScreenshots(uploadCtx, cfg.Client, prepared.ExistingScreenshots); err != nil {
			return asc.AppScreenshotUploadResult{}, err
		}
	} else if cfg.Replace {
		if err := deleteExistingScreenshots(uploadCtx, cfg.Client, prepared.ExistingScreenshots); err != nil {
			return asc.AppScreenshotUploadResult{}, err
		}
	}

	progress, uploadErr := uploadScreenshotsWithOrderStateWithOpenedFiles(uploadCtx, cfg.Client, prepared.Set.ID, prepared.OrderedIDs, prepared.Files, cfg.RootPath, false, true, openedFiles)
	if uploadErr == nil && cfg.SkipExisting && len(prepared.SkippedResults) > 0 {
		desiredIDs, err := syncSkippedScreenshotOrder(uploadCtx, cfg.Client, prepared.Set.ID, cfg.Files, prepared.SkippedResults, progress.Results)
		if err != nil {
			if len(desiredIDs) > 0 {
				progress.OrderedIDs = desiredIDs
			}
			uploadErr = err
		}
	}

	results := append(append([]asc.AssetUploadResultItem{}, prepared.SkippedResults...), progress.Results...)
	result := buildAppScreenshotUploadResult(cfg.LocalizationID, prepared.Set, false, results)

	if uploadErr == nil {
		return result, nil
	}

	result.Pending = len(progress.PendingFiles)
	appendScreenshotUploadFailure(&result, progress, uploadErr)
	result.Total = len(result.Results) + result.Pending
	finalizeAppScreenshotUploadResult(&result)

	orderedIDs := append([]string(nil), progress.OrderedIDs...)
	if cfg.SkipExisting && len(prepared.SkippedResults) > 0 && (len(prepared.Files) > 0 || strings.TrimSpace(progress.FailedFile) != "") {
		desiredIDs := orderAssetIDsForLocalFiles(prepared.OrderedIDs, cfg.Files, prepared.SkippedResults, progress.Results)
		if len(desiredIDs) > 0 {
			orderedIDs = desiredIDs
		}
	}

	artifact := screenshotUploadFailureArtifact{
		VersionLocalizationID: cfg.LocalizationID,
		Path:                  artifactPath,
		RootPath:              cfg.RootPath,
		DeviceType:            strings.TrimPrefix(cfg.DisplayType, "APP_"),
		DisplayType:           cfg.DisplayType,
		SkipExisting:          cfg.SkipExisting,
		Replace:               cfg.Replace,
		SetID:                 prepared.Set.ID,
		Files:                 append([]string(nil), cfg.Files...),
		OrderedIDs:            orderedIDs,
		PendingFiles:          append([]string(nil), progress.PendingFiles...),
		PendingAssets:         append([]screenshotPendingAsset(nil), progress.PendingAssets...),
		CleanupFailures:       append([]screenshotPendingAsset(nil), progress.CleanupFailures...),
		Results:               append([]asc.AssetUploadResultItem(nil), result.Results...),
		Failures:              append([]asc.AssetUploadFailureItem(nil), result.Failures...),
		Error:                 uploadErr.Error(),
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
	}
	if len(artifact.PendingFiles) == 0 && strings.TrimSpace(progress.FailedFile) != "" {
		artifact.PendingFiles = []string{progress.FailedFile}
	}

	writtenPath, artifactErr := persistScreenshotUploadFailureArtifact(artifactPath, artifact)
	if artifactErr != nil {
		return result, screenshotUploadArtifactWriteError(uploadErr, artifactErr)
	}
	result.FailureArtifactPath = writtenPath
	return result, screenshotUploadRetryError(progress)
}

// screenshotUploadArtifactWriteError reports that the upload failed and that no
// resume artifact could be written for it. The upload error is kept because it
// is the actionable cause, and the pending-retry framing is dropped because
// there is no artifact left to resume from.
func screenshotUploadArtifactWriteError(uploadErr, artifactErr error) error {
	return errors.Join(uploadErr, fmt.Errorf("write screenshot upload failure artifact: %w", artifactErr))
}

func resumeAppScreenshotUpload(ctx context.Context, client *asc.Client, artifactPath string) (asc.AppScreenshotUploadResult, error) {
	artifact, err := loadScreenshotUploadFailureArtifact(artifactPath)
	if err != nil {
		return asc.AppScreenshotUploadResult{}, fmt.Errorf("load resume artifact: %w", err)
	}
	if strings.TrimSpace(artifact.SetID) == "" {
		return asc.AppScreenshotUploadResult{}, fmt.Errorf("resume artifact %q is missing setId", artifactPath)
	}
	canRetrySkippedOrdering := artifact.SkipExisting && len(artifact.Files) > 0 && len(artifact.Results) > 0
	if len(artifact.PendingFiles) == 0 && len(artifact.OrderedIDs) == 0 && len(artifact.CleanupFailures) == 0 && !canRetrySkippedOrdering {
		return asc.AppScreenshotUploadResult{}, fmt.Errorf("resume artifact %q has no pending files or ordering work", artifactPath)
	}

	uploadCtx, cancel := contextWithAssetUploadTimeout(ctx)
	defer cancel()

	if len(artifact.CleanupFailures) > 0 {
		remainingCleanup, cleanupErr := cleanupScreenshotAssets(uploadCtx, client, artifact.CleanupFailures)
		if cleanupErr != nil {
			progress := screenshotUploadProgress{
				OrderedIDs:      append([]string(nil), artifact.OrderedIDs...),
				CleanupFailures: remainingCleanup,
				CleanupError:    cleanupErr,
			}
			result := asc.AppScreenshotUploadResult{
				VersionLocalizationID: artifact.VersionLocalizationID,
				SetID:                 artifact.SetID,
				DisplayType:           artifact.DisplayType,
				Resumed:               true,
				Results:               append([]asc.AssetUploadResultItem(nil), artifact.Results...),
			}
			appendScreenshotUploadFailure(&result, progress, cleanupErr)
			result.Pending = len(artifact.PendingFiles)
			result.Total = len(result.Results) + result.Pending
			finalizeAppScreenshotUploadResult(&result)
			nextArtifact := artifact
			nextArtifact.CleanupFailures = append([]screenshotPendingAsset(nil), remainingCleanup...)
			nextArtifact.Failures = append([]asc.AssetUploadFailureItem(nil), result.Failures...)
			nextArtifact.Error = cleanupErr.Error()
			nextArtifact.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
			writtenPath, artifactErr := persistScreenshotUploadFailureArtifact(artifactPath, nextArtifact)
			if artifactErr != nil {
				return result, screenshotUploadArtifactWriteError(cleanupErr, artifactErr)
			}
			result.FailureArtifactPath = writtenPath
			return result, screenshotUploadRetryError(progress)
		}
		artifact.CleanupFailures = nil
	}

	syncAfterUpload := !artifact.SkipExisting || len(artifact.Files) == 0
	sourceRootPath := strings.TrimSpace(artifact.RootPath)
	if screenshotArtifactNeedsSourceFiles(artifact) {
		sourceRootPath, err = resolveScreenshotUploadRoot(artifact.RootPath, screenshotArtifactSourcePaths(artifact))
		if err != nil {
			return asc.AppScreenshotUploadResult{}, fmt.Errorf("resolve resume source root: %w", err)
		}
	}
	if err := validateResumedScreenshotFiles(sourceRootPath, artifact.PendingFiles); err != nil {
		return asc.AppScreenshotUploadResult{}, shared.NewValidationError(err)
	}
	progress, uploadErr := resumeScreenshotsWithOrderState(uploadCtx, client, artifact.SetID, artifact.OrderedIDs, artifact.PendingFiles, artifact.PendingAssets, sourceRootPath, true, syncAfterUpload)

	result := asc.AppScreenshotUploadResult{
		VersionLocalizationID: artifact.VersionLocalizationID,
		SetID:                 artifact.SetID,
		DisplayType:           artifact.DisplayType,
		Resumed:               true,
		Results:               append(append([]asc.AssetUploadResultItem(nil), artifact.Results...), progress.Results...),
	}

	if uploadErr == nil && artifact.SkipExisting && len(artifact.Files) > 0 {
		skippedResults, uploadedResults := splitSkippedScreenshotResults(result.Results)
		currentOrder, err := GetOrderedAppScreenshotIDs(uploadCtx, client, artifact.SetID)
		if err != nil {
			uploadErr = err
		} else if desiredIDs := orderAssetIDsForLocalFiles(currentOrder, artifact.Files, skippedResults, uploadedResults); len(desiredIDs) > 0 && !sameAssetIDOrder(currentOrder, desiredIDs) {
			if err := SetOrderedAppScreenshots(uploadCtx, client, artifact.SetID, desiredIDs); err != nil {
				progress.OrderedIDs = desiredIDs
				uploadErr = err
			} else {
				progress.OrderedIDs = desiredIDs
			}
		}
	}

	if uploadErr == nil {
		finalizeAppScreenshotUploadResult(&result)
		return result, nil
	}

	result.Pending = len(progress.PendingFiles)
	appendScreenshotUploadFailure(&result, progress, uploadErr)
	result.Total = len(result.Results) + result.Pending
	finalizeAppScreenshotUploadResult(&result)

	nextArtifact := screenshotUploadFailureArtifact{
		VersionLocalizationID: artifact.VersionLocalizationID,
		Path:                  artifactPath,
		RootPath:              sourceRootPath,
		DeviceType:            artifact.DeviceType,
		DisplayType:           artifact.DisplayType,
		SkipExisting:          artifact.SkipExisting,
		Replace:               artifact.Replace,
		SetID:                 artifact.SetID,
		Files:                 append([]string(nil), artifact.Files...),
		OrderedIDs:            append([]string(nil), progress.OrderedIDs...),
		PendingFiles:          append([]string(nil), progress.PendingFiles...),
		PendingAssets:         append([]screenshotPendingAsset(nil), progress.PendingAssets...),
		CleanupFailures:       append([]screenshotPendingAsset(nil), progress.CleanupFailures...),
		Results:               append([]asc.AssetUploadResultItem(nil), result.Results...),
		Failures:              append([]asc.AssetUploadFailureItem(nil), result.Failures...),
		Error:                 uploadErr.Error(),
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339),
	}
	if len(nextArtifact.PendingFiles) == 0 && strings.TrimSpace(progress.FailedFile) != "" {
		nextArtifact.PendingFiles = []string{progress.FailedFile}
	}

	writtenPath, artifactErr := persistScreenshotUploadFailureArtifact(artifactPath, nextArtifact)
	if artifactErr != nil {
		return result, screenshotUploadArtifactWriteError(uploadErr, artifactErr)
	}
	result.FailureArtifactPath = writtenPath
	return result, screenshotUploadRetryError(progress)
}

func splitSkippedScreenshotResults(results []asc.AssetUploadResultItem) ([]asc.AssetUploadResultItem, []asc.AssetUploadResultItem) {
	skipped := make([]asc.AssetUploadResultItem, 0)
	uploaded := make([]asc.AssetUploadResultItem, 0, len(results))
	for _, item := range results {
		state := strings.ToLower(strings.TrimSpace(item.State))
		if item.Skipped || state == "skipped" {
			skipped = append(skipped, item)
			continue
		}
		uploaded = append(uploaded, item)
	}
	return skipped, uploaded
}

func defaultScreenshotUploadFailureArtifactPath() string {
	return filepath.Join(
		".asc",
		"reports",
		"screenshots-upload",
		fmt.Sprintf("failures-%d.json", time.Now().UTC().UnixNano()),
	)
}

func normalizeScreenshotUploadArtifactFilePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", nil
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed), nil
	}
	return filepath.Abs(trimmed)
}

func normalizeScreenshotUploadFailureArtifactPaths(artifact screenshotUploadFailureArtifact) (screenshotUploadFailureArtifact, error) {
	for i := range artifact.Files {
		normalized, err := normalizeScreenshotUploadArtifactFilePath(artifact.Files[i])
		if err != nil {
			return screenshotUploadFailureArtifact{}, err
		}
		artifact.Files[i] = normalized
	}

	for i := range artifact.PendingFiles {
		normalized, err := normalizeScreenshotUploadArtifactFilePath(artifact.PendingFiles[i])
		if err != nil {
			return screenshotUploadFailureArtifact{}, err
		}
		artifact.PendingFiles[i] = normalized
	}

	for i := range artifact.PendingAssets {
		normalized, err := normalizeScreenshotUploadArtifactFilePath(artifact.PendingAssets[i].FilePath)
		if err != nil {
			return screenshotUploadFailureArtifact{}, err
		}
		artifact.PendingAssets[i].FilePath = normalized
	}

	for i := range artifact.CleanupFailures {
		normalized, err := normalizeScreenshotUploadArtifactFilePath(artifact.CleanupFailures[i].FilePath)
		if err != nil {
			return screenshotUploadFailureArtifact{}, err
		}
		artifact.CleanupFailures[i].FilePath = normalized
	}

	for i := range artifact.Results {
		normalized, err := normalizeScreenshotUploadArtifactFilePath(artifact.Results[i].FilePath)
		if err != nil {
			return screenshotUploadFailureArtifact{}, err
		}
		artifact.Results[i].FilePath = normalized
	}

	for i := range artifact.Failures {
		normalized, err := normalizeScreenshotUploadArtifactFilePath(artifact.Failures[i].FilePath)
		if err != nil {
			return screenshotUploadFailureArtifact{}, err
		}
		artifact.Failures[i].FilePath = normalized
	}

	if screenshotArtifactNeedsSourceFiles(artifact) {
		rootPath, err := resolveScreenshotUploadRoot(artifact.RootPath, screenshotArtifactSourcePaths(artifact))
		if err != nil {
			return screenshotUploadFailureArtifact{}, err
		}
		artifact.RootPath = rootPath
	} else if strings.TrimSpace(artifact.RootPath) != "" {
		rootPath, err := filepath.Abs(artifact.RootPath)
		if err != nil {
			return screenshotUploadFailureArtifact{}, err
		}
		artifact.RootPath = filepath.Clean(rootPath)
	}

	return artifact, nil
}

// validateResumedScreenshotFiles preflights the files a resume will upload,
// before the first reservation, so an artifact written by an older build or a
// pending file replaced on disk after the original failure cannot deliver an
// asset the upload paths already reject.
//
// A single in-flight pending asset must match the first pending file, so the
// pending file list covers everything the resume reads. Only the format check
// runs here: it needs nothing but the bytes and the file name, so it holds for
// every artifact regardless of what the payload records. Sizes stay the
// business of the run that wrote the artifact, which validated them against
// the display type it had; re-deciding them here would reject artifacts that
// were valid when written.
func validateResumedScreenshotFiles(sourceRootPath string, pendingFiles []string) error {
	if len(pendingFiles) == 0 {
		return nil
	}

	// The upload opens pending files through the operator-selected root, so the
	// preflight reads them the same way: containment and open stay one
	// operation, and a path swapped after the root was resolved cannot send
	// this read outside it.
	root, err := rootfs.New(sourceRootPath)
	if err != nil {
		return err
	}

	for _, pendingFile := range pendingFiles {
		trimmed := strings.TrimSpace(pendingFile)
		if trimmed == "" {
			continue
		}
		path, err := filepath.Abs(trimmed)
		if err != nil {
			return err
		}
		if err := validateResumedScreenshotFileFormat(root, path); err != nil {
			return err
		}
	}
	return nil
}

func validateResumedScreenshotFileFormat(root rootfs.Root, path string) error {
	file, err := root.OpenFile(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return validateOpenedScreenshotFileFormat(path, file)
}

func screenshotArtifactNeedsSourceFiles(artifact screenshotUploadFailureArtifact) bool {
	return len(artifact.PendingFiles) > 0 || len(artifact.PendingAssets) > 0
}

func screenshotArtifactSourcePaths(artifact screenshotUploadFailureArtifact) []string {
	paths := make([]string, 0, len(artifact.Files)+len(artifact.PendingFiles)+len(artifact.PendingAssets))
	paths = append(paths, artifact.Files...)
	paths = append(paths, artifact.PendingFiles...)
	for _, pending := range artifact.PendingAssets {
		paths = append(paths, pending.FilePath)
	}
	return paths
}

func resolveScreenshotUploadRoot(rootPath string, filePaths []string) (string, error) {
	if err := validateScreenshotSourceAncestry(filePaths); err != nil {
		return "", err
	}

	rootPath = strings.TrimSpace(rootPath)
	if rootPath != "" {
		absolute, err := filepath.Abs(rootPath)
		if err != nil {
			return "", err
		}
		absolute = filepath.Clean(absolute)
		for _, filePath := range filePaths {
			fileAbsolute, err := filepath.Abs(strings.TrimSpace(filePath))
			if err != nil {
				return "", err
			}
			if filepath.Clean(fileAbsolute) == absolute {
				absolute = filepath.Dir(absolute)
				break
			}
		}
		root, err := rootfs.New(absolute)
		if err != nil {
			return "", err
		}
		if err := root.CheckContained("."); err != nil {
			return "", err
		}
		for _, filePath := range filePaths {
			if strings.TrimSpace(filePath) == "" {
				continue
			}
			fileAbsolute, err := filepath.Abs(filePath)
			if err != nil {
				return "", err
			}
			if err := root.CheckContained(fileAbsolute); err != nil {
				return "", err
			}
		}
		return root.Path(), nil
	}

	cleaned := make([]string, 0, len(filePaths))
	for _, filePath := range filePaths {
		filePath = strings.TrimSpace(filePath)
		if filePath == "" {
			continue
		}
		absolute, err := filepath.Abs(filePath)
		if err != nil {
			return "", err
		}
		cleaned = append(cleaned, filepath.Clean(absolute))
	}
	if len(cleaned) == 0 {
		return "", nil
	}

	root := filepath.Dir(cleaned[0])
	for _, filePath := range cleaned[1:] {
		for {
			relative, err := filepath.Rel(root, filePath)
			if err != nil {
				return "", err
			}
			if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				break
			}
			parent := filepath.Dir(root)
			if parent == root {
				return "", fmt.Errorf("screenshot files do not share a common source root")
			}
			root = parent
		}
	}
	trustedRoot, err := rootfs.New(root)
	if err != nil {
		return "", err
	}
	if err := trustedRoot.CheckContained("."); err != nil {
		return "", err
	}
	for _, filePath := range cleaned {
		if err := trustedRoot.CheckContained(filePath); err != nil {
			return "", err
		}
	}
	return trustedRoot.Path(), nil
}

func validateScreenshotSourceAncestry(filePaths []string) error {
	for _, filePath := range filePaths {
		filePath = strings.TrimSpace(filePath)
		if filePath == "" {
			continue
		}
		absolute, err := filepath.Abs(filePath)
		if err != nil {
			return err
		}
		validationRoot, ok := screenshotSourceValidationRoot(absolute)
		if !ok {
			// The source lives outside the working, home and system temporary
			// directories, so every remaining ancestor is one the operator
			// named. Platform aliases live there too (macOS reaches /tmp, /var
			// and /etc through symlinks into /private), and auditing that chain
			// only produces errors about paths the caller never wrote. Anchor
			// on the file's physical parent instead: the file itself is still
			// refused when it is a symlink, and resolveScreenshotUploadRoot
			// still refuses a symlinked source root and any symlink below it.
			validationRoot = physicalParentDir(absolute)
			absolute = filepath.Join(validationRoot, filepath.Base(absolute))
		}
		root, err := rootfs.New(validationRoot)
		if err != nil {
			return err
		}
		if err := root.CheckContained(absolute); err != nil {
			return err
		}
	}
	return nil
}

// screenshotSourceValidationRoot returns the deepest directory the operator
// controls for this process that contains absolutePath, reporting false when
// the path lives outside all of them.
func screenshotSourceValidationRoot(absolutePath string) (string, bool) {
	absolutePath = filepath.Clean(absolutePath)

	candidates := make([]string, 0, 3)
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, home)
	}
	if temporary := strings.TrimSpace(os.TempDir()); temporary != "" {
		candidates = append(candidates, temporary)
	}

	best := ""
	for _, candidate := range candidates {
		candidate, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		candidate = filepath.Clean(candidate)
		relative, err := filepath.Rel(candidate, absolutePath)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		if len(candidate) > len(best) {
			best = candidate
		}
	}
	return best, best != ""
}

// physicalParentDir resolves the parent directory of absolutePath through any
// symlinks so platform aliases do not read as untrusted path components. The
// base name is deliberately left unresolved so a symlinked source file is still
// rejected.
func physicalParentDir(absolutePath string) string {
	parent := filepath.Dir(absolutePath)
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return parent
	}
	return resolved
}

func persistScreenshotUploadFailureArtifact(path string, artifact screenshotUploadFailureArtifact) (string, error) {
	target := strings.TrimSpace(path)
	if target == "" {
		target = defaultScreenshotUploadFailureArtifactPath()
	}
	artifact.Path = filepath.Clean(target)

	artifact, err := normalizeScreenshotUploadFailureArtifactPaths(artifact)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return "", err
	}

	if _, err := shared.WriteFileNoSymlinkOverwrite(
		artifact.Path,
		bytes.NewReader(data),
		0o600,
		".screenshots-upload-*",
		".screenshots-upload-backup-*",
	); err != nil {
		return "", err
	}

	return artifact.Path, nil
}

func loadScreenshotUploadFailureArtifact(path string) (screenshotUploadFailureArtifact, error) {
	payload, err := shared.ReadJSONFilePayload(path)
	if err != nil {
		return screenshotUploadFailureArtifact{}, err
	}

	var artifact screenshotUploadFailureArtifact
	if err := json.Unmarshal(payload, &artifact); err != nil {
		return screenshotUploadFailureArtifact{}, err
	}
	return artifact, nil
}

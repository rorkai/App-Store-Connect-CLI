package assets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type screenshotUploadFailureArtifact struct {
	VersionLocalizationID string                       `json:"versionLocalizationId"`
	Path                  string                       `json:"path,omitempty"`
	DeviceType            string                       `json:"deviceType,omitempty"`
	DisplayType           string                       `json:"displayType,omitempty"`
	SkipExisting          bool                         `json:"skipExisting,omitempty"`
	Replace               bool                         `json:"replace,omitempty"`
	SetID                 string                       `json:"setId,omitempty"`
	Files                 []string                     `json:"files,omitempty"`
	OrderedIDs            []string                     `json:"orderedIds,omitempty"`
	PendingFiles          []string                     `json:"pendingFiles,omitempty"`
	Results               []asc.AssetUploadResultItem  `json:"results,omitempty"`
	Failures              []asc.AssetUploadFailureItem `json:"failures,omitempty"`
	Error                 string                       `json:"error,omitempty"`
	GeneratedAt           string                       `json:"generatedAt"`
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
		return
	}

	result.Failures = append(result.Failures, asc.AssetUploadFailureItem{
		FileName: "screenshot ordering",
		Error:    uploadErr.Error(),
	})
}

func screenshotUploadRetryError(progress screenshotUploadProgress) error {
	if len(progress.PendingFiles) > 0 {
		return shared.NewReportedError(fmt.Errorf("screenshots upload: %d file(s) pending retry", len(progress.PendingFiles)))
	}
	return shared.NewReportedError(fmt.Errorf("screenshots upload: retry needed to sync screenshot ordering"))
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

	requestCtx, reqCancel := cfg.RequestContext(ctx)
	var (
		set asc.Resource[asc.AppScreenshotSetAttributes]
		err error
	)
	if cfg.DryRun {
		set, err = findScreenshotSetWithAccess(requestCtx, cfg.Client, cfg.LocalizationID, cfg.DisplayType, cfg.Access)
	} else {
		set, err = ensureScreenshotSetWithAccess(requestCtx, cfg.Client, cfg.LocalizationID, cfg.DisplayType, cfg.Access)
	}
	reqCancel()
	if err != nil {
		return screenshotUploadPreparedState{}, err
	}

	existingScreenshots := make([]asc.Resource[asc.AppScreenshotAttributes], 0)
	if (cfg.SkipExisting || cfg.Replace || (!cfg.Replace && len(cfg.Files) > 0)) && set.ID != "" {
		fetchCtx, fetchCancel := cfg.RequestContext(ctx)
		existingResp, err := cfg.Client.GetAppScreenshots(fetchCtx, set.ID)
		fetchCancel()
		if err != nil {
			return screenshotUploadPreparedState{}, err
		}
		existingScreenshots = existingResp.Data
	}

	skippedResults := make([]asc.AssetUploadResultItem, 0)
	files := cfg.Files
	if cfg.SkipExisting {
		var filterErr error
		files, skippedResults, filterErr = filterExistingScreenshotFiles(cfg.Files, existingScreenshots)
		if filterErr != nil {
			return screenshotUploadPreparedState{}, filterErr
		}
	}
	files, err = limitScreenshotUploadFilesForExistingSet(files, cfg.MaxScreenshots, existingScreenshots, cfg.Replace, set.ID)
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

	if !cfg.DryRun && cfg.Replace {
		deleteCtx, deleteCancel := cfg.UploadContext(ctx)
		err = deleteExistingScreenshots(deleteCtx, cfg.Client, existingScreenshots)
		deleteCancel()
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

	progress, uploadErr := uploadScreenshotsWithOrderState(uploadCtx, cfg.Client, prepared.Set.ID, prepared.OrderedIDs, prepared.Files, false, true)
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
		desiredIDs := orderScreenshotIDsForLocalFiles(prepared.OrderedIDs, cfg.Files, prepared.SkippedResults, progress.Results)
		if len(desiredIDs) > 0 {
			orderedIDs = desiredIDs
		}
	}

	artifact := screenshotUploadFailureArtifact{
		VersionLocalizationID: cfg.LocalizationID,
		Path:                  artifactPath,
		DeviceType:            strings.TrimPrefix(cfg.DisplayType, "APP_"),
		DisplayType:           cfg.DisplayType,
		SkipExisting:          cfg.SkipExisting,
		Replace:               cfg.Replace,
		SetID:                 prepared.Set.ID,
		Files:                 append([]string(nil), cfg.Files...),
		OrderedIDs:            orderedIDs,
		PendingFiles:          append([]string(nil), progress.PendingFiles...),
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
		return result, fmt.Errorf("write screenshot upload failure artifact: %w", artifactErr)
	}
	result.FailureArtifactPath = writtenPath
	return result, screenshotUploadRetryError(progress)
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
	if len(artifact.PendingFiles) == 0 && len(artifact.OrderedIDs) == 0 && !canRetrySkippedOrdering {
		return asc.AppScreenshotUploadResult{}, fmt.Errorf("resume artifact %q has no pending files or ordering work", artifactPath)
	}

	uploadCtx, cancel := contextWithAssetUploadTimeout(ctx)
	defer cancel()

	// When the artifact recorded the original file list, ordering is
	// reconciled below from that list instead of appending resumed uploads
	// at the end, so previously uploaded and resumed screenshots end up in
	// the original local file order.
	syncAfterUpload := len(artifact.Files) == 0
	progress, uploadErr := uploadScreenshotsWithOrderState(uploadCtx, client, artifact.SetID, artifact.OrderedIDs, artifact.PendingFiles, true, syncAfterUpload)

	result := asc.AppScreenshotUploadResult{
		VersionLocalizationID: artifact.VersionLocalizationID,
		SetID:                 artifact.SetID,
		DisplayType:           artifact.DisplayType,
		Resumed:               true,
		Results:               append(append([]asc.AssetUploadResultItem(nil), artifact.Results...), progress.Results...),
	}

	if uploadErr == nil && len(artifact.Files) > 0 {
		if artifact.SkipExisting {
			skippedResults, uploadedResults := splitSkippedScreenshotResults(result.Results)
			currentOrder, err := GetOrderedAppScreenshotIDs(uploadCtx, client, artifact.SetID)
			if err != nil {
				uploadErr = err
			} else if desiredIDs := orderScreenshotIDsForLocalFiles(currentOrder, artifact.Files, skippedResults, uploadedResults); len(desiredIDs) > 0 && !sameScreenshotIDOrder(currentOrder, desiredIDs) {
				if err := SetOrderedAppScreenshots(uploadCtx, client, artifact.SetID, desiredIDs); err != nil {
					progress.OrderedIDs = desiredIDs
					uploadErr = err
				} else {
					progress.OrderedIDs = desiredIDs
				}
			}
		} else if desiredIDs := mergeResumedScreenshotOrder(progress.OrderedIDs, artifact.Files, result.Results); len(desiredIDs) > 0 {
			progress.OrderedIDs = desiredIDs
			if err := SetOrderedAppScreenshots(uploadCtx, client, artifact.SetID, desiredIDs); err != nil {
				uploadErr = err
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
		DeviceType:            artifact.DeviceType,
		DisplayType:           artifact.DisplayType,
		SkipExisting:          artifact.SkipExisting,
		Replace:               artifact.Replace,
		SetID:                 artifact.SetID,
		Files:                 append([]string(nil), artifact.Files...),
		OrderedIDs:            append([]string(nil), progress.OrderedIDs...),
		PendingFiles:          append([]string(nil), progress.PendingFiles...),
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
		return result, fmt.Errorf("write screenshot upload failure artifact: %w", artifactErr)
	}
	result.FailureArtifactPath = writtenPath
	return result, screenshotUploadRetryError(progress)
}

// mergeResumedScreenshotOrder returns orderedIDs with every ID that maps to a
// local file moved into the original file order. IDs that do not correspond to
// a local file (pre-existing screenshots) keep their relative position ahead
// of the file-backed IDs, matching the non-resume upload ordering.
func mergeResumedScreenshotOrder(orderedIDs, files []string, results []asc.AssetUploadResultItem) []string {
	idByPath := make(map[string]string, len(results))
	for _, item := range results {
		id := strings.TrimSpace(item.AssetID)
		path := strings.TrimSpace(item.FilePath)
		if id == "" || path == "" {
			continue
		}
		if _, exists := idByPath[path]; !exists {
			idByPath[path] = id
		}
	}

	fileIDs := make([]string, 0, len(files))
	fileIDSet := make(map[string]struct{}, len(files))
	for _, filePath := range files {
		id := idByPath[strings.TrimSpace(filePath)]
		if id == "" {
			continue
		}
		if _, exists := fileIDSet[id]; exists {
			continue
		}
		fileIDSet[id] = struct{}{}
		fileIDs = append(fileIDs, id)
	}

	merged := make([]string, 0, len(orderedIDs)+len(fileIDs))
	for _, id := range normalizeScreenshotIDs(orderedIDs) {
		if _, exists := fileIDSet[id]; exists {
			continue
		}
		merged = append(merged, id)
	}
	return append(merged, fileIDs...)
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

	return artifact, nil
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

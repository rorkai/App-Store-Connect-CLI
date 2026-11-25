package assets

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type screenshotReplacement struct {
	Checksums map[string]string        `json:"checksums"`
	Remove    []screenshotPendingAsset `json:"remove,omitempty"`
}

type screenshotResumeKey struct{}

func screenshotRecoveryIDArgument(id string) (string, bool) {
	if segment, err := asc.ValidateResourcePathSegment(id); err != nil || segment != id {
		return "", false
	}
	return shared.ShellQuote(id)
}

func screenshotReservationRecoveryError(setID, assetID, state string) error {
	setArg, setOK := screenshotRecoveryIDArgument(setID)
	assetArg, assetOK := screenshotRecoveryIDArgument(assetID)
	if !setOK || !assetOK {
		return fmt.Errorf("screenshot reservation %q in set %q is %s. Inspect the set in App Store Connect and remove the reservation only after confirming it is no longer in use, then preview a fresh replacement", assetID, setID, state)
	}
	return fmt.Errorf("screenshot reservation %q is %s. Inspect the selected set with asc screenshots list --set-id %s --fields fileName,assetDeliveryState --paginate. If this reservation is no longer in use, remove it with asc screenshots delete --id %s --confirm, then run a fresh --replace --dry-run for the same app, localization, and device before using --replace --confirm", assetID, state, setArg, assetArg)
}

func screenshotMutationAppID(d *shared.Deps, value, operation string) (string, error) {
	value = d.ResolveAppID(value)
	if value == "" {
		return "", fmt.Errorf("%s requires --app, ASC_APP_ID, or a configured default app", operation)
	}
	if !shared.IsNumericAppID(value) || strings.HasPrefix(value, "0") {
		return "", fmt.Errorf("%s requires a canonical numeric app ID; app names and bundle IDs are not accepted", operation)
	}
	return value, nil
}

func verifyScreenshotLocalizationApp(ctx context.Context, client *asc.Client, appID, localizationID string) error {
	query := url.Values{}
	query.Set("include", "appStoreVersion")
	query.Set("fields[appStoreVersionLocalizations]", "appStoreVersion")
	query.Set("fields[appStoreVersions]", "app")
	payload, err := client.RawRequestWithQuery(ctx, http.MethodGet, "/v1/appStoreVersionLocalizations/"+url.PathEscape(localizationID), query, nil)
	if err != nil {
		return err
	}
	var localization asc.AppStoreVersionLocalizationResponse
	if err := json.Unmarshal(payload, &localization); err != nil {
		return fmt.Errorf("cannot verify screenshot localization: %w", err)
	}
	if localization.Data.ID != localizationID || localization.Data.Type != asc.ResourceTypeAppStoreVersionLocalizations {
		return fmt.Errorf("cannot verify screenshot localization %q", localizationID)
	}
	var relationships struct {
		AppStoreVersion asc.Relationship `json:"appStoreVersion"`
	}
	if err := json.Unmarshal(localization.Data.Relationships, &relationships); err != nil {
		return fmt.Errorf("cannot verify screenshot localization version: %w", err)
	}
	version := relationships.AppStoreVersion.Data
	if version.Type != asc.ResourceTypeAppStoreVersions || strings.TrimSpace(version.ID) == "" {
		return fmt.Errorf("cannot verify the version owning screenshot localization %q", localizationID)
	}
	_, err = shared.ResolveOwnedAppStoreVersionByID(ctx, client, appID, version.ID, "")
	return err
}

func replacementInventory(ctx context.Context, cfg screenshotUploadConfig[asc.AppScreenshotUploadResult], setID string) ([]asc.Resource[asc.AppScreenshotAttributes], []string, error) {
	if cfg.AppID != "" {
		requestCtx, cancel := cfg.RequestContext(ctx)
		err := verifyScreenshotLocalizationApp(requestCtx, cfg.Client, cfg.AppID, cfg.LocalizationID)
		cancel()
		if err != nil {
			return nil, nil, err
		}
	}
	set, err := findScreenshotSetWithAccess(ctx, cfg.Client, cfg.LocalizationID, cfg.DisplayType, cfg.Access, cfg.RequestContext)
	if err != nil {
		return nil, nil, err
	}
	if set.ID != setID {
		return nil, nil, fmt.Errorf("replacement screenshot set no longer belongs to the selected localization and device")
	}
	response, err := getScopedAppScreenshots(ctx, cfg.Client, setID, cfg.RequestContext)
	if err != nil {
		return nil, nil, err
	}
	order, err := GetOrderedAppScreenshotIDs(ctx, cfg.Client, setID)
	if err != nil {
		return nil, nil, err
	}
	members := make(map[string]bool, len(response.Data))
	for _, item := range response.Data {
		if item.ID == "" || members[item.ID] || item.Type != asc.ResourceTypeAppScreenshots {
			return nil, nil, fmt.Errorf("cannot verify replacement screenshot membership")
		}
		members[item.ID] = true
	}
	for _, id := range order {
		if !members[id] {
			return nil, nil, fmt.Errorf("screenshot membership changed while reading the replacement set")
		}
		delete(members, id)
	}
	if len(members) != 0 {
		return nil, nil, fmt.Errorf("screenshot membership changed while reading the replacement set")
	}
	return response.Data, order, nil
}

func replacementChecksum(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	decoded, err := hex.DecodeString(value)
	return value, err == nil && len(decoded) == 16
}

func executeScreenshotReplacement(ctx context.Context, cfg screenshotUploadConfig[asc.AppScreenshotUploadResult], artifactPath string, previous *screenshotUploadFailureArtifact) (asc.AppScreenshotUploadResult, error) {
	if previous != nil && len(previous.Replacement.Remove) > 0 {
		return asc.AppScreenshotUploadResult{}, screenshotDestructiveRecoveryError(true)
	}
	if previous != nil {
		if scope, _ := ctx.Value(screenshotResumeScopeKey{}).(*screenshotResumeScope); scope == nil {
			return asc.AppScreenshotUploadResult{}, fmt.Errorf("screenshot resume requires an independently verified target")
		}
	}
	if cfg.Client == nil {
		return asc.AppScreenshotUploadResult{}, fmt.Errorf("client is required")
	}
	if len(cfg.Files) == 0 {
		return asc.AppScreenshotUploadResult{}, fmt.Errorf("replacement requires at least one screenshot file")
	}
	if cfg.RequestContext == nil {
		cfg.RequestContext = cfg.Deps.Timeout
	}
	if cfg.UploadContext == nil {
		cfg.UploadContext = assetUploadContext(cfg.Deps)
	}
	root, err := resolveScreenshotUploadRoot(cfg.Deps, cfg.RootPath, cfg.Files)
	if err != nil {
		return asc.AppScreenshotUploadResult{}, err
	}
	cfg.RootPath = root
	opened, err := openAndValidateScreenshotFiles(root, cfg.DisplayType, cfg.Files)
	if err != nil {
		return asc.AppScreenshotUploadResult{}, err
	}
	defer closeOpenedScreenshotFiles(opened)
	plan := &screenshotReplacement{Checksums: make(map[string]string, len(cfg.Files))}
	for _, path := range cfg.Files {
		file := openedScreenshotFileForPath(opened, path)
		checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
		if err != nil {
			return asc.AppScreenshotUploadResult{}, err
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return asc.AppScreenshotUploadResult{}, err
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return asc.AppScreenshotUploadResult{}, err
		}
		plan.Checksums[path] = checksum.Hash
		if previous != nil && !strings.EqualFold(previous.Replacement.Checksums[path], checksum.Hash) {
			return asc.AppScreenshotUploadResult{}, fmt.Errorf("replacement source changed after failure: %q", path)
		}
	}
	if previous == nil {
		cfg.AppID, err = screenshotMutationAppID(cfg.Deps, cfg.AppID, "--replace")
		if err != nil {
			return asc.AppScreenshotUploadResult{}, err
		}
		requestCtx, cancel := cfg.RequestContext(ctx)
		err = verifyScreenshotLocalizationApp(requestCtx, cfg.Client, cfg.AppID, cfg.LocalizationID)
		cancel()
		if err != nil {
			return asc.AppScreenshotUploadResult{}, err
		}
	}
	set, err := findScreenshotSetWithAccess(ctx, cfg.Client, cfg.LocalizationID, cfg.DisplayType, cfg.Access, cfg.RequestContext)
	if err != nil {
		return asc.AppScreenshotUploadResult{}, err
	}
	if previous != nil && (set.ID == "" || set.ID != previous.SetID) {
		return asc.AppScreenshotUploadResult{}, fmt.Errorf("resume target no longer matches the selected screenshot set")
	}
	var existing []asc.Resource[asc.AppScreenshotAttributes]
	var order []string
	if set.ID != "" {
		existing, order, err = replacementInventory(ctx, cfg, set.ID)
		if err != nil {
			return asc.AppScreenshotUploadResult{}, err
		}
	}
	known := make(map[string]bool)
	pending := make(map[string]screenshotPendingAsset)
	cleanup := make(map[string]bool)
	if previous != nil {
		for _, item := range previous.Results {
			if item.State != "deleted" {
				known[item.AssetID] = true
			}
		}
		for _, item := range previous.CleanupFailures {
			cleanup[item.AssetID] = true
		}
		for _, item := range append(append([]screenshotPendingAsset{}, previous.PendingAssets...), previous.CleanupFailures...) {
			known[item.AssetID], pending[item.AssetID] = true, item
		}
	}
	verified := make([]asc.Resource[asc.AppScreenshotAttributes], 0, len(existing))
	present := make(map[string]bool, len(existing))
	for _, item := range existing {
		present[item.ID] = true
		if previous != nil && !known[item.ID] {
			return asc.AppScreenshotUploadResult{}, fmt.Errorf("screenshot %s appeared after replacement failed; inspect the set before starting a new replacement", item.ID)
		}
		state := ""
		if item.Attributes.AssetDeliveryState != nil {
			state = item.Attributes.AssetDeliveryState.State
		}
		if owned, ok := pending[item.ID]; ok {
			source, err := filepath.Abs(owned.FilePath)
			if err != nil {
				return asc.AppScreenshotUploadResult{}, err
			}
			expected, ok := plan.Checksums[source]
			if !ok || (owned.Checksum != "" && !strings.EqualFold(owned.Checksum, expected)) {
				return asc.AppScreenshotUploadResult{}, fmt.Errorf("cannot verify pending screenshot %s against the replacement source", item.ID)
			}
			if err := validatePendingScreenshotRemoteChecksum(item.ID, item.Attributes.SourceFileChecksum, expected); err != nil {
				return asc.AppScreenshotUploadResult{}, err
			}
			switch state {
			case "COMPLETE", "FAILED":
			case "AWAITING_UPLOAD":
				if !cleanup[item.ID] && owned.hasUploadedBytes() {
					continue
				}
			case "UPLOAD_COMPLETE":
				if !cleanup[item.ID] {
					continue
				}
				return asc.AppScreenshotUploadResult{}, fmt.Errorf("screenshot %s is still processing; wait for it to finish before resuming cleanup", item.ID)
			default:
				return asc.AppScreenshotUploadResult{}, fmt.Errorf("cannot verify pending screenshot %s with state %q", item.ID, state)
			}
		}
		if state == "FAILED" || state == "AWAITING_UPLOAD" {
			err := screenshotReservationRecoveryError(set.ID, item.ID, state)
			if previous == nil {
				return asc.AppScreenshotUploadResult{}, err
			}
			return previous.blockedResume(artifactPath, item.Attributes.FileName, err)
		}
		checksum, valid := replacementChecksum(item.Attributes.SourceFileChecksum)
		if !valid || state != "COMPLETE" {
			return asc.AppScreenshotUploadResult{}, fmt.Errorf("cannot verify screenshot %s: replacement requires COMPLETE assets with source checksums; finish or inspect the existing upload first", item.ID)
		}
		item.Attributes.SourceFileChecksum = checksum
		verified = append(verified, item)
	}
	files, kept, err := filterScreenshotFilesByChecksum(cfg.Files, verified, screenshotInspectionCommand(cfg.LocalizationID), func(path string) (string, error) {
		absolute, err := filepath.Abs(path)
		return plan.Checksums[absolute], err
	})
	if err != nil {
		return asc.AppScreenshotUploadResult{}, err
	}
	retained := make(map[string]bool, len(kept))
	for _, item := range kept {
		retained[item.AssetID] = true
	}
	for _, item := range verified {
		if !retained[item.ID] {
			if previous != nil {
				return asc.AppScreenshotUploadResult{}, screenshotDestructiveRecoveryError(true)
			}
			plan.Remove = append(plan.Remove, screenshotPendingAsset{AssetID: item.ID, FileName: item.Attributes.FileName, Checksum: item.Attributes.SourceFileChecksum, State: "COMPLETE"})
		}
	}
	if cfg.DryRun {
		results := append([]asc.AssetUploadResultItem{}, kept...)
		if len(files) == 0 && len(plan.Remove) == 0 && !sameAssetIDOrder(order, orderAssetIDsForLocalFiles(nil, cfg.Files, kept, nil)) {
			for index := range results {
				results[index].State = "would-reorder"
			}
		}
		for _, item := range plan.Remove {
			results = append(results, asc.AssetUploadResultItem{AssetID: item.AssetID, FileName: item.FileName, State: "would-delete"})
		}
		for _, path := range files {
			results = append(results, asc.AssetUploadResultItem{FileName: filepath.Base(path), FilePath: path, State: "would-upload"})
		}
		return buildAppScreenshotUploadResult(cfg.LocalizationID, set, true, results), nil
	}
	if set.ID == "" {
		set, err = ensureScreenshotSetWithAccess(ctx, cfg.Client, cfg.LocalizationID, cfg.DisplayType, cfg.Access, cfg.RequestContext)
		if err != nil {
			return asc.AppScreenshotUploadResult{}, err
		}
		current, _, err := replacementInventory(ctx, cfg, set.ID)
		if err != nil {
			return asc.AppScreenshotUploadResult{}, err
		}
		if len(current) != 0 {
			return asc.AppScreenshotUploadResult{}, fmt.Errorf("new screenshot set is no longer empty; inspect it before replacing")
		}
	}
	uploadCtx, cancel := cfg.UploadContext(withAssetUploadWork(ctx, files))
	defer cancel()
	progress := screenshotUploadProgress{PendingFiles: append([]string{}, files...)}
	if previous != nil {
		progress.PendingAssets = presentReplacementAssets(previous.PendingAssets, present, retained)
		progress.CleanupFailures = presentReplacementAssets(previous.CleanupFailures, present, retained)
	}
	if previous != nil && (len(plan.Remove) > 0 || len(progress.CleanupFailures) > 0) {
		return asc.AppScreenshotUploadResult{}, screenshotDestructiveRecoveryError(true)
	}
	var deleted []asc.AssetUploadResultItem
	finish := func(cause error) (asc.AppScreenshotUploadResult, error) {
		items := append(append([]asc.AssetUploadResultItem{}, kept...), progress.Results...)
		result := buildAppScreenshotUploadResult(cfg.LocalizationID, set, false, append(items, deleted...))
		result.Resumed = previous != nil
		if cause == nil {
			return result, nil
		}
		result.Pending = len(progress.PendingFiles)
		if len(plan.Remove) > 0 {
			result.Failures = append(result.Failures, asc.AssetUploadFailureItem{FileName: plan.Remove[0].FileName, FailedPhase: "delete", Error: cause.Error()})
		} else {
			appendScreenshotUploadFailure(&result, progress, cause)
		}
		result.Total = len(result.Results) + result.Pending
		finalizeAppScreenshotUploadResult(&result)
		artifact := screenshotUploadFailureArtifact{VersionLocalizationID: cfg.LocalizationID, RootPath: root, Path: artifactPath, DisplayType: cfg.DisplayType, Replace: true, Replacement: plan, SetID: set.ID, Files: cfg.Files, OrderedIDs: progress.OrderedIDs, PendingFiles: progress.PendingFiles, PendingAssets: progress.PendingAssets, CleanupFailures: progress.CleanupFailures, Results: result.Results, Failures: result.Failures, Error: cause.Error(), GeneratedAt: cfg.Deps.Now().UTC().Format(time.RFC3339)}
		written, artifactErr := persistScreenshotUploadFailureArtifact(cfg.Deps, artifactPath, artifact)
		if artifactErr != nil {
			return result, screenshotUploadArtifactWriteError(cause, artifactErr)
		}
		result.FailureArtifactPath = written
		progress.PendingDeletions = len(plan.Remove)
		return result, screenshotUploadRetryError(cfg.Deps, result, progress, true, cause)
	}
	if len(plan.Remove) > 0 {
		current, currentOrder, err := replacementInventory(uploadCtx, cfg, set.ID)
		if err != nil {
			return finish(err)
		}
		if !sameReplacementInventory(existing, current) || !sameAssetIDOrder(order, currentOrder) {
			return finish(fmt.Errorf("screenshot set changed before replacement deletion; inspect it before retrying"))
		}
	}
	for len(plan.Remove) > 0 {
		item := plan.Remove[0]
		if err := cfg.Client.DeleteAppScreenshot(uploadCtx, item.AssetID); err != nil {
			return finish(err)
		}
		deleted = append(deleted, asc.AssetUploadResultItem{AssetID: item.AssetID, FileName: item.FileName, State: "deleted"})
		plan.Remove = plan.Remove[1:]
	}
	retainedOrder := make([]string, 0, len(kept))
	for _, id := range order {
		if retained[id] {
			retainedOrder = append(retainedOrder, id)
		}
	}
	if previous != nil {
		progress, err = resumeScreenshotsWithOrderStateWithOpenedFiles(uploadCtx, cfg.Deps.IO.ErrOut, cfg.RequestContext, cfg.Client, set.ID, retainedOrder, files, progress.PendingAssets, root, false, false, opened)
	} else {
		progress, err = uploadScreenshotsWithOrderStateWithOpenedFiles(uploadCtx, cfg.Deps.IO.ErrOut, cfg.RequestContext, cfg.Client, set.ID, retainedOrder, files, root, false, false, opened)
	}
	if err != nil {
		return finish(err)
	}
	desired := orderAssetIDsForLocalFiles(nil, cfg.Files, kept, progress.Results)
	progress.OrderedIDs = desired
	_, current, err := replacementInventory(uploadCtx, cfg, set.ID)
	if err != nil {
		return finish(err)
	}
	if len(current) != len(desired) {
		return finish(fmt.Errorf("screenshot set changed before replacement ordering"))
	}
	want := make(map[string]bool, len(desired))
	for _, id := range desired {
		want[id] = true
	}
	for _, id := range current {
		if !want[id] {
			return finish(fmt.Errorf("screenshot %s appeared before replacement ordering", id))
		}
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
			return finish(fmt.Errorf("replacement screenshot order was not confirmed by App Store Connect"))
		}
	}
	return finish(nil)
}

func presentReplacementAssets(assets []screenshotPendingAsset, present, retained map[string]bool) []screenshotPendingAsset {
	filtered := make([]screenshotPendingAsset, 0, len(assets))
	for _, item := range assets {
		if present[item.AssetID] && !retained[item.AssetID] {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func sameReplacementInventory(before, after []asc.Resource[asc.AppScreenshotAttributes]) bool {
	if len(before) != len(after) {
		return false
	}
	type identity struct{ checksum, state string }
	identities := make(map[string]identity, len(before))
	for _, item := range before {
		state := ""
		if item.Attributes.AssetDeliveryState != nil {
			state = item.Attributes.AssetDeliveryState.State
		}
		identities[item.ID] = identity{strings.ToLower(item.Attributes.SourceFileChecksum), state}
	}
	for _, item := range after {
		state := ""
		if item.Attributes.AssetDeliveryState != nil {
			state = item.Attributes.AssetDeliveryState.State
		}
		before, ok := identities[item.ID]
		if !ok || before != (identity{strings.ToLower(item.Attributes.SourceFileChecksum), state}) {
			return false
		}
	}
	return true
}

func prepareLegacyScreenshotReplacement(ctx context.Context, d *shared.Deps, client *asc.Client, artifact *screenshotUploadFailureArtifact, sourceRootPath string) error {
	if strings.TrimSpace(artifact.DisplayType) == "" && strings.TrimSpace(artifact.DeviceType) != "" {
		displayType, err := normalizeScreenshotDisplayType(artifact.DeviceType)
		if err != nil {
			return err
		}
		artifact.DisplayType = asc.CanonicalScreenshotDisplayTypeForAPI(displayType)
	}
	if strings.TrimSpace(artifact.VersionLocalizationID) == "" || strings.TrimSpace(artifact.DisplayType) == "" {
		return fmt.Errorf("cannot verify legacy replacement scope: the artifact needs versionLocalizationId and displayType; inspect the set and start a new scoped replacement")
	}
	cfg := screenshotUploadConfig[asc.AppScreenshotUploadResult]{
		Deps: d, Client: client, LocalizationID: artifact.VersionLocalizationID,
		DisplayType: artifact.DisplayType, Access: appStoreVersionScreenshotSetAccess, RequestContext: d.Timeout,
	}
	current, order, err := replacementInventory(ctx, cfg, artifact.SetID)
	if err != nil {
		return err
	}
	known := make(map[string]bool)
	for _, id := range artifact.OrderedIDs {
		known[id] = true
	}
	for _, item := range artifact.Results {
		if item.State != "deleted" {
			known[item.AssetID] = true
		}
	}
	for _, item := range append(append([]screenshotPendingAsset{}, artifact.PendingAssets...), artifact.CleanupFailures...) {
		known[item.AssetID] = true
	}
	orderOnly := len(artifact.PendingFiles) == 0 && len(artifact.PendingAssets) == 0 && len(artifact.CleanupFailures) == 0
	for _, item := range current {
		if !known[item.ID] {
			return fmt.Errorf("screenshot %s is not recorded in the legacy replacement artifact; inspect the set before starting a new replacement", item.ID)
		}
		if orderOnly {
			if _, valid := replacementChecksum(item.Attributes.SourceFileChecksum); !valid || item.Attributes.AssetDeliveryState == nil || item.Attributes.AssetDeliveryState.State != "COMPLETE" {
				return fmt.Errorf("cannot verify screenshot %s for legacy replacement ordering; inspect its state and checksum", item.ID)
			}
		}
	}
	if orderOnly {
		want := make(map[string]bool, len(artifact.OrderedIDs))
		for _, id := range artifact.OrderedIDs {
			want[id] = true
		}
		if len(want) != len(artifact.OrderedIDs) || len(order) != len(want) {
			return fmt.Errorf("legacy replacement membership changed; inspect the set before reordering")
		}
		for _, id := range order {
			if !want[id] {
				return fmt.Errorf("legacy replacement membership changed; inspect the set before reordering")
			}
		}
		return nil
	}
	if len(artifact.Files) == 0 {
		return fmt.Errorf("cannot verify legacy replacement source: the artifact has no complete file list; inspect the set and start a new scoped replacement")
	}
	plan := &screenshotReplacement{Checksums: make(map[string]string, len(artifact.Files))}
	for _, path := range artifact.Files {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		checksum, err := computeFileChecksumInRoot(sourceRootPath, absolute)
		if err != nil {
			return fmt.Errorf("cannot verify legacy replacement source %q: %w; restore the original files or inspect the set before starting a new replacement", path, err)
		}
		plan.Checksums[absolute] = checksum
	}
	for _, item := range current {
		found := false
		for _, result := range artifact.Results {
			found = found || result.AssetID == item.ID
		}
		if !found {
			for _, id := range artifact.OrderedIDs {
				if id == item.ID {
					artifact.Results = append(artifact.Results, asc.AssetUploadResultItem{AssetID: item.ID, FileName: item.Attributes.FileName, State: "COMPLETE"})
				}
			}
		}
	}
	artifact.Replacement = plan
	return nil
}

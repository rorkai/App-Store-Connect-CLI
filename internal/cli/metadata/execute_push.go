package metadata

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/storeassets"
)

// PushExecutionOptions controls metadata push planning and apply behavior.
type PushExecutionOptions struct {
	CommandName  string
	AppID        string
	AppInfoID    string
	Version      string
	Platform     string
	Dir          string
	Include      string
	DryRun       bool
	AllowDeletes bool
	Confirm      bool
	ReviewDir    string
	// IfExists selects what an apply does when App Store Connect rejects a
	// localization create because the locale already exists. An empty value
	// means fail, which is the historical behavior.
	IfExists string
}

// ExecutePush computes and optionally applies a metadata push plan.
//
// This is the command-agnostic execution path used by metadata push and
// release orchestration.
func ExecutePush(ctx context.Context, opts PushExecutionOptions) (PushPlanResult, error) {
	result, _, err := ExecutePushWithWarnings(ctx, opts)
	return result, err
}

// ExecutePushWithWarnings computes a metadata push plan plus create-scope
// submission warnings for callers that need to emit them after output succeeds.
func ExecutePushWithWarnings(ctx context.Context, opts PushExecutionOptions) (PushPlanResult, []shared.SubmitReadinessCreateWarning, error) {
	errorPrefix := metadataMutationErrorPrefix(opts.CommandName)
	resolvedAppID := shared.ResolveAppID(opts.AppID)
	if resolvedAppID == "" {
		return PushPlanResult{}, nil, metadataRequiredInputError("--app", "--app is required (or set ASC_APP_ID)")
	}

	versionValue := strings.TrimSpace(opts.Version)
	if versionValue == "" {
		return PushPlanResult{}, nil, metadataRequiredInputError("--version", "--version is required")
	}

	dirValue := strings.TrimSpace(opts.Dir)
	if dirValue == "" {
		return PushPlanResult{}, nil, metadataRequiredInputError("--dir", "--dir is required")
	}
	if strings.TrimSpace(opts.ReviewDir) != "" && !opts.DryRun && !opts.Confirm {
		return PushPlanResult{}, nil, shared.UsageError("--confirm is required when applying an approved metadata plan")
	}

	// Unset means fail: release orchestration builds these options without a
	// conflict policy. Command code validates the raw flag at its own boundary,
	// so an explicit --if-exists "" never reaches here as an empty value.
	ifExistsMode, err := shared.ParseOptionalIfExistsMode(opts.IfExists, shared.IfExistsSkip, shared.IfExistsUpdate)
	if err != nil {
		return PushPlanResult{}, nil, err
	}

	platformValue := strings.TrimSpace(opts.Platform)
	if platformValue != "" {
		normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(platformValue)
		if err != nil {
			return PushPlanResult{}, nil, shared.UsageError(err.Error())
		}
		platformValue = normalizedPlatform
	}

	includeValue := strings.TrimSpace(opts.Include)
	if includeValue == "" {
		includeValue = includeLocalizations
	}
	includes, err := parseIncludes(includeValue)
	if err != nil {
		return PushPlanResult{}, nil, shared.UsageError(err.Error())
	}

	clip, previews, cleanup, err := loadStoreAssetInputs(ctx, dirValue, includes)
	if err != nil {
		return PushPlanResult{}, nil, fmt.Errorf("%s: %w", errorPrefix, err)
	}
	defer cleanup()
	if err := requireConfirmedStoreAssets(opts, clip, previews); err != nil {
		return PushPlanResult{}, nil, err
	}
	localBundle := localMetadataBundle{}
	if includesScope(includes, includeLocalizations) {
		localBundle, err = loadLocalMetadataWithAssets(dirValue, versionValue, clip != nil || len(previews) > 0)
		if err != nil {
			return PushPlanResult{}, nil, fmt.Errorf("%s: %w", errorPrefix, err)
		}
	}

	if err := validateDefaultClearFields(localBundle, versionValue); err != nil {
		return PushPlanResult{}, nil, shared.UsageError(err.Error())
	}

	localizationsSelected := localBundle.appInfoManaged || localBundle.versionManaged

	client, err := shared.GetASCClient()
	if err != nil {
		return PushPlanResult{}, nil, fmt.Errorf("%s: %w", errorPrefix, err)
	}

	type versionResolution struct {
		id    string
		state string
	}
	resolvedVersion, err := shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (versionResolution, error) {
		id, state, resolveErr := resolveVersionID(requestCtx, client, resolvedAppID, versionValue, platformValue)
		return versionResolution{id: id, state: state}, resolveErr
	})
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return PushPlanResult{}, nil, err
		}
		return PushPlanResult{}, nil, fmt.Errorf("%s: %w", errorPrefix, err)
	}
	versionIDValue := resolvedVersion.id
	versionStateValue := resolvedVersion.state
	// An absent app-info directory leaves the app-info scope unmanaged, so the
	// app info is not resolved: an ambiguous app-info selection must not block
	// a version-only push.
	appInfoIDValue := strings.TrimSpace(opts.AppInfoID)
	if localBundle.appInfoManaged {
		appInfoIDValue, err = shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (string, error) {
			return resolveMetadataPushAppInfoID(
				requestCtx,
				client,
				opts.CommandName,
				resolvedAppID,
				strings.TrimSpace(opts.AppInfoID),
				versionValue,
				platformValue,
				dirValue,
				versionStateValue,
			)
		})
		if err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return PushPlanResult{}, nil, err
			}
			return PushPlanResult{}, nil, fmt.Errorf("%s: %w", errorPrefix, err)
		}
	}

	// Only managed JSON scopes participate in localization changes. Preview
	// uploads also need version localization IDs, but must not plan their deletion.
	var remoteAppInfoItems []asc.Resource[asc.AppInfoLocalizationAttributes]
	if localBundle.appInfoManaged {
		remoteAppInfoItems, err = fetchAppInfoLocalizations(ctx, client, appInfoIDValue)
		if err != nil {
			return PushPlanResult{}, nil, fmt.Errorf("%s: %w", errorPrefix, err)
		}
	}
	var remoteVersionItems []asc.Resource[asc.AppStoreVersionLocalizationAttributes]
	if localBundle.versionManaged || len(previews) > 0 {
		remoteVersionItems, err = fetchVersionLocalizations(ctx, client, versionIDValue)
		if err != nil {
			return PushPlanResult{}, nil, fmt.Errorf("%s: %w", errorPrefix, err)
		}
	}

	remoteAppInfo := make(map[string]AppInfoLocalization, len(remoteAppInfoItems))
	for _, item := range remoteAppInfoItems {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		remoteAppInfo[locale] = NormalizeAppInfoLocalization(AppInfoLocalization{
			Name:              item.Attributes.Name,
			Subtitle:          item.Attributes.Subtitle,
			PrivacyPolicyURL:  item.Attributes.PrivacyPolicyURL,
			PrivacyChoicesURL: item.Attributes.PrivacyChoicesURL,
			PrivacyPolicyText: item.Attributes.PrivacyPolicyText,
		})
	}

	// Preview uploads need localization IDs even when JSON version metadata is
	// unmanaged. Keep those IDs out of the localization mutation plan.
	managedVersionItems := remoteVersionItems
	if !localBundle.versionManaged {
		managedVersionItems = nil
	}
	remoteVersion := remoteVersionItemsToVersionMap(managedVersionItems)

	localAppInfo := applyDefaultAppInfoFallback(localBundle.appInfo, localBundle.defaultAppInfo, remoteAppInfo, opts.AllowDeletes)
	localVersion := applyDefaultVersionFallback(localBundle.version, localBundle.defaultVersion, remoteVersion, opts.AllowDeletes)
	if opts.AllowDeletes {
		for _, preview := range previews {
			_, remoteExists := remoteVersion[preview.Locale]
			_, localExists := localVersion[preview.Locale]
			if remoteExists && !localExists {
				return PushPlanResult{}, nil, shared.UsageErrorf("version localization %q is required by local previews but would be deleted; add version/%s/%s.json to retain it, or exclude previews before deleting it", preview.Locale, versionValue, preview.Locale)
			}
		}
	}
	if err := validateVersionClearOnlyLocales(localVersion, remoteVersion); err != nil {
		return PushPlanResult{}, nil, shared.UsageError(err.Error())
	}
	lateAppInfoIDs := map[string]string(nil)
	if err := validateMetadataCreatePrerequisites(localAppInfo, remoteAppInfo); err != nil {
		if opts.DryRun || ifExistsMode == shared.IfExistsFail {
			return PushPlanResult{}, nil, shared.UsageError(err.Error())
		}
		var missingLocale string
		lateAppInfoIDs, missingLocale, err = findLateExistingAppInfoLocalizations(ctx, client, appInfoIDValue, localAppInfo, remoteAppInfo)
		if err != nil {
			return PushPlanResult{}, nil, fmt.Errorf("%s: %w", errorPrefix, err)
		}
		if missingLocale != "" {
			return PushPlanResult{}, nil, shared.UsageError(fmt.Sprintf("app-info localization %q requires name when creating a new locale", missingLocale))
		}
	}
	warningMode := shared.SubmitReadinessCreateModePlanned
	if !opts.DryRun {
		warningMode = shared.SubmitReadinessCreateModeApplied
	}
	submitOpts := shared.SubmitReadinessOptions{}
	if versionCreateWarningsNeedUpdateContext(localVersion, remoteVersion) {
		readinessCtx, readinessCancel := shared.ContextWithTimeout(ctx)
		submitOpts = shared.ResolveSubmitReadinessOptionsForVersionBestEffort(readinessCtx, client, versionIDValue, resolvedAppID, platformValue)
		readinessCancel()
	}
	warnings := versionCreateWarningsForPatches(localVersion, remoteVersion, warningMode, submitOpts)

	adds, updates, deletes, appInfoCalls := buildScopePlan(
		appInfoDirName,
		"",
		appInfoPlanFields,
		appInfoToPlanFields(localAppInfo),
		appInfoToFieldMap(remoteAppInfo),
	)
	versionAdds, versionUpdates, versionDeletes, versionCalls := buildScopePlan(
		versionDirName,
		versionValue,
		versionPlanFields,
		versionToPlanFields(localVersion),
		versionToFieldMap(remoteVersion),
	)
	adds = append(adds, versionAdds...)
	updates = append(updates, versionUpdates...)
	deletes = append(deletes, versionDeletes...)

	sortPlanItems(adds)
	sortPlanItems(updates)
	sortPlanItems(deletes)

	apiCalls := buildAPICallSummary(appInfoCalls, versionCalls)

	result := PushPlanResult{
		AppID:     resolvedAppID,
		AppInfoID: appInfoIDValue,
		Version:   versionValue,
		VersionID: versionIDValue,
		Dir:       dirValue,
		DryRun:    opts.DryRun,
		Includes:  includes,
		Adds:      adds,
		Updates:   updates,
		Deletes:   deletes,
		APICalls:  apiCalls,
	}

	assetPlan, err := storeassets.PrepareImport(ctx, client, resolvedAppID, versionIDValue, clip, previews)
	if err != nil {
		return PushPlanResult{}, warnings, fmt.Errorf("%s: %w", errorPrefix, err)
	}
	addStoreAssetChanges(&result, assetPlan)

	if strings.TrimSpace(opts.ReviewDir) != "" {
		if err := VerifyApprovedMetadataPlan(opts, result, opts.ReviewDir); err != nil {
			return PushPlanResult{}, warnings, err
		}
	}

	if opts.DryRun {
		return result, warnings, nil
	}

	if len(result.Deletes) > 0 {
		if !opts.AllowDeletes {
			return PushPlanResult{}, nil, shared.UsageError("--allow-deletes is required to apply delete operations")
		}
		if !opts.Confirm {
			return PushPlanResult{}, nil, shared.UsageError("--confirm is required when applying delete operations")
		}
	}
	if !opts.Confirm {
		for _, update := range result.Updates {
			if update.Reason == "field cleared locally" {
				return PushPlanResult{}, nil, shared.UsageError("--confirm is required when applying field clear operations")
			}
		}
	}
	if ifExistsMode == shared.IfExistsUpdate && !opts.Confirm {
		if scope, locale, ok := firstPlannedCreateClear(localAppInfo, remoteAppInfo, localVersion, remoteVersion); ok {
			return PushPlanResult{}, nil, shared.UsageError(fmt.Sprintf("--confirm is required when --if-exists update may apply field clear operations (%s localization %q is planned as a create and clears fields if it already exists)", scope, locale))
		}
	}

	var actions []ApplyAction
	var applyErr error
	if localizationsSelected {
		actions, applyErr = applyMetadataPlan(
			ctx,
			client,
			appInfoIDValue,
			versionIDValue,
			versionValue,
			localAppInfo,
			localVersion,
			remoteAppInfoItems,
			managedVersionItems,
			opts.AllowDeletes,
			metadataIfExistsOptions{mode: ifExistsMode, prefix: errorPrefix, lateAppInfoIDs: lateAppInfoIDs},
		)
	}

	result.Actions = actions
	if applyErr == nil {
		localeIDs := map[string]string{}
		for _, item := range remoteVersionItems {
			localeIDs[item.Attributes.Locale] = item.ID
		}
		for _, action := range actions {
			if action.Scope == versionDirName && (action.Status == metadataActionStatusSucceeded || action.Status == metadataActionStatusSkipped) && action.LocalizationID != "" {
				localeIDs[action.Locale] = action.LocalizationID
			}
		}
		receipts, assetErr := assetPlan.Apply(ctx, client, localeIDs)
		appendStoreAssetActions(&result, receipts)
		if assetErr != nil {
			applyErr = assetErr
			hasFailure := false
			for _, receipt := range receipts {
				if receipt.Status == "failed" {
					hasFailure = true
				}
			}
			if !hasFailure {
				result.Actions = append(result.Actions, ApplyAction{Scope: "store-assets", Action: "apply", Status: "failed", Error: shared.SanitizeTerminal(assetErr.Error())})
			}
		}
	}
	result.Total = len(result.Actions)
	for _, action := range result.Actions {
		switch action.Status {
		case metadataActionStatusFailed:
			result.Failed++
		case metadataActionStatusSkipped:
			result.Skipped++
		default:
			result.Succeeded++
		}
	}
	result.Applied = applyErr == nil && result.Failed == 0

	if result.Failed > 0 {
		artifactPath, artifactErr := writeMetadataPushFailureArtifact(result, opts.CommandName)
		if artifactErr != nil {
			result.FailureArtifactError = artifactErr.Error()
		} else {
			result.FailureArtifactPath = artifactPath
		}
	}
	if !opts.DryRun {
		warnings = successfulVersionCreateWarnings(warnings, actions, remoteVersion)
	}

	if applyErr != nil {
		return result, warnings, fmt.Errorf("%s: %w", errorPrefix, applyErr)
	}
	return result, warnings, nil
}

// validateVersionClearOnlyLocales prevents a promotional-text clear from
// being silently treated as a no-op when its version localization is missing.
// Set fields can still create a localization; a null field on that new resource
// is already empty and remains omitted from the create payload.
// firstPlannedCreateClear reports the first locale planned as a create whose
// file also clears fields. A create cannot carry those clears, so the plan
// never lists them as "field cleared locally", but --if-exists update applies
// them when the locale turns out to exist and must be confirmed like any other
// clear.
func firstPlannedCreateClear(
	localAppInfo map[string]appInfoLocalPatch,
	remoteAppInfo map[string]AppInfoLocalization,
	localVersion map[string]versionLocalPatch,
	remoteVersion map[string]VersionLocalization,
) (string, string, bool) {
	for _, locale := range sortedKeys(localAppInfo) {
		if _, exists := remoteAppInfo[locale]; !exists && len(localAppInfo[locale].clearFields) > 0 {
			return appInfoDirName, locale, true
		}
	}
	for _, locale := range sortedKeys(localVersion) {
		if _, exists := remoteVersion[locale]; !exists && len(localVersion[locale].clearFields) > 0 {
			return versionDirName, locale, true
		}
	}
	return "", "", false
}

func validateVersionClearOnlyLocales(
	localVersion map[string]versionLocalPatch,
	remoteVersion map[string]VersionLocalization,
) error {
	for _, locale := range sortedKeys(localVersion) {
		patch := localVersion[locale]
		if len(patch.setFields) > 0 || len(patch.clearFields) == 0 {
			continue
		}
		if _, exists := remoteVersion[locale]; !exists {
			return fmt.Errorf("version localization %q cannot be cleared because no existing localization was found", locale)
		}
	}
	return nil
}

func validateDefaultClearFields(bundle localMetadataBundle, version string) error {
	if bundle.defaultAppInfo != nil && len(bundle.defaultAppInfo.clearFields) > 0 {
		return fmt.Errorf("clear fields in app-info/default.json are not supported; move null values to an explicit locale file")
	}
	if bundle.defaultVersion != nil && len(bundle.defaultVersion.clearFields) > 0 {
		return fmt.Errorf("clear fields in version/%s/default.json are not supported; move null values to an explicit locale file", version)
	}
	return nil
}

func metadataMutationErrorPrefix(commandName string) string {
	name := strings.TrimSpace(commandName)
	if name == "" {
		name = "push"
	}
	return "metadata " + name
}

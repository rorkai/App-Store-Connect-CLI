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

	localizationsSelected := includesScope(includes, includeLocalizations) && (len(localBundle.appInfo) > 0 || len(localBundle.version) > 0 || localBundle.defaultAppInfo != nil || localBundle.defaultVersion != nil)

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
	var appInfoIDValue string
	var remoteAppInfoItems []asc.Resource[asc.AppInfoLocalizationAttributes]
	if localizationsSelected {
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

		remoteAppInfoItems, err = fetchAppInfoLocalizations(ctx, client, appInfoIDValue)
		if err != nil {
			return PushPlanResult{}, nil, fmt.Errorf("%s: %w", errorPrefix, err)
		}
	}

	remoteVersionItems, err := fetchVersionLocalizations(ctx, client, versionIDValue)
	if err != nil {
		return PushPlanResult{}, nil, fmt.Errorf("%s: %w", errorPrefix, err)
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

	remoteVersion := remoteVersionItemsToVersionMap(remoteVersionItems)
	if !localizationsSelected {
		remoteVersion = map[string]VersionLocalization{}
	}

	localAppInfo := applyDefaultAppInfoFallback(localBundle.appInfo, localBundle.defaultAppInfo, remoteAppInfo, opts.AllowDeletes)
	localVersion := applyDefaultVersionFallback(localBundle.version, localBundle.defaultVersion, remoteVersion, opts.AllowDeletes)
	if err := validateMetadataCreatePrerequisites(localAppInfo, remoteAppInfo); err != nil {
		return PushPlanResult{}, nil, shared.UsageError(err.Error())
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
			remoteVersionItems,
			opts.AllowDeletes,
		)
	}

	result.Actions = actions
	if applyErr == nil {
		localeIDs := map[string]string{}
		for _, item := range remoteVersionItems {
			localeIDs[item.Attributes.Locale] = item.ID
		}
		for _, action := range actions {
			if action.Scope == versionDirName && action.Status == "succeeded" && action.LocalizationID != "" {
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
		if action.Status == "failed" {
			result.Failed++
			continue
		}
		result.Succeeded++
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

func metadataMutationErrorPrefix(commandName string) string {
	name := strings.TrimSpace(commandName)
	if name == "" {
		name = "push"
	}
	return "metadata " + name
}

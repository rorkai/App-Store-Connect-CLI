package metadata

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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

	localBundle, err := loadLocalMetadata(dirValue, versionValue)
	if err != nil {
		return PushPlanResult{}, nil, fmt.Errorf("%s: %w", errorPrefix, err)
	}

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
	appInfoIDValue, err := shared.RetryReadWithFreshTimeout(ctx, func(requestCtx context.Context) (string, error) {
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

	remoteAppInfoItems, err := fetchAppInfoLocalizations(ctx, client, appInfoIDValue)
	if err != nil {
		return PushPlanResult{}, nil, fmt.Errorf("%s: %w", errorPrefix, err)
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

	actions, applyErr := applyMetadataPlan(
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
		metadataIfExistsOptions{mode: ifExistsMode, prefix: errorPrefix},
	)
	result.Actions = actions
	result.Total = len(actions)
	for _, action := range actions {
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

func metadataMutationErrorPrefix(commandName string) string {
	name := strings.TrimSpace(commandName)
	if name == "" {
		name = "push"
	}
	return "metadata " + name
}

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
	AppID        string
	AppInfoID    string
	Version      string
	Platform     string
	Dir          string
	Include      string
	DryRun       bool
	AllowDeletes bool
	Confirm      bool
}

// ExecutePush computes and optionally applies a metadata push plan.
//
// This is the command-agnostic execution path used by metadata push and
// release orchestration.
func ExecutePush(ctx context.Context, opts PushExecutionOptions) (PushPlanResult, error) {
	resolvedAppID := shared.ResolveAppID(opts.AppID)
	if resolvedAppID == "" {
		return PushPlanResult{}, shared.UsageError("--app is required (or set ASC_APP_ID)")
	}

	versionValue := strings.TrimSpace(opts.Version)
	if versionValue == "" {
		return PushPlanResult{}, shared.UsageError("--version is required")
	}

	dirValue := strings.TrimSpace(opts.Dir)
	if dirValue == "" {
		return PushPlanResult{}, shared.UsageError("--dir is required")
	}

	platformValue := strings.TrimSpace(opts.Platform)
	if platformValue != "" {
		normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(platformValue)
		if err != nil {
			return PushPlanResult{}, shared.UsageError(err.Error())
		}
		platformValue = normalizedPlatform
	}

	includeValue := strings.TrimSpace(opts.Include)
	if includeValue == "" {
		includeValue = includeLocalizations
	}
	includes, err := parseIncludes(includeValue)
	if err != nil {
		return PushPlanResult{}, shared.UsageError(err.Error())
	}

	localBundle, err := loadLocalMetadata(dirValue, versionValue)
	if err != nil {
		return PushPlanResult{}, err
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return PushPlanResult{}, fmt.Errorf("metadata push: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	versionIDValue, versionStateValue, err := resolveVersionID(requestCtx, client, resolvedAppID, versionValue, platformValue)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return PushPlanResult{}, err
		}
		return PushPlanResult{}, fmt.Errorf("metadata push: %w", err)
	}
	appInfoIDValue, err := resolveMetadataPushAppInfoID(
		requestCtx,
		client,
		resolvedAppID,
		strings.TrimSpace(opts.AppInfoID),
		versionValue,
		platformValue,
		dirValue,
		versionStateValue,
	)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return PushPlanResult{}, err
		}
		return PushPlanResult{}, fmt.Errorf("metadata push: %w", err)
	}

	remoteAppInfoItems, err := fetchAppInfoLocalizations(requestCtx, client, appInfoIDValue)
	if err != nil {
		return PushPlanResult{}, fmt.Errorf("metadata push: %w", err)
	}
	remoteVersionItems, err := fetchVersionLocalizations(requestCtx, client, versionIDValue)
	if err != nil {
		return PushPlanResult{}, fmt.Errorf("metadata push: %w", err)
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

	if opts.DryRun {
		return result, nil
	}

	if len(result.Deletes) > 0 {
		if !opts.AllowDeletes {
			return PushPlanResult{}, shared.UsageError("--allow-deletes is required to apply delete operations")
		}
		if !opts.Confirm {
			return PushPlanResult{}, shared.UsageError("--confirm is required when applying delete operations")
		}
	}

	actions, applyErr := applyMetadataPlan(
		requestCtx,
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
	if applyErr != nil {
		return PushPlanResult{}, fmt.Errorf("metadata push: %w", applyErr)
	}
	result.Applied = true
	result.Actions = actions

	return result, nil
}

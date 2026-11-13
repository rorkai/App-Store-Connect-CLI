package optimize

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type resolvedSearchMetadata struct {
	AppID     string
	VersionID string
	AppInfoID string
	Platform  string
	Metadata  searchMetadataSnapshot
}

func resolveSearchMetadata(ctx context.Context, appSelector, version, platform, appInfoOverride, locale string) (resolvedSearchMetadata, error) {
	client, err := shared.GetASCClient()
	if err != nil {
		return resolvedSearchMetadata{}, err
	}

	lookupCtx, cancel := shared.ContextWithTimeout(ctx)
	appID, err := shared.ResolveAppIDWithLookup(lookupCtx, client, appSelector)
	cancel()
	if err != nil {
		return resolvedSearchMetadata{}, fmt.Errorf("resolve app: %w", err)
	}

	versionCtx, cancel := shared.ContextWithTimeout(ctx)
	versionID, versionState, err := shared.ResolveAppStoreVersionIDAndState(versionCtx, client, appID, version, platform)
	cancel()
	if err != nil {
		return resolvedSearchMetadata{}, fmt.Errorf("resolve version: %w", err)
	}

	versionLocalizationCtx, cancel := shared.ContextWithTimeout(ctx)
	versionLocalizations, err := client.GetAppStoreVersionLocalizations(
		versionLocalizationCtx,
		versionID,
		asc.WithAppStoreVersionLocalizationLocales([]string{locale}),
		asc.Limit[asc.AppStoreVersionLocalizationsOption](2),
	)
	cancel()
	if err != nil {
		return resolvedSearchMetadata{}, fmt.Errorf("read version localization %q: %w", locale, err)
	}
	if versionLocalizations == nil || len(versionLocalizations.Data) == 0 {
		if versionLocalizations != nil && strings.TrimSpace(versionLocalizations.Links.Next) != "" {
			return resolvedSearchMetadata{}, shared.MarkAmbiguousSelectionSample(shared.AmbiguousLocalizationError("version localization", locale, nil))
		}
		return resolvedSearchMetadata{}, fmt.Errorf("version localization %q not found", locale)
	}
	if len(versionLocalizations.Data) > 1 || strings.TrimSpace(versionLocalizations.Links.Next) != "" {
		ambiguous := shared.AmbiguousLocalizationError("version localization", locale, shared.LocalizationCandidates(versionLocalizations.Data, func(attributes asc.AppStoreVersionLocalizationAttributes) string { return attributes.Locale }))
		if strings.TrimSpace(versionLocalizations.Links.Next) != "" {
			return resolvedSearchMetadata{}, shared.MarkAmbiguousSelectionSample(ambiguous)
		}
		return resolvedSearchMetadata{}, ambiguous
	}

	appInfoCtx, cancel := shared.ContextWithTimeout(ctx)
	appInfoID, err := resolveSearchAppInfoID(appInfoCtx, client, appID, appInfoOverride, versionState)
	cancel()
	if err != nil {
		return resolvedSearchMetadata{}, fmt.Errorf("resolve app info: %w", err)
	}

	appInfoLocalizationCtx, cancel := shared.ContextWithTimeout(ctx)
	appInfoLocalizations, err := client.GetAppInfoLocalizations(
		appInfoLocalizationCtx,
		appInfoID,
		asc.WithAppInfoLocalizationLocales([]string{locale}),
		asc.Limit[asc.AppInfoLocalizationsOption](2),
	)
	cancel()
	if err != nil {
		return resolvedSearchMetadata{}, fmt.Errorf("read app info localization %q: %w", locale, err)
	}
	if appInfoLocalizations == nil || len(appInfoLocalizations.Data) == 0 {
		if appInfoLocalizations != nil && strings.TrimSpace(appInfoLocalizations.Links.Next) != "" {
			return resolvedSearchMetadata{}, shared.MarkAmbiguousSelectionSample(shared.AmbiguousLocalizationError("app info localization", locale, nil))
		}
		return resolvedSearchMetadata{}, fmt.Errorf("app info localization %q not found", locale)
	}
	if len(appInfoLocalizations.Data) > 1 || strings.TrimSpace(appInfoLocalizations.Links.Next) != "" {
		ambiguous := shared.AmbiguousLocalizationError("app info localization", locale, shared.LocalizationCandidates(appInfoLocalizations.Data, func(attributes asc.AppInfoLocalizationAttributes) string { return attributes.Locale }))
		if strings.TrimSpace(appInfoLocalizations.Links.Next) != "" {
			return resolvedSearchMetadata{}, shared.MarkAmbiguousSelectionSample(ambiguous)
		}
		return resolvedSearchMetadata{}, ambiguous
	}

	return resolvedSearchMetadata{
		AppID:     appID,
		VersionID: versionID,
		AppInfoID: appInfoID,
		Platform:  platform,
		Metadata: searchMetadataSnapshot{
			Name:     strings.TrimSpace(appInfoLocalizations.Data[0].Attributes.Name),
			Subtitle: strings.TrimSpace(appInfoLocalizations.Data[0].Attributes.Subtitle),
			Keywords: strings.TrimSpace(versionLocalizations.Data[0].Attributes.Keywords),
		},
	}, nil
}

func resolveSearchAppInfoID(ctx context.Context, client *asc.Client, appID, appInfoOverride, versionState string) (string, error) {
	if strings.TrimSpace(appInfoOverride) != "" {
		return shared.ResolveOwnedAppInfoID(ctx, client, appID, appInfoOverride)
	}

	appInfos, err := client.GetAppInfos(ctx, appID)
	if err != nil {
		return "", err
	}
	if appInfos == nil {
		return "", fmt.Errorf("empty app infos response for app %q", appID)
	}
	pageHasNext := strings.TrimSpace(appInfos.Links.Next) != ""
	if pageHasNext {
		return "", shared.MarkAmbiguousSelectionSample(shared.AmbiguousAppInfoError(appID, "--app-info", asc.AppInfoCandidates(appInfos.Data)))
	}
	if len(appInfos.Data) == 0 {
		return "", fmt.Errorf("no app info found for app %q", appID)
	}
	if len(appInfos.Data) == 1 {
		return strings.TrimSpace(appInfos.Data[0].ID), nil
	}
	candidates := asc.AppInfoCandidates(appInfos.Data)
	if resolvedID, ok := asc.AutoResolveAppInfoIDByVersionState(candidates, versionState); ok {
		return resolvedID, nil
	}
	return "", shared.AmbiguousAppInfoError(appID, "--app-info", candidates)
}

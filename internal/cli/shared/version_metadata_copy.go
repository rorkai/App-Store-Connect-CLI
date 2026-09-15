package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// VersionMetadataCopyOptions defines a metadata carry-forward operation between App Store versions.
type VersionMetadataCopyOptions struct {
	AppID                string
	Platform             string
	SourceVersion        string
	DestinationVersionID string
	SelectedFields       []string
	DryRun               bool
}

// NormalizeVersionMetadataCopyFields validates a comma-separated field list against version localization keys.
func NormalizeVersionMetadataCopyFields(value, flagName string) ([]string, error) {
	fields := SplitUniqueCSV(value)
	if len(fields) == 0 {
		return nil, nil
	}

	allowed := make(map[string]struct{}, len(VersionLocalizationKeys()))
	for _, field := range VersionLocalizationKeys() {
		allowed[field] = struct{}{}
	}
	for _, field := range fields {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("%s must be one of: %s", flagName, strings.Join(VersionLocalizationKeys(), ", "))
		}
	}

	return fields, nil
}

// ResolveVersionMetadataCopyFields applies explicit includes/excludes to the supported metadata field set.
func ResolveVersionMetadataCopyFields(copyFields, excludeFields []string) ([]string, error) {
	selected := make([]string, 0, len(VersionLocalizationKeys()))
	if len(copyFields) == 0 {
		selected = append(selected, VersionLocalizationKeys()...)
	} else {
		selected = append(selected, copyFields...)
	}

	if len(excludeFields) > 0 {
		excluded := make(map[string]struct{}, len(excludeFields))
		for _, field := range excludeFields {
			excluded[field] = struct{}{}
		}

		filtered := make([]string, 0, len(selected))
		for _, field := range selected {
			if _, skip := excluded[field]; skip {
				continue
			}
			filtered = append(filtered, field)
		}
		selected = filtered
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no metadata fields selected after applying --copy-fields and --exclude-fields")
	}

	return selected, nil
}

// CopyVersionMetadataFromSource copies selected localization metadata from one version into another.
func CopyVersionMetadataFromSource(
	ctx context.Context,
	client *asc.Client,
	opts VersionMetadataCopyOptions,
) (*asc.AppStoreVersionMetadataCopySummary, error) {
	sourceVersion, err := findSourceAppStoreVersion(
		ctx,
		client,
		strings.TrimSpace(opts.AppID),
		strings.TrimSpace(opts.Platform),
		strings.TrimSpace(opts.SourceVersion),
		strings.TrimSpace(opts.DestinationVersionID),
	)
	if err != nil {
		return nil, err
	}

	sourceLocalizations, err := listAllAppStoreVersionLocalizations(ctx, client, sourceVersion.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list source localizations: %w", err)
	}
	destinationLocalizations, err := listAllAppStoreVersionLocalizations(ctx, client, strings.TrimSpace(opts.DestinationVersionID))
	if err != nil {
		return nil, fmt.Errorf("failed to list destination localizations: %w", err)
	}

	destinationByLocale := make(map[string]string, len(destinationLocalizations))
	for _, item := range destinationLocalizations {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		destinationByLocale[locale] = item.ID
	}

	summary := &asc.AppStoreVersionMetadataCopySummary{
		SourceVersion:   strings.TrimSpace(opts.SourceVersion),
		SourceVersionID: sourceVersion.ID,
		SelectedFields:  append([]string(nil), opts.SelectedFields...),
	}

	for _, sourceLocalization := range sourceLocalizations {
		locale := strings.TrimSpace(sourceLocalization.Attributes.Locale)
		if locale == "" {
			continue
		}

		destinationLocalizationID, ok := destinationByLocale[locale]
		if !ok {
			summary.SkippedLocales = append(summary.SkippedLocales, locale)
			continue
		}

		attributes, copiedFields := buildMetadataCopyAttributes(sourceLocalization.Attributes, opts.SelectedFields)
		if copiedFields == 0 {
			continue
		}

		if !opts.DryRun {
			if _, err := client.UpdateAppStoreVersionLocalization(ctx, destinationLocalizationID, attributes); err != nil {
				return nil, fmt.Errorf("failed to copy metadata for locale %q: %w", locale, err)
			}
		}
		summary.CopiedLocales++
		summary.CopiedFieldUpdates += copiedFields
	}

	return summary, nil
}

func findSourceAppStoreVersion(
	ctx context.Context,
	client *asc.Client,
	appID string,
	platform string,
	sourceVersionString string,
	excludeVersionID string,
) (*asc.Resource[asc.AppStoreVersionAttributes], error) {
	versionValue := strings.TrimSpace(sourceVersionString)
	platformValue := strings.TrimSpace(platform)

	resp, err := client.GetAppStoreVersions(
		ctx,
		appID,
		asc.WithAppStoreVersionsVersionStrings([]string{versionValue}),
		asc.WithAppStoreVersionsPlatforms([]string{platformValue}),
		asc.WithAppStoreVersionsLimit(200),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup source version %q: %w", versionValue, err)
	}

	matches := make([]asc.Resource[asc.AppStoreVersionAttributes], 0, len(resp.Data))
	for _, version := range resp.Data {
		if strings.TrimSpace(version.ID) == strings.TrimSpace(excludeVersionID) {
			continue
		}
		if strings.TrimSpace(version.Attributes.VersionString) != versionValue {
			continue
		}
		if strings.TrimSpace(string(version.Attributes.Platform)) != platformValue {
			continue
		}
		matches = append(matches, version)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("source version %q not found for platform %s", versionValue, platformValue)
	}
	if len(matches) > 1 {
		return nil, &AmbiguousSelectionError{
			Kind:        "source app store version",
			Description: fmt.Sprintf("version %q on platform %s", versionValue, platformValue),
			Candidates:  AppStoreVersionCandidates(matches),
			Hint:        "App Store Connect returned duplicate versions for this string; --copy-metadata-from cannot select between them.",
		}
	}

	return &matches[0], nil
}

func listAllAppStoreVersionLocalizations(
	ctx context.Context,
	client *asc.Client,
	versionID string,
) ([]asc.Resource[asc.AppStoreVersionLocalizationAttributes], error) {
	firstPage, err := client.GetAppStoreVersionLocalizations(ctx, versionID, asc.WithAppStoreVersionLocalizationsLimit(200))
	if err != nil {
		return nil, err
	}
	allPages, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppStoreVersionLocalizations(ctx, versionID, asc.WithAppStoreVersionLocalizationsNextURL(nextURL))
	})
	if err != nil {
		return nil, err
	}

	allLocalizations, ok := allPages.(*asc.AppStoreVersionLocalizationsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected localizations response type: %T", allPages)
	}
	return allLocalizations.Data, nil
}

func buildMetadataCopyAttributes(
	source asc.AppStoreVersionLocalizationAttributes,
	selectedFields []string,
) (asc.AppStoreVersionLocalizationAttributes, int) {
	attrs := asc.AppStoreVersionLocalizationAttributes{}
	copiedFields := 0

	for _, field := range selectedFields {
		switch field {
		case "description":
			if strings.TrimSpace(source.Description) != "" {
				attrs.Description = source.Description
				copiedFields++
			}
		case "keywords":
			if strings.TrimSpace(source.Keywords) != "" {
				attrs.Keywords = source.Keywords
				copiedFields++
			}
		case "marketingUrl":
			if strings.TrimSpace(source.MarketingURL) != "" {
				attrs.MarketingURL = source.MarketingURL
				copiedFields++
			}
		case "promotionalText":
			if strings.TrimSpace(source.PromotionalText) != "" {
				attrs.PromotionalText = source.PromotionalText
				copiedFields++
			}
		case "supportUrl":
			if strings.TrimSpace(source.SupportURL) != "" {
				attrs.SupportURL = source.SupportURL
				copiedFields++
			}
		case "whatsNew":
			if strings.TrimSpace(source.WhatsNew) != "" {
				attrs.WhatsNew = source.WhatsNew
				copiedFields++
			}
		}
	}

	return attrs, copiedFields
}

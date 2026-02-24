package migrate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func resolveAppID(ctx context.Context, client *asc.Client, appFlag string, config DeliverfileConfig) (string, error) {
	if strings.TrimSpace(appFlag) != "" {
		return strings.TrimSpace(appFlag), nil
	}
	if strings.TrimSpace(config.AppIdentifier) != "" {
		if isNumeric(config.AppIdentifier) {
			return config.AppIdentifier, nil
		}
		if client == nil {
			return "", fmt.Errorf("deliverfile app_identifier requires API access to resolve app ID")
		}
		resp, err := client.GetApps(ctx, asc.WithAppsBundleIDs([]string{config.AppIdentifier}), asc.WithAppsLimit(10))
		if err != nil {
			return "", fmt.Errorf("failed to resolve app identifier %q: %w", config.AppIdentifier, err)
		}
		if len(resp.Data) == 0 {
			return "", fmt.Errorf("no app found for bundle ID %q", config.AppIdentifier)
		}
		if len(resp.Data) > 1 {
			return "", fmt.Errorf("multiple apps found for bundle ID %q; use --app", config.AppIdentifier)
		}
		return resp.Data[0].ID, nil
	}
	if appID := shared.ResolveAppID(""); appID != "" {
		return appID, nil
	}
	return "", fmt.Errorf("--app is required (or set ASC_APP_ID or provide Deliverfile app_identifier)")
}

func resolveVersionID(ctx context.Context, client *asc.Client, versionFlag string, appID string, config DeliverfileConfig) (string, error) {
	if strings.TrimSpace(versionFlag) != "" {
		return strings.TrimSpace(versionFlag), nil
	}
	if strings.TrimSpace(config.AppVersion) == "" || strings.TrimSpace(config.Platform) == "" {
		return "", fmt.Errorf("--version-id is required (or set Deliverfile app_version and platform)")
	}
	if client == nil {
		return "", fmt.Errorf("deliverfile app_version requires API access to resolve version ID")
	}
	normalizedPlatform, err := normalizeDeliverfilePlatform(config.Platform)
	if err != nil {
		return "", err
	}
	return shared.ResolveAppStoreVersionID(ctx, client, appID, config.AppVersion, normalizedPlatform)
}

func normalizeDeliverfilePlatform(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ios":
		return "IOS", nil
	case "macos", "mac":
		return "MAC_OS", nil
	case "tvos", "appletvos", "tv_os":
		return "TV_OS", nil
	case "visionos", "vision_os":
		return "VISION_OS", nil
	default:
		return "", fmt.Errorf("unsupported Deliverfile platform %q", value)
	}
}

func collectLocales(localizations []FastlaneLocalization, appInfos []AppInfoFastlaneLocalization, screenshots []ScreenshotPlan) []string {
	localeSet := make(map[string]struct{})
	for _, loc := range localizations {
		if loc.Locale != "" {
			localeSet[loc.Locale] = struct{}{}
		}
	}
	for _, loc := range appInfos {
		if loc.Locale != "" {
			localeSet[loc.Locale] = struct{}{}
		}
	}
	for _, shot := range screenshots {
		if shot.Locale != "" {
			localeSet[shot.Locale] = struct{}{}
		}
	}
	locales := make([]string, 0, len(localeSet))
	for locale := range localeSet {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	return locales
}

func buildMetadataFilePlans(localizations []FastlaneLocalization) []LocalizationFilePlan {
	plans := make([]LocalizationFilePlan, 0, len(localizations))
	for _, loc := range localizations {
		files := versionLocalizationFiles(loc)
		if len(files) == 0 {
			continue
		}
		plans = append(plans, LocalizationFilePlan{
			Locale: loc.Locale,
			Files:  files,
		})
	}
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].Locale < plans[j].Locale
	})
	return plans
}

func buildAppInfoFilePlans(localizations []AppInfoFastlaneLocalization) []LocalizationFilePlan {
	plans := make([]LocalizationFilePlan, 0, len(localizations))
	for _, loc := range localizations {
		files := appInfoLocalizationFiles(loc)
		if len(files) == 0 {
			continue
		}
		plans = append(plans, LocalizationFilePlan{
			Locale: loc.Locale,
			Files:  files,
		})
	}
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].Locale < plans[j].Locale
	})
	return plans
}

func uploadVersionLocalizations(ctx context.Context, client *asc.Client, versionID string, localizations []FastlaneLocalization, localeToID map[string]string) ([]LocalizationUploadItem, error) {
	results := make([]LocalizationUploadItem, 0, len(localizations))
	for _, loc := range localizations {
		attrs := asc.AppStoreVersionLocalizationAttributes{
			Locale:          loc.Locale,
			Description:     loc.Description,
			Keywords:        loc.Keywords,
			WhatsNew:        loc.WhatsNew,
			PromotionalText: loc.PromotionalText,
			SupportURL:      loc.SupportURL,
			MarketingURL:    loc.MarketingURL,
		}
		action := "create"
		localizationID := localeToID[loc.Locale]
		if localizationID != "" {
			action = "update"
			_, err := client.UpdateAppStoreVersionLocalization(ctx, localizationID, attrs)
			if err != nil {
				return nil, fmt.Errorf("migrate import: failed to update %s: %w", loc.Locale, err)
			}
		} else {
			resp, err := client.CreateAppStoreVersionLocalization(ctx, versionID, attrs)
			if err != nil {
				return nil, fmt.Errorf("migrate import: failed to create %s: %w", loc.Locale, err)
			}
			localizationID = resp.Data.ID
			localeToID[loc.Locale] = localizationID
		}

		results = append(results, LocalizationUploadItem{
			Locale:         loc.Locale,
			Fields:         countNonEmptyFields(loc),
			Action:         action,
			LocalizationID: localizationID,
		})
	}
	return results, nil
}

func uploadAppInfoLocalizations(ctx context.Context, client *asc.Client, appID string, appInfoLocs []AppInfoFastlaneLocalization) ([]LocalizationUploadItem, error) {
	if len(appInfoLocs) == 0 {
		return nil, nil
	}
	appInfos, err := client.GetAppInfos(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("migrate import: failed to get app info: %w", err)
	}
	if len(appInfos.Data) == 0 {
		return nil, fmt.Errorf("migrate import: no app info found for app")
	}
	appInfoID := shared.SelectBestAppInfoID(appInfos)
	if strings.TrimSpace(appInfoID) == "" {
		return nil, fmt.Errorf("migrate import: failed to select app info for app")
	}

	existingAppInfoLocs, err := client.GetAppInfoLocalizations(ctx, appInfoID)
	if err != nil {
		return nil, fmt.Errorf("migrate import: failed to fetch app info localizations: %w", err)
	}
	appInfoLocaleToID := make(map[string]string)
	for _, loc := range existingAppInfoLocs.Data {
		appInfoLocaleToID[loc.Attributes.Locale] = loc.ID
	}

	results := make([]LocalizationUploadItem, 0, len(appInfoLocs))
	for _, loc := range appInfoLocs {
		attrs := asc.AppInfoLocalizationAttributes{
			Locale:           loc.Locale,
			Name:             loc.Name,
			Subtitle:         loc.Subtitle,
			PrivacyPolicyURL: loc.PrivacyURL,
		}

		action := "create"
		localizationID := appInfoLocaleToID[loc.Locale]
		if localizationID != "" {
			action = "update"
			if _, err := client.UpdateAppInfoLocalization(ctx, localizationID, attrs); err != nil {
				return nil, fmt.Errorf("migrate import: failed to update app info %s: %w", loc.Locale, err)
			}
		} else {
			resp, err := client.CreateAppInfoLocalization(ctx, appInfoID, attrs)
			if err != nil {
				return nil, fmt.Errorf("migrate import: failed to create app info %s: %w", loc.Locale, err)
			}
			localizationID = resp.Data.ID
		}

		results = append(results, LocalizationUploadItem{
			Locale:         loc.Locale,
			Fields:         countAppInfoFields(loc),
			Action:         action,
			LocalizationID: localizationID,
		})
	}

	return results, nil
}

func uploadReviewInformation(ctx context.Context, client *asc.Client, versionID string, info *ReviewInformation) (*ReviewInfoResult, error) {
	if info == nil {
		return nil, nil
	}

	existing, err := client.GetAppStoreReviewDetailForVersion(ctx, versionID)
	if err != nil {
		if !isNotFoundReviewDetail(err) {
			return nil, fmt.Errorf("migrate import: failed to fetch review information: %w", err)
		}
		created, err := client.CreateAppStoreReviewDetail(ctx, versionID, buildReviewDetailCreateAttributes(info))
		if err != nil {
			return nil, fmt.Errorf("migrate import: failed to create review information: %w", err)
		}
		return &ReviewInfoResult{Action: "create", DetailID: created.Data.ID}, nil
	}

	if existing == nil || existing.Data.ID == "" {
		return nil, fmt.Errorf("migrate import: review information response missing ID")
	}
	if reviewInformationMatches(existing.Data.Attributes, info) {
		return &ReviewInfoResult{Action: "skip", DetailID: existing.Data.ID}, nil
	}
	if _, err := client.UpdateAppStoreReviewDetail(ctx, existing.Data.ID, buildReviewDetailUpdateAttributes(info)); err != nil {
		return nil, fmt.Errorf("migrate import: failed to update review information: %w", err)
	}
	return &ReviewInfoResult{Action: "update", DetailID: existing.Data.ID}, nil
}

func uploadScreenshots(ctx context.Context, client *asc.Client, versionID string, localeToID map[string]string, plans []ScreenshotPlan) ([]ScreenshotUploadResult, error) {
	if len(plans) == 0 {
		return nil, nil
	}

	plansByLocale := make(map[string][]ScreenshotPlan)
	for _, plan := range plans {
		plansByLocale[plan.Locale] = append(plansByLocale[plan.Locale], plan)
	}

	uploadCtx, cancel := assets.ContextWithAssetUploadTimeout(ctx)
	defer cancel()

	results := make([]ScreenshotUploadResult, 0, len(plans))
	for locale, localePlans := range plansByLocale {
		localizationID := localeToID[locale]
		if localizationID == "" {
			resp, err := client.CreateAppStoreVersionLocalization(uploadCtx, versionID, asc.AppStoreVersionLocalizationAttributes{Locale: locale})
			if err != nil {
				return nil, fmt.Errorf("migrate import: failed to create localization for screenshots %s: %w", locale, err)
			}
			localizationID = resp.Data.ID
			localeToID[locale] = localizationID
		}

		existingSets, err := client.GetAppScreenshotSets(uploadCtx, localizationID)
		if err != nil {
			return nil, fmt.Errorf("migrate import: failed to fetch screenshot sets for %s: %w", locale, err)
		}
		setByType := make(map[string]string)
		existingFiles := make(map[string]map[string]bool)
		for _, set := range existingSets.Data {
			setByType[set.Attributes.ScreenshotDisplayType] = set.ID
			screenshots, err := client.GetAppScreenshots(uploadCtx, set.ID)
			if err != nil {
				return nil, fmt.Errorf("migrate import: failed to fetch screenshots for %s: %w", set.ID, err)
			}
			fileNames := make(map[string]bool)
			for _, shot := range screenshots.Data {
				name := strings.TrimSpace(shot.Attributes.FileName)
				if name != "" {
					fileNames[name] = true
				}
			}
			existingFiles[set.Attributes.ScreenshotDisplayType] = fileNames
		}

		for _, plan := range localePlans {
			canonicalDisplayType := asc.CanonicalScreenshotDisplayTypeForAPI(plan.DisplayType)
			setID := setByType[canonicalDisplayType]
			if setID == "" {
				set, err := client.CreateAppScreenshotSet(uploadCtx, localizationID, canonicalDisplayType)
				if err != nil {
					return nil, fmt.Errorf("migrate import: failed to create screenshot set %s: %w", canonicalDisplayType, err)
				}
				setID = set.Data.ID
				setByType[canonicalDisplayType] = setID
				existingFiles[canonicalDisplayType] = make(map[string]bool)
			}

			fileNames := existingFiles[canonicalDisplayType]
			if fileNames == nil {
				fileNames = make(map[string]bool)
				existingFiles[canonicalDisplayType] = fileNames
			}

			result := ScreenshotUploadResult{
				Locale:      plan.Locale,
				DisplayType: canonicalDisplayType,
			}

			for _, filePath := range plan.Files {
				name := filepath.Base(filePath)
				if fileNames[name] {
					result.Skipped = append(result.Skipped, SkippedItem{
						Path:   filePath,
						Reason: "already exists",
					})
					continue
				}
				item, err := assets.UploadScreenshotAsset(uploadCtx, client, setID, filePath)
				if err != nil {
					return nil, fmt.Errorf("migrate import: failed to upload screenshot %s: %w", filePath, err)
				}
				fileNames[name] = true
				result.Uploaded = append(result.Uploaded, item)
			}

			results = append(results, result)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Locale == results[j].Locale {
			return results[i].DisplayType < results[j].DisplayType
		}
		return results[i].Locale < results[j].Locale
	})
	return results, nil
}

func isNotFoundReviewDetail(err error) bool {
	if asc.IsNotFound(err) {
		return true
	}
	if apiErr, ok := errors.AsType[*asc.APIError](err); ok {
		if apiErr.StatusCode == http.StatusNotFound {
			return true
		}
		if strings.EqualFold(apiErr.Code, "NOT_FOUND") {
			return true
		}
	}
	return false
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

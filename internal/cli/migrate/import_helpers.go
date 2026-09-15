package migrate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// migrateRequestContext bounds a single outbound request. It derives from the
// command context rather than the caller's context so a multi-locale import is
// not capped by one shared request deadline.
func migrateRequestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return shared.ContextWithTimeout(shared.ContextWithoutTimeout(ctx))
}

// migrateUploadContext gives one asset upload the asset upload budget, which a
// request deadline in the parent chain would otherwise truncate.
func migrateUploadContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return assets.ContextWithAssetUploadTimeout(shared.ContextWithoutTimeout(ctx))
}

func resolveAppID(ctx context.Context, client *asc.Client, appFlag string, config DeliverfileConfig) (string, error) {
	if strings.TrimSpace(appFlag) != "" {
		return strings.TrimSpace(appFlag), nil
	}
	if strings.TrimSpace(config.AppIdentifier) != "" {
		if shared.IsNumericAppID(config.AppIdentifier) {
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
			return "", shared.AmbiguousError("app", "--app", config.AppIdentifier, shared.AppCandidates(resp.Data))
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

// verifyExplicitVersionOwnership proves that an operator-supplied --version-id
// belongs to the selected app before any version is mutated. Deliverfile
// resolution already scopes its lookup to the app, so only the explicit flag
// needs the extra round trip.
func verifyExplicitVersionOwnership(ctx context.Context, client *asc.Client, versionFlag, appID, versionID string) error {
	if strings.TrimSpace(versionFlag) == "" {
		return nil
	}
	if client == nil {
		return fmt.Errorf("--version-id requires API access to verify that it belongs to app %s", appID)
	}
	if _, err := shared.ResolveOwnedAppStoreVersionByID(ctx, client, appID, versionID, ""); err != nil {
		return err
	}
	return nil
}

// normalizeDeliverfilePlatform maps a Deliverfile platform value onto the App
// Store Connect platform enum. fastlane deliver's own option values (ios, osx,
// appletvos, xros) are accepted alongside the App Store Connect spellings, so a
// Deliverfile copied verbatim from a fastlane project resolves without edits.
func normalizeDeliverfilePlatform(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ios":
		return "IOS", nil
	case "osx", "macos", "mac", "mac_os":
		return "MAC_OS", nil
	case "appletvos", "tvos", "tv_os":
		return "TV_OS", nil
	case "xros", "visionos", "vision_os":
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

type preparedVersionLocalization struct {
	localization FastlaneLocalization
	attributes   asc.AppStoreVersionLocalizationAttributes
}

type preparedAppInfoLocalization struct {
	localization   AppInfoFastlaneLocalization
	attributes     asc.AppInfoLocalizationAttributes
	localizationID string
}

type appInfoLocalizationPlan struct {
	appInfoID     string
	localizations []preparedAppInfoLocalization
}

func prepareVersionLocalizations(localizations []FastlaneLocalization) ([]preparedVersionLocalization, error) {
	prepared := make([]preparedVersionLocalization, 0, len(localizations))
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
		if err := shared.ValidateVersionLocalizationAttributes(attrs); err != nil {
			return nil, fmt.Errorf("migrate import: locale %q: %w", loc.Locale, err)
		}
		prepared = append(prepared, preparedVersionLocalization{
			localization: loc,
			attributes:   attrs,
		})
	}
	return prepared, nil
}

func validateVersionLocalizationCreateLocales(localizations []preparedVersionLocalization, localeToID map[string]string) error {
	for _, prepared := range localizations {
		locale := prepared.localization.Locale
		if err := validateLocalizationCreateTarget(locale, localeToID[locale]); err != nil {
			return err
		}
	}
	return nil
}

func validateScreenshotLocalizationCreateLocales(screenshots []ScreenshotPlan, localeToID map[string]string) error {
	for _, screenshot := range screenshots {
		if err := validateLocalizationCreateTarget(screenshot.Locale, localeToID[screenshot.Locale]); err != nil {
			return err
		}
	}
	return nil
}

func validateLocalizationCreateTarget(locale, localizationID string) error {
	if localizationID != "" {
		return nil
	}
	if _, err := shared.CanonicalizeAppStoreLocalizationLocale(locale); err != nil {
		return fmt.Errorf("migrate import: locale %q: %w", locale, err)
	}
	return nil
}

// validateCreateTargetLocales applies the locale half of the apply-time create
// preflight to every discovered locale. It is what a dry run can check without
// the remote localization list, and it reports the same error --confirm would.
func validateCreateTargetLocales(localizations []preparedVersionLocalization, appInfos []preparedAppInfoLocalization, screenshots []ScreenshotPlan) error {
	locales := make([]string, 0, len(localizations)+len(appInfos)+len(screenshots))
	for _, prepared := range localizations {
		locales = append(locales, prepared.localization.Locale)
	}
	for _, prepared := range appInfos {
		locales = append(locales, prepared.localization.Locale)
	}
	for _, screenshot := range screenshots {
		locales = append(locales, screenshot.Locale)
	}
	seen := make(map[string]struct{}, len(locales))
	for _, locale := range locales {
		if _, ok := seen[locale]; ok {
			continue
		}
		seen[locale] = struct{}{}
		if err := validateLocalizationCreateTarget(locale, ""); err != nil {
			return err
		}
	}
	return nil
}

// localeCreateIssue reports the locale rejection migrate import performs when a
// localization has to be created, so validate and import agree.
func localeCreateIssue(locale string) *ValidationIssue {
	if _, err := shared.CanonicalizeAppStoreLocalizationLocale(locale); err != nil {
		return &ValidationIssue{
			Locale:   locale,
			Field:    "locale",
			Severity: "error",
			Message:  err.Error(),
		}
	}
	return nil
}

func prepareAppInfoLocalizationAttributes(localizations []AppInfoFastlaneLocalization) ([]preparedAppInfoLocalization, error) {
	prepared := make([]preparedAppInfoLocalization, 0, len(localizations))
	for _, loc := range localizations {
		attrs := asc.AppInfoLocalizationAttributes{
			Locale:           loc.Locale,
			Name:             loc.Name,
			Subtitle:         loc.Subtitle,
			PrivacyPolicyURL: loc.PrivacyURL,
		}
		for _, issue := range validation.AppInfoLocalizationLengthIssues(validation.AppInfoLocalization{
			Name:     attrs.Name,
			Subtitle: attrs.Subtitle,
		}) {
			return nil, fmt.Errorf("migrate import: locale %q: %s exceeds %d %s", loc.Locale, issue.Field, issue.Limit, issue.Unit)
		}
		prepared = append(prepared, preparedAppInfoLocalization{
			localization: loc,
			attributes:   attrs,
		})
	}
	return prepared, nil
}

func uploadVersionLocalizations(ctx context.Context, client *asc.Client, versionID string, localizations []preparedVersionLocalization, localeToID map[string]string, submitOpts shared.SubmitReadinessOptions) ([]LocalizationUploadItem, []shared.SubmitReadinessCreateWarning, error) {
	results := make([]LocalizationUploadItem, 0, len(localizations))
	warnings := make([]shared.SubmitReadinessCreateWarning, 0, len(localizations))
	for _, prepared := range localizations {
		loc := prepared.localization
		attrs := prepared.attributes
		action := "create"
		localizationID := localeToID[loc.Locale]
		if localizationID != "" {
			action = "update"
			requestCtx, cancel := migrateRequestContext(ctx)
			_, err := client.UpdateAppStoreVersionLocalization(requestCtx, localizationID, attrs)
			cancel()
			if err != nil {
				// Report what already landed in App Store Connect; the caller
				// prints it before failing.
				return results, shared.NormalizeSubmitReadinessCreateWarnings(warnings), fmt.Errorf("migrate import: failed to update %s: %w", loc.Locale, err)
			}
		} else {
			requestCtx, cancel := migrateRequestContext(ctx)
			resp, err := client.CreateAppStoreVersionLocalization(requestCtx, versionID, attrs)
			cancel()
			if err != nil {
				return results, shared.NormalizeSubmitReadinessCreateWarnings(warnings), fmt.Errorf("migrate import: failed to create %s: %w", loc.Locale, err)
			}
			localizationID = resp.Data.ID
			localeToID[loc.Locale] = localizationID
			if warning, ok := shared.SubmitReadinessCreateWarningForLocaleWithOptions(loc.Locale, attrs, shared.SubmitReadinessCreateModeApplied, submitOpts); ok {
				warnings = append(warnings, warning)
			}
		}

		results = append(results, LocalizationUploadItem{
			Locale:         loc.Locale,
			Fields:         countNonEmptyFields(loc),
			Action:         action,
			LocalizationID: localizationID,
		})
	}
	return results, shared.NormalizeSubmitReadinessCreateWarnings(warnings), nil
}

func prepareAppInfoLocalizations(ctx context.Context, client *asc.Client, appID string, localizations []preparedAppInfoLocalization) (appInfoLocalizationPlan, error) {
	if len(localizations) == 0 {
		return appInfoLocalizationPlan{}, nil
	}
	appInfos, err := client.GetAppInfos(ctx, appID)
	if err != nil {
		return appInfoLocalizationPlan{}, fmt.Errorf("migrate import: failed to get app info: %w", err)
	}
	if len(appInfos.Data) == 0 {
		return appInfoLocalizationPlan{}, fmt.Errorf("migrate import: no app info found for app")
	}
	appInfoID := shared.SelectBestAppInfoID(appInfos)
	if strings.TrimSpace(appInfoID) == "" {
		return appInfoLocalizationPlan{}, fmt.Errorf("migrate import: failed to select app info for app")
	}

	existingAppInfoLocs, err := fetchAppInfoLocalizationsForPlan(ctx, client, appInfoID)
	if err != nil {
		return appInfoLocalizationPlan{}, fmt.Errorf("migrate import: failed to fetch app info localizations: %w", err)
	}
	appInfoLocaleToID := make(map[string]string)
	for _, loc := range existingAppInfoLocs {
		appInfoLocaleToID[loc.Attributes.Locale] = loc.ID
	}

	plan := appInfoLocalizationPlan{
		appInfoID:     appInfoID,
		localizations: make([]preparedAppInfoLocalization, 0, len(localizations)),
	}
	for _, prepared := range localizations {
		loc := prepared.localization
		attrs := prepared.attributes
		localizationID := appInfoLocaleToID[loc.Locale]
		if err := validateLocalizationCreateTarget(loc.Locale, localizationID); err != nil {
			return appInfoLocalizationPlan{}, err
		}
		if localizationID == "" {
			if strings.TrimSpace(attrs.Name) == "" {
				return appInfoLocalizationPlan{}, fmt.Errorf("migrate import: locale %q: name is required when creating app info localization", loc.Locale)
			}
		}
		prepared.localizationID = localizationID
		plan.localizations = append(plan.localizations, prepared)
	}
	return plan, nil
}

func fetchAppInfoLocalizationsForPlan(ctx context.Context, client *asc.Client, appInfoID string) ([]asc.Resource[asc.AppInfoLocalizationAttributes], error) {
	firstPage, err := client.GetAppInfoLocalizations(ctx, appInfoID, asc.WithAppInfoLocalizationsLimit(200))
	if err != nil {
		return nil, err
	}
	if firstPage == nil {
		return nil, fmt.Errorf("empty app info localizations response")
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		nextPage, err := client.GetAppInfoLocalizations(pageCtx, appInfoID, asc.WithAppInfoLocalizationsNextURL(nextURL))
		if err != nil {
			return nil, err
		}
		if nextPage == nil {
			return nil, fmt.Errorf("empty app info localizations response")
		}
		return nextPage, nil
	})
	if err != nil {
		return nil, err
	}
	allPages, ok := paginated.(*asc.AppInfoLocalizationsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected app info localization pagination response type")
	}
	return allPages.Data, nil
}

func fetchVersionLocalizationsForPlan(ctx context.Context, client *asc.Client, versionID string) ([]asc.Resource[asc.AppStoreVersionLocalizationAttributes], error) {
	firstPage, err := client.GetAppStoreVersionLocalizations(ctx, versionID, asc.WithAppStoreVersionLocalizationsLimit(200))
	if err != nil {
		return nil, err
	}
	if firstPage == nil {
		return nil, fmt.Errorf("empty version localizations response")
	}

	paginated, err := asc.PaginateAll(ctx, firstPage, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		nextPage, err := client.GetAppStoreVersionLocalizations(pageCtx, versionID, asc.WithAppStoreVersionLocalizationsNextURL(nextURL))
		if err != nil {
			return nil, err
		}
		if nextPage == nil {
			return nil, fmt.Errorf("empty version localizations response")
		}
		return nextPage, nil
	})
	if err != nil {
		return nil, err
	}
	allPages, ok := paginated.(*asc.AppStoreVersionLocalizationsResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected version localization pagination response type")
	}
	return allPages.Data, nil
}

func uploadAppInfoLocalizations(ctx context.Context, client *asc.Client, plan appInfoLocalizationPlan) ([]LocalizationUploadItem, error) {
	results := make([]LocalizationUploadItem, 0, len(plan.localizations))
	for _, prepared := range plan.localizations {
		loc := prepared.localization
		action := "create"
		localizationID := prepared.localizationID
		if localizationID != "" {
			action = "update"
			requestCtx, cancel := migrateRequestContext(ctx)
			_, err := client.UpdateAppInfoLocalization(requestCtx, localizationID, prepared.attributes)
			cancel()
			if err != nil {
				return results, fmt.Errorf("migrate import: failed to update app info %s: %w", loc.Locale, err)
			}
		} else {
			requestCtx, cancel := migrateRequestContext(ctx)
			resp, err := client.CreateAppInfoLocalization(requestCtx, plan.appInfoID, prepared.attributes)
			cancel()
			if err != nil {
				return results, fmt.Errorf("migrate import: failed to create app info %s: %w", loc.Locale, err)
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

	fetchCtx, fetchCancel := migrateRequestContext(ctx)
	existing, err := client.GetAppStoreReviewDetailForVersion(fetchCtx, versionID)
	fetchCancel()
	if err != nil {
		if !isNotFoundReviewDetail(err) {
			return nil, fmt.Errorf("migrate import: failed to fetch review information: %w", err)
		}
		createCtx, createCancel := migrateRequestContext(ctx)
		created, err := client.CreateAppStoreReviewDetail(createCtx, versionID, buildReviewDetailCreateAttributes(info))
		createCancel()
		if err != nil {
			return nil, fmt.Errorf("migrate import: failed to create review information: %w", err)
		}
		return &ReviewInfoResult{Action: migrateReviewInfoActionCreate, DetailID: created.Data.ID}, nil
	}

	if existing == nil || existing.Data.ID == "" {
		return nil, fmt.Errorf("migrate import: review information response missing ID")
	}
	if reviewInformationMatches(existing.Data.Attributes, info) {
		return &ReviewInfoResult{Action: migrateReviewInfoActionSkip, DetailID: existing.Data.ID}, nil
	}
	updateCtx, updateCancel := migrateRequestContext(ctx)
	_, err = client.UpdateAppStoreReviewDetail(updateCtx, existing.Data.ID, buildReviewDetailUpdateAttributes(info))
	updateCancel()
	if err != nil {
		return nil, fmt.Errorf("migrate import: failed to update review information: %w", err)
	}
	return &ReviewInfoResult{Action: migrateReviewInfoActionUpdate, DetailID: existing.Data.ID}, nil
}

func uploadScreenshots(ctx context.Context, client *asc.Client, versionID string, localeToID map[string]string, plans []ScreenshotPlan) (results []ScreenshotUploadResult, err error) {
	if len(plans) == 0 {
		return nil, nil
	}

	plansByLocale := make(map[string][]ScreenshotPlan)
	for _, plan := range plans {
		plansByLocale[plan.Locale] = append(plansByLocale[plan.Locale], plan)
	}

	// A failure can leave a localization this stage created out of every result,
	// so name each unreported localization on stderr instead of leaving it
	// behind silently.
	var createdLocales []string
	defer func() {
		if err == nil {
			return
		}
		reportedLocales := make(map[string]struct{}, len(results))
		for _, result := range results {
			reportedLocales[result.Locale] = struct{}{}
		}
		unreportedLocales := make([]string, 0, len(createdLocales))
		for _, locale := range createdLocales {
			if _, reported := reportedLocales[locale]; !reported {
				unreportedLocales = append(unreportedLocales, locale)
			}
		}
		warnCreatedScreenshotLocalizations(os.Stderr, unreportedLocales)
	}()

	results = make([]ScreenshotUploadResult, 0, len(plans))
	for locale, localePlans := range plansByLocale {
		localizationID := localeToID[locale]
		if localizationID == "" {
			createCtx, createCancel := migrateRequestContext(ctx)
			resp, err := client.CreateAppStoreVersionLocalization(createCtx, versionID, asc.AppStoreVersionLocalizationAttributes{Locale: locale})
			createCancel()
			if err != nil {
				return sortedScreenshotResults(results), fmt.Errorf("migrate import: failed to create localization for screenshots %s: %w", locale, err)
			}
			localizationID = resp.Data.ID
			localeToID[locale] = localizationID
			createdLocales = append(createdLocales, locale)
		}

		existingSets, err := client.GetAllAppScreenshotSets(ctx, localizationID, asc.WithAppScreenshotSetsRequestContext(migrateRequestContext))
		if err != nil {
			return sortedScreenshotResults(results), fmt.Errorf("migrate import: failed to fetch screenshot sets for %s: %w", locale, err)
		}
		setByType := make(map[string]string)
		existingFiles := make(map[string]map[string]bool)
		existingIDsByName := make(map[string]map[string]string)
		existingOrderByType := make(map[string][]string)
		for _, set := range existingSets.Data {
			setByType[set.Attributes.ScreenshotDisplayType] = set.ID
			orderCtx, orderCancel := migrateRequestContext(ctx)
			orderedIDs, err := assets.GetOrderedAppScreenshotIDs(orderCtx, client, set.ID)
			orderCancel()
			if err != nil {
				return sortedScreenshotResults(results), fmt.Errorf("migrate import: failed to fetch screenshot relationship order for %s: %w", set.ID, err)
			}
			existingOrderByType[set.Attributes.ScreenshotDisplayType] = orderedIDs
			screenshots, err := client.GetAllAppScreenshots(ctx, set.ID, asc.WithAppScreenshotsRequestContext(migrateRequestContext))
			if err != nil {
				return sortedScreenshotResults(results), fmt.Errorf("migrate import: failed to fetch screenshots for %s: %w", set.ID, err)
			}
			fileNames := make(map[string]bool)
			fileIDs := make(map[string]string)
			for _, shot := range screenshots.Data {
				name := strings.TrimSpace(shot.Attributes.FileName)
				if name != "" {
					fileNames[name] = true
					if fileIDs[name] == "" {
						fileIDs[name] = shot.ID
					}
				}
			}
			existingFiles[set.Attributes.ScreenshotDisplayType] = fileNames
			existingIDsByName[set.Attributes.ScreenshotDisplayType] = fileIDs
		}

		for _, plan := range localePlans {
			canonicalDisplayType := asc.CanonicalScreenshotDisplayTypeForAPI(plan.DisplayType)
			setID := setByType[canonicalDisplayType]
			createdSet := false
			if setID == "" {
				setCtx, setCancel := migrateRequestContext(ctx)
				set, err := client.CreateAppScreenshotSet(setCtx, localizationID, canonicalDisplayType)
				setCancel()
				if err != nil {
					return sortedScreenshotResults(results), fmt.Errorf("migrate import: failed to create screenshot set %s: %w", canonicalDisplayType, err)
				}
				setID = set.Data.ID
				createdSet = true
				setByType[canonicalDisplayType] = setID
				existingFiles[canonicalDisplayType] = make(map[string]bool)
				existingIDsByName[canonicalDisplayType] = make(map[string]string)
				existingOrderByType[canonicalDisplayType] = nil
			}

			fileNames := existingFiles[canonicalDisplayType]
			if fileNames == nil {
				fileNames = make(map[string]bool)
				existingFiles[canonicalDisplayType] = fileNames
			}
			fileIDs := existingIDsByName[canonicalDisplayType]
			if fileIDs == nil {
				fileIDs = make(map[string]string)
				existingIDsByName[canonicalDisplayType] = fileIDs
			}

			result := ScreenshotUploadResult{
				Locale:      plan.Locale,
				DisplayType: canonicalDisplayType,
				createdSet:  createdSet,
			}
			uploadedIDsByName := make(map[string]string)

			for _, filePath := range plan.Files {
				name := filepath.Base(filePath)
				if fileNames[name] {
					result.Skipped = append(result.Skipped, SkippedItem{
						Path:   filePath,
						Reason: "already exists",
					})
					continue
				}
				var item asc.AssetUploadResultItem
				var err error
				// Each asset reserves, transfers, and commits under its own
				// upload budget; a shared request deadline would truncate the
				// transfer of the first large screenshot.
				uploadCtx, uploadCancel := migrateUploadContext(ctx)
				if opened, ok, openErr := plan.openedFile(filePath); openErr != nil {
					err = openErr
				} else if ok {
					item, err = assets.UploadScreenshotAssetFromFile(uploadCtx, client, setID, filePath, opened)
					if closeErr := opened.Close(); err == nil {
						err = closeErr
					}
				} else {
					// Keep compatibility for callers that construct plans
					// directly; migrate import discovery always supplies a
					// pinned rooted handle.
					item, err = assets.UploadScreenshotAsset(uploadCtx, client, setID, filePath)
				}
				uploadCancel()
				if err != nil {
					// Keep the assets that already uploaded for this set so the
					// caller can report them.
					results = append(results, result)
					return sortedScreenshotResults(results), fmt.Errorf("migrate import: failed to upload screenshot %s: %w", filePath, err)
				}
				fileNames[name] = true
				uploadedIDsByName[name] = item.AssetID
				result.Uploaded = append(result.Uploaded, item)
			}
			orderedIDs := buildPlannedScreenshotOrder(plan.Files, existingOrderByType[canonicalDisplayType], fileIDs, uploadedIDsByName)
			reorderCtx, reorderCancel := migrateRequestContext(ctx)
			err := assets.SetOrderedAppScreenshots(reorderCtx, client, setID, orderedIDs)
			reorderCancel()
			if err != nil {
				results = append(results, result)
				return sortedScreenshotResults(results), fmt.Errorf("migrate import: failed to reorder screenshots for %s: %w", setID, err)
			}
			existingOrderByType[canonicalDisplayType] = orderedIDs
			for name, id := range uploadedIDsByName {
				if strings.TrimSpace(id) != "" {
					fileIDs[name] = id
				}
			}

			results = append(results, result)
		}
	}

	return sortedScreenshotResults(results), nil
}

// warnCreatedScreenshotLocalizations names the localizations the screenshot
// stage created before a failure that left them without a result, so the
// operator can re-run the import or remove the empty localizations by hand.
func warnCreatedScreenshotLocalizations(w io.Writer, locales []string) {
	// Locales are collected in map order, so sort them for a stable report.
	ordered := append([]string(nil), locales...)
	sort.Strings(ordered)
	for _, locale := range ordered {
		fmt.Fprintf(w, "Warning: created localization %q before the failure; re-run import or remove it manually\n", locale)
	}
}

func sortedScreenshotResults(results []ScreenshotUploadResult) []ScreenshotUploadResult {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Locale == results[j].Locale {
			return results[i].DisplayType < results[j].DisplayType
		}
		return results[i].Locale < results[j].Locale
	})
	return results
}

func buildPlannedScreenshotOrder(planFiles []string, existingOrder []string, existingIDsByName map[string]string, uploadedIDsByName map[string]string) []string {
	orderedIDs := make([]string, 0, len(planFiles)+len(existingOrder))
	seen := make(map[string]struct{}, len(planFiles)+len(existingOrder))
	appendUnique := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, exists := seen[id]; exists {
			return
		}
		seen[id] = struct{}{}
		orderedIDs = append(orderedIDs, id)
	}

	for _, filePath := range planFiles {
		name := filepath.Base(filePath)
		if id := uploadedIDsByName[name]; strings.TrimSpace(id) != "" {
			appendUnique(id)
			continue
		}
		appendUnique(existingIDsByName[name])
	}

	for _, id := range existingOrder {
		appendUnique(id)
	}

	return orderedIDs
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

package apps

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppsInfoCommand returns the apps info command with subcommands.
func AppsInfoCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps info", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "info",
		ShortUsage: "asc apps info <subcommand> [flags]",
		ShortHelp:  "Manage App Store version metadata.",
		LongHelp: `Manage App Store version metadata like description, keywords, and what's new.

Examples:
  asc apps info list --app "APP_ID"
  asc apps info view --app "APP_ID"
  asc apps info view --app "APP_ID" --version "1.2.3" --platform IOS
  asc apps info edit --app "APP_ID" --locale "en-US" --whats-new "Bug fixes"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppsInfoListCommand(),
			AppsInfoViewCommand(),
			AppsInfoEditCommand(),
			AppsInfoRelationshipsCommand(),
			AppsInfoTerritoryAgeRatingsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppsInfoViewCommand returns the view subcommand.
func AppsInfoViewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps info view", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	infoID := fs.String("info-id", "", "App Info ID (optional override)")
	legacyAppInfoID := fs.String("app-info", "", "Deprecated alias for --info-id")
	versionID := fs.String("version-id", "", "App Store version ID (optional override)")
	version := fs.String("version", "", "App Store version string (optional)")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS (required with --version)")
	state := fs.String("state", "", "Filter by app store state(s), comma-separated")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	include := fs.String("include", "", "Include related resources: "+strings.Join(appInfoIncludeList(), ", "))
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc apps info view [flags]",
		ShortHelp:  "View app store version localization metadata.",
		LongHelp: `Get App Store version localization metadata.

If multiple versions exist and no --version-id/--version is provided, the most
recently created version is used.

Examples:
  asc apps info view --app "APP_ID"
  asc apps info view --app "APP_ID" --version "1.2.3" --platform IOS
  asc apps info view --version-id "VERSION_ID"
  asc apps info view --info-id "APP_INFO_ID" --include "ageRatingDeclaration"
  asc apps info view --app "APP_ID" --include "ageRatingDeclaration,territoryAgeRatings"
  asc apps info view --app "APP_ID" --locale "en-US" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			infoIDValue, err := resolveInfoIDFlags(*infoID, *legacyAppInfoID, "--app-info")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("--limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageError(err.Error())
			}
			if strings.TrimSpace(*version) != "" && strings.TrimSpace(*versionID) != "" {
				return shared.UsageError("--version and --version-id are mutually exclusive")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if strings.TrimSpace(*versionID) == "" && resolvedAppID == "" && infoIDValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app or --info-id is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			includeValues, err := normalizeAppInfoInclude(*include)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if infoIDValue != "" && len(includeValues) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --info-id requires --include")
				return flag.ErrHelp
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platform))
			if err != nil {
				return shared.UsageError(err.Error())
			}
			states, err := shared.NormalizeAppStoreVersionStates(shared.SplitCSVUpper(*state))
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if strings.TrimSpace(*version) != "" && len(platforms) != 1 {
				fmt.Fprintln(os.Stderr, "Error: --platform is required with --version")
				return flag.ErrHelp
			}

			if len(includeValues) > 0 {
				if strings.TrimSpace(*versionID) != "" ||
					strings.TrimSpace(*version) != "" ||
					strings.TrimSpace(*platform) != "" ||
					strings.TrimSpace(*state) != "" ||
					strings.TrimSpace(*locale) != "" ||
					*limit != 0 ||
					strings.TrimSpace(*next) != "" ||
					*paginate {
					fmt.Fprintln(os.Stderr, "Error: --include cannot be used with version localization flags")
					return flag.ErrHelp
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps info view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if len(includeValues) > 0 {
				appInfoIDValue, err := shared.ResolveAppInfoID(requestCtx, client, resolvedAppID, infoIDValue)
				if err != nil {
					return fmt.Errorf("apps info view: %w", err)
				}

				resp, err := client.GetAppInfo(requestCtx, appInfoIDValue, asc.WithAppInfoInclude(includeValues))
				if err != nil {
					return fmt.Errorf("apps info view: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			versionResource, err := resolveAppStoreVersionForAppInfo(
				requestCtx,
				client,
				resolvedAppID,
				strings.TrimSpace(*versionID),
				strings.TrimSpace(*version),
				platforms,
				states,
			)
			if err != nil {
				return fmt.Errorf("apps info view: %w", err)
			}

			opts := []asc.AppStoreVersionLocalizationsOption{
				asc.WithAppStoreVersionLocalizationsLimit(*limit),
				asc.WithAppStoreVersionLocalizationsNextURL(*next),
			}
			locales := shared.SplitCSV(*locale)
			if len(locales) > 0 {
				opts = append(opts, asc.WithAppStoreVersionLocalizationLocales(locales))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppStoreVersionLocalizationsLimit(200))
				firstPage, err := client.GetAppStoreVersionLocalizations(requestCtx, versionResource.ID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("apps info view: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionLocalizations(ctx, versionResource.ID, asc.WithAppStoreVersionLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("apps info view: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppStoreVersionLocalizations(requestCtx, versionResource.ID, opts...)
			if err != nil {
				return fmt.Errorf("apps info view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppsInfoEditCommand returns the edit subcommand.
func AppsInfoEditCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps info edit", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	versionID := fs.String("version-id", "", "App Store version ID (optional override)")
	version := fs.String("version", "", "App Store version string (optional)")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS (required with --version)")
	state := fs.String("state", "", "Filter by app store state(s), comma-separated")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	copyFromLocale := fs.String("copy-from-locale", "", "Copy submit-required fields (description, keywords, support-url) from this locale when missing")
	locales := fs.String("locales", "", "Locales (comma-separated, e.g., en-US,de-DE)")
	fromDir := fs.String("from-dir", "", "Directory with per-locale JSON files (<locale>.json)")
	dryRun := fs.Bool("dry-run", false, "Show planned per-locale actions without writing changes")
	description := fs.String("description", "", "App description")
	keywords := fs.String("keywords", "", "Keywords (comma-separated)")
	supportURL := fs.String("support-url", "", "Support URL")
	marketingURL := fs.String("marketing-url", "", "Marketing URL")
	promotionalText := fs.String("promotional-text", "", "Promotional text")
	whatsNew := fs.String("whats-new", "", "What's New text")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "edit",
		ShortUsage: "asc apps info edit [flags]",
		ShortHelp:  "Create or update app store version metadata.",
		LongHelp: `Create or update App Store version metadata.

Examples:
  asc apps info edit --app "APP_ID" --locale "en-US" --whats-new "Bug fixes"
  asc apps info edit --app "APP_ID" --locale "fr-FR" --copy-from-locale "en-US" --whats-new "Corrections"
  asc apps info edit --app "APP_ID" --version "1.2.3" --platform IOS --locale "en-US" --description "New release"
  asc apps info edit --app "APP_ID" --version "1.2.3" --platform IOS --locales "en-US,de-DE" --whats-new "Bug fixes"
  asc apps info edit --app "APP_ID" --version "1.2.3" --platform IOS --from-dir "./metadata/version/1.2.3"
  asc apps info edit --app "APP_ID" --locales "en-US,de-DE" --whats-new "Bug fixes" --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*version) != "" && strings.TrimSpace(*versionID) != "" {
				return shared.UsageError("--version and --version-id are mutually exclusive")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if strings.TrimSpace(*versionID) == "" && resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platform))
			if err != nil {
				return shared.UsageError(err.Error())
			}
			states, err := shared.NormalizeAppStoreVersionStates(shared.SplitCSVUpper(*state))
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if strings.TrimSpace(*version) != "" && len(platforms) != 1 {
				fmt.Fprintln(os.Stderr, "Error: --platform is required with --version")
				return flag.ErrHelp
			}

			localeValue := strings.TrimSpace(*locale)
			localesValue := shared.SplitUniqueCSV(*locales)
			fromDirValue := strings.TrimSpace(*fromDir)
			if localeValue != "" && len(localesValue) > 0 {
				return shared.UsageError("--locale and --locales are mutually exclusive")
			}
			if fromDirValue != "" {
				if localeValue != "" || len(localesValue) > 0 {
					return shared.UsageError("--from-dir cannot be used with --locale or --locales")
				}
			}
			if fromDirValue == "" && localeValue == "" && len(localesValue) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}
			if localeValue != "" {
				if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
					return shared.UsageError(err.Error())
				}
			}
			for _, localeItem := range localesValue {
				if err := shared.ValidateBuildLocalizationLocale(localeItem); err != nil {
					return shared.UsageError(err.Error())
				}
			}
			copyFromLocaleValue := strings.TrimSpace(*copyFromLocale)
			if copyFromLocaleValue != "" {
				if err := shared.ValidateBuildLocalizationLocale(copyFromLocaleValue); err != nil {
					return shared.UsageError(err.Error())
				}
				if localeValue == "" || len(localesValue) > 0 || fromDirValue != "" || *dryRun {
					return shared.UsageError("--copy-from-locale can only be used with --locale in single-locale mode")
				}
				if strings.EqualFold(copyFromLocaleValue, localeValue) {
					return shared.UsageError("--copy-from-locale must be different from --locale")
				}
			}

			inlineAttrs := asc.AppStoreVersionLocalizationAttributes{
				Description:     strings.TrimSpace(*description),
				Keywords:        strings.TrimSpace(*keywords),
				SupportURL:      strings.TrimSpace(*supportURL),
				MarketingURL:    strings.TrimSpace(*marketingURL),
				PromotionalText: strings.TrimSpace(*promotionalText),
				WhatsNew:        strings.TrimSpace(*whatsNew),
			}
			hasInlineUpdates := appInfoSetHasAnyUpdates(inlineAttrs)
			if fromDirValue != "" && hasInlineUpdates {
				return shared.UsageError("--from-dir cannot be used with inline update flags")
			}
			if fromDirValue == "" && !hasInlineUpdates && copyFromLocaleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps info edit: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			versionResource, err := resolveAppStoreVersionForAppInfo(
				requestCtx,
				client,
				resolvedAppID,
				strings.TrimSpace(*versionID),
				strings.TrimSpace(*version),
				platforms,
				states,
			)
			if err != nil {
				return fmt.Errorf("apps info edit: %w", err)
			}

			// Keep existing single-locale output compatibility unless batch planning is explicitly requested.
			if fromDirValue == "" && len(localesValue) == 0 && localeValue != "" && !*dryRun {
				return runAppInfoSetSingleLocale(
					requestCtx,
					client,
					versionResource.ID,
					localeValue,
					copyFromLocaleValue,
					inlineAttrs,
					output,
				)
			}

			var valuesByLocale map[string]asc.AppStoreVersionLocalizationAttributes
			if fromDirValue != "" {
				valuesByLocale, err = readAppInfoSetLocalesFromDir(fromDirValue)
				if err != nil {
					return shared.UsageError(err.Error())
				}
			} else {
				targetLocales := make([]string, 0, len(localesValue)+1)
				if localeValue != "" {
					targetLocales = append(targetLocales, localeValue)
				}
				targetLocales = append(targetLocales, localesValue...)
				valuesByLocale = make(map[string]asc.AppStoreVersionLocalizationAttributes, len(targetLocales))
				for _, targetLocale := range targetLocales {
					valuesByLocale[targetLocale] = inlineAttrs
				}
			}

			batchResult, err := runAppInfoSetBatch(
				requestCtx,
				client,
				resolvedAppID,
				versionResource.ID,
				valuesByLocale,
				*dryRun,
			)
			if err != nil {
				return fmt.Errorf("apps info edit: %w", err)
			}
			if err := shared.PrintOutput(batchResult, *output.Output, *output.Pretty); err != nil {
				return err
			}
			if batchResult.Failed > 0 {
				summaryErr := fmt.Errorf("apps info edit: %d locale(s) failed", batchResult.Failed)
				fmt.Fprintf(os.Stderr, "Error: %s\n", summaryErr.Error())
				return shared.NewReportedError(summaryErr)
			}
			return nil
		},
	}
}

func resolveInfoIDFlags(infoID, legacyValue, legacyFlagName string) (string, error) {
	infoIDValue := strings.TrimSpace(infoID)
	legacyValue = strings.TrimSpace(legacyValue)
	if infoIDValue != "" && legacyValue != "" && infoIDValue != legacyValue {
		return "", fmt.Errorf("--info-id and %s are mutually exclusive", legacyFlagName)
	}
	if infoIDValue != "" {
		return infoIDValue, nil
	}
	return legacyValue, nil
}

func runAppInfoSetSingleLocale(
	ctx context.Context,
	client *asc.Client,
	versionID string,
	locale string,
	copyFromLocale string,
	attrs asc.AppStoreVersionLocalizationAttributes,
	output shared.OutputFlags,
) error {
	localizationOpts := []asc.AppStoreVersionLocalizationsOption{
		asc.WithAppStoreVersionLocalizationsLimit(200),
		asc.WithAppStoreVersionLocalizationLocales(appInfoSetRequestedLocales(locale, copyFromLocale)),
	}
	localizations, err := client.GetAppStoreVersionLocalizations(ctx, versionID, localizationOpts...)
	if err != nil {
		return fmt.Errorf("apps info edit: failed to fetch localizations: %w", err)
	}

	targetLocalization, targetExists := findAppInfoSetLocalizationByLocale(localizations.Data, locale)

	descriptionValue := strings.TrimSpace(attrs.Description)
	keywordsValue := strings.TrimSpace(attrs.Keywords)
	supportURLValue := strings.TrimSpace(attrs.SupportURL)
	marketingURLValue := strings.TrimSpace(attrs.MarketingURL)
	promotionalTextValue := strings.TrimSpace(attrs.PromotionalText)
	whatsNewValue := strings.TrimSpace(attrs.WhatsNew)

	if copyFromLocale != "" {
		sourceLocalization, found := findAppInfoSetLocalizationByLocale(localizations.Data, copyFromLocale)
		if !found {
			return fmt.Errorf("apps info edit: --copy-from-locale %q was not found for this app version", copyFromLocale)
		}
		if shouldBackfillAppInfoSetField(descriptionValue, targetExists, targetLocalization.Attributes.Description) {
			descriptionValue = strings.TrimSpace(sourceLocalization.Attributes.Description)
		}
		if shouldBackfillAppInfoSetField(keywordsValue, targetExists, targetLocalization.Attributes.Keywords) {
			keywordsValue = strings.TrimSpace(sourceLocalization.Attributes.Keywords)
		}
		if shouldBackfillAppInfoSetField(supportURLValue, targetExists, targetLocalization.Attributes.SupportURL) {
			supportURLValue = strings.TrimSpace(sourceLocalization.Attributes.SupportURL)
		}
	}

	updateAttrs := applyAppInfoSetValues(
		asc.AppStoreVersionLocalizationAttributes{},
		descriptionValue,
		keywordsValue,
		supportURLValue,
		marketingURLValue,
		promotionalTextValue,
		whatsNewValue,
	)

	effectiveAttrs := asc.AppStoreVersionLocalizationAttributes{Locale: locale}
	if targetExists {
		effectiveAttrs = targetLocalization.Attributes
		effectiveAttrs.Locale = locale
	}
	effectiveAttrs = applyAppInfoSetValues(
		effectiveAttrs,
		descriptionValue,
		keywordsValue,
		supportURLValue,
		marketingURLValue,
		promotionalTextValue,
		whatsNewValue,
	)

	if !targetExists {
		updateAttrs.Locale = locale
		resp, createErr := client.CreateAppStoreVersionLocalization(ctx, versionID, updateAttrs)
		if createErr != nil {
			return fmt.Errorf("apps info edit: %w", createErr)
		}
		warnAppInfoSetSubmitIncompleteLocale(locale, effectiveAttrs)
		return shared.PrintOutput(resp, *output.Output, *output.Pretty)
	}

	localizationID := strings.TrimSpace(targetLocalization.ID)
	if localizationID == "" {
		return fmt.Errorf("apps info edit: localization id is empty")
	}
	resp, updateErr := client.UpdateAppStoreVersionLocalization(ctx, localizationID, updateAttrs)
	if updateErr != nil {
		return fmt.Errorf("apps info edit: %w", updateErr)
	}
	warnAppInfoSetSubmitIncompleteLocale(locale, effectiveAttrs)
	return shared.PrintOutput(resp, *output.Output, *output.Pretty)
}

func runAppInfoSetBatch(
	ctx context.Context,
	client *asc.Client,
	appID string,
	versionID string,
	valuesByLocale map[string]asc.AppStoreVersionLocalizationAttributes,
	dryRun bool,
) (*asc.AppInfoSetBatchResult, error) {
	if len(valuesByLocale) == 0 {
		return nil, fmt.Errorf("no locales to update")
	}

	localizations, err := client.GetAppStoreVersionLocalizations(
		ctx,
		versionID,
		asc.WithAppStoreVersionLocalizationsLimit(200),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch localizations: %w", err)
	}

	existingByLocale := make(map[string]string, len(localizations.Data))
	for _, localization := range localizations.Data {
		locale := strings.ToLower(strings.TrimSpace(localization.Attributes.Locale))
		if locale == "" {
			continue
		}
		existingByLocale[locale] = strings.TrimSpace(localization.ID)
	}

	locales := make([]string, 0, len(valuesByLocale))
	for locale := range valuesByLocale {
		locales = append(locales, locale)
	}
	sort.Strings(locales)

	results := make([]asc.AppInfoSetLocaleResult, len(locales))
	if dryRun {
		for idx, locale := range locales {
			existingID := existingByLocale[strings.ToLower(locale)]
			action := "create"
			if existingID != "" {
				action = "update"
			}
			results[idx] = asc.AppInfoSetLocaleResult{
				Locale:         locale,
				Action:         action,
				Status:         "planned",
				LocalizationID: existingID,
			}
		}
		return buildAppInfoSetBatchResult(appID, versionID, true, results), nil
	}

	workers := len(locales)
	if workers > 4 {
		workers = 4
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for idx, locale := range locales {
		idx := idx
		locale := locale
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			attrs := valuesByLocale[locale]
			existingID := existingByLocale[strings.ToLower(locale)]
			action := "create"
			if existingID != "" {
				action = "update"
			}

			localeResult := asc.AppInfoSetLocaleResult{
				Locale: locale,
				Action: action,
				Status: "success",
			}

			if existingID == "" {
				attrs.Locale = locale
				resp, createErr := client.CreateAppStoreVersionLocalization(ctx, versionID, attrs)
				if createErr != nil {
					localeResult.Status = "failed"
					localeResult.Error = createErr.Error()
					results[idx] = localeResult
					return
				}
				localeResult.LocalizationID = strings.TrimSpace(resp.Data.ID)
				results[idx] = localeResult
				return
			}

			resp, updateErr := client.UpdateAppStoreVersionLocalization(ctx, existingID, attrs)
			if updateErr != nil {
				localeResult.Status = "failed"
				localeResult.Error = updateErr.Error()
				localeResult.LocalizationID = existingID
				results[idx] = localeResult
				return
			}
			localizationID := strings.TrimSpace(resp.Data.ID)
			if localizationID == "" {
				localizationID = existingID
			}
			localeResult.LocalizationID = localizationID
			results[idx] = localeResult
		}()
	}
	wg.Wait()

	return buildAppInfoSetBatchResult(appID, versionID, false, results), nil
}

func buildAppInfoSetBatchResult(appID string, versionID string, dryRun bool, results []asc.AppInfoSetLocaleResult) *asc.AppInfoSetBatchResult {
	result := &asc.AppInfoSetBatchResult{
		AppID:     strings.TrimSpace(appID),
		VersionID: strings.TrimSpace(versionID),
		DryRun:    dryRun,
		Total:     len(results),
		Results:   results,
	}
	for _, item := range results {
		switch item.Status {
		case "failed":
			result.Failed++
		case "planned":
			result.Planned++
		default:
			result.Succeeded++
		}
	}
	return result
}

func appInfoSetHasAnyUpdates(attrs asc.AppStoreVersionLocalizationAttributes) bool {
	return strings.TrimSpace(attrs.Description) != "" ||
		strings.TrimSpace(attrs.Keywords) != "" ||
		strings.TrimSpace(attrs.SupportURL) != "" ||
		strings.TrimSpace(attrs.MarketingURL) != "" ||
		strings.TrimSpace(attrs.PromotionalText) != "" ||
		strings.TrimSpace(attrs.WhatsNew) != ""
}

func readAppInfoSetLocalesFromDir(dirPath string) (map[string]asc.AppStoreVersionLocalizationAttributes, error) {
	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("--from-dir path error: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("--from-dir must be a directory")
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	valuesByLocale := make(map[string]asc.AppStoreVersionLocalizationAttributes)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		locale := strings.TrimSpace(strings.TrimSuffix(entry.Name(), ".json"))
		if locale == "" {
			continue
		}
		if err := shared.ValidateBuildLocalizationLocale(locale); err != nil {
			return nil, fmt.Errorf("invalid locale %q from --from-dir file %q: %w", locale, entry.Name(), err)
		}
		if _, exists := valuesByLocale[locale]; exists {
			return nil, fmt.Errorf("duplicate locale %q in --from-dir", locale)
		}

		path := filepath.Join(dirPath, entry.Name())
		file, err := shared.OpenExistingNoFollow(path)
		if err != nil {
			return nil, fmt.Errorf("open %q: %w", entry.Name(), err)
		}
		data, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", entry.Name(), err)
		}

		attrs, err := parseAppInfoSetLocaleJSON(data, entry.Name())
		if err != nil {
			return nil, err
		}
		valuesByLocale[locale] = attrs
	}

	if len(valuesByLocale) == 0 {
		return nil, fmt.Errorf("no locale JSON files found in %q", dirPath)
	}
	return valuesByLocale, nil
}

func parseAppInfoSetLocaleJSON(data []byte, fileName string) (asc.AppStoreVersionLocalizationAttributes, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return asc.AppStoreVersionLocalizationAttributes{}, fmt.Errorf("locale file %q is empty", fileName)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return asc.AppStoreVersionLocalizationAttributes{}, fmt.Errorf("invalid JSON in %q: %w", fileName, err)
	}
	if len(payload) == 0 {
		return asc.AppStoreVersionLocalizationAttributes{}, fmt.Errorf("locale file %q has no fields", fileName)
	}

	attrs := asc.AppStoreVersionLocalizationAttributes{}
	unknownKeys := make([]string, 0)
	for key, raw := range payload {
		value, ok := raw.(string)
		if !ok {
			return asc.AppStoreVersionLocalizationAttributes{}, fmt.Errorf("field %q in %q must be a string", key, fileName)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		normalizedKey = strings.ReplaceAll(normalizedKey, "-", "")
		normalizedKey = strings.ReplaceAll(normalizedKey, "_", "")
		switch normalizedKey {
		case "description":
			attrs.Description = value
		case "keywords":
			attrs.Keywords = value
		case "supporturl":
			attrs.SupportURL = value
		case "marketingurl":
			attrs.MarketingURL = value
		case "promotionaltext":
			attrs.PromotionalText = value
		case "whatsnew":
			attrs.WhatsNew = value
		default:
			unknownKeys = append(unknownKeys, key)
		}
	}
	if len(unknownKeys) > 0 {
		sort.Strings(unknownKeys)
		return asc.AppStoreVersionLocalizationAttributes{}, fmt.Errorf("unsupported keys in %q: %s", fileName, strings.Join(unknownKeys, ", "))
	}
	if !appInfoSetHasAnyUpdates(attrs) {
		return asc.AppStoreVersionLocalizationAttributes{}, fmt.Errorf("locale file %q has no update values", fileName)
	}
	return attrs, nil
}

func appInfoSetRequestedLocales(locale, copyFromLocale string) []string {
	locales := []string{locale}
	if strings.TrimSpace(copyFromLocale) != "" {
		locales = append(locales, copyFromLocale)
	}
	return locales
}

func findAppInfoSetLocalizationByLocale(
	localizations []asc.Resource[asc.AppStoreVersionLocalizationAttributes],
	locale string,
) (asc.Resource[asc.AppStoreVersionLocalizationAttributes], bool) {
	for _, localization := range localizations {
		if strings.EqualFold(strings.TrimSpace(localization.Attributes.Locale), strings.TrimSpace(locale)) {
			return localization, true
		}
	}
	return asc.Resource[asc.AppStoreVersionLocalizationAttributes]{}, false
}

func applyAppInfoSetValues(
	attrs asc.AppStoreVersionLocalizationAttributes,
	description string,
	keywords string,
	supportURL string,
	marketingURL string,
	promotionalText string,
	whatsNew string,
) asc.AppStoreVersionLocalizationAttributes {
	if strings.TrimSpace(description) != "" {
		attrs.Description = strings.TrimSpace(description)
	}
	if strings.TrimSpace(keywords) != "" {
		attrs.Keywords = strings.TrimSpace(keywords)
	}
	if strings.TrimSpace(supportURL) != "" {
		attrs.SupportURL = strings.TrimSpace(supportURL)
	}
	if strings.TrimSpace(marketingURL) != "" {
		attrs.MarketingURL = strings.TrimSpace(marketingURL)
	}
	if strings.TrimSpace(promotionalText) != "" {
		attrs.PromotionalText = strings.TrimSpace(promotionalText)
	}
	if strings.TrimSpace(whatsNew) != "" {
		attrs.WhatsNew = strings.TrimSpace(whatsNew)
	}
	return attrs
}

func shouldBackfillAppInfoSetField(explicitValue string, targetExists bool, targetValue string) bool {
	if strings.TrimSpace(explicitValue) != "" {
		return false
	}
	if !targetExists {
		return true
	}
	return strings.TrimSpace(targetValue) == ""
}

func warnAppInfoSetSubmitIncompleteLocale(locale string, attrs asc.AppStoreVersionLocalizationAttributes) {
	warning := shared.SubmitIncompleteLocaleWarning(locale, attrs)
	if warning == "" {
		return
	}

	fmt.Fprint(os.Stderr, warning)
}

func resolveAppStoreVersionForAppInfo(
	ctx context.Context,
	client *asc.Client,
	appID string,
	versionID string,
	version string,
	platforms []string,
	states []string,
) (asc.Resource[asc.AppStoreVersionAttributes], error) {
	if strings.TrimSpace(versionID) != "" {
		resp, err := client.GetAppStoreVersion(ctx, versionID)
		if err != nil {
			return asc.Resource[asc.AppStoreVersionAttributes]{}, err
		}
		return resp.Data, nil
	}

	if strings.TrimSpace(appID) == "" {
		return asc.Resource[asc.AppStoreVersionAttributes]{}, fmt.Errorf("app id is required")
	}

	if strings.TrimSpace(version) != "" {
		if len(platforms) != 1 {
			return asc.Resource[asc.AppStoreVersionAttributes]{}, fmt.Errorf("--platform is required with --version")
		}
		resolvedVersionID, err := shared.ResolveAppStoreVersionID(ctx, client, appID, strings.TrimSpace(version), platforms[0])
		if err != nil {
			return asc.Resource[asc.AppStoreVersionAttributes]{}, err
		}
		resp, err := client.GetAppStoreVersion(ctx, resolvedVersionID)
		if err != nil {
			return asc.Resource[asc.AppStoreVersionAttributes]{}, err
		}
		return resp.Data, nil
	}

	opts := []asc.AppStoreVersionsOption{
		asc.WithAppStoreVersionsLimit(200),
		asc.WithAppStoreVersionsPlatforms(platforms),
		asc.WithAppStoreVersionsStates(states),
	}
	resp, err := client.GetAppStoreVersions(ctx, appID, opts...)
	if err != nil {
		return asc.Resource[asc.AppStoreVersionAttributes]{}, err
	}
	if len(resp.Data) == 0 {
		return asc.Resource[asc.AppStoreVersionAttributes]{}, fmt.Errorf("no app store versions found for app %q", appID)
	}

	return selectLatestAppStoreVersion(resp.Data), nil
}

func selectLatestAppStoreVersion(versions []asc.Resource[asc.AppStoreVersionAttributes]) asc.Resource[asc.AppStoreVersionAttributes] {
	sort.SliceStable(versions, func(i, j int) bool {
		return parseAppStoreVersionCreatedDate(versions[i]).After(parseAppStoreVersionCreatedDate(versions[j]))
	})
	return versions[0]
}

func parseAppStoreVersionCreatedDate(version asc.Resource[asc.AppStoreVersionAttributes]) time.Time {
	created := strings.TrimSpace(version.Attributes.CreatedDate)
	if created == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339, created); err == nil {
		return parsed
	}
	if parsed, err := time.Parse(time.RFC3339Nano, created); err == nil {
		return parsed
	}
	return time.Time{}
}

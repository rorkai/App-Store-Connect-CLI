package apps

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppInfoCommand returns the app-info command with subcommands.
func AppInfoCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-info", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-info",
		ShortUsage: "asc app-info <subcommand> [flags]",
		ShortHelp:  "Manage App Store version metadata.",
		LongHelp: `Manage App Store version metadata like description, keywords, and what's new.

Examples:
  asc app-info get --app "APP_ID"
  asc app-info get --app "APP_ID" --version "1.2.3" --platform IOS
  asc app-info set --app "APP_ID" --locale "en-US" --whats-new "Bug fixes"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppInfoGetCommand(),
			AppInfoSetCommand(),
			AppInfoRelationshipsCommand(),
			AppInfoTerritoryAgeRatingsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppInfoGetCommand returns the get subcommand.
func AppInfoGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-info get", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	appInfoID := fs.String("app-info", "", "App Info ID (optional override)")
	versionID := fs.String("version-id", "", "App Store version ID (optional override)")
	version := fs.String("version", "", "App Store version string (optional)")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS (required with --version)")
	state := fs.String("state", "", "Filter by app store state(s), comma-separated")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	include := fs.String("include", "", "Include related resources: "+strings.Join(appInfoIncludeList(), ", "))
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-info get [flags]",
		ShortHelp:  "Get app store version localization metadata.",
		LongHelp: `Get App Store version localization metadata.

If multiple versions exist and no --version-id/--version is provided, the most
recently created version is used.

Examples:
  asc app-info get --app "APP_ID"
  asc app-info get --app "APP_ID" --version "1.2.3" --platform IOS
  asc app-info get --version-id "VERSION_ID"
  asc app-info get --app-info "APP_INFO_ID" --include "ageRatingDeclaration"
  asc app-info get --app "APP_ID" --include "ageRatingDeclaration,territoryAgeRatings"
  asc app-info get --app "APP_ID" --locale "en-US" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
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
			if strings.TrimSpace(*versionID) == "" && resolvedAppID == "" && strings.TrimSpace(*appInfoID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app or --app-info is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			includeValues, err := normalizeAppInfoInclude(*include)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if strings.TrimSpace(*appInfoID) != "" && len(includeValues) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --app-info requires --include")
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
				return fmt.Errorf("app-info get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if len(includeValues) > 0 {
				appInfoIDValue, err := shared.ResolveAppInfoID(requestCtx, client, resolvedAppID, strings.TrimSpace(*appInfoID))
				if err != nil {
					return fmt.Errorf("app-info get: %w", err)
				}

				resp, err := client.GetAppInfo(requestCtx, appInfoIDValue, asc.WithAppInfoInclude(includeValues))
				if err != nil {
					return fmt.Errorf("app-info get: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
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
				return fmt.Errorf("app-info get: %w", err)
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
					return fmt.Errorf("app-info get: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionLocalizations(ctx, versionResource.ID, asc.WithAppStoreVersionLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-info get: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppStoreVersionLocalizations(requestCtx, versionResource.ID, opts...)
			if err != nil {
				return fmt.Errorf("app-info get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AppInfoSetCommand returns the set subcommand.
func AppInfoSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-info set", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	versionID := fs.String("version-id", "", "App Store version ID (optional override)")
	version := fs.String("version", "", "App Store version string (optional)")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS (required with --version)")
	state := fs.String("state", "", "Filter by app store state(s), comma-separated")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	description := fs.String("description", "", "App description")
	keywords := fs.String("keywords", "", "Keywords (comma-separated)")
	supportURL := fs.String("support-url", "", "Support URL")
	marketingURL := fs.String("marketing-url", "", "Marketing URL")
	promotionalText := fs.String("promotional-text", "", "Promotional text")
	whatsNew := fs.String("whats-new", "", "What's New text")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc app-info set [flags]",
		ShortHelp:  "Create or update app store version metadata.",
		LongHelp: `Create or update App Store version metadata.

Examples:
  asc app-info set --app "APP_ID" --locale "en-US" --whats-new "Bug fixes"
  asc app-info set --app "APP_ID" --version "1.2.3" --platform IOS --locale "en-US" --description "New release"`,
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
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}
			if err := shared.ValidateBuildLocalizationLocale(localeValue); err != nil {
				return shared.UsageError(err.Error())
			}

			descriptionValue := strings.TrimSpace(*description)
			keywordsValue := strings.TrimSpace(*keywords)
			supportURLValue := strings.TrimSpace(*supportURL)
			marketingURLValue := strings.TrimSpace(*marketingURL)
			promotionalTextValue := strings.TrimSpace(*promotionalText)
			whatsNewValue := strings.TrimSpace(*whatsNew)
			if descriptionValue == "" &&
				keywordsValue == "" &&
				supportURLValue == "" &&
				marketingURLValue == "" &&
				promotionalTextValue == "" &&
				whatsNewValue == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-info set: %w", err)
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
				return fmt.Errorf("app-info set: %w", err)
			}

			localizationOpts := []asc.AppStoreVersionLocalizationsOption{
				asc.WithAppStoreVersionLocalizationsLimit(200),
				asc.WithAppStoreVersionLocalizationLocales([]string{localeValue}),
			}
			localizations, err := client.GetAppStoreVersionLocalizations(requestCtx, versionResource.ID, localizationOpts...)
			if err != nil {
				return fmt.Errorf("app-info set: failed to fetch localizations: %w", err)
			}

			attrs := asc.AppStoreVersionLocalizationAttributes{}
			if descriptionValue != "" {
				attrs.Description = descriptionValue
			}
			if keywordsValue != "" {
				attrs.Keywords = keywordsValue
			}
			if supportURLValue != "" {
				attrs.SupportURL = supportURLValue
			}
			if marketingURLValue != "" {
				attrs.MarketingURL = marketingURLValue
			}
			if promotionalTextValue != "" {
				attrs.PromotionalText = promotionalTextValue
			}
			if whatsNewValue != "" {
				attrs.WhatsNew = whatsNewValue
			}

			if len(localizations.Data) == 0 {
				attrs.Locale = localeValue
				resp, err := client.CreateAppStoreVersionLocalization(requestCtx, versionResource.ID, attrs)
				if err != nil {
					return fmt.Errorf("app-info set: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			localizationID := strings.TrimSpace(localizations.Data[0].ID)
			if localizationID == "" {
				return fmt.Errorf("app-info set: localization id is empty")
			}
			resp, err := client.UpdateAppStoreVersionLocalization(requestCtx, localizationID, attrs)
			if err != nil {
				return fmt.Errorf("app-info set: %w", err)
			}
			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
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

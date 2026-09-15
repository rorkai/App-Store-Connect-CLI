package apps

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppsSearchKeywordsCommand returns the search keywords command group.
func AppsSearchKeywordsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("search-keywords", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "search-keywords",
		ShortUsage: "asc apps search-keywords <subcommand> [flags]",
		ShortHelp:  "Read app keywords and update version-localized keyword text.",
		LongHelp: `Read app keywords and update version-localized keyword text.

Apple exposes the app-level App Store Connect ` + "`searchKeywords`" + `
resource as read-only. The set command preserves its released spelling but
updates the supported App Store version-localization keywords attribute.

Examples:
  asc apps search-keywords list --app "APP_ID"
  asc apps search-keywords list --app "APP_ID" --platform IOS --locale "en-US"
  asc apps search-keywords set --app "APP_ID" --version "1.2.3" --locale "en-US" --platform IOS --keywords "kw1,kw2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppsSearchKeywordsListCommand(),
			AppsSearchKeywordsSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppsSearchKeywordsSetCommand returns the search keywords set subcommand.
func AppsSearchKeywordsSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps search-keywords set", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App Store version string (required)")
	locale := fs.String("locale", "", "Version localization locale (required; for example en-US)")
	platform := fs.String("platform", "", "App Store version platform: IOS, MAC_OS, TV_OS, VISION_OS (required only when ambiguous)")
	keywords := shared.BindOnceCSVFlag(fs, "keywords", "Version-localized keywords (comma-separated)")
	confirm := fs.Bool("confirm", false, "Confirm replacing the localization's keywords")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc apps search-keywords set --app \"APP_ID\" --version \"1.2.3\" --locale \"en-US\" --keywords \"kw1,kw2\" --confirm",
		ShortHelp:  "Replace keyword text for an App Store version localization.",
		LongHelp: `Replace keyword text for an App Store version localization.

The app-level searchKeywords relationship is read-only. This command resolves
the requested App Store version and locale, then updates
appStoreVersionLocalizations.attributes.keywords.

Existing scripts must add --version and --locale. Use --platform when the same
version string exists on more than one platform.

Examples:
  asc apps search-keywords set --app "APP_ID" --version "1.2.3" --locale "en-US" --keywords "kw1,kw2" --confirm
  asc apps search-keywords set --app "APP_ID" --version "1.2.3" --locale "en-US" --platform IOS --keywords "kw1,kw2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("apps search-keywords set does not accept positional arguments")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			versionValue := strings.TrimSpace(*version)
			if versionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --version is required to select an App Store version; existing invocations must add --version and --locale")
				return shared.MissingRequiredUsageError("--version")
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required to select a version localization; existing invocations must add --version and --locale")
				return shared.MissingRequiredUsageError("--locale")
			}
			canonicalLocale, err := shared.CanonicalizeAppStoreLocalizationLocale(localeValue)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			platformValue := strings.TrimSpace(*platform)
			if platformValue != "" {
				platformValue, err = shared.NormalizeAppStoreVersionPlatform(platformValue)
				if err != nil {
					return shared.UsageError(err.Error())
				}
			}

			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError("--confirm")
			}

			keywordValue := strings.TrimSpace(keywords.String())
			if keywordValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --keywords is required")
				return shared.MissingRequiredUsageError("--keywords")
			}
			if err := shared.ValidateVersionLocalizationAttributes(asc.AppStoreVersionLocalizationAttributes{Keywords: keywordValue}); err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps search-keywords set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			versionID, err := resolveSearchKeywordsVersionID(requestCtx, client, resolvedAppID, versionValue, platformValue)
			if err != nil {
				return fmt.Errorf("apps search-keywords set: %w", err)
			}

			localizationID, err := resolveSearchKeywordsLocalizationID(requestCtx, client, versionID, canonicalLocale)
			if err != nil {
				return fmt.Errorf("apps search-keywords set: %w", err)
			}

			resp, err := client.UpdateAppStoreVersionLocalizationFields(requestCtx, localizationID, map[string]string{
				"keywords": keywordValue,
			})
			if err != nil {
				return fmt.Errorf("apps search-keywords set: failed to update locale %q: %w", canonicalLocale, err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func resolveSearchKeywordsVersionID(ctx context.Context, client *asc.Client, appID, version, platform string) (string, error) {
	opts := []asc.AppStoreVersionsOption{
		asc.WithAppStoreVersionsVersionStrings([]string{version}),
		asc.WithAppStoreVersionsLimit(200),
	}
	if platform != "" {
		opts = append(opts, asc.WithAppStoreVersionsPlatforms([]string{platform}))
	}

	firstPage, err := client.GetAppStoreVersions(ctx, appID, opts...)
	if err != nil {
		return "", fmt.Errorf("failed to resolve App Store version: %w", err)
	}
	allPages, err := asc.PaginateAll(ctx, firstPage, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppStoreVersions(pageCtx, appID, asc.WithAppStoreVersionsNextURL(nextURL))
	})
	if err != nil {
		return "", fmt.Errorf("failed to resolve App Store version: %w", err)
	}
	versions, ok := allPages.(*asc.AppStoreVersionsResponse)
	if !ok {
		return "", fmt.Errorf("failed to resolve App Store version: unexpected response type %T", allPages)
	}

	candidates := make([]asc.Resource[asc.AppStoreVersionAttributes], 0, len(versions.Data))
	for _, item := range versions.Data {
		if !strings.EqualFold(strings.TrimSpace(item.Attributes.VersionString), version) {
			continue
		}
		if platform != "" && !strings.EqualFold(strings.TrimSpace(string(item.Attributes.Platform)), platform) {
			continue
		}
		candidates = append(candidates, item)
	}

	switch len(candidates) {
	case 0:
		if platform != "" {
			return "", fmt.Errorf("app store version not found for version %q and platform %q", version, platform)
		}
		return "", fmt.Errorf("app store version not found for version %q", version)
	case 1:
		return candidates[0].ID, nil
	}

	return "", shared.AmbiguousAppStoreVersionError(version, platform, candidates, "--platform", "")
}

func resolveSearchKeywordsLocalizationID(ctx context.Context, client *asc.Client, versionID, locale string) (string, error) {
	firstPage, err := client.GetAppStoreVersionLocalizations(
		ctx,
		versionID,
		asc.WithAppStoreVersionLocalizationLocales([]string{locale}),
		asc.WithAppStoreVersionLocalizationsLimit(200),
	)
	if err != nil {
		return "", fmt.Errorf("failed to resolve version localization: %w", err)
	}
	allPages, err := asc.PaginateAll(ctx, firstPage, func(pageCtx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppStoreVersionLocalizations(pageCtx, versionID, asc.WithAppStoreVersionLocalizationsNextURL(nextURL))
	})
	if err != nil {
		return "", fmt.Errorf("failed to resolve version localization: %w", err)
	}
	localizations, ok := allPages.(*asc.AppStoreVersionLocalizationsResponse)
	if !ok {
		return "", fmt.Errorf("failed to resolve version localization: unexpected response type %T", allPages)
	}

	candidates := make([]asc.Resource[asc.AppStoreVersionLocalizationAttributes], 0, len(localizations.Data))
	for _, item := range localizations.Data {
		if strings.EqualFold(strings.TrimSpace(item.Attributes.Locale), locale) {
			candidates = append(candidates, item)
		}
	}

	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("no existing version localization found for locale %q", locale)
	case 1:
		return candidates[0].ID, nil
	default:
		return "", shared.AmbiguousLocalizationError("version localization", locale, shared.LocalizationCandidates(candidates, func(attributes asc.AppStoreVersionLocalizationAttributes) string { return attributes.Locale }))
	}
}

// AppsSearchKeywordsListCommand returns the search keywords list subcommand.
func AppsSearchKeywordsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps search-keywords list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS (comma-separated)")
	locale := fs.String("locale", "", "Filter by locale(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc apps search-keywords list --app \"APP_ID\"",
		ShortHelp:  "List search keywords for an app.",
		LongHelp: `List search keywords for an app.

Examples:
  asc apps search-keywords list --app "APP_ID"
  asc apps search-keywords list --app "APP_ID" --platform IOS --locale "en-US"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("apps search-keywords list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("apps search-keywords list: %v", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platform))
			if err != nil {
				return fmt.Errorf("apps search-keywords list: %w", err)
			}

			locales := shared.SplitCSV(*locale)
			if err := shared.ValidateBuildLocalizationLocales(locales); err != nil {
				return fmt.Errorf("apps search-keywords list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps search-keywords list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppSearchKeywordsOption{
				asc.WithAppSearchKeywordsLimit(*limit),
				asc.WithAppSearchKeywordsNextURL(*next),
			}
			if len(platforms) > 0 {
				opts = append(opts, asc.WithAppSearchKeywordsPlatforms(platforms))
			}
			if len(locales) > 0 {
				opts = append(opts, asc.WithAppSearchKeywordsLocales(locales))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppSearchKeywordsLimit(200))
				firstPage, err := client.GetAppSearchKeywords(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("apps search-keywords list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppSearchKeywords(ctx, resolvedAppID, asc.WithAppSearchKeywordsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("apps search-keywords list: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppSearchKeywords(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("apps search-keywords list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

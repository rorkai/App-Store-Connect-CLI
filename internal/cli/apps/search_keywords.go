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
		ShortHelp:  "Manage search keywords for an app.",
		LongHelp: `Manage search keywords for an app.

Examples:
  asc apps search-keywords list --app "APP_ID"
  asc apps search-keywords list --app "APP_ID" --platform IOS --locale "en-US"
  asc apps search-keywords set --app "APP_ID" --keywords "kw1,kw2" --confirm`,
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
				return fmt.Errorf("apps search-keywords list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("apps search-keywords list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
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

// AppsSearchKeywordsSetCommand returns the search keywords set subcommand.
func AppsSearchKeywordsSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps search-keywords set", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	keywords := fs.String("keywords", "", "Keywords (comma-separated)")
	confirm := fs.Bool("confirm", false, "Confirm replacing all keywords")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc apps search-keywords set --app \"APP_ID\" --keywords \"kw1,kw2\" --confirm",
		ShortHelp:  "Replace search keywords for an app.",
		LongHelp: `Replace search keywords for an app.

Examples:
  asc apps search-keywords set --app "APP_ID" --keywords "kw1,kw2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			keywordValues := shared.SplitCSV(*keywords)
			if len(keywordValues) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --keywords is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps search-keywords set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.SetAppSearchKeywords(requestCtx, resolvedAppID, keywordValues); err != nil {
				return fmt.Errorf("apps search-keywords set: failed to update: %w", err)
			}

			return shared.PrintOutput(buildAppKeywordsResponse(keywordValues), *output.Output, *output.Pretty)
		},
	}
}

func buildAppKeywordsResponse(keywords []string) *asc.AppKeywordsResponse {
	resp := &asc.AppKeywordsResponse{
		Data: make([]asc.Resource[asc.AppKeywordAttributes], 0, len(keywords)),
	}
	for _, keyword := range keywords {
		resp.Data = append(resp.Data, asc.Resource[asc.AppKeywordAttributes]{
			Type:       asc.ResourceTypeAppKeywords,
			ID:         keyword,
			Attributes: asc.AppKeywordAttributes{},
		})
	}
	return resp
}

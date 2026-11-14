package productpages

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newCustomPageLocalizationsSearchKeywordsCommand returns the search keywords command group.
func newCustomPageLocalizationsSearchKeywordsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("search-keywords", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "search-keywords",
		ShortUsage: "asc product-pages custom-pages localizations search-keywords <subcommand> [flags]",
		ShortHelp:  "Manage search keywords for a custom product page localization.",
		LongHelp: `Manage search keywords for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations search-keywords list --localization-id "LOCALIZATION_ID"
  asc product-pages custom-pages localizations search-keywords add --localization-id "LOCALIZATION_ID" --keywords "kw1,kw2"
  asc product-pages custom-pages localizations search-keywords delete --localization-id "LOCALIZATION_ID" --keywords "kw1,kw2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newCustomPageLocalizationsSearchKeywordsListCommand(d),
			newCustomPageLocalizationsSearchKeywordsAddCommand(d),
			newCustomPageLocalizationsSearchKeywordsDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newCustomPageLocalizationsSearchKeywordsListCommand returns the search keywords list subcommand.
func newCustomPageLocalizationsSearchKeywordsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-localizations search-keywords list", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appCustomProductPageLocalizations", "Custom product page localization ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages custom-pages localizations search-keywords list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List search keywords for a custom product page localization.",
		LongHelp: `List search keywords for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations search-keywords list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				return d.RequiredFlag("--localization-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages localizations search-keywords list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppCustomProductPageLocalizationSearchKeywords(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("custom-pages localizations search-keywords list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCustomPageLocalizationsSearchKeywordsAddCommand returns the search keywords add subcommand.
func newCustomPageLocalizationsSearchKeywordsAddCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-localizations search-keywords add", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appCustomProductPageLocalizations", "Custom product page localization ID")
	keywords := fs.String("keywords", "", "Keywords (comma-separated)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add",
		ShortUsage: "asc product-pages custom-pages localizations search-keywords add --localization-id \"LOCALIZATION_ID\" --keywords \"kw1,kw2\"",
		ShortHelp:  "Add search keywords to a custom product page localization.",
		LongHelp: `Add search keywords to a custom product page localization.

Examples:
  asc product-pages custom-pages localizations search-keywords add --localization-id "LOCALIZATION_ID" --keywords "kw1,kw2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				return d.RequiredFlag("--localization-id")
			}

			keywordValues := shared.SplitCSV(*keywords)
			if len(keywordValues) == 0 {
				return d.RequiredFlag("--keywords")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages localizations search-keywords add: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.AddAppCustomProductPageLocalizationSearchKeywords(requestCtx, trimmedID, keywordValues); err != nil {
				return fmt.Errorf("custom-pages localizations search-keywords add: failed to add: %w", err)
			}

			return d.PrintOutput(shared.BuildAppKeywordsResponse(keywordValues), *output.Output, *output.Pretty)
		},
	}
}

// newCustomPageLocalizationsSearchKeywordsDeleteCommand returns the search keywords delete subcommand.
func newCustomPageLocalizationsSearchKeywordsDeleteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-localizations search-keywords delete", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appCustomProductPageLocalizations", "Custom product page localization ID")
	keywords := fs.String("keywords", "", "Keywords (comma-separated)")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc product-pages custom-pages localizations search-keywords delete --localization-id \"LOCALIZATION_ID\" --keywords \"kw1,kw2\" --confirm",
		ShortHelp:  "Delete search keywords from a custom product page localization.",
		LongHelp: `Delete search keywords from a custom product page localization.

Examples:
  asc product-pages custom-pages localizations search-keywords delete --localization-id "LOCALIZATION_ID" --keywords "kw1,kw2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				return d.RequiredFlag("--localization-id")
			}
			if !*confirm {
				return d.RequiredFlag("--confirm")
			}

			keywordValues := shared.SplitCSV(*keywords)
			if len(keywordValues) == 0 {
				return d.RequiredFlag("--keywords")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages localizations search-keywords delete: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.DeleteAppCustomProductPageLocalizationSearchKeywords(requestCtx, trimmedID, keywordValues); err != nil {
				return fmt.Errorf("custom-pages localizations search-keywords delete: failed to delete: %w", err)
			}

			return d.PrintOutput(shared.BuildAppKeywordsResponse(keywordValues), *output.Output, *output.Pretty)
		},
	}
}

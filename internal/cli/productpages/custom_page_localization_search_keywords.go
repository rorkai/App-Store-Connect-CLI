package productpages

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// CustomPageLocalizationsSearchKeywordsCommand returns the search keywords command group.
func CustomPageLocalizationsSearchKeywordsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("search-keywords", flag.ExitOnError)

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
			CustomPageLocalizationsSearchKeywordsListCommand(),
			CustomPageLocalizationsSearchKeywordsAddCommand(),
			CustomPageLocalizationsSearchKeywordsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CustomPageLocalizationsSearchKeywordsListCommand returns the search keywords list subcommand.
func CustomPageLocalizationsSearchKeywordsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations search-keywords list", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages localizations search-keywords list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppCustomProductPageLocalizationSearchKeywords(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("custom-pages localizations search-keywords list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// CustomPageLocalizationsSearchKeywordsAddCommand returns the search keywords add subcommand.
func CustomPageLocalizationsSearchKeywordsAddCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations search-keywords add", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	keywords := fs.String("keywords", "", "Keywords (comma-separated)")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return shared.MissingRequiredUsageError()
			}

			keywordValues := shared.SplitCSV(*keywords)
			if len(keywordValues) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --keywords is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages localizations search-keywords add: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.AddAppCustomProductPageLocalizationSearchKeywords(requestCtx, trimmedID, keywordValues); err != nil {
				return fmt.Errorf("custom-pages localizations search-keywords add: failed to add: %w", err)
			}

			return shared.PrintOutput(shared.BuildAppKeywordsResponse(keywordValues), *output.Output, *output.Pretty)
		},
	}
}

// CustomPageLocalizationsSearchKeywordsDeleteCommand returns the search keywords delete subcommand.
func CustomPageLocalizationsSearchKeywordsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations search-keywords delete", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	keywords := fs.String("keywords", "", "Keywords (comma-separated)")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}

			keywordValues := shared.SplitCSV(*keywords)
			if len(keywordValues) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --keywords is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages localizations search-keywords delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppCustomProductPageLocalizationSearchKeywords(requestCtx, trimmedID, keywordValues); err != nil {
				return fmt.Errorf("custom-pages localizations search-keywords delete: failed to delete: %w", err)
			}

			return shared.PrintOutput(shared.BuildAppKeywordsResponse(keywordValues), *output.Output, *output.Pretty)
		},
	}
}

package localizations

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// LocalizationsSearchKeywordsCommand returns the search keywords command group.
func newLocalizationsSearchKeywordsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("search-keywords", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "search-keywords",
		ShortUsage: "asc localizations search-keywords <subcommand> [flags]",
		ShortHelp:  "Manage raw search-keyword relationships for an App Store localization.",
		LongHelp: `Manage raw search-keyword relationships for an App Store localization.

This command wraps the low-level App Store Connect ` + "`searchKeywords`" + `
relationship API for App Store version localizations.

For canonical version-localization keyword workflows, use
` + "`asc metadata keywords ...`" + ` instead.

Examples:
  asc localizations search-keywords list --localization-id "LOCALIZATION_ID"
  asc localizations search-keywords add --localization-id "LOCALIZATION_ID" --keywords "kw1,kw2"
  asc localizations search-keywords delete --localization-id "LOCALIZATION_ID" --keywords "kw1,kw2" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newLocalizationsSearchKeywordsListCommand(d),
			newLocalizationsSearchKeywordsAddCommand(d),
			newLocalizationsSearchKeywordsDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// LocalizationsSearchKeywordsListCommand returns the search keywords list subcommand.
func newLocalizationsSearchKeywordsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations search-keywords list", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appStoreVersionLocalizations", "App Store version localization ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc localizations search-keywords list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List search keywords for an App Store localization.",
		LongHelp: `List search keywords for an App Store localization.

Examples:
  asc localizations search-keywords list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				return d.RequiredFlag("--localization-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("localizations search-keywords list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreVersionLocalizationSearchKeywords(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("localizations search-keywords list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// LocalizationsSearchKeywordsAddCommand returns the search keywords add subcommand.
func newLocalizationsSearchKeywordsAddCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations search-keywords add", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appStoreVersionLocalizations", "App Store version localization ID")
	keywords := fs.String("keywords", "", "Keywords (comma-separated)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add",
		ShortUsage: "asc localizations search-keywords add --localization-id \"LOCALIZATION_ID\" --keywords \"kw1,kw2\"",
		ShortHelp:  "Add search keywords to an App Store localization.",
		LongHelp: `Add search keywords to an App Store localization.

Examples:
  asc localizations search-keywords add --localization-id "LOCALIZATION_ID" --keywords "kw1,kw2"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
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
				return fmt.Errorf("localizations search-keywords add: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.AddAppStoreVersionLocalizationSearchKeywords(requestCtx, trimmedID, keywordValues); err != nil {
				return fmt.Errorf("localizations search-keywords add: failed to add: %w", err)
			}

			return d.PrintOutput(shared.BuildAppKeywordsResponse(keywordValues), *output.Output, *output.Pretty)
		},
	}
}

// LocalizationsSearchKeywordsDeleteCommand returns the search keywords delete subcommand.
func newLocalizationsSearchKeywordsDeleteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations search-keywords delete", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appStoreVersionLocalizations", "App Store version localization ID")
	keywords := fs.String("keywords", "", "Keywords (comma-separated)")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc localizations search-keywords delete --localization-id \"LOCALIZATION_ID\" --keywords \"kw1,kw2\" --confirm",
		ShortHelp:  "Delete search keywords from an App Store localization.",
		LongHelp: `Delete search keywords from an App Store localization.

Examples:
  asc localizations search-keywords delete --localization-id "LOCALIZATION_ID" --keywords "kw1,kw2" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				return d.RequiredFlag("--localization-id")
			}

			keywordValues := shared.SplitCSV(*keywords)
			if len(keywordValues) == 0 {
				return d.RequiredFlag("--keywords")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("localizations search-keywords delete: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.DeleteAppStoreVersionLocalizationSearchKeywords(requestCtx, trimmedID, keywordValues); err != nil {
				return fmt.Errorf("localizations search-keywords delete: failed to delete: %w", err)
			}

			return d.PrintOutput(shared.BuildAppKeywordsResponse(keywordValues), *output.Output, *output.Pretty)
		},
	}
}

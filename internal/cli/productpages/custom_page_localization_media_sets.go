package productpages

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// CustomPageLocalizationsPreviewSetsCommand returns the preview sets command group.
func CustomPageLocalizationsPreviewSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("preview-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "preview-sets",
		ShortUsage: "asc product-pages custom-pages localizations preview-sets <subcommand> [flags]",
		ShortHelp:  "Manage preview sets for a custom product page localization.",
		LongHelp: `Manage preview sets for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations preview-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			CustomPageLocalizationsPreviewSetsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CustomPageLocalizationsPreviewSetsListCommand returns the preview sets list subcommand.
func CustomPageLocalizationsPreviewSetsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations preview-sets list", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages custom-pages localizations preview-sets list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List preview sets for a custom product page localization.",
		LongHelp: `List preview sets for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations preview-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("custom-pages localizations preview-sets list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("custom-pages localizations preview-sets list: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages localizations preview-sets list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppCustomProductPageLocalizationPreviewSetsOption{
				asc.WithAppCustomProductPageLocalizationPreviewSetsLimit(*limit),
				asc.WithAppCustomProductPageLocalizationPreviewSetsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppCustomProductPageLocalizationPreviewSetsLimit(200))
				firstPage, err := client.GetAppCustomProductPageLocalizationPreviewSets(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("custom-pages localizations preview-sets list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppCustomProductPageLocalizationPreviewSets(ctx, trimmedID, asc.WithAppCustomProductPageLocalizationPreviewSetsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("custom-pages localizations preview-sets list: %w", err)
				}
				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppCustomProductPageLocalizationPreviewSets(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("custom-pages localizations preview-sets list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// CustomPageLocalizationsScreenshotSetsCommand returns the screenshot sets command group.
func CustomPageLocalizationsScreenshotSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshot-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshot-sets",
		ShortUsage: "asc product-pages custom-pages localizations screenshot-sets <subcommand> [flags]",
		ShortHelp:  "Manage screenshot sets for a custom product page localization.",
		LongHelp: `Manage screenshot sets for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations screenshot-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			CustomPageLocalizationsScreenshotSetsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// CustomPageLocalizationsScreenshotSetsListCommand returns the screenshot sets list subcommand.
func CustomPageLocalizationsScreenshotSetsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations screenshot-sets list", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages custom-pages localizations screenshot-sets list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List screenshot sets for a custom product page localization.",
		LongHelp: `List screenshot sets for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations screenshot-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("custom-pages localizations screenshot-sets list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("custom-pages localizations screenshot-sets list: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("custom-pages localizations screenshot-sets list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppCustomProductPageLocalizationScreenshotSetsOption{
				asc.WithAppCustomProductPageLocalizationScreenshotSetsLimit(*limit),
				asc.WithAppCustomProductPageLocalizationScreenshotSetsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppCustomProductPageLocalizationScreenshotSetsLimit(200))
				firstPage, err := client.GetAppCustomProductPageLocalizationScreenshotSets(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("custom-pages localizations screenshot-sets list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppCustomProductPageLocalizationScreenshotSets(ctx, trimmedID, asc.WithAppCustomProductPageLocalizationScreenshotSetsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("custom-pages localizations screenshot-sets list: %w", err)
				}
				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppCustomProductPageLocalizationScreenshotSets(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("custom-pages localizations screenshot-sets list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

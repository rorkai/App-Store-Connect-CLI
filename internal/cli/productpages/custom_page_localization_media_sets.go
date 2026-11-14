package productpages

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newCustomPageLocalizationsPreviewSetsCommand returns the preview sets command group.
func newCustomPageLocalizationsPreviewSetsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("preview-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "preview-sets",
		ShortUsage: "asc product-pages custom-pages localizations preview-sets <subcommand> [flags]",
		ShortHelp:  "Manage preview sets for a custom product page localization.",
		LongHelp: `Manage preview sets for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations preview-sets list --localization-id "LOCALIZATION_ID"
  asc product-pages custom-pages localizations preview-sets upload --localization-id "LOCALIZATION_ID" --path "./previews" --device-type "IPHONE_65"
  asc product-pages custom-pages localizations preview-sets sync --localization-id "LOCALIZATION_ID" --path "./previews" --device-type "IPHONE_65" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newCustomPageLocalizationsPreviewSetsListCommand(d),
			newCustomPageLocalizationsPreviewSetsUploadCommand(d),
			newCustomPageLocalizationsPreviewSetsSyncCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newCustomPageLocalizationsPreviewSetsListCommand returns the preview sets list subcommand.
func newCustomPageLocalizationsPreviewSetsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-localizations preview-sets list", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appCustomProductPageLocalizations", "Custom product page localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages custom-pages localizations preview-sets list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List preview sets for a custom product page localization.",
		LongHelp: `List preview sets for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations preview-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				return d.RequiredFlag("--localization-id")
			}
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return d.UsageErrorf("custom-pages localizations preview-sets list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("custom-pages localizations preview-sets list: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages localizations preview-sets list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppCustomProductPageLocalizationPreviewSetsOption{
				asc.Limit[asc.AppCustomProductPageLocalizationPreviewSetsOption](*limit),
				asc.NextURL[asc.AppCustomProductPageLocalizationPreviewSetsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppCustomProductPageLocalizationPreviewSetsOption](productPagesMaxLimit))
				firstPage, err := client.GetAppCustomProductPageLocalizationPreviewSets(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("custom-pages localizations preview-sets list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppCustomProductPageLocalizationPreviewSets(ctx, trimmedID, asc.NextURL[asc.AppCustomProductPageLocalizationPreviewSetsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("custom-pages localizations preview-sets list: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppCustomProductPageLocalizationPreviewSets(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("custom-pages localizations preview-sets list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newCustomPageLocalizationsScreenshotSetsCommand returns the screenshot sets command group.
func newCustomPageLocalizationsScreenshotSetsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("screenshot-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshot-sets",
		ShortUsage: "asc product-pages custom-pages localizations screenshot-sets <subcommand> [flags]",
		ShortHelp:  "Manage screenshot sets for a custom product page localization.",
		LongHelp: `Manage screenshot sets for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations screenshot-sets list --localization-id "LOCALIZATION_ID"
  asc product-pages custom-pages localizations screenshot-sets upload --localization-id "LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65"
  asc product-pages custom-pages localizations screenshot-sets sync --localization-id "LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newCustomPageLocalizationsScreenshotSetsListCommand(d),
			newCustomPageLocalizationsScreenshotSetsUploadCommand(d),
			newCustomPageLocalizationsScreenshotSetsSyncCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newCustomPageLocalizationsScreenshotSetsListCommand returns the screenshot sets list subcommand.
func newCustomPageLocalizationsScreenshotSetsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("custom-page-localizations screenshot-sets list", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appCustomProductPageLocalizations", "Custom product page localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	includeScreenshots := fs.Bool("include-screenshots", false, "Include screenshot IDs and metadata for each set (requires --localization-id and --paginate)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages custom-pages localizations screenshot-sets list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List screenshot sets for a custom product page localization.",
		LongHelp: `List screenshot sets for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations screenshot-sets list --localization-id "LOCALIZATION_ID"
  asc product-pages custom-pages localizations screenshot-sets list --localization-id "LOCALIZATION_ID" --include-screenshots --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			trimmedNext := strings.TrimSpace(*next)
			if *includeScreenshots {
				if trimmedID == "" {
					return d.RequiredFlag("--localization-id")
				}
				if trimmedNext != "" {
					return d.UsageError("custom-pages localizations screenshot-sets list: --include-screenshots cannot be combined with --next")
				}
				if !*paginate {
					return d.UsageError("custom-pages localizations screenshot-sets list: --include-screenshots requires --paginate")
				}
			}
			if trimmedID == "" && trimmedNext == "" {
				return d.RequiredFlag("--localization-id")
			}
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return d.UsageErrorf("custom-pages localizations screenshot-sets list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("custom-pages localizations screenshot-sets list: %v", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("custom-pages localizations screenshot-sets list: %w", err)
			}

			opts := []asc.AppCustomProductPageLocalizationScreenshotSetsOption{
				asc.Limit[asc.AppCustomProductPageLocalizationScreenshotSetsOption](*limit),
				asc.NextURL[asc.AppCustomProductPageLocalizationScreenshotSetsOption](*next),
			}
			if *includeScreenshots {
				opts = append(opts, asc.WithAppCustomProductPageLocalizationScreenshotSetsIncludeScreenshots())
			}

			if *paginate {
				paginateOpts := make([]asc.AppCustomProductPageLocalizationScreenshotSetsOption, 0, len(opts)+2)
				paginateOpts = append(paginateOpts, opts...)
				paginateOpts = append(
					paginateOpts,
					asc.Limit[asc.AppCustomProductPageLocalizationScreenshotSetsOption](productPagesMaxLimit),
					asc.WithAppCustomProductPageLocalizationScreenshotSetsRequestContext(d.Timeout),
				)
				resp, err := client.GetAllAppCustomProductPageLocalizationScreenshotSets(ctx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("custom-pages localizations screenshot-sets list: %w", err)
				}
				if *includeScreenshots {
					result, err := screenshotSetListResult(ctx, d, client, trimmedID, resp)
					if err != nil {
						return fmt.Errorf("custom-pages localizations screenshot-sets list: %w", err)
					}
					return d.PrintOutput(result, *output.Output, *output.Pretty)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			resp, err := client.GetAppCustomProductPageLocalizationScreenshotSets(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("custom-pages localizations screenshot-sets list: failed to fetch: %w", err)
			}
			if *includeScreenshots {
				result, err := screenshotSetListResult(ctx, d, client, trimmedID, resp)
				if err != nil {
					return fmt.Errorf("custom-pages localizations screenshot-sets list: %w", err)
				}
				return d.PrintOutput(result, *output.Output, *output.Pretty)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

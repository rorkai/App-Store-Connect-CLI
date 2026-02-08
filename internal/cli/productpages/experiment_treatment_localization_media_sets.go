package productpages

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

// ExperimentTreatmentLocalizationPreviewSetsCommand returns the preview sets command group.
func ExperimentTreatmentLocalizationPreviewSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("preview-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "preview-sets",
		ShortUsage: "asc product-pages experiments treatments localizations preview-sets <subcommand> [flags]",
		ShortHelp:  "Manage preview sets for a treatment localization.",
		LongHelp: `Manage preview sets for a treatment localization.

Examples:
  asc product-pages experiments treatments localizations preview-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ExperimentTreatmentLocalizationPreviewSetsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ExperimentTreatmentLocalizationPreviewSetsListCommand returns the preview sets list subcommand.
func ExperimentTreatmentLocalizationPreviewSetsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("treatment-localizations preview-sets list", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Treatment localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages experiments treatments localizations preview-sets list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List preview sets for a treatment localization.",
		LongHelp: `List preview sets for a treatment localization.

Examples:
  asc product-pages experiments treatments localizations preview-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return fmt.Errorf("experiments treatments localizations preview-sets list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("experiments treatments localizations preview-sets list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments treatments localizations preview-sets list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionExperimentTreatmentLocalizationPreviewSetsOption{
				asc.WithAppStoreVersionExperimentTreatmentLocalizationPreviewSetsLimit(*limit),
				asc.WithAppStoreVersionExperimentTreatmentLocalizationPreviewSetsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppStoreVersionExperimentTreatmentLocalizationPreviewSetsLimit(productPagesMaxLimit))
				firstPage, err := client.GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("experiments treatments localizations preview-sets list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets(ctx, trimmedID, asc.WithAppStoreVersionExperimentTreatmentLocalizationPreviewSetsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("experiments treatments localizations preview-sets list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppStoreVersionExperimentTreatmentLocalizationPreviewSets(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations preview-sets list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// ExperimentTreatmentLocalizationScreenshotSetsCommand returns the screenshot sets command group.
func ExperimentTreatmentLocalizationScreenshotSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshot-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshot-sets",
		ShortUsage: "asc product-pages experiments treatments localizations screenshot-sets <subcommand> [flags]",
		ShortHelp:  "Manage screenshot sets for a treatment localization.",
		LongHelp: `Manage screenshot sets for a treatment localization.

Examples:
  asc product-pages experiments treatments localizations screenshot-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ExperimentTreatmentLocalizationScreenshotSetsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ExperimentTreatmentLocalizationScreenshotSetsListCommand returns the screenshot sets list subcommand.
func ExperimentTreatmentLocalizationScreenshotSetsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("treatment-localizations screenshot-sets list", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Treatment localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages experiments treatments localizations screenshot-sets list --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "List screenshot sets for a treatment localization.",
		LongHelp: `List screenshot sets for a treatment localization.

Examples:
  asc product-pages experiments treatments localizations screenshot-sets list --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			trimmedNext := strings.TrimSpace(*next)
			if trimmedID == "" && trimmedNext == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return fmt.Errorf("experiments treatments localizations screenshot-sets list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("experiments treatments localizations screenshot-sets list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments treatments localizations screenshot-sets list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionExperimentTreatmentLocalizationScreenshotSetsOption{
				asc.WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsLimit(*limit),
				asc.WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsLimit(productPagesMaxLimit))
				firstPage, err := client.GetAppStoreVersionExperimentTreatmentLocalizationScreenshotSets(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("experiments treatments localizations screenshot-sets list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionExperimentTreatmentLocalizationScreenshotSets(ctx, trimmedID, asc.WithAppStoreVersionExperimentTreatmentLocalizationScreenshotSetsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("experiments treatments localizations screenshot-sets list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppStoreVersionExperimentTreatmentLocalizationScreenshotSets(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations screenshot-sets list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

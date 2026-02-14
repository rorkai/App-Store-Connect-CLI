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

// ExperimentTreatmentLocalizationsCommand returns the treatment localizations command group.
func ExperimentTreatmentLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc product-pages experiments treatments localizations <subcommand> [flags]",
		ShortHelp:  "Manage treatment localizations.",
		LongHelp: `Manage treatment localizations.

Examples:
  asc product-pages experiments treatments localizations list --treatment-id "TREATMENT_ID"
  asc product-pages experiments treatments localizations create --treatment-id "TREATMENT_ID" --locale "en-US"
  asc product-pages experiments treatments localizations delete --localization-id "LOCALIZATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ExperimentTreatmentLocalizationsListCommand(),
			ExperimentTreatmentLocalizationsGetCommand(),
			ExperimentTreatmentLocalizationsCreateCommand(),
			ExperimentTreatmentLocalizationsDeleteCommand(),
			ExperimentTreatmentLocalizationPreviewSetsCommand(),
			ExperimentTreatmentLocalizationScreenshotSetsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ExperimentTreatmentLocalizationsListCommand returns the treatment localizations list subcommand.
func ExperimentTreatmentLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("treatment-localizations list", flag.ExitOnError)

	treatmentID := fs.String("treatment-id", "", "Treatment ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages experiments treatments localizations list --treatment-id \"TREATMENT_ID\" [flags]",
		ShortHelp:  "List treatment localizations.",
		LongHelp: `List treatment localizations.

Examples:
  asc product-pages experiments treatments localizations list --treatment-id "TREATMENT_ID"
  asc product-pages experiments treatments localizations list --treatment-id "TREATMENT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return fmt.Errorf("experiments treatments localizations list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("experiments treatments localizations list: %w", err)
			}

			trimmedID := strings.TrimSpace(*treatmentID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --treatment-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments treatments localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionExperimentTreatmentLocalizationsOption{
				asc.WithAppStoreVersionExperimentTreatmentLocalizationsLimit(*limit),
				asc.WithAppStoreVersionExperimentTreatmentLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppStoreVersionExperimentTreatmentLocalizationsLimit(productPagesMaxLimit))
				firstPage, err := client.GetAppStoreVersionExperimentTreatmentLocalizations(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("experiments treatments localizations list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionExperimentTreatmentLocalizations(ctx, trimmedID, asc.WithAppStoreVersionExperimentTreatmentLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("experiments treatments localizations list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppStoreVersionExperimentTreatmentLocalizations(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentTreatmentLocalizationsGetCommand returns the treatment localizations get subcommand.
func ExperimentTreatmentLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("treatment-localizations get", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Treatment localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc product-pages experiments treatments localizations get --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get a treatment localization by ID.",
		LongHelp: `Get a treatment localization by ID.

Examples:
  asc product-pages experiments treatments localizations get --localization-id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments treatments localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreVersionExperimentTreatmentLocalization(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentTreatmentLocalizationsCreateCommand returns the treatment localizations create subcommand.
func ExperimentTreatmentLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("treatment-localizations create", flag.ExitOnError)

	treatmentID := fs.String("treatment-id", "", "Treatment ID")
	locale := fs.String("locale", "", "Localization locale (e.g., en-US)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc product-pages experiments treatments localizations create --treatment-id \"TREATMENT_ID\" --locale \"en-US\"",
		ShortHelp:  "Create a treatment localization.",
		LongHelp: `Create a treatment localization.

Examples:
  asc product-pages experiments treatments localizations create --treatment-id "TREATMENT_ID" --locale "en-US"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*treatmentID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --treatment-id is required")
				return flag.ErrHelp
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments treatments localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppStoreVersionExperimentTreatmentLocalization(requestCtx, trimmedID, localeValue)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentTreatmentLocalizationsDeleteCommand returns the treatment localizations delete subcommand.
func ExperimentTreatmentLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("treatment-localizations delete", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Treatment localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc product-pages experiments treatments localizations delete --localization-id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:  "Delete a treatment localization.",
		LongHelp: `Delete a treatment localization.

Examples:
  asc product-pages experiments treatments localizations delete --localization-id "LOCALIZATION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments treatments localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppStoreVersionExperimentTreatmentLocalization(requestCtx, trimmedID); err != nil {
				return fmt.Errorf("experiments treatments localizations delete: failed to delete: %w", err)
			}

			result := &asc.AppStoreVersionExperimentTreatmentLocalizationDeleteResult{ID: trimmedID, Deleted: true}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

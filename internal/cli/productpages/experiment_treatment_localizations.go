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

// newExperimentTreatmentLocalizationsCommand returns the treatment localizations command group.
func newExperimentTreatmentLocalizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations", flag.ExitOnError)

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
			newExperimentTreatmentLocalizationsListCommand(d),
			newExperimentTreatmentLocalizationsGetCommand(d),
			newExperimentTreatmentLocalizationsCreateCommand(d),
			newExperimentTreatmentLocalizationsDeleteCommand(d),
			newExperimentTreatmentLocalizationPreviewSetsCommand(d),
			newExperimentTreatmentLocalizationScreenshotSetsCommand(d),
			shared.CreativePlacementsCommand("appStoreVersionExperimentTreatmentLocalizations", "product-pages experiments treatments localizations", "a product page optimization treatment localization"),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func treatmentLocalizationIncludeList() []string {
	return []string{"appStoreVersionExperimentTreatment", "appScreenshotSets", "appPreviewSets", "placements"}
}

// newExperimentTreatmentLocalizationsListCommand returns the treatment localizations list subcommand.
func newExperimentTreatmentLocalizationsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("treatment-localizations list", flag.ExitOnError)

	treatmentID := shared.BindResourceIDFlag(fs, "treatment-id", "appStoreVersionExperimentTreatments", "Treatment ID")
	include := shared.BindLocalizationIncludeFlags(fs, "Include related resources", treatmentLocalizationIncludeList())
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages experiments treatments localizations list --treatment-id \"TREATMENT_ID\" [flags]",
		ShortHelp:  "List treatment localizations.",
		LongHelp: `List treatment localizations.

Examples:
  asc product-pages experiments treatments localizations list --treatment-id "TREATMENT_ID"
  asc product-pages experiments treatments localizations list --treatment-id "TREATMENT_ID" --include placements --placements-limit 50
  asc product-pages experiments treatments localizations list --treatment-id "TREATMENT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return d.UsageErrorf("experiments treatments localizations list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("experiments treatments localizations list: %v", err)
			}
			if err := d.RejectNextFlagConflicts(fs, *next, "experiments treatments localizations list", "include", "placements-limit"); err != nil {
				return err
			}
			includeValues, placementsLimit, err := include.Resolve()
			if err != nil {
				return err
			}

			trimmedID := strings.TrimSpace(*treatmentID)
			if trimmedID == "" {
				return d.RequiredFlag("--treatment-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments treatments localizations list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionExperimentTreatmentLocalizationsOption{
				asc.Limit[asc.AppStoreVersionExperimentTreatmentLocalizationsOption](*limit),
				asc.NextURL[asc.AppStoreVersionExperimentTreatmentLocalizationsOption](*next),
				asc.WithAppStoreVersionExperimentTreatmentLocalizationsInclude(includeValues),
				asc.WithAppStoreVersionExperimentTreatmentLocalizationsPlacementsLimit(placementsLimit),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppStoreVersionExperimentTreatmentLocalizationsOption](productPagesMaxLimit))
				firstPage, err := client.GetAppStoreVersionExperimentTreatmentLocalizations(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("experiments treatments localizations list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionExperimentTreatmentLocalizations(ctx, trimmedID, asc.NextURL[asc.AppStoreVersionExperimentTreatmentLocalizationsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("experiments treatments localizations list: %w", err)
				}

				return d.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppStoreVersionExperimentTreatmentLocalizations(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentTreatmentLocalizationsGetCommand returns the treatment localizations get subcommand.
func newExperimentTreatmentLocalizationsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("treatment-localizations view", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appStoreVersionExperimentTreatmentLocalizations", "Treatment localization ID")
	include := shared.BindLocalizationIncludeFlags(fs, "Include related resources", treatmentLocalizationIncludeList())
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc product-pages experiments treatments localizations view --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:  "View a treatment localization by ID.",
		LongHelp: `View a treatment localization by ID.

Examples:
  asc product-pages experiments treatments localizations view --localization-id "LOCALIZATION_ID"
  asc product-pages experiments treatments localizations view --localization-id "LOCALIZATION_ID" --include placements --placements-limit 50`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*localizationID)
			if trimmedID == "" {
				return d.RequiredFlag("--localization-id")
			}
			includeValues, placementsLimit, err := include.Resolve()
			if err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments treatments localizations view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreVersionExperimentTreatmentLocalization(requestCtx, trimmedID, asc.WithLocalizationReadInclude(includeValues), asc.WithLocalizationReadPlacementsLimit(placementsLimit))
			if err != nil {
				return fmt.Errorf("experiments treatments localizations view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentTreatmentLocalizationsCreateCommand returns the treatment localizations create subcommand.
func newExperimentTreatmentLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("treatment-localizations create", flag.ExitOnError)

	treatmentID := shared.BindResourceIDFlag(fs, "treatment-id", "appStoreVersionExperimentTreatments", "Treatment ID")
	locale := fs.String("locale", "", "Localization locale (e.g., en-US)")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--treatment-id")
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				return d.RequiredFlag("--locale")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments treatments localizations create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateAppStoreVersionExperimentTreatmentLocalization(requestCtx, trimmedID, localeValue)
			if err != nil {
				return fmt.Errorf("experiments treatments localizations create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentTreatmentLocalizationsDeleteCommand returns the treatment localizations delete subcommand.
func newExperimentTreatmentLocalizationsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "treatment-localizations delete",
		Name:        "delete",
		ShortUsage:  "asc product-pages experiments treatments localizations delete --localization-id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:   "Delete a treatment localization.",
		LongHelp: `Delete a treatment localization.

Examples:
  asc product-pages experiments treatments localizations delete --localization-id "LOCALIZATION_ID" --confirm`,
		IDFlag:             "localization-id",
		IDUsage:            "Treatment localization ID",
		IDType:             "appStoreVersionExperimentTreatmentLocalizations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "experiments treatments localizations delete",
		DeleteErrorPrefix:  "experiments treatments localizations delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppStoreVersionExperimentTreatmentLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AppStoreVersionExperimentTreatmentLocalizationDeleteResult{ID: id, Deleted: true}
		},
	})
}

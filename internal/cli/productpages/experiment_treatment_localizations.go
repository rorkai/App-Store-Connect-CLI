package productpages

import (
	"context"
	"flag"

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
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newExperimentTreatmentLocalizationsListCommand(d),
			newExperimentTreatmentLocalizationsGetCommand(d),
			newExperimentTreatmentLocalizationsCreateCommand(d),
			newExperimentTreatmentLocalizationsDeleteCommand(d),
			newExperimentTreatmentLocalizationPreviewSetsCommand(d),
			newExperimentTreatmentLocalizationScreenshotSetsCommand(d),
			d.CreativePlacementsCommand("appStoreVersionExperimentTreatmentLocalizations", "product-pages experiments treatments localizations", "a product page optimization treatment localization"),
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
	var (
		includeValues   []string
		placementsLimit int
	)

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "treatment-localizations list",
		Name:        "list",
		ShortUsage:  "asc product-pages experiments treatments localizations list --treatment-id \"TREATMENT_ID\" [flags]",
		ShortHelp:   "List treatment localizations.",
		LongHelp: `List treatment localizations.

Examples:
  asc product-pages experiments treatments localizations list --treatment-id "TREATMENT_ID"
  asc product-pages experiments treatments localizations list --treatment-id "TREATMENT_ID" --include placements --placements-limit 50
  asc product-pages experiments treatments localizations list --treatment-id "TREATMENT_ID" --paginate`,
		ParentFlag:           "treatment-id",
		ParentUsage:          "Treatment ID",
		ParentType:           "appStoreVersionExperimentTreatments",
		RequiredDiagnostic:   true,
		LimitMax:             productPagesMaxLimit,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"include", "placements-limit"},
		ErrorPrefix:          "experiments treatments localizations list",
		Deps:                 d,
		Flags: func(fs *flag.FlagSet) func() error {
			include := shared.BindLocalizationIncludeFlags(fs, "Include related resources", treatmentLocalizationIncludeList())
			return func() error {
				var err error
				includeValues, placementsLimit, err = include.Resolve()
				return err
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, treatmentID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppStoreVersionExperimentTreatmentLocalizations(ctx, treatmentID,
				asc.Limit[asc.AppStoreVersionExperimentTreatmentLocalizationsOption](limit),
				asc.NextURL[asc.AppStoreVersionExperimentTreatmentLocalizationsOption](next),
				asc.WithAppStoreVersionExperimentTreatmentLocalizationsInclude(includeValues),
				asc.WithAppStoreVersionExperimentTreatmentLocalizationsPlacementsLimit(placementsLimit),
			)
		},
	})
}

// newExperimentTreatmentLocalizationsGetCommand returns the treatment localizations get subcommand.
func newExperimentTreatmentLocalizationsGetCommand(d *shared.Deps) *ffcli.Command {
	var (
		includeValues   []string
		placementsLimit int
	)

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "treatment-localizations view",
		Name:        "view",
		ShortUsage:  "asc product-pages experiments treatments localizations view --localization-id \"LOCALIZATION_ID\"",
		ShortHelp:   "View a treatment localization by ID.",
		LongHelp: `View a treatment localization by ID.

Examples:
  asc product-pages experiments treatments localizations view --localization-id "LOCALIZATION_ID"
  asc product-pages experiments treatments localizations view --localization-id "LOCALIZATION_ID" --include placements --placements-limit 50`,
		IDFlag:             "localization-id",
		IDUsage:            "Treatment localization ID",
		IDType:             "appStoreVersionExperimentTreatmentLocalizations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "experiments treatments localizations view",
		Deps:               d,
		Flags: func(fs *flag.FlagSet) func() error {
			include := shared.BindLocalizationIncludeFlags(fs, "Include related resources", treatmentLocalizationIncludeList())
			return func() (err error) {
				includeValues, placementsLimit, err = include.Resolve()
				return err
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppStoreVersionExperimentTreatmentLocalization(ctx, id, asc.WithLocalizationReadInclude(includeValues), asc.WithLocalizationReadPlacementsLimit(placementsLimit))
		},
	})
}

// newExperimentTreatmentLocalizationsCreateCommand returns the treatment localizations create subcommand.
func newExperimentTreatmentLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	var locale string
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		FlagSetName: "treatment-localizations create",
		Name:        "create",
		ShortUsage:  "asc product-pages experiments treatments localizations create --treatment-id \"TREATMENT_ID\" --locale \"en-US\"",
		ShortHelp:   "Create a treatment localization.",
		LongHelp: `Create a treatment localization.

Examples:
  asc product-pages experiments treatments localizations create --treatment-id "TREATMENT_ID" --locale "en-US"`,
		ErrorPrefix: "experiments treatments localizations create",
		IDFlag:      "treatment-id",
		IDUsage:     "Treatment ID",
		IDType:      "appStoreVersionExperimentTreatments",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&locale, "locale", "Localization locale (e.g., en-US)"),
		},
		Deps: d,
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateAppStoreVersionExperimentTreatmentLocalization(ctx, id, locale)
		},
	})
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
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppStoreVersionExperimentTreatmentLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}

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

// newExperimentTreatmentsCommand returns the experiment treatments command group.
func newExperimentTreatmentsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("treatments", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "treatments",
		ShortUsage: "asc product-pages experiments treatments <subcommand> [flags]",
		ShortHelp:  "Manage experiment treatments.",
		LongHelp: `Manage experiment treatments.

Examples:
  asc product-pages experiments treatments list --experiment-id "EXPERIMENT_ID"
  asc product-pages experiments treatments create --experiment-id "EXPERIMENT_ID" --name "Variant A"
  asc product-pages experiments treatments delete --treatment-id "TREATMENT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newExperimentTreatmentsListCommand(d),
			newExperimentTreatmentsGetCommand(d),
			newExperimentTreatmentsCreateCommand(d),
			newExperimentTreatmentsUpdateCommand(d),
			newExperimentTreatmentsDeleteCommand(d),
			newExperimentTreatmentLocalizationsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newExperimentTreatmentsListCommand returns the treatments list subcommand.
func newExperimentTreatmentsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("experiment-treatments list", flag.ExitOnError)

	experimentID := shared.BindResourceIDFlag(fs, "experiment-id", "appStoreVersionExperiments", "Experiment ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)
	v2 := fs.Bool("v2", false, "Use v2 experiments endpoint")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages experiments treatments list --experiment-id \"EXPERIMENT_ID\" [flags]",
		ShortHelp:  "List experiment treatments.",
		LongHelp: `List experiment treatments.

Examples:
  asc product-pages experiments treatments list --experiment-id "EXPERIMENT_ID"
  asc product-pages experiments treatments list --experiment-id "EXPERIMENT_ID" --paginate
  asc product-pages experiments treatments list --experiment-id "EXPERIMENT_ID" --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return d.UsageErrorf("experiments treatments list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("experiments treatments list: %v", err)
			}

			trimmedID := strings.TrimSpace(*experimentID)
			if trimmedID == "" {
				return d.RequiredFlag("--experiment-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments treatments list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionExperimentTreatmentsOption{
				asc.Limit[asc.AppStoreVersionExperimentTreatmentsOption](*limit),
				asc.NextURL[asc.AppStoreVersionExperimentTreatmentsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppStoreVersionExperimentTreatmentsOption](productPagesMaxLimit))
				var firstPage *asc.AppStoreVersionExperimentTreatmentsResponse
				if *v2 {
					firstPage, err = client.GetAppStoreVersionExperimentTreatmentsV2(requestCtx, trimmedID, paginateOpts...)
				} else {
					firstPage, err = client.GetAppStoreVersionExperimentTreatments(requestCtx, trimmedID, paginateOpts...)
				}
				if err != nil {
					return fmt.Errorf("experiments treatments list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					if *v2 {
						return client.GetAppStoreVersionExperimentTreatmentsV2(ctx, trimmedID, asc.NextURL[asc.AppStoreVersionExperimentTreatmentsOption](nextURL))
					}
					return client.GetAppStoreVersionExperimentTreatments(ctx, trimmedID, asc.NextURL[asc.AppStoreVersionExperimentTreatmentsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("experiments treatments list: %w", err)
				}

				return d.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			var resp *asc.AppStoreVersionExperimentTreatmentsResponse
			if *v2 {
				resp, err = client.GetAppStoreVersionExperimentTreatmentsV2(requestCtx, trimmedID, opts...)
			} else {
				resp, err = client.GetAppStoreVersionExperimentTreatments(requestCtx, trimmedID, opts...)
			}
			if err != nil {
				return fmt.Errorf("experiments treatments list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentTreatmentsGetCommand returns the treatments get subcommand.
func newExperimentTreatmentsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("experiment-treatments view", flag.ExitOnError)

	treatmentID := shared.BindResourceIDFlag(fs, "treatment-id", "appStoreVersionExperimentTreatments", "Treatment ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc product-pages experiments treatments view --treatment-id \"TREATMENT_ID\"",
		ShortHelp:  "View a treatment by ID.",
		LongHelp: `View a treatment by ID.

Examples:
  asc product-pages experiments treatments view --treatment-id "TREATMENT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*treatmentID)
			if trimmedID == "" {
				return d.RequiredFlag("--treatment-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments treatments view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreVersionExperimentTreatment(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("experiments treatments view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentTreatmentsCreateCommand returns the treatments create subcommand.
func newExperimentTreatmentsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("experiment-treatments create", flag.ExitOnError)

	experimentID := shared.BindResourceIDFlag(fs, "experiment-id", "appStoreVersionExperiments", "Experiment ID")
	name := fs.String("name", "", "Treatment name")
	v2 := fs.Bool("v2", false, "Create a treatment for a v2 experiment")
	appIconName := fs.String("app-icon-name", "", "App icon asset name")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc product-pages experiments treatments create --experiment-id \"EXPERIMENT_ID\" --name \"NAME\"",
		ShortHelp:  "Create a treatment.",
		LongHelp: `Create a treatment. Use --v2 when the experiment was created with --v2.

Examples:
  asc product-pages experiments treatments create --experiment-id "EXPERIMENT_ID" --name "Variant A"
  asc product-pages experiments treatments create --experiment-id "EXPERIMENT_ID" --name "Variant A" --app-icon-name "Icon A"
  asc product-pages experiments treatments create --experiment-id "EXPERIMENT_ID" --name "Variant A" --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*experimentID)
			if trimmedID == "" {
				return d.RequiredFlag("--experiment-id")
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				return d.RequiredFlag("--name")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments treatments create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			var resp *asc.AppStoreVersionExperimentTreatmentResponse
			if *v2 {
				resp, err = client.CreateAppStoreVersionExperimentTreatmentV2(requestCtx, trimmedID, nameValue, strings.TrimSpace(*appIconName))
			} else {
				resp, err = client.CreateAppStoreVersionExperimentTreatment(requestCtx, trimmedID, nameValue, strings.TrimSpace(*appIconName))
			}
			if err != nil {
				return fmt.Errorf("experiments treatments create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentTreatmentsUpdateCommand returns the treatments update subcommand.
func newExperimentTreatmentsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("experiment-treatments update", flag.ExitOnError)

	treatmentID := shared.BindResourceIDFlag(fs, "treatment-id", "appStoreVersionExperimentTreatments", "Treatment ID")
	name := fs.String("name", "", "Update treatment name")
	appIconName := fs.String("app-icon-name", "", "Update app icon asset name")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc product-pages experiments treatments update --treatment-id \"TREATMENT_ID\" [--name \"NAME\"] [--app-icon-name \"NAME\"]",
		ShortHelp:  "Update a treatment.",
		LongHelp: `Update a treatment.

Examples:
  asc product-pages experiments treatments update --treatment-id "TREATMENT_ID" --name "Updated"
  asc product-pages experiments treatments update --treatment-id "TREATMENT_ID" --app-icon-name "Icon B"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*treatmentID)
			if trimmedID == "" {
				return d.RequiredFlag("--treatment-id")
			}

			nameValue := strings.TrimSpace(*name)
			appIconValue := strings.TrimSpace(*appIconName)
			if nameValue == "" && appIconValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --name or --app-icon-name is required")
				return shared.MissingRequiredUsageError("")
			}

			attrs := asc.AppStoreVersionExperimentTreatmentUpdateAttributes{}
			if nameValue != "" {
				attrs.Name = &nameValue
			}
			if appIconValue != "" {
				attrs.AppIconName = &appIconValue
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments treatments update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppStoreVersionExperimentTreatment(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("experiments treatments update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentTreatmentsDeleteCommand returns the treatments delete subcommand.
func newExperimentTreatmentsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
		FlagSetName: "experiment-treatments delete",
		Name:        "delete",
		ShortUsage:  "asc product-pages experiments treatments delete --treatment-id \"TREATMENT_ID\" --confirm",
		ShortHelp:   "Delete a treatment.",
		LongHelp: `Delete a treatment.

Examples:
  asc product-pages experiments treatments delete --treatment-id "TREATMENT_ID" --confirm`,
		IDFlag:             "treatment-id",
		IDUsage:            "Treatment ID",
		IDType:             "appStoreVersionExperimentTreatments",
		RequiredDiagnostic: true,
		ErrorPrefix:        "experiments treatments delete",
		DeleteErrorPrefix:  "experiments treatments delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppStoreVersionExperimentTreatment(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AppStoreVersionExperimentTreatmentDeleteResult{ID: id, Deleted: true}
		},
	})
}

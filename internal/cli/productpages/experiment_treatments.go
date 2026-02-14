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

// ExperimentTreatmentsCommand returns the experiment treatments command group.
func ExperimentTreatmentsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("treatments", flag.ExitOnError)

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
			ExperimentTreatmentsListCommand(),
			ExperimentTreatmentsGetCommand(),
			ExperimentTreatmentsCreateCommand(),
			ExperimentTreatmentsUpdateCommand(),
			ExperimentTreatmentsDeleteCommand(),
			ExperimentTreatmentLocalizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ExperimentTreatmentsListCommand returns the treatments list subcommand.
func ExperimentTreatmentsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiment-treatments list", flag.ExitOnError)

	experimentID := fs.String("experiment-id", "", "Experiment ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)
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
				return fmt.Errorf("experiments treatments list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("experiments treatments list: %w", err)
			}

			trimmedID := strings.TrimSpace(*experimentID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --experiment-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments treatments list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionExperimentTreatmentsOption{
				asc.WithAppStoreVersionExperimentTreatmentsLimit(*limit),
				asc.WithAppStoreVersionExperimentTreatmentsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppStoreVersionExperimentTreatmentsLimit(productPagesMaxLimit))
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
						return client.GetAppStoreVersionExperimentTreatmentsV2(ctx, trimmedID, asc.WithAppStoreVersionExperimentTreatmentsNextURL(nextURL))
					}
					return client.GetAppStoreVersionExperimentTreatments(ctx, trimmedID, asc.WithAppStoreVersionExperimentTreatmentsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("experiments treatments list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
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

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentTreatmentsGetCommand returns the treatments get subcommand.
func ExperimentTreatmentsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiment-treatments get", flag.ExitOnError)

	treatmentID := fs.String("treatment-id", "", "Treatment ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc product-pages experiments treatments get --treatment-id \"TREATMENT_ID\"",
		ShortHelp:  "Get a treatment by ID.",
		LongHelp: `Get a treatment by ID.

Examples:
  asc product-pages experiments treatments get --treatment-id "TREATMENT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*treatmentID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --treatment-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments treatments get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppStoreVersionExperimentTreatment(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("experiments treatments get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentTreatmentsCreateCommand returns the treatments create subcommand.
func ExperimentTreatmentsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiment-treatments create", flag.ExitOnError)

	experimentID := fs.String("experiment-id", "", "Experiment ID")
	name := fs.String("name", "", "Treatment name")
	appIconName := fs.String("app-icon-name", "", "App icon asset name")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc product-pages experiments treatments create --experiment-id \"EXPERIMENT_ID\" --name \"NAME\"",
		ShortHelp:  "Create a treatment.",
		LongHelp: `Create a treatment.

Examples:
  asc product-pages experiments treatments create --experiment-id "EXPERIMENT_ID" --name "Variant A"
  asc product-pages experiments treatments create --experiment-id "EXPERIMENT_ID" --name "Variant A" --app-icon-name "Icon A"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*experimentID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --experiment-id is required")
				return flag.ErrHelp
			}

			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments treatments create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppStoreVersionExperimentTreatment(requestCtx, trimmedID, nameValue, strings.TrimSpace(*appIconName))
			if err != nil {
				return fmt.Errorf("experiments treatments create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentTreatmentsUpdateCommand returns the treatments update subcommand.
func ExperimentTreatmentsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiment-treatments update", flag.ExitOnError)

	treatmentID := fs.String("treatment-id", "", "Treatment ID")
	name := fs.String("name", "", "Update treatment name")
	appIconName := fs.String("app-icon-name", "", "Update app icon asset name")
	output := shared.BindOutputFlags(fs)

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
				fmt.Fprintln(os.Stderr, "Error: --treatment-id is required")
				return flag.ErrHelp
			}

			nameValue := strings.TrimSpace(*name)
			appIconValue := strings.TrimSpace(*appIconName)
			if nameValue == "" && appIconValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name or --app-icon-name is required")
				return flag.ErrHelp
			}

			attrs := asc.AppStoreVersionExperimentTreatmentUpdateAttributes{}
			if nameValue != "" {
				attrs.Name = &nameValue
			}
			if appIconValue != "" {
				attrs.AppIconName = &appIconValue
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments treatments update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppStoreVersionExperimentTreatment(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("experiments treatments update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentTreatmentsDeleteCommand returns the treatments delete subcommand.
func ExperimentTreatmentsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiment-treatments delete", flag.ExitOnError)

	treatmentID := fs.String("treatment-id", "", "Treatment ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc product-pages experiments treatments delete --treatment-id \"TREATMENT_ID\" --confirm",
		ShortHelp:  "Delete a treatment.",
		LongHelp: `Delete a treatment.

Examples:
  asc product-pages experiments treatments delete --treatment-id "TREATMENT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*treatmentID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --treatment-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments treatments delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppStoreVersionExperimentTreatment(requestCtx, trimmedID); err != nil {
				return fmt.Errorf("experiments treatments delete: failed to delete: %w", err)
			}

			result := &asc.AppStoreVersionExperimentTreatmentDeleteResult{ID: trimmedID, Deleted: true}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

package productpages

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newExperimentsCommand returns the experiments command group.
func newExperimentsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("experiments", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "experiments",
		ShortUsage: "asc product-pages experiments <subcommand> [flags]",
		ShortHelp:  "Manage product page optimization experiments.",
		LongHelp: `Manage product page optimization experiments.

Examples:
  asc product-pages experiments list --version-id "VERSION_ID"
  asc product-pages experiments list --v2 --app "APP_ID"
  asc product-pages experiments create --version-id "VERSION_ID" --name "Icon Test" --traffic-proportion 25
  asc product-pages experiments create --v2 --app "APP_ID" --platform IOS --name "Icon Test" --traffic-proportion 25`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newExperimentsListCommand(d),
			newExperimentsGetCommand(d),
			newExperimentsCreateCommand(d),
			newExperimentsUpdateCommand(d),
			newExperimentsDeleteCommand(d),
			newExperimentTreatmentsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newExperimentsListCommand returns the experiments list subcommand.
func newExperimentsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("experiments list", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID (v1 experiments)")
	appID := fs.String("app", "", "App Store Connect app ID (v2 experiments)")
	state := fs.String("state", "", "Filter by state(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := d.BindOutputFlags(fs)
	v2 := fs.Bool("v2", false, "Use v2 experiments endpoint")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc product-pages experiments list [--version-id \"VERSION_ID\" | --v2 --app \"APP_ID\"] [flags]",
		ShortHelp:  "List product page optimization experiments.",
		LongHelp: `List product page optimization experiments.

Examples:
  asc product-pages experiments list --version-id "VERSION_ID"
  asc product-pages experiments list --v2 --app "APP_ID"
  asc product-pages experiments list --version-id "VERSION_ID" --state IN_REVIEW
  asc product-pages experiments list --v2 --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > productPagesMaxLimit) {
				return d.UsageErrorf("experiments list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return d.UsageErrorf("experiments list: %v", err)
			}
			if err := d.RejectNextFlagConflicts(fs, *next, "product-pages experiments list", "state"); err != nil {
				return err
			}

			stateValues, err := normalizeExperimentStates(shared.SplitCSVUpper(*state))
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err)
				return flag.ErrHelp
			}

			if *v2 {
				resolvedAppID := shared.ResolveAppID(*appID)
				if resolvedAppID == "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
					return shared.MissingRequiredUsageError("--app")
				}

				client, err := d.Client()
				if err != nil {
					return fmt.Errorf("experiments list: %w", err)
				}

				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()

				opts := []asc.AppStoreVersionExperimentsV2Option{
					asc.Limit[asc.AppStoreVersionExperimentsV2Option](*limit),
					asc.NextURL[asc.AppStoreVersionExperimentsV2Option](*next),
					asc.WithAppStoreVersionExperimentsV2State(stateValues),
				}

				if *paginate {
					paginateOpts := append(opts, asc.Limit[asc.AppStoreVersionExperimentsV2Option](productPagesMaxLimit))
					firstPage, err := client.GetAppStoreVersionExperimentsV2(requestCtx, resolvedAppID, paginateOpts...)
					if err != nil {
						return fmt.Errorf("experiments list: failed to fetch: %w", err)
					}

					paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetAppStoreVersionExperimentsV2(ctx, resolvedAppID, asc.NextURL[asc.AppStoreVersionExperimentsV2Option](nextURL))
					})
					if err != nil {
						return fmt.Errorf("experiments list: %w", err)
					}

					return d.PrintOutput(paginated, *output.Output, *output.Pretty)
				}

				resp, err := client.GetAppStoreVersionExperimentsV2(requestCtx, resolvedAppID, opts...)
				if err != nil {
					return fmt.Errorf("experiments list: failed to fetch: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			trimmedVersionID := strings.TrimSpace(*versionID)
			if trimmedVersionID == "" {
				return d.RequiredFlag("--version-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionExperimentsOption{
				asc.Limit[asc.AppStoreVersionExperimentsOption](*limit),
				asc.NextURL[asc.AppStoreVersionExperimentsOption](*next),
				asc.WithAppStoreVersionExperimentsState(stateValues),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppStoreVersionExperimentsOption](productPagesMaxLimit))
				firstPage, err := client.GetAppStoreVersionExperiments(requestCtx, trimmedVersionID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("experiments list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionExperiments(ctx, trimmedVersionID, asc.NextURL[asc.AppStoreVersionExperimentsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("experiments list: %w", err)
				}

				return d.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppStoreVersionExperiments(requestCtx, trimmedVersionID, opts...)
			if err != nil {
				return fmt.Errorf("experiments list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentsGetCommand returns the experiments view subcommand.
func newExperimentsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("experiments view", flag.ExitOnError)

	experimentID := shared.BindResourceIDFlag(fs, "experiment-id", "appStoreVersionExperiments", "Experiment ID")
	output := d.BindOutputFlags(fs)
	v2 := fs.Bool("v2", false, "Use v2 experiments endpoint")

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc product-pages experiments view --experiment-id \"EXPERIMENT_ID\" [--v2]",
		ShortHelp:  "View an experiment by ID.",
		LongHelp: `View an experiment by ID.

Examples:
  asc product-pages experiments view --experiment-id "EXPERIMENT_ID"
  asc product-pages experiments view --experiment-id "EXPERIMENT_ID" --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*experimentID)
			if trimmedID == "" {
				return d.RequiredFlag("--experiment-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if *v2 {
				resp, err := client.GetAppStoreVersionExperimentV2(requestCtx, trimmedID)
				if err != nil {
					return fmt.Errorf("experiments view: failed to fetch: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppStoreVersionExperiment(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("experiments view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentsCreateCommand returns the experiments create subcommand.
func newExperimentsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("experiments create", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID (v1 experiments)")
	appID := fs.String("app", "", "App Store Connect app ID (v2 experiments)")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS (v2 experiments)")
	name := fs.String("name", "", "Experiment name")
	trafficProportion := fs.String("traffic-proportion", "", "Traffic proportion (integer)")
	output := d.BindOutputFlags(fs)
	v2 := fs.Bool("v2", false, "Use v2 experiments endpoint")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc product-pages experiments create --name \"NAME\" --traffic-proportion 25 [--version-id \"VERSION_ID\" | --v2 --app \"APP_ID\" --platform IOS]",
		ShortHelp:  "Create an experiment.",
		LongHelp: `Create an experiment.

Examples:
  asc product-pages experiments create --version-id "VERSION_ID" --name "Icon Test" --traffic-proportion 25
  asc product-pages experiments create --v2 --app "APP_ID" --platform IOS --name "Icon Test" --traffic-proportion 25`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				return d.RequiredFlag("--name")
			}

			trafficValue, err := parseTrafficProportion(*trafficProportion)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err)
				return flag.ErrHelp
			}

			if *v2 {
				resolvedAppID := shared.ResolveAppID(*appID)
				if resolvedAppID == "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
					return shared.MissingRequiredUsageError("--app")
				}

				platformValue, err := shared.NormalizePlatform(*platform)
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err)
					return flag.ErrHelp
				}

				client, err := d.Client()
				if err != nil {
					return fmt.Errorf("experiments create: %w", err)
				}

				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()

				resp, err := client.CreateAppStoreVersionExperimentV2(requestCtx, resolvedAppID, platformValue, nameValue, trafficValue)
				if err != nil {
					return fmt.Errorf("experiments create: failed to create: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			trimmedVersionID := strings.TrimSpace(*versionID)
			if trimmedVersionID == "" {
				return d.RequiredFlag("--version-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateAppStoreVersionExperiment(requestCtx, trimmedVersionID, nameValue, trafficValue)
			if err != nil {
				return fmt.Errorf("experiments create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentsUpdateCommand returns the experiments update subcommand.
func newExperimentsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("experiments update", flag.ExitOnError)

	experimentID := shared.BindResourceIDFlag(fs, "experiment-id", "appStoreVersionExperiments", "Experiment ID")
	name := fs.String("name", "", "Update experiment name")
	trafficProportion := fs.String("traffic-proportion", "", "Update traffic proportion (integer)")
	var started shared.OptionalBool
	fs.Var(&started, "started", "Start or stop the experiment: true or false")
	output := d.BindOutputFlags(fs)
	v2 := fs.Bool("v2", false, "Use v2 experiments endpoint")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc product-pages experiments update --experiment-id \"EXPERIMENT_ID\" [--name \"NAME\"] [--traffic-proportion 25] [--started true|false] [--v2]",
		ShortHelp:  "Update an experiment.",
		LongHelp: `Update an experiment.

Examples:
  asc product-pages experiments update --experiment-id "EXPERIMENT_ID" --name "Updated"
  asc product-pages experiments update --experiment-id "EXPERIMENT_ID" --traffic-proportion 50
  asc product-pages experiments update --experiment-id "EXPERIMENT_ID" --started true
  asc product-pages experiments update --experiment-id "EXPERIMENT_ID" --v2 --name "Updated"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*experimentID)
			if trimmedID == "" {
				return d.RequiredFlag("--experiment-id")
			}

			attrsName := strings.TrimSpace(*name)
			var trafficPtr *int
			if strings.TrimSpace(*trafficProportion) != "" {
				value, err := parseTrafficProportion(*trafficProportion)
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err)
					return flag.ErrHelp
				}
				trafficPtr = &value
			}

			if attrsName == "" && trafficPtr == nil && !started.IsSet() {
				fmt.Fprintln(d.IO.ErrOut, "Error: --name, --traffic-proportion, or --started is required")
				return shared.MissingRequiredUsageError("")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if *v2 {
				attrs := asc.AppStoreVersionExperimentV2UpdateAttributes{}
				if attrsName != "" {
					attrs.Name = &attrsName
				}
				if trafficPtr != nil {
					attrs.TrafficProportion = trafficPtr
				}
				if started.IsSet() {
					value := started.Value()
					attrs.Started = &value
				}

				resp, err := client.UpdateAppStoreVersionExperimentV2(requestCtx, trimmedID, attrs)
				if err != nil {
					return fmt.Errorf("experiments update: failed to update: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			attrs := asc.AppStoreVersionExperimentUpdateAttributes{}
			if attrsName != "" {
				attrs.Name = &attrsName
			}
			if trafficPtr != nil {
				attrs.TrafficProportion = trafficPtr
			}
			if started.IsSet() {
				value := started.Value()
				attrs.Started = &value
			}

			resp, err := client.UpdateAppStoreVersionExperiment(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("experiments update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newExperimentsDeleteCommand returns the experiments delete subcommand.
func newExperimentsDeleteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("experiments delete", flag.ExitOnError)

	experimentID := shared.BindResourceIDFlag(fs, "experiment-id", "appStoreVersionExperiments", "Experiment ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := d.BindOutputFlags(fs)
	v2 := fs.Bool("v2", false, "Use v2 experiments endpoint")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc product-pages experiments delete --experiment-id \"EXPERIMENT_ID\" --confirm [--v2]",
		ShortHelp:  "Delete an experiment.",
		LongHelp: `Delete an experiment.

Examples:
  asc product-pages experiments delete --experiment-id "EXPERIMENT_ID" --confirm
  asc product-pages experiments delete --experiment-id "EXPERIMENT_ID" --confirm --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*experimentID)
			if trimmedID == "" {
				return d.RequiredFlag("--experiment-id")
			}
			if !*confirm {
				return d.RequiredFlag("--confirm")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("experiments delete: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if *v2 {
				if err := client.DeleteAppStoreVersionExperimentV2(requestCtx, trimmedID); err != nil {
					return fmt.Errorf("experiments delete: failed to delete: %w", err)
				}
			} else {
				if err := client.DeleteAppStoreVersionExperiment(requestCtx, trimmedID); err != nil {
					return fmt.Errorf("experiments delete: failed to delete: %w", err)
				}
			}

			result := &asc.AppStoreVersionExperimentDeleteResult{ID: trimmedID, Deleted: true}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

var experimentStateValues = map[string]struct{}{
	"PREPARE_FOR_SUBMISSION": {},
	"READY_FOR_REVIEW":       {},
	"WAITING_FOR_REVIEW":     {},
	"IN_REVIEW":              {},
	"ACCEPTED":               {},
	"APPROVED":               {},
	"REJECTED":               {},
	"COMPLETED":              {},
	"STOPPED":                {},
}

func normalizeExperimentStates(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	for _, value := range values {
		if _, ok := experimentStateValues[value]; !ok {
			return nil, fmt.Errorf("--state must be one of: %s", strings.Join(experimentStateList(), ", "))
		}
	}
	return values, nil
}

func experimentStateList() []string {
	return []string{
		"PREPARE_FOR_SUBMISSION",
		"READY_FOR_REVIEW",
		"WAITING_FOR_REVIEW",
		"IN_REVIEW",
		"ACCEPTED",
		"APPROVED",
		"REJECTED",
		"COMPLETED",
		"STOPPED",
	}
}

func parseTrafficProportion(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("--traffic-proportion is required")
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("--traffic-proportion must be an integer")
	}
	if value < 0 {
		return 0, fmt.Errorf("--traffic-proportion must be 0 or greater")
	}
	return value, nil
}

package productpages

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ExperimentsCommand returns the experiments command group.
func ExperimentsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiments", flag.ExitOnError)

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
			ExperimentsListCommand(),
			ExperimentsGetCommand(),
			ExperimentsCreateCommand(),
			ExperimentsUpdateCommand(),
			ExperimentsDeleteCommand(),
			ExperimentTreatmentsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ExperimentsListCommand returns the experiments list subcommand.
func ExperimentsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiments list", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (v1 experiments)")
	appID := fs.String("app", "", "App Store Connect app ID (v2 experiments)")
	state := fs.String("state", "", "Filter by state(s), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)
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
				return fmt.Errorf("experiments list: --limit must be between 1 and %d", productPagesMaxLimit)
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("experiments list: %w", err)
			}

			stateValues, err := normalizeExperimentStates(shared.SplitCSVUpper(*state))
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return flag.ErrHelp
			}

			if *v2 {
				resolvedAppID := shared.ResolveAppID(*appID)
				if resolvedAppID == "" {
					fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
					return flag.ErrHelp
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("experiments list: %w", err)
				}

				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				opts := []asc.AppStoreVersionExperimentsV2Option{
					asc.WithAppStoreVersionExperimentsV2Limit(*limit),
					asc.WithAppStoreVersionExperimentsV2NextURL(*next),
					asc.WithAppStoreVersionExperimentsV2State(stateValues),
				}

				if *paginate {
					paginateOpts := append(opts, asc.WithAppStoreVersionExperimentsV2Limit(productPagesMaxLimit))
					firstPage, err := client.GetAppStoreVersionExperimentsV2(requestCtx, resolvedAppID, paginateOpts...)
					if err != nil {
						return fmt.Errorf("experiments list: failed to fetch: %w", err)
					}

					paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetAppStoreVersionExperimentsV2(ctx, resolvedAppID, asc.WithAppStoreVersionExperimentsV2NextURL(nextURL))
					})
					if err != nil {
						return fmt.Errorf("experiments list: %w", err)
					}

					return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
				}

				resp, err := client.GetAppStoreVersionExperimentsV2(requestCtx, resolvedAppID, opts...)
				if err != nil {
					return fmt.Errorf("experiments list: failed to fetch: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			trimmedVersionID := strings.TrimSpace(*versionID)
			if trimmedVersionID == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppStoreVersionExperimentsOption{
				asc.WithAppStoreVersionExperimentsLimit(*limit),
				asc.WithAppStoreVersionExperimentsNextURL(*next),
				asc.WithAppStoreVersionExperimentsState(stateValues),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppStoreVersionExperimentsLimit(productPagesMaxLimit))
				firstPage, err := client.GetAppStoreVersionExperiments(requestCtx, trimmedVersionID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("experiments list: failed to fetch: %w", err)
				}

				paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionExperiments(ctx, trimmedVersionID, asc.WithAppStoreVersionExperimentsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("experiments list: %w", err)
				}

				return shared.PrintOutput(paginated, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppStoreVersionExperiments(requestCtx, trimmedVersionID, opts...)
			if err != nil {
				return fmt.Errorf("experiments list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentsGetCommand returns the experiments get subcommand.
func ExperimentsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiments get", flag.ExitOnError)

	experimentID := fs.String("experiment-id", "", "Experiment ID")
	output := shared.BindOutputFlags(fs)
	v2 := fs.Bool("v2", false, "Use v2 experiments endpoint")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc product-pages experiments get --experiment-id \"EXPERIMENT_ID\" [--v2]",
		ShortHelp:  "Get an experiment by ID.",
		LongHelp: `Get an experiment by ID.

Examples:
  asc product-pages experiments get --experiment-id "EXPERIMENT_ID"
  asc product-pages experiments get --experiment-id "EXPERIMENT_ID" --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*experimentID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --experiment-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *v2 {
				resp, err := client.GetAppStoreVersionExperimentV2(requestCtx, trimmedID)
				if err != nil {
					return fmt.Errorf("experiments get: failed to fetch: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppStoreVersionExperiment(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("experiments get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentsCreateCommand returns the experiments create subcommand.
func ExperimentsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiments create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "App Store version ID (v1 experiments)")
	appID := fs.String("app", "", "App Store Connect app ID (v2 experiments)")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS (v2 experiments)")
	name := fs.String("name", "", "Experiment name")
	trafficProportion := fs.String("traffic-proportion", "", "Traffic proportion (integer)")
	output := shared.BindOutputFlags(fs)
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
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			trafficValue, err := parseTrafficProportion(*trafficProportion)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return flag.ErrHelp
			}

			if *v2 {
				resolvedAppID := shared.ResolveAppID(*appID)
				if resolvedAppID == "" {
					fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
					return flag.ErrHelp
				}

				platformValue, err := shared.NormalizePlatform(*platform)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err)
					return flag.ErrHelp
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("experiments create: %w", err)
				}

				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				resp, err := client.CreateAppStoreVersionExperimentV2(requestCtx, resolvedAppID, platformValue, nameValue, trafficValue)
				if err != nil {
					return fmt.Errorf("experiments create: failed to create: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			trimmedVersionID := strings.TrimSpace(*versionID)
			if trimmedVersionID == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppStoreVersionExperiment(requestCtx, trimmedVersionID, nameValue, trafficValue)
			if err != nil {
				return fmt.Errorf("experiments create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentsUpdateCommand returns the experiments update subcommand.
func ExperimentsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiments update", flag.ExitOnError)

	experimentID := fs.String("experiment-id", "", "Experiment ID")
	name := fs.String("name", "", "Update experiment name")
	trafficProportion := fs.String("traffic-proportion", "", "Update traffic proportion (integer)")
	var started shared.OptionalBool
	fs.Var(&started, "started", "Start or stop the experiment: true or false")
	output := shared.BindOutputFlags(fs)
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
				fmt.Fprintln(os.Stderr, "Error: --experiment-id is required")
				return flag.ErrHelp
			}

			attrsName := strings.TrimSpace(*name)
			var trafficPtr *int
			if strings.TrimSpace(*trafficProportion) != "" {
				value, err := parseTrafficProportion(*trafficProportion)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err)
					return flag.ErrHelp
				}
				trafficPtr = &value
			}

			if attrsName == "" && trafficPtr == nil && !started.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: --name, --traffic-proportion, or --started is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
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

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ExperimentsDeleteCommand returns the experiments delete subcommand.
func ExperimentsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("experiments delete", flag.ExitOnError)

	experimentID := fs.String("experiment-id", "", "Experiment ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)
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
				fmt.Fprintln(os.Stderr, "Error: --experiment-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("experiments delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
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

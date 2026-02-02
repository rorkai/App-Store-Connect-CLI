package pricing

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

// PricingCommand returns the pricing command group.
func PricingCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "pricing",
		ShortUsage: "asc pricing <subcommand> [flags]",
		ShortHelp:  "Manage app pricing and availability.",
		LongHelp: `Manage app pricing and availability.

Examples:
  asc pricing territories list
  asc pricing price-points --app "123456789"
  asc pricing price-points --app "123456789" --territory "USA"
  asc pricing price-points get --price-point "PRICE_POINT_ID"
  asc pricing price-points equalizations --price-point "PRICE_POINT_ID"
  asc pricing schedule get --app "123456789"
  asc pricing schedule get --id "SCHEDULE_ID"
  asc pricing schedule create --app "123456789" --price-point "PRICE_POINT_ID" --base-territory "USA" --start-date "2024-03-01"
  asc pricing schedule manual-prices --schedule "SCHEDULE_ID"
  asc pricing schedule automatic-prices --schedule "SCHEDULE_ID"
  asc pricing availability get --app "123456789"
  asc pricing availability get --id "AVAILABILITY_ID"
  asc pricing availability set --app "123456789" --territory "USA,GBR,DEU" --available true
  asc pricing availability territory-availabilities --availability "AVAILABILITY_ID"`,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PricingTerritoriesCommand(),
			PricingPricePointsCommand(),
			PricingScheduleCommand(),
			PricingAvailabilityCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PricingTerritoriesCommand returns the territories subcommand group.
func PricingTerritoriesCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "territories",
		ShortUsage: "asc pricing territories <subcommand> [flags]",
		ShortHelp:  "List pricing territories.",
		LongHelp: `List pricing territories.

Examples:
  asc pricing territories list`,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PricingTerritoriesListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PricingTerritoriesListCommand returns the territories list subcommand.
func PricingTerritoriesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing territories list", flag.ExitOnError)

	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Next page URL from a previous response")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc pricing territories list [flags]",
		ShortHelp:  "List App Store Connect territories.",
		LongHelp: `List App Store Connect territories.

Examples:
  asc pricing territories list
  asc pricing territories list --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pricing territories list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("pricing territories list: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pricing territories list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.TerritoriesOption{
				asc.WithTerritoriesLimit(*limit),
				asc.WithTerritoriesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithTerritoriesLimit(200))
				firstPage, err := client.GetTerritories(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pricing territories list: failed to fetch: %w", err)
				}

				territories, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetTerritories(ctx, asc.WithTerritoriesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("pricing territories list: %w", err)
				}

				return printOutput(territories, *output, *pretty)
			}

			resp, err := client.GetTerritories(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("pricing territories list: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// PricingPricePointsCommand returns the price points command.
func PricingPricePointsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing price-points", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	territory := fs.String("territory", "", "Filter by territory (e.g., USA)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Next page URL from a previous response")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "price-points",
		ShortUsage: "asc pricing price-points [subcommand] [flags]",
		ShortHelp:  "List and inspect app price points.",
		LongHelp: `List app price points for an app.

Examples:
  asc pricing price-points --app "123456789"
  asc pricing price-points --app "123456789" --territory "USA"
  asc pricing price-points --app "123456789" --paginate
  asc pricing price-points get --price-point "PRICE_POINT_ID"
  asc pricing price-points equalizations --price-point "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PricingPricePointsGetCommand(),
			PricingPricePointsEqualizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pricing price-points: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("pricing price-points: %w", err)
			}

			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pricing price-points: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.PricePointsOption{
				asc.WithPricePointsLimit(*limit),
				asc.WithPricePointsNextURL(*next),
				asc.WithPricePointsTerritory(*territory),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithPricePointsLimit(200))
				firstPage, err := client.GetAppPricePoints(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pricing price-points: failed to fetch: %w", err)
				}

				points, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppPricePoints(ctx, resolvedAppID, asc.WithPricePointsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("pricing price-points: %w", err)
				}

				return printOutput(points, *output, *pretty)
			}

			points, err := client.GetAppPricePoints(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("pricing price-points: %w", err)
			}

			return printOutput(points, *output, *pretty)
		},
	}
}

// PricingPricePointsGetCommand returns the price point get subcommand.
func PricingPricePointsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing price-points get", flag.ExitOnError)

	pricePointID := fs.String("price-point", "", "App price point ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc pricing price-points get --price-point PRICE_POINT_ID",
		ShortHelp:  "Get a single app price point.",
		LongHelp: `Get a single app price point.

Examples:
  asc pricing price-points get --price-point "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedPricePointID := strings.TrimSpace(*pricePointID)
			if trimmedPricePointID == "" {
				fmt.Fprintln(os.Stderr, "Error: --price-point is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pricing price-points get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppPricePoint(requestCtx, trimmedPricePointID)
			if err != nil {
				return fmt.Errorf("pricing price-points get: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// PricingPricePointsEqualizationsCommand returns the price point equalizations subcommand.
func PricingPricePointsEqualizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing price-points equalizations", flag.ExitOnError)

	pricePointID := fs.String("price-point", "", "App price point ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "equalizations",
		ShortUsage: "asc pricing price-points equalizations --price-point PRICE_POINT_ID",
		ShortHelp:  "List equalized price points for a price point.",
		LongHelp: `List equalized price points for a price point.

Examples:
  asc pricing price-points equalizations --price-point "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedPricePointID := strings.TrimSpace(*pricePointID)
			if trimmedPricePointID == "" {
				fmt.Fprintln(os.Stderr, "Error: --price-point is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pricing price-points equalizations: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppPricePointEqualizations(requestCtx, trimmedPricePointID)
			if err != nil {
				return fmt.Errorf("pricing price-points equalizations: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// PricingScheduleCommand returns the pricing schedule command group.
func PricingScheduleCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "schedule",
		ShortUsage: "asc pricing schedule <subcommand> [flags]",
		ShortHelp:  "Manage app price schedules.",
		LongHelp: `Manage app price schedules.

Examples:
  asc pricing schedule get --app "123456789"
  asc pricing schedule get --id "SCHEDULE_ID"
  asc pricing schedule create --app "123456789" --price-point "PRICE_POINT_ID" --start-date "2024-03-01"
  asc pricing schedule manual-prices --schedule "SCHEDULE_ID"
  asc pricing schedule automatic-prices --schedule "SCHEDULE_ID"`,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PricingScheduleGetCommand(),
			PricingScheduleCreateCommand(),
			PricingScheduleManualPricesCommand(),
			PricingScheduleAutomaticPricesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PricingScheduleGetCommand returns the schedule get subcommand.
func PricingScheduleGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing schedule get", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	id := fs.String("id", "", "App price schedule ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc pricing schedule get --app \"APP_ID\" | asc pricing schedule get --id \"SCHEDULE_ID\"",
		ShortHelp:  "Get the current app price schedule.",
		LongHelp: `Get the current app price schedule.

Examples:
  asc pricing schedule get --app "123456789"
  asc pricing schedule get --id "SCHEDULE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			appValue := ""
			if idValue == "" {
				appValue = resolveAppID(*appID)
			}
			if idValue == "" && appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app or --id is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			if idValue != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(os.Stderr, "Error: --id and --app are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pricing schedule get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			var resp *asc.AppPriceScheduleResponse
			if idValue != "" {
				resp, err = client.GetAppPriceScheduleByID(requestCtx, idValue)
			} else {
				resp, err = client.GetAppPriceSchedule(requestCtx, appValue)
			}
			if err != nil {
				return fmt.Errorf("pricing schedule get: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// PricingScheduleCreateCommand returns the schedule create subcommand.
func PricingScheduleCreateCommand() *ffcli.Command {
	return shared.NewPricingSetCommand(shared.PricingSetCommandConfig{
		FlagSetName: "pricing schedule create",
		CommandName: "create",
		ShortUsage:  "asc pricing schedule create [flags]",
		ShortHelp:   "Create an app price schedule.",
		LongHelp: `Create an app price schedule.

Examples:
  asc pricing schedule create --app "123456789" --price-point "PRICE_POINT_ID" --base-territory "USA" --start-date "2024-03-01"`,
		ErrorPrefix:          "pricing schedule create",
		StartDateHelp:        "Start date (YYYY-MM-DD)",
		RequireBaseTerritory: true,
	})
}

// PricingScheduleManualPricesCommand returns the schedule manual-prices subcommand.
func PricingScheduleManualPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing schedule manual-prices", flag.ExitOnError)

	scheduleID := fs.String("schedule", "", "App price schedule ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "manual-prices",
		ShortUsage: "asc pricing schedule manual-prices --schedule SCHEDULE_ID",
		ShortHelp:  "List manual prices for a schedule.",
		LongHelp: `List manual prices for a schedule.

Examples:
  asc pricing schedule manual-prices --schedule "SCHEDULE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedScheduleID := strings.TrimSpace(*scheduleID)
			if trimmedScheduleID == "" {
				fmt.Fprintln(os.Stderr, "Error: --schedule is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pricing schedule manual-prices: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppPriceScheduleManualPrices(requestCtx, trimmedScheduleID)
			if err != nil {
				return fmt.Errorf("pricing schedule manual-prices: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// PricingScheduleAutomaticPricesCommand returns the schedule automatic-prices subcommand.
func PricingScheduleAutomaticPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing schedule automatic-prices", flag.ExitOnError)

	scheduleID := fs.String("schedule", "", "App price schedule ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "automatic-prices",
		ShortUsage: "asc pricing schedule automatic-prices --schedule SCHEDULE_ID",
		ShortHelp:  "List automatic prices for a schedule.",
		LongHelp: `List automatic prices for a schedule.

Examples:
  asc pricing schedule automatic-prices --schedule "SCHEDULE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedScheduleID := strings.TrimSpace(*scheduleID)
			if trimmedScheduleID == "" {
				fmt.Fprintln(os.Stderr, "Error: --schedule is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pricing schedule automatic-prices: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppPriceScheduleAutomaticPrices(requestCtx, trimmedScheduleID)
			if err != nil {
				return fmt.Errorf("pricing schedule automatic-prices: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// PricingAvailabilityCommand returns the availability command group.
func PricingAvailabilityCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "availability",
		ShortUsage: "asc pricing availability <subcommand> [flags]",
		ShortHelp:  "Manage app availability.",
		LongHelp: `Manage app availability.

Examples:
  asc pricing availability get --app "123456789"
  asc pricing availability get --id "AVAILABILITY_ID"
  asc pricing availability set --app "123456789" --territory "USA,GBR,DEU" --available true
  asc pricing availability territory-availabilities --availability "AVAILABILITY_ID"`,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PricingAvailabilityGetCommand(),
			PricingAvailabilityTerritoryAvailabilitiesCommand(),
			PricingAvailabilitySetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PricingAvailabilityGetCommand returns the availability get subcommand.
func PricingAvailabilityGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing availability get", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	id := fs.String("id", "", "App availability ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc pricing availability get --app \"APP_ID\" | asc pricing availability get --id \"AVAILABILITY_ID\"",
		ShortHelp:  "Get app availability.",
		LongHelp: `Get app availability.

Examples:
  asc pricing availability get --app "123456789"
  asc pricing availability get --id "AVAILABILITY_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			appValue := ""
			if idValue == "" {
				appValue = resolveAppID(*appID)
			}
			if idValue == "" && appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app or --id is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}
			if idValue != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(os.Stderr, "Error: --id and --app are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pricing availability get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			var resp *asc.AppAvailabilityV2Response
			if idValue != "" {
				resp, err = client.GetAppAvailabilityV2ByID(requestCtx, idValue)
			} else {
				resp, err = client.GetAppAvailabilityV2(requestCtx, appValue)
			}
			if err != nil {
				if idValue == "" && isAppAvailabilityMissing(err) {
					return fmt.Errorf("pricing availability get: app availability not found for app %q", appValue)
				}
				return fmt.Errorf("pricing availability get: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// PricingAvailabilityTerritoryAvailabilitiesCommand returns the availability territory-availabilities subcommand.
func PricingAvailabilityTerritoryAvailabilitiesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing availability territory-availabilities", flag.ExitOnError)

	availabilityID := fs.String("availability", "", "App availability ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "territory-availabilities",
		ShortUsage: "asc pricing availability territory-availabilities --availability AVAILABILITY_ID",
		ShortHelp:  "List territory availabilities for an app availability.",
		LongHelp: `List territory availabilities for an app availability.

Examples:
  asc pricing availability territory-availabilities --availability "AVAILABILITY_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAvailabilityID := strings.TrimSpace(*availabilityID)
			if trimmedAvailabilityID == "" {
				fmt.Fprintln(os.Stderr, "Error: --availability is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("pricing availability territory-availabilities: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetTerritoryAvailabilities(requestCtx, trimmedAvailabilityID)
			if err != nil {
				return fmt.Errorf("pricing availability territory-availabilities: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// PricingAvailabilitySetCommand returns the availability set subcommand.
func PricingAvailabilitySetCommand() *ffcli.Command {
	return shared.NewAvailabilitySetCommand(shared.AvailabilitySetCommandConfig{
		FlagSetName: "pricing availability set",
		CommandName: "set",
		ShortUsage:  "asc pricing availability set [flags]",
		ShortHelp:   "Set app availability for territories.",
		LongHelp: `Set app availability for territories.

Examples:
  asc pricing availability set --app "123456789" --territory "USA,GBR,DEU" --available true`,
		ErrorPrefix: "pricing availability set",
	})
}

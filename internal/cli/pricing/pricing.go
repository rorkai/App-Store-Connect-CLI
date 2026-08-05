package pricing

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var pricingAvailabilityClientFactory = shared.GetASCClient

// PricingCommand returns the pricing command group.
func PricingCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "pricing",
		ShortUsage: "asc pricing <subcommand> [flags]",
		ShortHelp:  "Manage app pricing and availability.",
		LongHelp: `Manage app pricing and availability.

Examples:
  asc pricing current --app "123456789"
  asc pricing territories list
  asc pricing price-points --app "123456789"
  asc pricing price-points --app "123456789" --territory "France"
  asc pricing price-points view --price-point "PRICE_POINT_ID"
  asc pricing price-points equalizations --price-point "PRICE_POINT_ID"
  asc pricing tiers --app "123456789" --territory "US"
  asc pricing schedule view --app "123456789"
  asc pricing schedule view --id "SCHEDULE_ID"
  asc pricing schedule create --app "123456789" --price-point "PRICE_POINT_ID" --base-territory "United States" --start-date "2024-03-01"
  asc pricing schedule create --app "123456789" --free --base-territory "US" --start-date "2024-03-01"
  asc pricing schedule manual-prices --schedule "SCHEDULE_ID"
  asc pricing schedule automatic-prices --schedule "SCHEDULE_ID"
  asc pricing availability view --app "123456789"
  asc pricing availability view --id "AVAILABILITY_ID"
  asc pricing availability create --app "123456789" --territory "USA,GBR,DEU" --available true --available-in-new-territories true
  asc pricing availability edit --app "123456789" --territory "US,France,DEU" --available true --available-in-new-territories true
  asc pricing availability edit --app "123456789" --all-territories --available true --available-in-new-territories true
  asc pricing availability territory-availabilities --availability "AVAILABILITY_ID"`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PricingCurrentCommand(),
			PricingTerritoriesCommand(),
			PricingPricePointsCommand(),
			PricingTiersCommand(),
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
		UsageFunc: shared.DefaultUsageFunc,
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
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc pricing territories list [flags]",
		ShortHelp:  "List territories in App Store Connect.",
		LongHelp: `List territories in App Store Connect.

Examples:
  asc pricing territories list
  asc pricing territories list --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pricing territories list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("pricing territories list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pricing territories list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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

				return shared.PrintOutput(territories, *output.Output, *output.Pretty)
			}

			resp, err := client.GetTerritories(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("pricing territories list: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PricingPricePointsCommand returns the price points command.
func PricingPricePointsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing price-points", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	territory := fs.String("territory", "", "Filter by territory (accepts alpha-2, alpha-3, or exact English country name)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Next page URL from a previous response")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "price-points",
		ShortUsage: "asc pricing price-points [subcommand] [flags]",
		ShortHelp:  "List and inspect app price points.",
		LongHelp: `List app price points for an app.

Examples:
  asc pricing price-points --app "123456789"
  asc pricing price-points --app "123456789" --territory "United States"
  asc pricing price-points --app "123456789" --paginate
  asc pricing price-points view --price-point "PRICE_POINT_ID"
  asc pricing price-points equalizations --price-point "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PricingPricePointsGetCommand(),
			PricingPricePointsEqualizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pricing price-points: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("pricing price-points: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pricing price-points: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			territoryID := strings.TrimSpace(*territory)
			if territoryID != "" {
				territoryID, err = ascterritory.Normalize(territoryID)
				if err != nil {
					return shared.UsageError(err.Error())
				}
			}

			opts := []asc.PricePointsOption{
				asc.WithPricePointsLimit(*limit),
				asc.WithPricePointsNextURL(*next),
				asc.WithPricePointsTerritory(territoryID),
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

				return shared.PrintOutput(points, *output.Output, *output.Pretty)
			}

			points, err := client.GetAppPricePoints(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("pricing price-points: %w", err)
			}

			return shared.PrintOutput(points, *output.Output, *output.Pretty)
		},
	}
}

// PricingPricePointsGetCommand returns the price point get subcommand.
func PricingPricePointsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing price-points view", flag.ExitOnError)

	pricePointID := fs.String("price-point", "", "App price point ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc pricing price-points view --price-point PRICE_POINT_ID",
		ShortHelp:  "View a single app price point.",
		LongHelp: `View a single app price point.

Examples:
  asc pricing price-points view --price-point "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedPricePointID := strings.TrimSpace(*pricePointID)
			if trimmedPricePointID == "" {
				fmt.Fprintln(os.Stderr, "Error: --price-point is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pricing price-points view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppPricePoint(requestCtx, trimmedPricePointID)
			if err != nil {
				return fmt.Errorf("pricing price-points view: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PricingPricePointsEqualizationsCommand returns the price point equalizations subcommand.
func PricingPricePointsEqualizationsCommand() *ffcli.Command {
	return shared.BuildPricePointEqualizationsCommand(shared.PricePointEqualizationsCommandConfig{
		FlagSetName: "pricing price-points equalizations",
		Name:        "equalizations",
		ShortUsage:  "asc pricing price-points equalizations --price-point PRICE_POINT_ID",
		BaseExample: `asc pricing price-points equalizations --price-point "PRICE_POINT_ID"`,
		Subject:     "a price point",
		ParentFlag:  "price-point",
		ParentUsage: "App price point ID",
		LimitMax:    200,
		ErrorPrefix: "pricing price-points equalizations",
		FetchPage: func(ctx context.Context, client *asc.Client, pricePointID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.PricePointsOption{
				asc.WithPricePointsLimit(limit),
				asc.WithPricePointsNextURL(next),
			}
			return client.GetAppPricePointEqualizations(ctx, pricePointID, opts...)
		},
	})
}

// PricingScheduleCommand returns the pricing schedule command group.
func PricingScheduleCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "schedule",
		ShortUsage: "asc pricing schedule <subcommand> [flags]",
		ShortHelp:  "Manage app price schedules.",
		LongHelp: `Manage app price schedules.

Examples:
  asc pricing schedule view --app "123456789"
  asc pricing schedule view --id "SCHEDULE_ID"
  asc pricing schedule create --app "123456789" --price-point "PRICE_POINT_ID" --start-date "2024-03-01"
  asc pricing schedule create --app "123456789" --free --base-territory "US" --start-date "2024-03-01"
  asc pricing schedule manual-prices --schedule "SCHEDULE_ID"
  asc pricing schedule automatic-prices --schedule "SCHEDULE_ID"`,
		UsageFunc: shared.DefaultUsageFunc,
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
	fs := flag.NewFlagSet("pricing schedule view", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	id := fs.String("id", "", "App price schedule ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc pricing schedule view --app \"APP_ID\" | asc pricing schedule view --id \"SCHEDULE_ID\"",
		ShortHelp:  "View the current app price schedule.",
		LongHelp: `View the current app price schedule.

Examples:
  asc pricing schedule view --app "123456789"
  asc pricing schedule view --id "SCHEDULE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			appValue := ""
			if idValue == "" {
				appValue = shared.ResolveAppID(*appID)
			}
			if idValue == "" && appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app or --id is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}
			if idValue != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(os.Stderr, "Error: --id and --app are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pricing schedule view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var resp *asc.AppPriceScheduleResponse
			if idValue != "" {
				resp, err = client.GetAppPriceScheduleByID(requestCtx, idValue)
			} else {
				resp, err = client.GetAppPriceSchedule(requestCtx, appValue)
			}
			if err != nil {
				return fmt.Errorf("pricing schedule view: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
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
  asc pricing schedule create --app "123456789" --price-point "PRICE_POINT_ID" --base-territory "United States" --start-date "2024-03-01"
  asc pricing schedule create --app "123456789" --free --base-territory "US" --start-date "2024-03-01"`,
		ErrorPrefix:          "pricing schedule create",
		StartDateHelp:        "Start date (YYYY-MM-DD)",
		RequireBaseTerritory: true,
	})
}

// PricingScheduleManualPricesCommand returns the schedule manual-prices subcommand.
func PricingScheduleManualPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing schedule manual-prices", flag.ExitOnError)

	scheduleID := fs.String("schedule", "", "App price schedule ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	resolved := fs.Bool("resolved", false, "Return the current effective price per territory")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "manual-prices",
		ShortUsage: "asc pricing schedule manual-prices --schedule SCHEDULE_ID",
		ShortHelp:  "List manual prices for a schedule.",
		LongHelp: `List manual prices for a schedule.

Examples:
  asc pricing schedule manual-prices --schedule "SCHEDULE_ID"
  asc pricing schedule manual-prices --schedule "SCHEDULE_ID" --paginate
  asc pricing schedule manual-prices --schedule "SCHEDULE_ID" --resolved`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				fmt.Fprintln(os.Stderr, "Error: pricing schedule manual-prices: --limit must be between 1 and 200")
				return flag.ErrHelp
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("pricing schedule manual-prices: %w", err)
			}
			if *resolved && strings.TrimSpace(*next) != "" {
				fmt.Fprintln(os.Stderr, "Error: --resolved cannot be combined with --next")
				return flag.ErrHelp
			}

			trimmedScheduleID := strings.TrimSpace(*scheduleID)
			if trimmedScheduleID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --schedule is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pricing schedule manual-prices: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *resolved {
				resp, err := fetchResolvedAppSchedulePrices(requestCtx, client, trimmedScheduleID, "manual", *limit, *next, time.Now().UTC())
				if err != nil {
					return fmt.Errorf("pricing schedule manual-prices: failed to resolve: %w", err)
				}
				return shared.PrintResolvedPrices(resp, *output.Output, *output.Pretty)
			}

			opts := []asc.AppPriceSchedulePricesOption{
				asc.WithAppPriceSchedulePricesLimit(*limit),
				asc.WithAppPriceSchedulePricesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppPriceSchedulePricesLimit(200))
				firstPage, err := client.GetAppPriceScheduleManualPrices(requestCtx, trimmedScheduleID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pricing schedule manual-prices: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppPriceScheduleManualPrices(ctx, trimmedScheduleID, asc.WithAppPriceSchedulePricesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("pricing schedule manual-prices: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppPriceScheduleManualPrices(requestCtx, trimmedScheduleID, opts...)
			if err != nil {
				return fmt.Errorf("pricing schedule manual-prices: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PricingScheduleAutomaticPricesCommand returns the schedule automatic-prices subcommand.
func PricingScheduleAutomaticPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing schedule automatic-prices", flag.ExitOnError)

	scheduleID := fs.String("schedule", "", "App price schedule ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	resolved := fs.Bool("resolved", false, "Return the current effective price per territory")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "automatic-prices",
		ShortUsage: "asc pricing schedule automatic-prices --schedule SCHEDULE_ID",
		ShortHelp:  "List automatic prices for a schedule.",
		LongHelp: `List automatic prices for a schedule.

Examples:
  asc pricing schedule automatic-prices --schedule "SCHEDULE_ID"
  asc pricing schedule automatic-prices --schedule "SCHEDULE_ID" --paginate
  asc pricing schedule automatic-prices --schedule "SCHEDULE_ID" --resolved`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				fmt.Fprintln(os.Stderr, "Error: pricing schedule automatic-prices: --limit must be between 1 and 200")
				return flag.ErrHelp
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("pricing schedule automatic-prices: %w", err)
			}
			if *resolved && strings.TrimSpace(*next) != "" {
				fmt.Fprintln(os.Stderr, "Error: --resolved cannot be combined with --next")
				return flag.ErrHelp
			}

			trimmedScheduleID := strings.TrimSpace(*scheduleID)
			if trimmedScheduleID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --schedule is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pricing schedule automatic-prices: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *resolved {
				resp, err := fetchResolvedAppSchedulePrices(requestCtx, client, trimmedScheduleID, "automatic", *limit, *next, time.Now().UTC())
				if err != nil {
					return fmt.Errorf("pricing schedule automatic-prices: failed to resolve: %w", err)
				}
				return shared.PrintResolvedPrices(resp, *output.Output, *output.Pretty)
			}

			opts := []asc.AppPriceSchedulePricesOption{
				asc.WithAppPriceSchedulePricesLimit(*limit),
				asc.WithAppPriceSchedulePricesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppPriceSchedulePricesLimit(200))
				firstPage, err := client.GetAppPriceScheduleAutomaticPrices(requestCtx, trimmedScheduleID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pricing schedule automatic-prices: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppPriceScheduleAutomaticPrices(ctx, trimmedScheduleID, asc.WithAppPriceSchedulePricesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("pricing schedule automatic-prices: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppPriceScheduleAutomaticPrices(requestCtx, trimmedScheduleID, opts...)
			if err != nil {
				return fmt.Errorf("pricing schedule automatic-prices: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
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
  asc pricing availability view --app "123456789"
  asc pricing availability view --id "AVAILABILITY_ID"
  asc pricing availability create --app "123456789" --territory "USA,GBR,DEU" --available true --available-in-new-territories true
  asc pricing availability edit --app "123456789" --territory "US,France,DEU" --available true --available-in-new-territories true
  asc pricing availability edit --app "123456789" --all-territories --available true --available-in-new-territories true
  asc pricing availability territory-availabilities --availability "AVAILABILITY_ID"`,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PricingAvailabilityGetCommand(),
			PricingAvailabilityCreateCommand(),
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
	fs := flag.NewFlagSet("pricing availability view", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	id := fs.String("id", "", "App availability ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc pricing availability view --app \"APP_ID\" | asc pricing availability view --id \"AVAILABILITY_ID\"",
		ShortHelp:  "View app availability.",
		LongHelp: `View app availability.

Examples:
  asc pricing availability view --app "123456789"
  asc pricing availability view --id "AVAILABILITY_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			appValue := ""
			if idValue == "" {
				appValue = shared.ResolveAppID(*appID)
			}
			if idValue == "" && appValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --app or --id is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}
			if idValue != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(os.Stderr, "Error: --id and --app are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := pricingAvailabilityClientFactory()
			if err != nil {
				return fmt.Errorf("pricing availability view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var resp *asc.AppAvailabilityV2Response
			if idValue != "" {
				resp, err = client.GetAppAvailabilityV2ByID(requestCtx, idValue)
			} else {
				resp, err = client.GetAppAvailabilityV2(requestCtx, appValue)
			}
			if err != nil {
				if idValue == "" && shared.IsAppAvailabilityMissing(err) {
					return shared.NewErrorWithCause(
						fmt.Errorf("pricing availability view: app availability not found for app %q: %w", appValue, asc.ErrNotFound),
						err,
					)
				}
				return fmt.Errorf("pricing availability view: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// PricingAvailabilityTerritoryAvailabilitiesCommand returns the availability territory-availabilities subcommand.
func PricingAvailabilityTerritoryAvailabilitiesCommand() *ffcli.Command {
	cmd := shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "pricing availability territory-availabilities",
		Name:        "territory-availabilities",
		ShortUsage:  "asc pricing availability territory-availabilities --availability AVAILABILITY_ID [--limit N] [--next URL] [--paginate]",
		ShortHelp:   "List territory availabilities for an app availability.",
		LongHelp: `List territory availabilities for an app availability.

Examples:
  asc pricing availability territory-availabilities --availability "AVAILABILITY_ID"
  asc pricing availability territory-availabilities --availability "AVAILABILITY_ID" --limit 175
  asc pricing availability territory-availabilities --availability "AVAILABILITY_ID" --paginate
  asc pricing availability territory-availabilities --next "NEXT_URL"`,
		ParentFlag:  "availability",
		ParentUsage: "App availability ID",
		LimitMax:    200,
		ErrorPrefix: "pricing availability territory-availabilities",
		FetchPage: func(ctx context.Context, client *asc.Client, availabilityID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := make([]asc.TerritoryAvailabilitiesOption, 0, 2)
			if limit > 0 {
				opts = append(opts, asc.WithTerritoryAvailabilitiesLimit(limit))
			}
			if strings.TrimSpace(next) != "" {
				opts = append(opts, asc.WithTerritoryAvailabilitiesNextURL(next))
			}
			return client.GetTerritoryAvailabilities(ctx, availabilityID, opts...)
		},
	})

	originalExec := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		err := originalExec(ctx, args)
		if err == nil || errors.Is(err, flag.ErrHelp) {
			return err
		}
		if isPricingAvailabilityTerritoryAvailabilitiesUsageError(err) {
			return shared.UsageError(err.Error())
		}
		return err
	}

	return cmd
}

func isPricingAvailabilityTerritoryAvailabilitiesUsageError(err error) bool {
	message := err.Error()
	return strings.HasPrefix(message, "pricing availability territory-availabilities: --limit must be between 1 and ") ||
		strings.HasPrefix(message, "pricing availability territory-availabilities: --next ")
}

// PricingAvailabilityCreateCommand returns the availability create subcommand.
func PricingAvailabilityCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing availability create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	var availableInNewTerritories shared.OptionalBool
	fs.Var(&availableInNewTerritories, "available-in-new-territories", "Automatically make app available in new territories: true or false (required)")
	territory := fs.String("territory", "", "Territory inputs (comma-separated; accepts alpha-2, alpha-3, or exact English country names, e.g., US,USA,France)")
	var available shared.OptionalBool
	fs.Var(&available, "available", "Set availability for specified territories: true or false (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc pricing availability create --app \"APP_ID\" --territory \"USA,GBR\" --available true --available-in-new-territories true",
		ShortHelp:  "Initialize app availability for territories.",
		LongHelp: `Initialize app availability for territories.

Creates the initial app availability record and its territory availability
entries through the public App Store Connect API. Apple can reject this
bootstrap request even when it matches the published schema. If that happens,
the command fails without treating availability as configured and explains the
authenticated web-session or App Store Connect fallback. Once created, use
"asc pricing availability edit" to update the record.

Examples:
  asc pricing availability create --app "123456789" --territory "USA,GBR,DEU" --available true --available-in-new-territories true
  asc pricing availability create --app "123456789" --territory "USA,GBR,DEU" --available false --available-in-new-territories false`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("pricing availability create does not accept positional arguments")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}
			if !availableInNewTerritories.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: --available-in-new-territories is required (true or false)")
				return shared.MissingRequiredUsageError()
			}

			territories, err := shared.NormalizeASCTerritoryCSV(*territory)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if len(territories) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --territory must include at least one value")
				return shared.MissingRequiredUsageError()
			}
			if !available.IsSet() {
				fmt.Fprintln(os.Stderr, "Error: --available is required (true or false)")
				return shared.MissingRequiredUsageError()
			}

			client, err := pricingAvailabilityClientFactory()
			if err != nil {
				return fmt.Errorf("pricing availability create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			availableInNewTerritoriesValue := availableInNewTerritories.Value()
			availableValue := available.Value()
			territoryAvailabilities := make([]asc.TerritoryAvailabilityCreate, 0, len(territories))
			seenTerritories := make(map[string]struct{}, len(territories))
			for _, territoryID := range territories {
				if _, exists := seenTerritories[territoryID]; exists {
					continue
				}
				seenTerritories[territoryID] = struct{}{}
				territoryAvailabilities = append(territoryAvailabilities, asc.TerritoryAvailabilityCreate{
					TerritoryID: territoryID,
					Available:   availableValue,
				})
			}

			resp, err := client.CreateAppAvailabilityV2(requestCtx, resolvedAppID, asc.AppAvailabilityV2CreateAttributes{
				AvailableInNewTerritories: &availableInNewTerritoriesValue,
				TerritoryAvailabilities:   territoryAvailabilities,
			})
			if err != nil {
				if isAvailabilityBootstrapRelationshipRejection(err) {
					return fmt.Errorf(
						"pricing availability create: Apple rejected the initial availability request through the public API; availability was not configured. Authenticate a web session with \"asc web auth login --apple-id EMAIL\", then retry with \"asc web apps availability create\", or configure Pricing and Availability in App Store Connect: %w",
						err,
					)
				}
				return fmt.Errorf("pricing availability create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func isAvailabilityBootstrapRelationshipRejection(err error) bool {
	var apiErr *asc.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "ENTITY_ERROR.RELATIONSHIP.INVALID" {
		return false
	}

	detail := apiErr.Detail
	return strings.Contains(detail, "territoryAvailabilities.territory") &&
		strings.Contains(detail, "expects an included resource with type 'territories'") &&
		strings.Contains(detail, "no matching resource was included")
}

// PricingAvailabilitySetCommand returns the availability edit subcommand.
func PricingAvailabilitySetCommand() *ffcli.Command {
	return shared.NewAvailabilitySetCommand(shared.AvailabilitySetCommandConfig{
		FlagSetName: "pricing availability edit",
		CommandName: "edit",
		ShortUsage:  "asc pricing availability edit [flags]",
		ShortHelp:   "Edit app availability for territories.",
		LongHelp: `Edit app availability for territories.

Examples:
  asc pricing availability edit --app "123456789" --territory "US,France,DEU" --available true --available-in-new-territories true
  asc pricing availability edit --app "123456789" --all-territories --available true --available-in-new-territories true

Note:
  This command only updates an existing app availability. If the app has no
  availability record yet, use "asc pricing availability create" first. If
  Apple rejects public-API bootstrap, authenticate with
  "asc web auth login --apple-id EMAIL" and use
  "asc web apps availability create", or configure Pricing and Availability in
  App Store Connect.`,
		ErrorPrefix:                      "pricing availability edit",
		IncludeAvailableInNewTerritories: true,
	})
}

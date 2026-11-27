package pricing

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewPricingCommand returns the pricing command group.
func NewPricingCommand(d *shared.Deps) *ffcli.Command {
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
  asc pricing ppp --app "123456789" --base-price "4.99" --dry-run
  asc pricing schedule view --app "123456789"
  asc pricing schedule view --id "SCHEDULE_ID"
  asc pricing schedule create --app "123456789" --price-point "PRICE_POINT_ID" --base-territory "United States" --start-date "YYYY-MM-DD"
  asc pricing schedule create --app "123456789" --free --base-territory "US" --start-date "YYYY-MM-DD"
  asc pricing schedule manual-prices --schedule "SCHEDULE_ID"
  asc pricing schedule automatic-prices --schedule "SCHEDULE_ID"
  asc pricing availability view --app "123456789"
  asc pricing availability view --id "AVAILABILITY_ID"
  asc pricing availability create --app "123456789" --territory "USA,GBR,DEU" --available true --available-in-new-territories true
  asc pricing availability create --app "123456789" --all-territories --available true --available-in-new-territories true
  asc pricing availability edit --app "123456789" --territory "US,France,DEU" --available true
  asc pricing availability edit --app "123456789" --all-territories --available true
  asc pricing availability platforms --app "123456789"
  asc pricing availability remove-from-sale --app "123456789" --confirm
  asc pricing availability territory-availabilities --availability "AVAILABILITY_ID"`,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPricingCurrentCommand(d),
			newPricingTerritoriesCommand(d),
			newPricingPricePointsCommand(d),
			newPricingTiersCommand(d),
			newPricingScheduleCommand(d),
			newPricingPPPCommand(d),
			newPricingAvailabilityCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newPricingTerritoriesCommand returns the territories subcommand group.
func newPricingTerritoriesCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "territories",
		ShortUsage: "asc pricing territories <subcommand> [flags]",
		ShortHelp:  "List pricing territories.",
		LongHelp: `List pricing territories.

Examples:
  asc pricing territories list`,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPricingTerritoriesListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newPricingTerritoriesListCommand returns the territories list subcommand.
func newPricingTerritoriesListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing territories list", flag.ExitOnError)

	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc pricing territories list [flags]",
		ShortHelp:  "List territories in App Store Connect.",
		LongHelp: `List territories in App Store Connect.

Examples:
  asc pricing territories list
  asc pricing territories list --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "pricing territories list", 200); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pricing territories list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.TerritoriesOption{
				asc.Limit[asc.TerritoriesOption](*limit),
				asc.NextURL[asc.TerritoriesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.TerritoriesOption](200))
				firstPage, err := client.GetTerritories(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pricing territories list: failed to fetch: %w", err)
				}

				territories, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetTerritories(ctx, asc.NextURL[asc.TerritoriesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("pricing territories list: %w", err)
				}

				return d.PrintOutput(territories, *output.Output, *output.Pretty)
			}

			resp, err := client.GetTerritories(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("pricing territories list: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPricingPricePointsCommand returns the price points command.
func newPricingPricePointsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing price-points", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	territory := fs.String("territory", "", "Filter by territories, comma-separated (accepts alpha-2, alpha-3, or exact English country name)")
	include := fs.String("include", "", "Relationships to include (comma-separated): territory")
	includedFields := shared.BindIncludedFieldFlags(fs, map[string]string{"territory": "territories"})
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "price-points",
		ShortUsage: "asc pricing price-points [subcommand] [flags]",
		ShortHelp:  "List and inspect app price points.",
		LongHelp: `List app price points for an app.

--territory accepts several territories. Use --include territory to return each
price point's territory and currency.

Examples:
  asc pricing price-points --app "123456789"
  asc pricing price-points --app "123456789" --territory "United States"
  asc pricing price-points --app "123456789" --territory "DEU,GBR" --include territory --paginate
  asc pricing price-points --app "123456789" --paginate
  asc pricing price-points view --price-point "PRICE_POINT_ID"
  asc pricing price-points equalizations --price-point "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPricingPricePointsGetCommand(d),
			newPricingPricePointsEqualizationsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "pricing price-points", 200, "territory", "include"); err != nil {
				return err
			}
			includes := shared.SplitCSV(*include)
			for _, value := range includes {
				if value != "territory" {
					return d.UsageError("--include must be: territory")
				}
			}
			included, err := includedFields.ResolveWith(d, "pricing price-points", *next, includes)
			if err != nil {
				return err
			}

			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pricing price-points: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			territoryIDs, err := shared.NormalizeASCTerritoryCSV(*territory)
			if err != nil {
				return d.UsageError(err.Error())
			}

			opts := []asc.PricePointsOption{
				asc.Limit[asc.PricePointsOption](*limit),
				asc.NextURL[asc.PricePointsOption](*next),
				asc.WithPricePointsTerritory(strings.Join(territoryIDs, ",")),
				asc.WithPricePointsInclude(includes),
				asc.WithIncludedFields[asc.PricePointsOption](included),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.PricePointsOption](200))
				firstPage, err := client.GetAppPricePoints(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pricing price-points: failed to fetch: %w", err)
				}

				points, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppPricePoints(ctx, resolvedAppID, asc.NextURL[asc.PricePointsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("pricing price-points: %w", err)
				}

				return d.PrintOutput(points, *output.Output, *output.Pretty)
			}

			points, err := client.GetAppPricePoints(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("pricing price-points: %w", err)
			}

			return d.PrintOutput(points, *output.Output, *output.Pretty)
		},
	}
}

// newPricingPricePointsGetCommand returns the price point get subcommand.
func newPricingPricePointsGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing price-points view", flag.ExitOnError)

	pricePointID := shared.BindResourceIDFlag(fs, "price-point", "appPricePoints", "App price point ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc pricing price-points view --price-point PRICE_POINT_ID",
		ShortHelp:  "View a single app price point.",
		LongHelp: `View a single app price point.

Examples:
  asc pricing price-points view --price-point "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedPricePointID := strings.TrimSpace(*pricePointID)
			if trimmedPricePointID == "" {
				return d.RequiredFlag("--price-point")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pricing price-points view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetAppPricePoint(requestCtx, trimmedPricePointID)
			if err != nil {
				return fmt.Errorf("pricing price-points view: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPricingPricePointsEqualizationsCommand returns the price point equalizations subcommand.
func newPricingPricePointsEqualizationsCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPricePointEqualizationsCommand(shared.PricePointEqualizationsCommandConfig{
		Deps:        d,
		FlagSetName: "pricing price-points equalizations",
		Name:        "equalizations",
		ShortUsage:  "asc pricing price-points equalizations --price-point PRICE_POINT_ID",
		BaseExample: `asc pricing price-points equalizations --price-point "PRICE_POINT_ID"`,
		Subject:     "a price point",
		ParentFlag:  "price-point",
		ParentUsage: "App price point ID",
		ParentType:  "appPricePoints",
		LimitMax:    200,
		ErrorPrefix: "pricing price-points equalizations",
		FetchPage: func(ctx context.Context, client *asc.Client, pricePointID string, limit int, next string, include []string, included asc.IncludedFields) (asc.PaginatedResponse, error) {
			opts := []asc.PricePointsOption{
				asc.Limit[asc.PricePointsOption](limit),
				asc.NextURL[asc.PricePointsOption](next),
				asc.WithPricePointsInclude(include),
				asc.WithIncludedFields[asc.PricePointsOption](included),
			}
			return client.GetAppPricePointEqualizations(ctx, pricePointID, opts...)
		},
	})
}

// newPricingScheduleCommand returns the pricing schedule command group.
func newPricingScheduleCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "schedule",
		ShortUsage: "asc pricing schedule <subcommand> [flags]",
		ShortHelp:  "Manage app price schedules.",
		LongHelp: `Manage app price schedules.

Examples:
  asc pricing schedule view --app "123456789"
  asc pricing schedule view --id "SCHEDULE_ID"
  asc pricing schedule create --app "123456789" --price-point "PRICE_POINT_ID" --base-territory "United States" --start-date "YYYY-MM-DD"
  asc pricing schedule create --app "123456789" --free --base-territory "US" --start-date "YYYY-MM-DD"
  asc pricing schedule manual-prices --schedule "SCHEDULE_ID"
  asc pricing schedule automatic-prices --schedule "SCHEDULE_ID"`,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPricingScheduleGetCommand(d),
			newPricingScheduleCreateCommand(d),
			newPricingScheduleEditCommand(d),
			newPricingScheduleManualPricesCommand(d),
			newPricingScheduleAutomaticPricesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newPricingScheduleGetCommand returns the schedule get subcommand.
func newPricingScheduleGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing schedule view", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	id := shared.BindResourceIDFlag(fs, "id", "appPriceSchedules", "App price schedule ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc pricing schedule view --app \"APP_ID\" | asc pricing schedule view --id \"SCHEDULE_ID\"",
		ShortHelp:  "View the current app price schedule.",
		LongHelp: `View the current app price schedule.

Examples:
  asc pricing schedule view --app "123456789"
  asc pricing schedule view --id "SCHEDULE_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			appValue := ""
			if idValue == "" {
				appValue = d.ResolveAppID(*appID)
			}
			if idValue == "" && appValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app or --id is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("")
			}
			if idValue != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --id and --app are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pricing schedule view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPricingScheduleCreateCommand returns the schedule create subcommand.
func newPricingScheduleCreateCommand(d *shared.Deps) *ffcli.Command {
	return shared.NewPricingSetCommand(shared.PricingSetCommandConfig{
		Deps:        d,
		FlagSetName: "pricing schedule create",
		CommandName: "create",
		ShortUsage:  "asc pricing schedule create [flags]",
		ShortHelp:   "Create an app price schedule.",
		LongHelp: `Create an app price schedule.

This replaces the app's whole price schedule: App Store Connect drops existing
scheduled prices and manual prices in other territories, which fall back to
automatic equalization. Run asc pricing schedule manual-prices to see them first.

--start-date defaults to today's date in US Pacific time when omitted, because
App Store Connect uses that date as today; the chosen date is printed on
stderr. Apple requires the start date to be today or later.

--end-date makes the price temporary and requires --return-price-point and
--confirm. A price stops applying on its end date, in US Pacific time, and the
return price takes over. The return price must be a base territory price point
from asc pricing price-points. With a future --start-date, the return price also
applies from today until the start date, so pass the current price to keep it.

Examples:
  asc pricing schedule create --app "123456789" --price-point "PRICE_POINT_ID" --base-territory "United States" --start-date "YYYY-MM-DD"
  asc pricing schedule create --app "123456789" --price-point "PRICE_POINT_ID" --base-territory "United States"
  asc pricing schedule create --app "123456789" --free --base-territory "US" --start-date "YYYY-MM-DD"
  asc pricing schedule create --app "123456789" --price-point "SALE_PRICE_POINT_ID" --base-territory "US" --start-date "YYYY-MM-DD" --end-date "YYYY-MM-DD" --return-price-point "REGULAR_PRICE_POINT_ID" --confirm`,
		ErrorPrefix:           "pricing schedule create",
		StartDateHelp:         "Start date (YYYY-MM-DD, default: today in US Pacific time; Apple requires today or later)",
		StartDateDefaultToday: true,
		RequireBaseTerritory:  true,
		TemporaryPrice:        true,
	})
}

// newPricingScheduleManualPricesCommand returns the schedule manual-prices subcommand.
func newPricingScheduleManualPricesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing schedule manual-prices", flag.ExitOnError)

	scheduleID := shared.BindResourceIDFlag(fs, "schedule", "appPriceSchedules", "App price schedule ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	resolved := fs.Bool("resolved", false, "Return the current effective price per territory")
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "pricing schedule manual-prices", 200); err != nil {
				return err
			}
			if *resolved && strings.TrimSpace(*next) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --resolved cannot be combined with --next")
				return flag.ErrHelp
			}

			trimmedScheduleID := strings.TrimSpace(*scheduleID)
			if trimmedScheduleID == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--schedule")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pricing schedule manual-prices: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if *resolved {
				resp, err := fetchResolvedAppSchedulePrices(requestCtx, client, trimmedScheduleID, "manual", *limit, *next, d.Now(), false)
				if err != nil {
					return fmt.Errorf("pricing schedule manual-prices: failed to resolve: %w", err)
				}
				return d.PrintResolvedPrices(resp, *output.Output, *output.Pretty)
			}

			opts := []asc.AppPriceSchedulePricesOption{
				asc.Limit[asc.AppPriceSchedulePricesOption](*limit),
				asc.NextURL[asc.AppPriceSchedulePricesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppPriceSchedulePricesOption](200))
				firstPage, err := client.GetAppPriceScheduleManualPrices(requestCtx, trimmedScheduleID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pricing schedule manual-prices: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppPriceScheduleManualPrices(ctx, trimmedScheduleID, asc.NextURL[asc.AppPriceSchedulePricesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("pricing schedule manual-prices: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppPriceScheduleManualPrices(requestCtx, trimmedScheduleID, opts...)
			if err != nil {
				return fmt.Errorf("pricing schedule manual-prices: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPricingScheduleAutomaticPricesCommand returns the schedule automatic-prices subcommand.
func newPricingScheduleAutomaticPricesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing schedule automatic-prices", flag.ExitOnError)

	scheduleID := shared.BindResourceIDFlag(fs, "schedule", "appPriceSchedules", "App price schedule ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	resolved := fs.Bool("resolved", false, "Return the current effective price per territory")
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "pricing schedule automatic-prices", 200); err != nil {
				return err
			}
			if *resolved && strings.TrimSpace(*next) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --resolved cannot be combined with --next")
				return flag.ErrHelp
			}

			trimmedScheduleID := strings.TrimSpace(*scheduleID)
			if trimmedScheduleID == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--schedule")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pricing schedule automatic-prices: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if *resolved {
				resp, err := fetchResolvedAppSchedulePrices(requestCtx, client, trimmedScheduleID, "automatic", *limit, *next, d.Now(), false)
				if err != nil {
					return fmt.Errorf("pricing schedule automatic-prices: failed to resolve: %w", err)
				}
				return d.PrintResolvedPrices(resp, *output.Output, *output.Pretty)
			}

			opts := []asc.AppPriceSchedulePricesOption{
				asc.Limit[asc.AppPriceSchedulePricesOption](*limit),
				asc.NextURL[asc.AppPriceSchedulePricesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppPriceSchedulePricesOption](200))
				firstPage, err := client.GetAppPriceScheduleAutomaticPrices(requestCtx, trimmedScheduleID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pricing schedule automatic-prices: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppPriceScheduleAutomaticPrices(ctx, trimmedScheduleID, asc.NextURL[asc.AppPriceSchedulePricesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("pricing schedule automatic-prices: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppPriceScheduleAutomaticPrices(requestCtx, trimmedScheduleID, opts...)
			if err != nil {
				return fmt.Errorf("pricing schedule automatic-prices: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newPricingAvailabilityCommand returns the availability command group.
func newPricingAvailabilityCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "availability",
		ShortUsage: "asc pricing availability <subcommand> [flags]",
		ShortHelp:  "Manage app availability.",
		LongHelp: `Manage app availability.

Examples:
  asc pricing availability view --app "123456789"
  asc pricing availability view --id "AVAILABILITY_ID"
  asc pricing availability create --app "123456789" --territory "USA,GBR,DEU" --available true --available-in-new-territories true
  asc pricing availability create --app "123456789" --all-territories --available true --available-in-new-territories true
  asc pricing availability edit --app "123456789" --territory "US,France,DEU" --available true
  asc pricing availability edit --app "123456789" --all-territories --available true
  asc pricing availability platforms --app "123456789"
  asc pricing availability remove-from-sale --app "123456789" --confirm
  asc pricing availability territory-availabilities --availability "AVAILABILITY_ID"`,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPricingAvailabilityGetCommand(d),
			newPricingAvailabilityCreateCommand(d),
			newPricingAvailabilityTerritoryAvailabilitiesCommand(d),
			newPricingAvailabilitySetCommand(d),
			newPricingAvailabilityPlatformsCommand(d),
			newPricingAvailabilityRemoveFromSaleCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newPricingAvailabilityGetCommand returns the availability view subcommand.
func newPricingAvailabilityGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.NewAvailabilityViewCommand(d, "pricing availability")
}

const territoryAvailabilitiesLimitMax = 200

// newPricingAvailabilityTerritoryAvailabilitiesCommand returns the availability territory-availabilities subcommand.
func newPricingAvailabilityTerritoryAvailabilitiesCommand(d *shared.Deps) *ffcli.Command {
	cmd := shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:        d,
		FlagSetName: "pricing availability territory-availabilities",
		Name:        "territory-availabilities",
		ShortUsage:  "asc pricing availability territory-availabilities --app APP_ID | --availability AVAILABILITY_ID [--limit N] [--next URL] [--paginate]",
		ShortHelp:   "List territory availabilities for an app availability.",
		LongHelp: `List territory availabilities for an app availability.

Examples:
  asc pricing availability territory-availabilities --app "123456789"
  asc pricing availability territory-availabilities --availability "AVAILABILITY_ID"
  asc pricing availability territory-availabilities --availability "AVAILABILITY_ID" --limit 175
  asc pricing availability territory-availabilities --availability "AVAILABILITY_ID" --paginate
  asc pricing availability territory-availabilities --next "NEXT_URL"`,
		ParentFlag:       "availability",
		ParentUsage:      "App availability ID",
		ParentType:       "appAvailabilities",
		LimitMax:         territoryAvailabilitiesLimitMax,
		FetchErrorPrefix: "pricing availability territory-availabilities",
		FetchPage: func(ctx context.Context, client *asc.Client, availabilityID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := make([]asc.TerritoryAvailabilitiesOption, 0, 2)
			if limit > 0 {
				opts = append(opts, asc.Limit[asc.TerritoryAvailabilitiesOption](limit))
			}
			if strings.TrimSpace(next) != "" {
				opts = append(opts, asc.NextURL[asc.TerritoryAvailabilitiesOption](next))
			}
			return client.GetTerritoryAvailabilities(ctx, availabilityID, opts...)
		},
	})

	appID := cmd.FlagSet.String("app", "", "App Store Connect app ID (or ASC_APP_ID); lists its app availability's territories")

	originalExec := cmd.Exec
	cmd.Exec = func(ctx context.Context, args []string) error {
		if err := d.ValidatePaginationFlags(ctx, cmd.FlagSet, "pricing availability territory-availabilities", territoryAvailabilitiesLimitMax); err != nil {
			return err
		}
		if err := resolveTerritoryAvailabilitiesAvailability(ctx, d, cmd.FlagSet, *appID); err != nil {
			return err
		}
		err := originalExec(ctx, args)
		if err == nil || errors.Is(err, flag.ErrHelp) {
			return err
		}
		if isPricingAvailabilityTerritoryAvailabilitiesUsageError(err) {
			return d.UsageError(err.Error())
		}
		return err
	}

	return cmd
}

// resolveTerritoryAvailabilitiesAvailability sets --availability from the
// app's v2 availability when the caller names an app instead.
func resolveTerritoryAvailabilitiesAvailability(ctx context.Context, d *shared.Deps, fs *flag.FlagSet, appFlag string) error {
	availabilityID := strings.TrimSpace(fs.Lookup("availability").Value.String())
	next := strings.TrimSpace(fs.Lookup("next").Value.String())
	explicitApp := strings.TrimSpace(appFlag) != ""
	if explicitApp && availabilityID != "" {
		return d.UsageError("--app and --availability are mutually exclusive")
	}
	if availabilityID != "" || next != "" {
		return nil
	}
	appID := d.ResolveAppID(appFlag)
	if appID == "" {
		return d.UsageError("--app or --availability is required (or set ASC_APP_ID)")
	}

	client, err := d.Client()
	if err != nil {
		return fmt.Errorf("pricing availability territory-availabilities: %w", err)
	}
	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()

	resp, err := client.GetAppAvailabilityV2(requestCtx, appID)
	if err != nil {
		if shared.IsAppAvailabilityMissing(err) {
			return shared.NewErrorWithCause(
				fmt.Errorf("pricing availability territory-availabilities: app availability not found for app %q: %w", appID, asc.ErrNotFound),
				err,
			)
		}
		return fmt.Errorf("pricing availability territory-availabilities: %w", err)
	}
	resolvedID := strings.TrimSpace(resp.Data.ID)
	if resolvedID == "" {
		return fmt.Errorf("pricing availability territory-availabilities: app availability not found for app %q: %w", appID, asc.ErrNotFound)
	}
	return fs.Set("availability", resolvedID)
}

func isPricingAvailabilityTerritoryAvailabilitiesUsageError(err error) bool {
	message := err.Error()
	return strings.HasPrefix(message, "pricing availability territory-availabilities: --limit must be between 1 and ") ||
		strings.HasPrefix(message, "pricing availability territory-availabilities: --next ")
}

// newPricingAvailabilityCreateCommand returns the availability create subcommand.
func newPricingAvailabilityCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing availability create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	var availableInNewTerritories shared.OptionalBool
	fs.Var(&availableInNewTerritories, "available-in-new-territories", "Automatically make app available in new territories: true or false (required)")
	territory := fs.String("territory", "", "Territory inputs (comma-separated; accepts alpha-2, alpha-3, or exact English country names, e.g., US,USA,France; required unless --all-territories)")
	allTerritories := fs.Bool("all-territories", false, "Apply --available to every territory in Apple's current territory catalog (mutually exclusive with --territory)")
	var available shared.OptionalBool
	fs.Var(&available, "available", "Set availability for specified territories: true or false (required)")
	releaseDate := fs.String("release-date", "", "Release date for the selected territories (YYYY-MM-DD); requires --available true")
	var preOrderEnabled shared.OptionalBool
	fs.Var(&preOrderEnabled, "pre-order-enabled", "Set pre-order enabled for the selected territories: true or false; requires --available true")
	ifExists := shared.BindIfExistsFlag(fs, shared.IfExistsSkip, shared.IfExistsUpdate)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc pricing availability create --app \"APP_ID\" (--territory \"USA,GBR\" | --all-territories) --available true --available-in-new-territories true",
		ShortHelp:  "Initialize app availability for territories.",
		LongHelp: `Initialize app availability for territories.

Creates the initial app availability record through the public App Store Connect
API. Every current territory is included: selected territories use --available,
and unselected territories are initialized as unavailable. --all-territories
selects every territory in Apple's current territory catalog instead of a
--territory list. Once created, use "asc pricing availability edit" to update
the record.

--release-date and --pre-order-enabled set those attributes on the selected
territories in the same request, so pre-order setup needs no per-territory
follow-up. Both require --available true and neither works with --if-exists
update; use "asc pre-orders enable" or "asc pre-orders update" on an existing
record.

Examples:
  asc pricing availability create --app "123456789" --territory "USA,GBR,DEU" --available true --available-in-new-territories true
  asc pricing availability create --app "123456789" --all-territories --available true --available-in-new-territories true
  asc pricing availability create --app "123456789" --territory "USA,GBR,DEU" --available true --available-in-new-territories true --release-date "2026-12-01" --pre-order-enabled true
  asc pricing availability create --app "123456789" --territory "USA,GBR,DEU" --available false --available-in-new-territories false
  asc pricing availability create --app "123456789" --territory "USA,GBR,DEU" --available true --available-in-new-territories true --if-exists skip

--if-exists controls what happens when App Store Connect answers 409 because
the app already has an availability record. fail (default) returns the error.
skip reads the existing record back, prints it, and exits 0 without changing
it. update applies --territory (or, with --all-territories, every territory in
the existing record) and --available to the existing record through the same
path as "asc pricing availability edit"; Apple exposes no update for
availableInNewTerritories, so on update that flag is only verified against the
existing policy. Any other 409, including Apple's rejection of public-API
bootstrap, keeps failing.`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}
			if !availableInNewTerritories.IsSet() {
				fmt.Fprintln(d.IO.ErrOut, "Error: --available-in-new-territories is required (true or false)")
				return shared.MissingRequiredUsageError("--available-in-new-territories")
			}

			territoryProvided := false
			releaseDateProvided := false
			fs.Visit(func(f *flag.Flag) {
				switch f.Name {
				case "territory":
					territoryProvided = true
				case "release-date":
					releaseDateProvided = true
				}
			})
			if territoryProvided && *allTerritories {
				fmt.Fprintln(d.IO.ErrOut, "Error: --territory and --all-territories are mutually exclusive")
				return shared.WithDiagnostic(shared.InvalidValueUsageError("--all-territories"), shared.DiagnosticConflictingInput, "--all-territories")
			}
			if !territoryProvided && !*allTerritories {
				fmt.Fprintln(d.IO.ErrOut, "Error: --territory or --all-territories is required")
				return shared.MissingRequiredUsageError("--territory")
			}
			var territories []string
			if !*allTerritories {
				normalizedTerritories, err := shared.NormalizeASCTerritoryCSV(*territory)
				if err != nil {
					return d.UsageError(err.Error())
				}
				if len(normalizedTerritories) == 0 {
					fmt.Fprintln(d.IO.ErrOut, "Error: --territory must include at least one value")
					return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticInvalidInput, "--territory")
				}
				territories = normalizedTerritories
			}
			if !available.IsSet() {
				fmt.Fprintln(d.IO.ErrOut, "Error: --available is required (true or false)")
				return shared.MissingRequiredUsageError("--available")
			}

			ifExistsMode, err := d.ParseIfExistsMode(*ifExists, shared.IfExistsSkip, shared.IfExistsUpdate)
			if err != nil {
				return err
			}

			selection := asc.TerritoryAvailabilityCreate{Available: available.Value()}
			if releaseDateProvided || preOrderEnabled.IsSet() {
				if !available.Value() {
					return d.UsageError("--release-date and --pre-order-enabled require --available true")
				}
				if ifExistsMode == shared.IfExistsUpdate {
					return d.UsageError("--release-date and --pre-order-enabled cannot be used with --if-exists update; use \"asc pre-orders enable\" or \"asc pre-orders update\" on the existing record")
				}
			}
			if releaseDateProvided {
				selection.ReleaseDate, err = shared.NormalizeDate(*releaseDate, "--release-date")
				if err != nil {
					return d.UsageError(err.Error())
				}
			}
			if preOrderEnabled.IsSet() {
				v := preOrderEnabled.Value()
				if !v && releaseDateProvided {
					return d.UsageError("--release-date cannot be set when disabling pre-orders (releaseDate must be null)")
				}
				selection.PreOrderEnabled = &v
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pricing availability create: %w", err)
			}

			requestCtx, cancel := d.AvailabilityTimeout(ctx, *allTerritories)
			defer cancel()

			availableInNewTerritoriesValue := availableInNewTerritories.Value()
			territoryAvailabilities, err := initialTerritoryAvailabilities(requestCtx, client, territories, *allTerritories, selection)
			if err != nil {
				return fmt.Errorf("pricing availability create: %w", err)
			}

			resp, err := client.CreateAppAvailabilityV2(requestCtx, resolvedAppID, asc.AppAvailabilityV2CreateAttributes{
				AvailableInNewTerritories: &availableInNewTerritoriesValue,
				TerritoryAvailabilities:   territoryAvailabilities,
			})
			if err != nil {
				// Apple answers the bootstrap rejection with the same 409 code as
				// the existence conflict, so it is classified first and keeps its
				// own remediation. Nothing was created, so a read-back would find
				// nothing anyway.
				if isAvailabilityBootstrapRelationshipRejection(err) {
					return fmt.Errorf(
						"pricing availability create: Apple rejected the initial availability request through the public API; availability was not configured. Authenticate a web session with \"asc web auth login --apple-id EMAIL\", then retry with \"asc web apps availability create\", or configure Pricing and Availability in App Store Connect: %w",
						err,
					)
				}
				existing, handled, resolveErr := shared.ResolveIfExistsConflict(ifExistsMode, err, availabilityCreateExistsCodes, func() (*asc.AppAvailabilityV2Response, bool, error) {
					return findExistingAppAvailability(requestCtx, client, resolvedAppID)
				})
				if resolveErr != nil {
					return fmt.Errorf("pricing availability create: %w", resolveErr)
				}
				if !handled {
					return fmt.Errorf("pricing availability create: %w", err)
				}
				resp = existing
				outcome := "left unchanged"
				if ifExistsMode == shared.IfExistsUpdate {
					updated, changedTerritories, updateErr := d.ApplyTerritoryAvailabilityUpdate(requestCtx, client, shared.TerritoryAvailabilityUpdateRequest{
						AppID:                             resolvedAppID,
						Territories:                       territories,
						AllTerritories:                    *allTerritories,
						Available:                         selection.Available,
						ExpectedAvailableInNewTerritories: &availableInNewTerritoriesValue,
						ErrorPrefix:                       "pricing availability create",
					})
					if updateErr != nil {
						return updateErr
					}
					resp = updated
					// An update that changed no territory left the record as
					// it was, so the diagnostic must not claim an update.
					outcome = "every requested territory already matched; left unchanged"
					if changedTerritories > 0 {
						outcome = "updated it in place"
					}
				}
				fmt.Fprintf(d.IO.ErrOut, "pricing availability create: app %s already has availability %s; %s (--if-exists %s)\n",
					resolvedAppID, existing.Data.ID, outcome, ifExistsMode)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
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

// newPricingAvailabilitySetCommand returns the availability edit subcommand.
func newPricingAvailabilitySetCommand(d *shared.Deps) *ffcli.Command {
	return shared.NewAvailabilitySetCommand(shared.AvailabilitySetCommandConfig{
		Deps:        d,
		FlagSetName: "pricing availability edit",
		CommandName: "edit",
		ShortUsage:  "asc pricing availability edit [flags]",
		ShortHelp:   "Edit app availability for territories.",
		LongHelp: `Edit app availability for territories.

Examples:
  asc pricing availability edit --app "123456789" --territory "US,France,DEU" --available true
  asc pricing availability edit --app "123456789" --all-territories --available true

Note:
  This command only updates an existing app availability. If the app has no
  availability record yet, use "asc pricing availability create" first.
  If --available-in-new-territories is supplied, it verifies the existing
  policy; Apple does not expose an update operation for that setting. If
  Apple rejects public-API bootstrap, authenticate with
  "asc web auth login --apple-id EMAIL" and use
  "asc web apps availability create", or configure Pricing and Availability in
  App Store Connect.`,
		ErrorPrefix:                      "pricing availability edit",
		IncludeAvailableInNewTerritories: true,
	})
}

// newPricingAvailabilityPlatformsCommand returns the platforms subcommand.
func newPricingAvailabilityPlatformsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing availability platforms", flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "platforms",
		ShortUsage: "asc pricing availability platforms --app \"APP_ID\"",
		ShortHelp:  "Summarize each platform's App Store listing.",
		LongHelp: `Summarize each platform's App Store listing.

Shows one row per platform: the live listing when one exists, otherwise the
newest version and its state. Availability is app-wide — every platform
listing shares one availability record — so removing an app from sale removes
every live platform at once. Use this command to preview that blast radius
before "asc pricing availability remove-from-sale".

Examples:
  asc pricing availability platforms --app "123456789"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("pricing availability platforms: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			listings, err := shared.FetchAvailabilityPlatformListings(requestCtx, client, resolvedAppID)
			if err != nil {
				return fmt.Errorf("pricing availability platforms: %w", err)
			}

			result := &asc.AvailabilityPlatformsResult{AppID: resolvedAppID, Platforms: listings}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newPricingAvailabilityRemoveFromSaleCommand returns the remove-from-sale subcommand.
func newPricingAvailabilityRemoveFromSaleCommand(d *shared.Deps) *ffcli.Command {
	return shared.NewAvailabilityRemoveFromSaleCommand(shared.AvailabilityRemoveFromSaleCommandConfig{
		ClientFactory: d.Client,
		Deps:          d,
	})
}

// availabilityCreateExistsCodes lists the Apple 409 codes accepted as "this app
// already has an appAvailability" on POST /v2/appAvailabilities. Apple rejects
// the duplicate on the app relationship. The same code also carries Apple's
// public-API bootstrap rejection, which is classified before this list and
// keeps its own remediation; the read-back is what finally proves existence,
// and STATE_ERROR.* keeps failing.
var availabilityCreateExistsCodes = []string{
	"ENTITY_ERROR.RELATIONSHIP.INVALID",
	"ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS",
}

// findExistingAppAvailability reads back the availability record a 409 conflict
// referred to. It reports found=false when the app has no record so the caller
// can surface the original conflict.
func findExistingAppAvailability(ctx context.Context, client *asc.Client, appID string) (*asc.AppAvailabilityV2Response, bool, error) {
	resp, err := client.GetAppAvailabilityV2(ctx, appID)
	if err != nil {
		if shared.IsAppAvailabilityMissing(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if strings.TrimSpace(resp.Data.ID) == "" {
		return nil, false, nil
	}
	return resp, true, nil
}

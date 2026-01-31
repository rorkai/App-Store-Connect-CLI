package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// IAPPriceSchedulesCommand returns the price schedules command group.
func IAPPriceSchedulesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-schedules", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "price-schedules",
		ShortUsage: "asc iap price-schedules <subcommand> [flags]",
		ShortHelp:  "Manage in-app purchase price schedules.",
		LongHelp: `Manage in-app purchase price schedules.

Examples:
  asc iap price-schedules get --iap-id "IAP_ID"
  asc iap price-schedules create --iap-id "IAP_ID" --base-territory "USA" --prices "PRICE_POINT_ID:2024-03-01"
  asc iap price-schedules manual-prices --schedule-id "SCHEDULE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPPriceSchedulesGetCommand(),
			IAPPriceSchedulesCreateCommand(),
			IAPPriceSchedulesManualPricesCommand(),
			IAPPriceSchedulesAutomaticPricesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPPriceSchedulesGetCommand returns the price schedules get subcommand.
func IAPPriceSchedulesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-schedules get", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	scheduleID := fs.String("schedule-id", "", "Price schedule ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc iap price-schedules get --iap-id \"IAP_ID\"",
		ShortHelp:  "Get in-app purchase price schedule.",
		LongHelp: `Get in-app purchase price schedule.

Examples:
  asc iap price-schedules get --iap-id "IAP_ID"
  asc iap price-schedules get --schedule-id "SCHEDULE_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			scheduleValue := strings.TrimSpace(*scheduleID)
			if iapValue == "" && scheduleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id or --schedule-id is required")
				return flag.ErrHelp
			}
			if iapValue != "" && scheduleValue != "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id and --schedule-id are mutually exclusive")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap price-schedules get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if scheduleValue != "" {
				resp, err := client.GetInAppPurchasePriceScheduleByID(requestCtx, scheduleValue)
				if err != nil {
					return fmt.Errorf("iap price-schedules get: failed to fetch: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetInAppPurchasePriceSchedule(requestCtx, iapValue)
			if err != nil {
				return fmt.Errorf("iap price-schedules get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// IAPPriceSchedulesCreateCommand returns the price schedules create subcommand.
func IAPPriceSchedulesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-schedules create", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	baseTerritory := fs.String("base-territory", "", "Base territory ID (e.g., USA)")
	prices := fs.String("prices", "", "Manual prices: PRICE_POINT_ID[:START_DATE[:END_DATE]] entries")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc iap price-schedules create --iap-id \"IAP_ID\" --base-territory \"USA\" --prices \"PRICE_POINT_ID:2024-03-01\"",
		ShortHelp:  "Create an in-app purchase price schedule.",
		LongHelp: `Create an in-app purchase price schedule.

Examples:
  asc iap price-schedules create --iap-id "IAP_ID" --base-territory "USA" --prices "PRICE_POINT_ID:2024-03-01"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return flag.ErrHelp
			}
			baseTerritoryValue := strings.TrimSpace(*baseTerritory)
			if baseTerritoryValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --base-territory is required")
				return flag.ErrHelp
			}

			priceEntries, err := parsePriceSchedulePrices(*prices)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}
			if len(priceEntries) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --prices is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap price-schedules create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateInAppPurchasePriceSchedule(requestCtx, iapValue, asc.InAppPurchasePriceScheduleCreateAttributes{
				BaseTerritoryID: baseTerritoryValue,
				Prices:          priceEntries,
			})
			if err != nil {
				return fmt.Errorf("iap price-schedules create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// IAPPriceSchedulesManualPricesCommand returns the price schedules manual prices subcommand.
func IAPPriceSchedulesManualPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-schedules manual-prices", flag.ExitOnError)

	scheduleID := fs.String("schedule-id", "", "Price schedule ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "manual-prices",
		ShortUsage: "asc iap price-schedules manual-prices --schedule-id \"SCHEDULE_ID\"",
		ShortHelp:  "List manual prices for an in-app purchase price schedule.",
		LongHelp: `List manual prices for an in-app purchase price schedule.

Examples:
  asc iap price-schedules manual-prices --schedule-id "SCHEDULE_ID"
  asc iap price-schedules manual-prices --schedule-id "SCHEDULE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap price-schedules manual-prices: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("iap price-schedules manual-prices: %w", err)
			}

			id := strings.TrimSpace(*scheduleID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --schedule-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap price-schedules manual-prices: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPPriceSchedulePricesOption{
				asc.WithIAPPriceSchedulePricesLimit(*limit),
				asc.WithIAPPriceSchedulePricesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithIAPPriceSchedulePricesLimit(200))
				firstPage, err := client.GetInAppPurchasePriceScheduleManualPrices(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap price-schedules manual-prices: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchasePriceScheduleManualPrices(ctx, id, asc.WithIAPPriceSchedulePricesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap price-schedules manual-prices: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetInAppPurchasePriceScheduleManualPrices(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap price-schedules manual-prices: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// IAPPriceSchedulesAutomaticPricesCommand returns the price schedules automatic prices subcommand.
func IAPPriceSchedulesAutomaticPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-schedules automatic-prices", flag.ExitOnError)

	scheduleID := fs.String("schedule-id", "", "Price schedule ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "automatic-prices",
		ShortUsage: "asc iap price-schedules automatic-prices --schedule-id \"SCHEDULE_ID\"",
		ShortHelp:  "List automatic prices for an in-app purchase price schedule.",
		LongHelp: `List automatic prices for an in-app purchase price schedule.

Examples:
  asc iap price-schedules automatic-prices --schedule-id "SCHEDULE_ID"
  asc iap price-schedules automatic-prices --schedule-id "SCHEDULE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap price-schedules automatic-prices: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("iap price-schedules automatic-prices: %w", err)
			}

			id := strings.TrimSpace(*scheduleID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --schedule-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap price-schedules automatic-prices: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPPriceSchedulePricesOption{
				asc.WithIAPPriceSchedulePricesLimit(*limit),
				asc.WithIAPPriceSchedulePricesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithIAPPriceSchedulePricesLimit(200))
				firstPage, err := client.GetInAppPurchasePriceScheduleAutomaticPrices(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap price-schedules automatic-prices: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchasePriceScheduleAutomaticPrices(ctx, id, asc.WithIAPPriceSchedulePricesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap price-schedules automatic-prices: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetInAppPurchasePriceScheduleAutomaticPrices(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap price-schedules automatic-prices: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

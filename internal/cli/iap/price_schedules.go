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
  asc iap price-schedules create --iap-id "IAP_ID" --base-territory "USA" --prices "PRICE_POINT_ID:2024-03-01"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPPriceSchedulesGetCommand(),
			IAPPriceSchedulesCreateCommand(),
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
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc iap price-schedules get --iap-id \"IAP_ID\"",
		ShortHelp:  "Get in-app purchase price schedule.",
		LongHelp: `Get in-app purchase price schedule.

Examples:
  asc iap price-schedules get --iap-id "IAP_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap price-schedules get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

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

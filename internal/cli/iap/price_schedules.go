package iap

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
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPPriceSchedulesGetCommand(),
			IAPPriceSchedulesBaseTerritoryCommand(),
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
	include := fs.String("include", "", "Include relationships: baseTerritory,manualPrices,automaticPrices")
	scheduleFields := fs.String("schedule-fields", "", "fields[inAppPurchasePriceSchedules] (comma-separated)")
	territoryFields := fs.String("territory-fields", "", "fields[territories] (comma-separated)")
	priceFields := fs.String("price-fields", "", "fields[inAppPurchasePrices] (comma-separated)")
	manualPricesLimit := fs.Int("manual-prices-limit", 0, "limit[manualPrices] when included (1-50)")
	automaticPricesLimit := fs.Int("automatic-prices-limit", 0, "limit[automaticPrices] when included (1-50)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc iap price-schedules get --iap-id \"IAP_ID\"",
		ShortHelp:  "Get in-app purchase price schedule.",
		LongHelp: `Get in-app purchase price schedule.

Examples:
  asc iap price-schedules get --iap-id "IAP_ID"
  asc iap price-schedules get --schedule-id "SCHEDULE_ID"
  asc iap price-schedules get --iap-id "IAP_ID" --include "baseTerritory,manualPrices,automaticPrices" --price-fields "startDate,endDate,manual,inAppPurchasePricePoint,territory" --territory-fields "currency" --manual-prices-limit 50 --automatic-prices-limit 50`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
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
			if *manualPricesLimit != 0 && (*manualPricesLimit < 1 || *manualPricesLimit > 50) {
				fmt.Fprintln(os.Stderr, "Error: --manual-prices-limit must be between 1 and 50")
				return flag.ErrHelp
			}
			if *automaticPricesLimit != 0 && (*automaticPricesLimit < 1 || *automaticPricesLimit > 50) {
				fmt.Fprintln(os.Stderr, "Error: --automatic-prices-limit must be between 1 and 50")
				return flag.ErrHelp
			}

			includeValues, err := normalizeIAPPriceScheduleInclude(*include)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap price-schedules get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := make([]asc.IAPPriceScheduleOption, 0, 6)
			if len(includeValues) > 0 {
				opts = append(opts, asc.WithIAPPriceScheduleInclude(includeValues))
			}
			if values := shared.SplitCSV(*scheduleFields); len(values) > 0 {
				opts = append(opts, asc.WithIAPPriceScheduleFields(values))
			}
			if values := shared.SplitCSV(*territoryFields); len(values) > 0 {
				opts = append(opts, asc.WithIAPPriceScheduleTerritoryFields(values))
			}
			if values := shared.SplitCSV(*priceFields); len(values) > 0 {
				opts = append(opts, asc.WithIAPPriceSchedulePriceFields(values))
			}
			if *manualPricesLimit > 0 {
				opts = append(opts, asc.WithIAPPriceScheduleManualPricesLimit(*manualPricesLimit))
			}
			if *automaticPricesLimit > 0 {
				opts = append(opts, asc.WithIAPPriceScheduleAutomaticPricesLimit(*automaticPricesLimit))
			}

			if scheduleValue != "" {
				resp, err := client.GetInAppPurchasePriceScheduleByID(requestCtx, scheduleValue, opts...)
				if err != nil {
					return fmt.Errorf("iap price-schedules get: failed to fetch: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchasePriceSchedule(requestCtx, iapValue, opts...)
			if err != nil {
				return fmt.Errorf("iap price-schedules get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func normalizeIAPPriceScheduleInclude(value string) ([]string, error) {
	parts := shared.SplitCSV(value)
	if len(parts) == 0 {
		return nil, nil
	}

	allowed := map[string]struct{}{
		"baseTerritory":   {},
		"manualPrices":    {},
		"automaticPrices": {},
	}

	unique := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if _, ok := allowed[part]; !ok {
			return nil, fmt.Errorf("--include must be one of: baseTerritory, manualPrices, automaticPrices")
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		unique = append(unique, part)
	}
	return unique, nil
}

// IAPPriceSchedulesBaseTerritoryCommand returns the price schedules base territory subcommand.
func IAPPriceSchedulesBaseTerritoryCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-schedules base-territory", flag.ExitOnError)

	scheduleID := fs.String("schedule-id", "", "Price schedule ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "base-territory",
		ShortUsage: "asc iap price-schedules base-territory --schedule-id \"SCHEDULE_ID\"",
		ShortHelp:  "Get base territory for a price schedule.",
		LongHelp: `Get base territory for a price schedule.

Examples:
  asc iap price-schedules base-territory --schedule-id "SCHEDULE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*scheduleID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --schedule-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap price-schedules base-territory: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchasePriceScheduleBaseTerritory(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap price-schedules base-territory: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// IAPPriceSchedulesCreateCommand returns the price schedules create subcommand.
func IAPPriceSchedulesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-schedules create", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	appID := fs.String("app", "", "App ID (optional; retained for backward compatibility)")
	baseTerritory := fs.String("base-territory", "", "Base territory ID (e.g., USA)")
	prices := fs.String("prices", "", "Manual prices: PRICE_POINT_ID[:START_DATE[:END_DATE]] entries")
	tier := fs.Int("tier", 0, "Pricing tier number (use instead of --prices for single-price schedule)")
	price := fs.String("price", "", "Customer price (use instead of --prices for single-price schedule)")
	startDate := fs.String("start-date", "", "Start date when using --tier or --price (YYYY-MM-DD)")
	refresh := fs.Bool("refresh", false, "Force refresh of tier cache")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc iap price-schedules create --iap-id \"IAP_ID\" --base-territory \"USA\" --prices \"PRICE_POINT_ID:2024-03-01\"",
		ShortHelp:  "Create an in-app purchase price schedule.",
		LongHelp: `Create an in-app purchase price schedule.

Examples:
  asc iap price-schedules create --iap-id "IAP_ID" --base-territory "USA" --prices "PRICE_POINT_ID:2024-03-01"
  asc iap price-schedules create --iap-id "IAP_ID" --base-territory "USA" --tier 5 --start-date "2024-03-01"
  asc iap price-schedules create --iap-id "IAP_ID" --base-territory "USA" --price "4.99" --start-date "2024-03-01"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
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

			tierValue := *tier
			priceValue := strings.TrimSpace(*price)
			pricesValue := strings.TrimSpace(*prices)
			_ = strings.TrimSpace(*appID)

			if tierValue < 0 {
				fmt.Fprintln(os.Stderr, "Error: --tier must be a positive integer")
				return flag.ErrHelp
			}
			hasTierOrPrice := tierValue > 0 || priceValue != ""
			if hasTierOrPrice && pricesValue != "" {
				fmt.Fprintln(os.Stderr, "Error: --prices and --tier/--price are mutually exclusive")
				return flag.ErrHelp
			}
			if tierValue > 0 && priceValue != "" {
				fmt.Fprintln(os.Stderr, "Error: --tier and --price are mutually exclusive")
				return flag.ErrHelp
			}
			if err := shared.ValidateFinitePriceFlag("--price", priceValue); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return flag.ErrHelp
			}

			var priceEntries []asc.InAppPurchasePriceSchedulePrice

			if hasTierOrPrice {
				resolvedStartDate := strings.TrimSpace(*startDate)
				if resolvedStartDate != "" {
					normalizedStartDate, err := normalizeIAPDate(resolvedStartDate, "--start-date")
					if err != nil {
						fmt.Fprintln(os.Stderr, "Error:", err.Error())
						return flag.ErrHelp
					}
					resolvedStartDate = normalizedStartDate
				}

				client, err := shared.GetASCClient()
				if err != nil {
					return fmt.Errorf("iap price-schedules create: %w", err)
				}

				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()

				tiers, err := shared.ResolveIAPTiers(requestCtx, client, iapValue, baseTerritoryValue, *refresh)
				if err != nil {
					return fmt.Errorf("iap price-schedules create: resolve tiers: %w", err)
				}

				var resolvedID string
				if tierValue > 0 {
					resolvedID, err = shared.ResolvePricePointByTier(tiers, tierValue)
				} else {
					resolvedID, err = shared.ResolvePricePointByPrice(tiers, priceValue)
				}
				if err != nil {
					return fmt.Errorf("iap price-schedules create: %w", err)
				}

				priceEntries = []asc.InAppPurchasePriceSchedulePrice{
					{PricePointID: resolvedID, StartDate: resolvedStartDate},
				}
			} else {
				var err error
				priceEntries, err = parsePriceSchedulePrices(pricesValue)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				if len(priceEntries) == 0 {
					fmt.Fprintln(os.Stderr, "Error: --prices (or --tier/--price) is required")
					return flag.ErrHelp
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap price-schedules create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateInAppPurchasePriceSchedule(requestCtx, iapValue, asc.InAppPurchasePriceScheduleCreateAttributes{
				BaseTerritoryID: baseTerritoryValue,
				Prices:          priceEntries,
			})
			if err != nil {
				return fmt.Errorf("iap price-schedules create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
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
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "manual-prices",
		ShortUsage: "asc iap price-schedules manual-prices --schedule-id \"SCHEDULE_ID\"",
		ShortHelp:  "List manual prices for an in-app purchase price schedule.",
		LongHelp: `List manual prices for an in-app purchase price schedule.

Examples:
  asc iap price-schedules manual-prices --schedule-id "SCHEDULE_ID"
  asc iap price-schedules manual-prices --schedule-id "SCHEDULE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap price-schedules manual-prices: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("iap price-schedules manual-prices: %w", err)
			}

			id := strings.TrimSpace(*scheduleID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --schedule-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap price-schedules manual-prices: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchasePriceScheduleManualPrices(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap price-schedules manual-prices: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
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
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "automatic-prices",
		ShortUsage: "asc iap price-schedules automatic-prices --schedule-id \"SCHEDULE_ID\"",
		ShortHelp:  "List automatic prices for an in-app purchase price schedule.",
		LongHelp: `List automatic prices for an in-app purchase price schedule.

Examples:
  asc iap price-schedules automatic-prices --schedule-id "SCHEDULE_ID"
  asc iap price-schedules automatic-prices --schedule-id "SCHEDULE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap price-schedules automatic-prices: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("iap price-schedules automatic-prices: %w", err)
			}

			id := strings.TrimSpace(*scheduleID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --schedule-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap price-schedules automatic-prices: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
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

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchasePriceScheduleAutomaticPrices(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap price-schedules automatic-prices: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

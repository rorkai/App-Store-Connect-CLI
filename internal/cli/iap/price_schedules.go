package iap

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newIAPPriceSchedulesCommand returns the canonical pricing schedules command group.
func newIAPPriceSchedulesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("schedules", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "schedules",
		ShortUsage: "asc iap pricing schedules <subcommand> [flags]",
		ShortHelp:  "Manage in-app purchase price schedules.",
		LongHelp: `Manage in-app purchase price schedules.

Examples:
  asc iap pricing schedules view --iap-id "IAP_ID"
  asc iap pricing schedules create --iap-id "IAP_ID" --base-territory "USA" --prices "PRICE_POINT_ID:2024-03-01"
  asc iap pricing schedules manual-prices --schedule-id "SCHEDULE_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newIAPPriceSchedulesGetCommand(d),
			newIAPPriceSchedulesBaseTerritoryCommand(d),
			newIAPPriceSchedulesCreateCommand(d),
			newIAPPriceSchedulesManualPricesCommand(d),
			newIAPPriceSchedulesAutomaticPricesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newIAPPriceSchedulesGetCommand returns the price schedules get subcommand.
func newIAPPriceSchedulesGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("schedules view", flag.ExitOnError)

	iapID := shared.BindResourceIDFlag(fs, "iap-id", "inAppPurchases", "In-app purchase ID, product ID, or exact current name")
	appID := addIAPLookupAppFlag(fs)
	scheduleID := shared.BindResourceIDFlag(fs, "schedule-id", "inAppPurchasePriceSchedules", "Price schedule ID")
	include := fs.String("include", "", "Include relationships: baseTerritory,manualPrices,automaticPrices")
	scheduleFields := fs.String("schedule-fields", "", "fields[inAppPurchasePriceSchedules] (comma-separated)")
	territoryFields := fs.String("territory-fields", "", "fields[territories] (comma-separated)")
	priceFields := fs.String("price-fields", "", "fields[inAppPurchasePrices] (comma-separated)")
	manualPricesLimit := fs.Int("manual-prices-limit", 0, "limit[manualPrices] when included (1-50)")
	automaticPricesLimit := fs.Int("automatic-prices-limit", 0, "limit[automaticPrices] when included (1-50)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc iap pricing schedules view --iap-id \"IAP_ID\"",
		ShortHelp:  "View in-app purchase price schedule.",
		LongHelp: `View in-app purchase price schedule.

Examples:
  asc iap pricing schedules view --iap-id "IAP_ID"
  asc iap pricing schedules view --schedule-id "SCHEDULE_ID"
  asc iap pricing schedules view --iap-id "IAP_ID" --include "baseTerritory,manualPrices,automaticPrices" --price-fields "startDate,endDate,manual,inAppPurchasePricePoint,territory" --territory-fields "currency" --manual-prices-limit 50 --automatic-prices-limit 50`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			scheduleValue := strings.TrimSpace(*scheduleID)
			if iapValue == "" && scheduleValue == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --iap-id or --schedule-id is required")
				return shared.MissingRequiredUsageError("")
			}
			if iapValue != "" && scheduleValue != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --iap-id and --schedule-id are mutually exclusive")
				return flag.ErrHelp
			}
			if *manualPricesLimit != 0 && (*manualPricesLimit < 1 || *manualPricesLimit > 50) {
				fmt.Fprintln(d.IO.ErrOut, "Error: --manual-prices-limit must be between 1 and 50")
				return flag.ErrHelp
			}
			if *automaticPricesLimit != 0 && (*automaticPricesLimit < 1 || *automaticPricesLimit > 50) {
				fmt.Fprintln(d.IO.ErrOut, "Error: --automatic-prices-limit must be between 1 and 50")
				return flag.ErrHelp
			}

			includeValues, err := normalizeIAPPriceScheduleInclude(*include)
			if err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
				return flag.ErrHelp
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("iap pricing schedules view: %w", err)
			}

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
				requestCtx, cancel := d.Timeout(ctx)
				defer cancel()

				resp, err := client.GetInAppPurchasePriceScheduleByID(requestCtx, scheduleValue, opts...)
				if err != nil {
					return fmt.Errorf("iap pricing schedules view: failed to fetch: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			iapValue, err = resolveIAPLookupIDWithTimeout(ctx, d, client, *appID, iapValue)
			if err != nil {
				return err
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchasePriceSchedule(requestCtx, iapValue, opts...)
			if err != nil {
				return fmt.Errorf("iap pricing schedules view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
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

// newIAPPriceSchedulesBaseTerritoryCommand returns the price schedules base territory subcommand.
func newIAPPriceSchedulesBaseTerritoryCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("schedules base-territory", flag.ExitOnError)

	scheduleID := shared.BindResourceIDFlag(fs, "schedule-id", "inAppPurchasePriceSchedules", "Price schedule ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "base-territory",
		ShortUsage: "asc iap pricing schedules base-territory --schedule-id \"SCHEDULE_ID\"",
		ShortHelp:  "Get base territory for a price schedule.",
		LongHelp: `Get base territory for a price schedule.

Examples:
  asc iap pricing schedules base-territory --schedule-id "SCHEDULE_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*scheduleID)
			if id == "" {
				return d.RequiredFlag("--schedule-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("iap pricing schedules base-territory: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchasePriceScheduleBaseTerritory(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap pricing schedules base-territory: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newIAPPriceSchedulesCreateCommand returns the price schedules create subcommand.
func newIAPPriceSchedulesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("schedules create", flag.ExitOnError)

	iapID := shared.BindResourceIDFlag(fs, "iap-id", "inAppPurchases", "In-app purchase ID, product ID, or exact current name")
	appID := fs.String("app", "", iapLookupAppUsage)
	baseTerritory := fs.String("base-territory", "", "Base territory: alpha-2, alpha-3, or English country name (e.g., US, USA, United States)")
	prices := fs.String("prices", "", "Manual prices: PRICE_POINT_ID[:START_DATE[:END_DATE]] entries")
	tier := fs.Int("tier", 0, "Pricing tier number (use instead of --prices for single-price schedule)")
	price := fs.String("price", "", "Customer price (use instead of --prices for single-price schedule)")
	startDate := fs.String("start-date", "", "Start date when using --tier or --price (YYYY-MM-DD)")
	refresh := fs.Bool("refresh", false, "Force refresh of tier cache")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc iap pricing schedules create --iap-id \"IAP_ID\" --base-territory \"USA\" --prices \"PRICE_POINT_ID:2024-03-01\"",
		ShortHelp:  "Create an in-app purchase price schedule.",
		LongHelp: `Create an in-app purchase price schedule.

Examples:
  asc iap pricing schedules create --iap-id "IAP_ID" --base-territory "USA" --prices "PRICE_POINT_ID:2024-03-01"
  asc iap pricing schedules create --iap-id "IAP_ID" --base-territory "USA" --tier 5 --start-date "2024-03-01"
  asc iap pricing schedules create --iap-id "IAP_ID" --base-territory "USA" --price "4.99" --start-date "2024-03-01"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" {
				return d.RequiredFlag("--iap-id")
			}
			baseTerritoryValue := strings.TrimSpace(*baseTerritory)
			if baseTerritoryValue == "" {
				return d.RequiredFlag("--base-territory")
			}
			baseTerritoryValue, err := ascterritory.Normalize(baseTerritoryValue)
			if err != nil {
				return d.UsageError(err.Error())
			}

			tierValue := *tier
			priceValue := strings.TrimSpace(*price)
			pricesValue := strings.TrimSpace(*prices)

			if flagSet(fs, "tier") && tierValue <= 0 {
				fmt.Fprintln(d.IO.ErrOut, "Error: --tier must be a positive integer")
				return flag.ErrHelp
			}
			hasTierOrPrice := tierValue > 0 || priceValue != ""
			if hasTierOrPrice && pricesValue != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --prices and --tier/--price are mutually exclusive")
				return flag.ErrHelp
			}
			if tierValue > 0 && priceValue != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --tier and --price are mutually exclusive")
				return flag.ErrHelp
			}
			if err := shared.ValidateFinitePriceFlag("--price", priceValue); err != nil {
				fmt.Fprintln(d.IO.ErrOut, "Error:", err)
				return flag.ErrHelp
			}
			if !hasTierOrPrice {
				parsedPrices, err := parsePriceSchedulePrices(pricesValue)
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
					return flag.ErrHelp
				}
				if len(parsedPrices) == 0 {
					fmt.Fprintln(d.IO.ErrOut, "Error: --prices (or --tier/--price) is required")
					return shared.MissingRequiredUsageError("")
				}
			}

			resolvedStartDate := strings.TrimSpace(*startDate)
			if hasTierOrPrice && resolvedStartDate != "" {
				normalizedStartDate, err := normalizeIAPDate(resolvedStartDate, "--start-date")
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
					return flag.ErrHelp
				}
				resolvedStartDate = normalizedStartDate
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("iap pricing schedules create: %w", err)
			}

			iapValue, err = resolveIAPLookupIDWithTimeout(ctx, d, client, *appID, iapValue)
			if err != nil {
				return err
			}

			var priceEntries []asc.InAppPurchasePriceSchedulePrice

			if hasTierOrPrice {
				tierCtx, tierCancel := d.Timeout(ctx)
				tiers, err := d.ResolveIAPTiers(tierCtx, client, iapValue, baseTerritoryValue, *refresh)
				tierCancel()
				if err != nil {
					return fmt.Errorf("iap pricing schedules create: resolve tiers: %w", err)
				}

				var resolvedID string
				if tierValue > 0 {
					resolvedID, err = shared.ResolvePricePointByTier(tiers, tierValue)
				} else {
					resolvedID, err = shared.ResolvePricePointByPrice(tiers, priceValue)
				}
				if err != nil {
					return fmt.Errorf("iap pricing schedules create: %w", err)
				}

				priceEntries = []asc.InAppPurchasePriceSchedulePrice{
					{PricePointID: resolvedID, StartDate: resolvedStartDate},
				}
			} else {
				var err error
				priceEntries, err = parsePriceSchedulePrices(pricesValue)
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
					return flag.ErrHelp
				}
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateInAppPurchasePriceSchedule(requestCtx, iapValue, asc.InAppPurchasePriceScheduleCreateAttributes{
				BaseTerritoryID: baseTerritoryValue,
				Prices:          priceEntries,
			})
			if err != nil {
				return fmt.Errorf("iap pricing schedules create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newIAPPriceSchedulesManualPricesCommand returns the price schedules manual prices subcommand.
func newIAPPriceSchedulesManualPricesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("schedules manual-prices", flag.ExitOnError)

	scheduleID := shared.BindResourceIDFlag(fs, "schedule-id", "inAppPurchasePriceSchedules", "Price schedule ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	resolved := fs.Bool("resolved", false, "Return the current effective price per territory")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "manual-prices",
		ShortUsage: "asc iap pricing schedules manual-prices --schedule-id \"SCHEDULE_ID\"",
		ShortHelp:  "List manual prices for an in-app purchase price schedule.",
		LongHelp: `List manual prices for an in-app purchase price schedule.

Examples:
  asc iap pricing schedules manual-prices --schedule-id "SCHEDULE_ID"
  asc iap pricing schedules manual-prices --schedule-id "SCHEDULE_ID" --paginate
  asc iap pricing schedules manual-prices --schedule-id "SCHEDULE_ID" --resolved`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "iap pricing schedules manual-prices", 200); err != nil {
				return err
			}
			if *resolved && strings.TrimSpace(*next) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --resolved cannot be combined with --next")
				return flag.ErrHelp
			}

			id := strings.TrimSpace(*scheduleID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--schedule-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("iap pricing schedules manual-prices: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if *resolved {
				resp, err := fetchResolvedIAPSchedulePrices(requestCtx, client, id, "manual", *limit, *next, d.Now(), false)
				if err != nil {
					return fmt.Errorf("iap pricing schedules manual-prices: failed to resolve: %w", err)
				}
				return d.PrintResolvedPrices(resp, *output.Output, *output.Pretty)
			}

			opts := []asc.IAPPriceSchedulePricesOption{
				asc.Limit[asc.IAPPriceSchedulePricesOption](*limit),
				asc.NextURL[asc.IAPPriceSchedulePricesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.IAPPriceSchedulePricesOption](200))
				firstPage, err := client.GetInAppPurchasePriceScheduleManualPrices(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap pricing schedules manual-prices: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchasePriceScheduleManualPrices(ctx, id, asc.NextURL[asc.IAPPriceSchedulePricesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap pricing schedules manual-prices: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchasePriceScheduleManualPrices(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap pricing schedules manual-prices: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newIAPPriceSchedulesAutomaticPricesCommand returns the price schedules automatic prices subcommand.
func newIAPPriceSchedulesAutomaticPricesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("schedules automatic-prices", flag.ExitOnError)

	scheduleID := shared.BindResourceIDFlag(fs, "schedule-id", "inAppPurchasePriceSchedules", "Price schedule ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	resolved := fs.Bool("resolved", false, "Return the current effective price per territory")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "automatic-prices",
		ShortUsage: "asc iap pricing schedules automatic-prices --schedule-id \"SCHEDULE_ID\"",
		ShortHelp:  "List automatic prices for an in-app purchase price schedule.",
		LongHelp: `List automatic prices for an in-app purchase price schedule.

Examples:
  asc iap pricing schedules automatic-prices --schedule-id "SCHEDULE_ID"
  asc iap pricing schedules automatic-prices --schedule-id "SCHEDULE_ID" --paginate
  asc iap pricing schedules automatic-prices --schedule-id "SCHEDULE_ID" --resolved`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "iap pricing schedules automatic-prices", 200); err != nil {
				return err
			}
			if *resolved && strings.TrimSpace(*next) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --resolved cannot be combined with --next")
				return flag.ErrHelp
			}

			id := strings.TrimSpace(*scheduleID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--schedule-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("iap pricing schedules automatic-prices: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if *resolved {
				resp, err := fetchResolvedIAPSchedulePrices(requestCtx, client, id, "automatic", *limit, *next, d.Now(), false)
				if err != nil {
					return fmt.Errorf("iap pricing schedules automatic-prices: failed to resolve: %w", err)
				}
				return d.PrintResolvedPrices(resp, *output.Output, *output.Pretty)
			}

			opts := []asc.IAPPriceSchedulePricesOption{
				asc.Limit[asc.IAPPriceSchedulePricesOption](*limit),
				asc.NextURL[asc.IAPPriceSchedulePricesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.IAPPriceSchedulePricesOption](200))
				firstPage, err := client.GetInAppPurchasePriceScheduleAutomaticPrices(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap pricing schedules automatic-prices: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchasePriceScheduleAutomaticPrices(ctx, id, asc.NextURL[asc.IAPPriceSchedulePricesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap pricing schedules automatic-prices: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchasePriceScheduleAutomaticPrices(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("iap pricing schedules automatic-prices: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

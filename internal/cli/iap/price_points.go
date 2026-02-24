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

// IAPPricePointsCommand returns the price points command group.
func IAPPricePointsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-points", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "price-points",
		ShortUsage: "asc iap price-points <subcommand> [flags]",
		ShortHelp:  "List in-app purchase price points.",
		LongHelp: `List in-app purchase price points.

Examples:
  asc iap price-points list --iap-id "IAP_ID"
  asc iap price-points equalizations --id "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPPricePointsListCommand(),
			IAPPricePointsEqualizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPPricePointsListCommand returns the price points list subcommand.
func IAPPricePointsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-points list", flag.ExitOnError)

	iapID := fs.String("iap-id", "", "In-app purchase ID")
	territory := fs.String("territory", "", "Territory ID (e.g., USA)")
	price := fs.String("price", "", "Filter by exact customer price (e.g., 4.99)")
	minPrice := fs.String("min-price", "", "Filter by minimum customer price")
	maxPrice := fs.String("max-price", "", "Filter by maximum customer price")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc iap price-points list --iap-id \"IAP_ID\"",
		ShortHelp:  "List price points for an in-app purchase.",
		LongHelp: `List price points for an in-app purchase.

Use --price to find a specific customer price, or --min-price/--max-price for
a range. These filters are applied client-side after fetching.

Examples:
  asc iap price-points list --iap-id "IAP_ID"
  asc iap price-points list --iap-id "IAP_ID" --territory "USA"
  asc iap price-points list --iap-id "IAP_ID" --territory "USA" --paginate --price "4.99"
  asc iap price-points list --iap-id "IAP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap price-points list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("iap price-points list: %w", err)
			}

			priceFilter := shared.PriceFilter{
				Price:    strings.TrimSpace(*price),
				MinPrice: strings.TrimSpace(*minPrice),
				MaxPrice: strings.TrimSpace(*maxPrice),
			}
			if err := priceFilter.Validate(); err != nil {
				return shared.UsageError(err.Error())
			}

			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap price-points list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPPricePointsOption{
				asc.WithIAPPricePointsLimit(*limit),
				asc.WithIAPPricePointsNextURL(*next),
			}
			territoryID := strings.ToUpper(strings.TrimSpace(*territory))
			if territoryID != "" {
				opts = append(opts, asc.WithIAPPricePointsTerritory(territoryID))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithIAPPricePointsLimit(200))
				firstPage, err := client.GetInAppPurchasePricePoints(requestCtx, iapValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("iap price-points list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetInAppPurchasePricePoints(ctx, iapValue, asc.WithIAPPricePointsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("iap price-points list: %w", err)
				}

				if priceFilter.HasFilter() {
					if typed, ok := resp.(*asc.InAppPurchasePricePointsResponse); ok {
						filterIAPPricePoints(typed, priceFilter)
						return shared.PrintOutput(typed, *output.Output, *output.Pretty)
					}
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetInAppPurchasePricePoints(requestCtx, iapValue, opts...)
			if err != nil {
				return fmt.Errorf("iap price-points list: failed to fetch: %w", err)
			}

			if priceFilter.HasFilter() {
				filterIAPPricePoints(resp, priceFilter)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// filterIAPPricePoints removes data entries that don't match the price filter.
func filterIAPPricePoints(resp *asc.InAppPurchasePricePointsResponse, pf shared.PriceFilter) {
	filtered := resp.Data[:0]
	for _, item := range resp.Data {
		if pf.MatchesPrice(item.Attributes.CustomerPrice) {
			filtered = append(filtered, item)
		}
	}
	resp.Data = filtered
}

// IAPPricePointsEqualizationsCommand returns the price point equalizations subcommand.
func IAPPricePointsEqualizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-points equalizations", flag.ExitOnError)

	pricePointID := fs.String("id", "", "In-app purchase price point ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "equalizations",
		ShortUsage: "asc iap price-points equalizations --id \"PRICE_POINT_ID\"",
		ShortHelp:  "List equalized price points for an in-app purchase price point.",
		LongHelp: `List equalized price points for an in-app purchase price point.

Examples:
  asc iap price-points equalizations --id "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*pricePointID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("iap price-points equalizations: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchasePricePointEqualizations(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap price-points equalizations: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

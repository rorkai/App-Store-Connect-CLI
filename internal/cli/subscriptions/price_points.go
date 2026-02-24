package subscriptions

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

// SubscriptionsPricePointsCommand returns the subscription price points command group.
func SubscriptionsPricePointsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-points", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "price-points",
		ShortUsage: "asc subscriptions price-points <subcommand> [flags]",
		ShortHelp:  "Manage subscription price points.",
		LongHelp: `Manage subscription price points.

Examples:
  asc subscriptions price-points list --subscription-id "SUB_ID"
  asc subscriptions price-points get --id "PRICE_POINT_ID"
  asc subscriptions price-points equalizations --id "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsPricePointsListCommand(),
			SubscriptionsPricePointsGetCommand(),
			SubscriptionsPricePointsEqualizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsPricePointsListCommand returns the price points list subcommand.
func SubscriptionsPricePointsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-points list", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	territory := fs.String("territory", "", "Filter by territory (e.g., USA) to reduce results")
	price := fs.String("price", "", "Filter by exact customer price (e.g., 4.99)")
	minPrice := fs.String("min-price", "", "Filter by minimum customer price")
	maxPrice := fs.String("max-price", "", "Filter by maximum customer price")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	stream := fs.Bool("stream", false, "Stream pages as NDJSON (one JSON object per page, requires --paginate)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions price-points list [flags]",
		ShortHelp:  "List price points for a subscription.",
		LongHelp: `List price points for a subscription.

Use --territory to filter by a specific territory. Without it, all territories
are returned (140K+ results for subscriptions). Filtering by territory reduces
results to ~800 and completes in seconds instead of 20+ minutes.

Use --price to find a specific customer price, or --min-price/--max-price for
a range. These filters are applied client-side after fetching. Combine with
--territory and --paginate for best results.

Use --stream with --paginate to emit each page as a separate JSON line (NDJSON)
instead of buffering all pages in memory. This gives immediate feedback and
reduces memory usage for very large result sets.

Examples:
  asc subscriptions price-points list --subscription-id "SUB_ID"
  asc subscriptions price-points list --subscription-id "SUB_ID" --territory "USA"
  asc subscriptions price-points list --subscription-id "SUB_ID" --territory "USA" --paginate
  asc subscriptions price-points list --subscription-id "SUB_ID" --territory "USA" --paginate --price "4.99"
  asc subscriptions price-points list --subscription-id "SUB_ID" --territory "USA" --paginate --min-price "1.00" --max-price "9.99"
  asc subscriptions price-points list --subscription-id "SUB_ID" --paginate --stream`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions price-points list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions price-points list: %w", err)
			}
			if *stream && !*paginate {
				fmt.Fprintln(os.Stderr, "Error: --stream requires --paginate")
				return flag.ErrHelp
			}

			priceFilter := shared.PriceFilter{
				Price:    strings.TrimSpace(*price),
				MinPrice: strings.TrimSpace(*minPrice),
				MaxPrice: strings.TrimSpace(*maxPrice),
			}
			if err := priceFilter.Validate(); err != nil {
				return shared.UsageError(err.Error())
			}
			if priceFilter.HasFilter() && *stream {
				fmt.Fprintln(os.Stderr, "Error: price filtering is not supported with --stream")
				return flag.ErrHelp
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions price-points list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionPricePointsOption{
				asc.WithSubscriptionPricePointsTerritory(*territory),
				asc.WithSubscriptionPricePointsLimit(*limit),
				asc.WithSubscriptionPricePointsNextURL(*next),
			}

			if *paginate && *stream {
				// Streaming mode: emit each page as a separate JSON line
				paginateOpts := append(opts, asc.WithSubscriptionPricePointsLimit(200))
				firstPageCtx, firstPageCancel := shared.ContextWithTimeout(ctx)
				page, err := client.GetSubscriptionPricePoints(firstPageCtx, id, paginateOpts...)
				firstPageCancel()
				if err != nil {
					return fmt.Errorf("subscriptions price-points list: failed to fetch: %w", err)
				}

				seenNext := make(map[string]struct{})
				for {
					if err := shared.PrintStreamPage(page); err != nil {
						return fmt.Errorf("subscriptions price-points list: write stream page: %w", err)
					}

					if page.Links.Next == "" {
						break
					}
					if _, exists := seenNext[page.Links.Next]; exists {
						return fmt.Errorf("subscriptions price-points list: %w", asc.ErrRepeatedPaginationURL)
					}
					seenNext[page.Links.Next] = struct{}{}

					pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
					page, err = client.GetSubscriptionPricePoints(pageCtx, id, asc.WithSubscriptionPricePointsNextURL(page.Links.Next))
					pageCancel()
					if err != nil {
						return fmt.Errorf("subscriptions price-points list: %w", err)
					}
				}
				return nil
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionPricePointsLimit(200))
				firstPageCtx, firstPageCancel := shared.ContextWithTimeout(ctx)
				firstPage, err := client.GetSubscriptionPricePoints(firstPageCtx, id, paginateOpts...)
				firstPageCancel()
				if err != nil {
					return fmt.Errorf("subscriptions price-points list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(ctx, firstPage, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
					pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
					defer pageCancel()
					return client.GetSubscriptionPricePoints(pageCtx, id, asc.WithSubscriptionPricePointsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions price-points list: %w", err)
				}

				if priceFilter.HasFilter() {
					if typed, ok := resp.(*asc.SubscriptionPricePointsResponse); ok {
						filterSubscriptionPricePoints(typed, priceFilter)
						return shared.PrintOutput(typed, *output.Output, *output.Pretty)
					}
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetSubscriptionPricePoints(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions price-points list: failed to fetch: %w", err)
			}

			if priceFilter.HasFilter() {
				filterSubscriptionPricePoints(resp, priceFilter)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// filterSubscriptionPricePoints removes data entries that don't match the price filter.
func filterSubscriptionPricePoints(resp *asc.SubscriptionPricePointsResponse, pf shared.PriceFilter) {
	filtered := resp.Data[:0]
	for _, item := range resp.Data {
		if pf.MatchesPrice(item.Attributes.CustomerPrice) {
			filtered = append(filtered, item)
		}
	}
	resp.Data = filtered
}

// SubscriptionsPricePointsGetCommand returns the price points get subcommand.
func SubscriptionsPricePointsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-points get", flag.ExitOnError)

	pricePointID := fs.String("id", "", "Subscription price point ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions price-points get --id \"PRICE_POINT_ID\"",
		ShortHelp:  "Get a subscription price point by ID.",
		LongHelp: `Get a subscription price point by ID.

Examples:
  asc subscriptions price-points get --id "PRICE_POINT_ID"`,
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
				return fmt.Errorf("subscriptions price-points get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionPricePoint(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions price-points get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// SubscriptionsPricePointsEqualizationsCommand returns the price point equalizations subcommand.
func SubscriptionsPricePointsEqualizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-points equalizations", flag.ExitOnError)

	pricePointID := fs.String("id", "", "Subscription price point ID")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "equalizations",
		ShortUsage: "asc subscriptions price-points equalizations --id \"PRICE_POINT_ID\"",
		ShortHelp:  "List equalized price points for a subscription price point.",
		LongHelp: `List equalized price points for a subscription price point.

Examples:
  asc subscriptions price-points equalizations --id "PRICE_POINT_ID"
  asc subscriptions price-points equalizations --id "PRICE_POINT_ID" --paginate`,
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
				return fmt.Errorf("subscriptions price-points equalizations: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// When paginating, request with limit=200 to minimize API calls
			var resp *asc.SubscriptionPricePointsResponse
			if *paginate {
				resp, err = client.GetSubscriptionPricePointEqualizations(requestCtx, id, asc.WithSubscriptionPricePointsLimit(200))
			} else {
				resp, err = client.GetSubscriptionPricePointEqualizations(requestCtx, id)
			}
			if err != nil {
				return fmt.Errorf("subscriptions price-points equalizations: %w", err)
			}

			if *paginate {
				allPages, pErr := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
					pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
					defer pageCancel()
					return client.GetSubscriptionPricePointEqualizations(pageCtx, id, asc.WithSubscriptionPricePointsNextURL(nextURL))
				})
				if pErr != nil {
					return fmt.Errorf("subscriptions price-points equalizations: %w", pErr)
				}
				if typed, ok := allPages.(*asc.SubscriptionPricePointsResponse); ok {
					resp = typed
				}
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

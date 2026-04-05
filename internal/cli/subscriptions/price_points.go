package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/ascterritory"
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
  asc subscriptions price-points get --price-point-id "PRICE_POINT_ID"
  asc subscriptions price-points equalizations --price-point-id "PRICE_POINT_ID"`,
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

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	territory := fs.String("territory", "", "Filter by territory (accepts alpha-2, alpha-3, or exact English country name) to reduce results")
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
  asc subscriptions price-points list --subscription-id "SUB_ID" --territory "United States"
  asc subscriptions price-points list --subscription-id "SUB_ID" --territory "US" --paginate
  asc subscriptions price-points list --subscription-id "SUB_ID" --territory "France" --paginate --price "4.99"
  asc subscriptions price-points list --subscription-id "SUB_ID" --territory "DE" --paginate --min-price "1.00" --max-price "9.99"
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
				return shared.UsageError("--stream requires --paginate")
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
				return shared.UsageError("price filtering is not supported with --stream")
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.UsageError("--subscription-id is required")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions price-points list: %w", err)
			}

			if strings.TrimSpace(*next) == "" {
				id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
				if err != nil {
					return err
				}
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			territoryFilter := strings.TrimSpace(*territory)
			if territoryFilter != "" {
				territoryFilter, err = ascterritory.Normalize(territoryFilter)
				if err != nil {
					return shared.UsageError(err.Error())
				}
			}

			opts := []asc.SubscriptionPricePointsOption{
				asc.WithSubscriptionPricePointsTerritory(territoryFilter),
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
				if err := asc.PaginateEach(
					ctx,
					page,
					func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
						pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
						defer pageCancel()
						return client.GetSubscriptionPricePoints(pageCtx, id, asc.WithSubscriptionPricePointsNextURL(nextURL))
					},
					func(page asc.PaginatedResponse) error {
						typed, ok := page.(*asc.SubscriptionPricePointsResponse)
						if !ok {
							return fmt.Errorf("unexpected pagination response type %T", page)
						}
						return shared.PrintStreamPage(typed)
					},
				); err != nil {
					return fmt.Errorf("subscriptions price-points list: %w", err)
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

	pricePointID := fs.String("price-point-id", "", "Subscription price point ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions price-points get --price-point-id \"PRICE_POINT_ID\"",
		ShortHelp:  "Get a subscription price point by ID.",
		LongHelp: `Get a subscription price point by ID.

Examples:
  asc subscriptions price-points get --price-point-id "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*pricePointID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --price-point-id is required")
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
	return shared.BuildPricePointEqualizationsCommand(shared.PricePointEqualizationsCommandConfig{
		FlagSetName: "price-points equalizations",
		Name:        "equalizations",
		ShortUsage:  `asc subscriptions price-points equalizations --price-point-id "PRICE_POINT_ID"`,
		BaseExample: `asc subscriptions price-points equalizations --price-point-id "PRICE_POINT_ID"`,
		Subject:     "a subscription price point",
		ParentFlag:  "price-point-id",
		ParentUsage: "Subscription price point ID",
		LimitMax:    200,
		ErrorPrefix: "subscriptions price-points equalizations",
		FetchPage: func(ctx context.Context, client *asc.Client, pricePointID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.SubscriptionPricePointsOption{
				asc.WithSubscriptionPricePointsLimit(limit),
				asc.WithSubscriptionPricePointsNextURL(next),
			}
			return client.GetSubscriptionPricePointEqualizations(ctx, pricePointID, opts...)
		},
	})
}

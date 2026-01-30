package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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
		UsageFunc: DefaultUsageFunc,
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
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc subscriptions price-points list [flags]",
		ShortHelp:  "List price points for a subscription.",
		LongHelp: `List price points for a subscription.

Examples:
  asc subscriptions price-points list --subscription-id "SUB_ID"
  asc subscriptions price-points list --subscription-id "SUB_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("subscriptions price-points list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("subscriptions price-points list: %w", err)
			}

			id := strings.TrimSpace(*subscriptionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions price-points list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.SubscriptionPricePointsOption{
				asc.WithSubscriptionPricePointsLimit(*limit),
				asc.WithSubscriptionPricePointsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithSubscriptionPricePointsLimit(200))
				firstPage, err := client.GetSubscriptionPricePoints(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("subscriptions price-points list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetSubscriptionPricePoints(ctx, id, asc.WithSubscriptionPricePointsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("subscriptions price-points list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetSubscriptionPricePoints(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("subscriptions price-points list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsPricePointsGetCommand returns the price points get subcommand.
func SubscriptionsPricePointsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-points get", flag.ExitOnError)

	pricePointID := fs.String("id", "", "Subscription price point ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions price-points get --id \"PRICE_POINT_ID\"",
		ShortHelp:  "Get a subscription price point by ID.",
		LongHelp: `Get a subscription price point by ID.

Examples:
  asc subscriptions price-points get --id "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*pricePointID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions price-points get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionPricePoint(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions price-points get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// SubscriptionsPricePointsEqualizationsCommand returns the price point equalizations subcommand.
func SubscriptionsPricePointsEqualizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-points equalizations", flag.ExitOnError)

	pricePointID := fs.String("id", "", "Subscription price point ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "equalizations",
		ShortUsage: "asc subscriptions price-points equalizations --id \"PRICE_POINT_ID\"",
		ShortHelp:  "List equalized price points for a subscription price point.",
		LongHelp: `List equalized price points for a subscription price point.

Examples:
  asc subscriptions price-points equalizations --id "PRICE_POINT_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*pricePointID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions price-points equalizations: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionPricePointEqualizations(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions price-points equalizations: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

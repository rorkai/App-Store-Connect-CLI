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

// IAPPricePointsCommand returns the price points command group.
func IAPPricePointsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("price-points", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "price-points",
		ShortUsage: "asc iap price-points <subcommand> [flags]",
		ShortHelp:  "List in-app purchase price points.",
		LongHelp: `List in-app purchase price points.

Examples:
  asc iap price-points list --iap-id "IAP_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPPricePointsListCommand(),
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
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc iap price-points list --iap-id \"IAP_ID\"",
		ShortHelp:  "List price points for an in-app purchase.",
		LongHelp: `List price points for an in-app purchase.

Examples:
  asc iap price-points list --iap-id "IAP_ID"
  asc iap price-points list --iap-id "IAP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("iap price-points list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("iap price-points list: %w", err)
			}

			iapValue := strings.TrimSpace(*iapID)
			if iapValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --iap-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap price-points list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.IAPPricePointsOption{
				asc.WithIAPPricePointsLimit(*limit),
				asc.WithIAPPricePointsNextURL(*next),
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

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetInAppPurchasePricePoints(requestCtx, iapValue, opts...)
			if err != nil {
				return fmt.Errorf("iap price-points list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
